use crate::mail::{app_url, Mailer};
use anyhow::{Context, Result};
use bcrypt::{hash_with_result, verify, Version};
use rand::{rngs::OsRng, RngCore};
use reqwest::Url;
use sha2::{Digest, Sha256};
use sqlx::{PgPool, Postgres, Transaction};
use std::{env, sync::Arc};
use uuid::Uuid;

#[derive(Clone)]
pub struct Auth {
    pool: PgPool,
    public_url: Url,
    mail: Option<Arc<Mailer>>,
}

#[derive(Clone, Copy)]
pub struct Session {
    pub id: Uuid,
    pub user_id: Uuid,
}

impl Auth {
    pub fn new(pool: PgPool) -> Self {
        Self {
            pool,
            public_url: Url::parse("http://127.0.0.1").expect("valid loopback origin"),
            mail: None,
        }
    }

    pub fn configured(pool: PgPool, origin: &str, mail: Option<Mailer>) -> Result<Self> {
        Ok(Self {
            pool,
            public_url: app_url(origin)?,
            mail: mail.map(Arc::new),
        })
    }

    pub fn from_env(pool: PgPool, cors_origin: &str) -> Result<Self> {
        let address = env::var("PUBLIC_APP_URL").unwrap_or_else(|_| cors_origin.to_owned());
        Self::configured(pool, &address, Mailer::from_env()?)
    }

    pub fn pool(&self) -> &PgPool {
        &self.pool
    }

    pub fn public_origin(&self) -> String {
        self.public_url.origin().ascii_serialization()
    }

    pub fn cookie(&self, token: &str, max_age_seconds: i64) -> String {
        let secure = if self.public_url.scheme() == "https" {
            "; Secure"
        } else {
            ""
        };
        format!(
            "wb_session={token}; Path=/; Max-Age={max_age_seconds}; HttpOnly; SameSite=Lax{secure}"
        )
    }

    pub(crate) fn email_available(&self) -> bool {
        self.mail.is_some()
    }

    pub(crate) async fn send_link(&self, email: &str, action: &str, token: &str) -> Result<()> {
        self.mail
            .as_ref()
            .context("SMTP is not configured")?
            .send_link(&self.public_url, email, action, token)
            .await
    }

    // Used by the single-owner compatibility fixtures; server startup never creates an account.
    #[doc(hidden)]
    pub async fn ensure_user(&self, password: &str) -> Result<()> {
        let mut tx = self.pool.begin().await?;
        sqlx::query("LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE")
            .execute(&mut *tx)
            .await?;
        let has_legacy: bool =
            sqlx::query_scalar("SELECT EXISTS (SELECT FROM users WHERE is_legacy)")
                .fetch_one(&mut *tx)
                .await?;
        if !has_legacy {
            let other_users: bool = sqlx::query_scalar("SELECT EXISTS (SELECT FROM users)")
                .fetch_one(&mut *tx)
                .await?;
            anyhow::ensure!(
                !other_users,
                "cannot bootstrap an installation with accounts"
            );
            let hash = password_hash(password.to_owned(), Version::TwoA).await?;
            let user_id = sqlx::query_scalar(
                "INSERT INTO users (password_hash,is_legacy) VALUES ($1,true) RETURNING id",
            )
            .bind(hash)
            .fetch_one(&mut *tx)
            .await?;
            create_workspace(&mut tx, user_id, "personal", "Personal", true).await?;
        }
        tx.commit().await?;
        Ok(())
    }

    pub async fn login(&self, password: &str) -> Result<Option<String>> {
        self.login_identity(None, password).await
    }

    pub(crate) async fn login_identity(
        &self,
        email: Option<&str>,
        password: &str,
    ) -> Result<Option<String>> {
        if password.is_empty() || password.len() > 1024 {
            return Ok(None);
        }
        let identity: Option<(Uuid, String, bool)> = sqlx::query_as(
            "SELECT id, password_hash, is_legacy FROM users
             WHERE email=$1 OR ($1 IS NULL AND email IS NULL AND is_legacy)",
        )
        .bind(email)
        .fetch_optional(&self.pool)
        .await?;
        // An unknown address still performs the same bcrypt work.
        let hash = identity
            .as_ref()
            .map(|(_, hash, _)| hash.as_str())
            .unwrap_or("$2a$10$DCq7YPn5Rq63x1Lad4cll.TV4S6ytwfsfvkgY8jIucDrjc8deX1s.");
        if !password_matches(password.to_owned(), hash.to_owned()).await? {
            return Ok(None);
        }
        let Some((user_id, hash, is_legacy)) = identity else {
            return Ok(None);
        };
        // Imported installation hashes used 2a and accepted bcrypt's 72-byte truncation.
        // Every newly selected password uses 2b and must match the full input.
        if password.len() > 72 && !(is_legacy && hash.starts_with("$2a$")) {
            return Ok(None);
        }
        let mut tx = self.pool.begin().await?;
        // A reset or legacy binding may have finished during bcrypt verification.
        let still_valid: Option<Uuid> = sqlx::query_scalar(
            "SELECT id FROM users WHERE id=$1 AND password_hash=$2
             AND email IS NOT DISTINCT FROM $3 FOR UPDATE",
        )
        .bind(user_id)
        .bind(hash)
        .bind(email)
        .fetch_optional(&mut *tx)
        .await?;
        if still_valid.is_none() {
            return Ok(None);
        }
        let token = new_token();
        sqlx::query("DELETE FROM sessions WHERE user_id=$1 AND expires_at <= now()")
            .bind(user_id)
            .execute(&mut *tx)
            .await?;
        sqlx::query(
            "INSERT INTO sessions (user_id, token_hash, expires_at)
             VALUES ($1, $2, now() + interval '30 days')",
        )
        .bind(user_id)
        .bind(hash_token(&token))
        .execute(&mut *tx)
        .await?;
        tx.commit().await?;
        Ok(Some(token))
    }

    pub async fn authenticate_session(&self, token: &str) -> Result<Option<Session>> {
        if token.is_empty() || token.len() > 256 {
            return Ok(None);
        }
        let row: Option<(Uuid, Uuid)> = sqlx::query_as(
            "SELECT id,user_id FROM sessions WHERE token_hash=$1 AND expires_at>clock_timestamp()",
        )
        .bind(hash_token(token))
        .fetch_optional(&self.pool)
        .await?;
        Ok(row.map(|(id, user_id)| Session { id, user_id }))
    }

    pub async fn authenticate(&self, token: &str) -> Result<bool> {
        Ok(self.authenticate_session(token).await?.is_some())
    }

    pub async fn logout(&self, token: &str) -> Result<()> {
        sqlx::query("DELETE FROM sessions WHERE token_hash=$1")
            .bind(hash_token(token))
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    pub(crate) async fn allow_attempt(
        &self,
        action: &str,
        subject: &str,
        limit: i32,
    ) -> Result<bool, sqlx::Error> {
        sqlx::query("DELETE FROM auth_limits WHERE expires_at<=clock_timestamp()")
            .execute(&self.pool)
            .await?;
        let result = sqlx::query(
            "INSERT INTO auth_limits (key, attempts, expires_at)
             VALUES ($1, 1, clock_timestamp()+interval '15 minutes')
             ON CONFLICT (key) DO UPDATE SET attempts=auth_limits.attempts+1
             WHERE auth_limits.attempts<$2",
        )
        .bind(format!("{action}:{}", hash_token(subject)))
        .bind(limit)
        .execute(&self.pool)
        .await?;
        Ok(result.rows_affected() == 1)
    }
}

pub(crate) async fn create_workspace(
    tx: &mut Transaction<'_, Postgres>,
    user_id: Uuid,
    kind: &str,
    name: &str,
    legacy_paid: bool,
) -> Result<Uuid, sqlx::Error> {
    let workspace_id: Uuid = sqlx::query_scalar(
        "INSERT INTO workspaces (owner_id, kind, name, plan, legacy_paid)
         VALUES ($1, $2, $3, $4, $5) RETURNING id",
    )
    .bind(user_id)
    .bind(kind)
    .bind(name)
    .bind(if legacy_paid { "paid" } else { "free" })
    .bind(legacy_paid)
    .fetch_one(&mut **tx)
    .await?;
    sqlx::query("INSERT INTO workspace_members (workspace_id,user_id,role) VALUES ($1,$2,'owner')")
        .bind(workspace_id)
        .bind(user_id)
        .execute(&mut **tx)
        .await?;
    crate::workspace::seed(tx, workspace_id, user_id).await?;
    Ok(workspace_id)
}

pub(crate) fn new_token() -> String {
    let mut raw = [0_u8; 32];
    OsRng.fill_bytes(&mut raw);
    hex::encode(raw)
}

pub(crate) fn hash_token(token: &str) -> String {
    hex::encode(Sha256::digest(token.as_bytes()))
}

pub(crate) async fn password_hash(value: String, version: Version) -> Result<String> {
    tokio::task::spawn_blocking(move || {
        hash_with_result(value, 10).map(|hash| hash.format_for_version(version))
    })
    .await?
    .context("hash password")
}

pub(crate) async fn password_matches(value: String, hash: String) -> Result<bool> {
    tokio::task::spawn_blocking(move || verify(value, &hash))
        .await?
        .context("verify password")
}

#[cfg(test)]
mod tests {
    use super::hash_token;
    #[test]
    fn session_hash_matches_sha256_hex() {
        assert_eq!(
            hash_token("abc"),
            "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
        );
    }
}
