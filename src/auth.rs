use anyhow::{Context, Result};
use bcrypt::{hash_with_result, verify, Version};
use chrono::{Duration, Utc};
use rand::rngs::OsRng;
use rand::RngCore;
use sha2::{Digest, Sha256};
use sqlx::PgPool;
use uuid::Uuid;

#[derive(Clone)]
pub struct Auth {
    pool: PgPool,
}

impl Auth {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }

    pub async fn ensure_user(&self, password: &str) -> Result<()> {
        let count: i64 = sqlx::query_scalar("SELECT count(*) FROM users")
            .fetch_one(&self.pool)
            .await?;
        if count == 0 {
            let password_hash = hash_with_result(password, 10)
                .context("hash password")?
                .format_for_version(Version::TwoA);
            sqlx::query("INSERT INTO users (id, password_hash, created_at) VALUES ($1, $2, $3)")
                .bind(Uuid::new_v4())
                .bind(password_hash)
                .bind(Utc::now())
                .execute(&self.pool)
                .await?;
        }
        sqlx::query("DELETE FROM sessions WHERE expires_at <= $1")
            .bind(Utc::now())
            .execute(&self.pool)
            .await?;
        Ok(())
    }

    pub async fn login(&self, password: &str) -> Result<Option<String>> {
        let user: Option<(Uuid, String)> =
            sqlx::query_as("SELECT id, password_hash FROM users ORDER BY created_at LIMIT 1")
                .fetch_optional(&self.pool)
                .await?;
        let Some((user_id, password_hash)) = user else {
            return Ok(None);
        };
        if !verify(password, &password_hash).context("verify password")? {
            return Ok(None);
        }
        let mut raw = [0_u8; 32];
        OsRng.fill_bytes(&mut raw);
        let token = hex::encode(raw);
        let token_hash = hash_token(&token);
        let now = Utc::now();
        sqlx::query(
            "INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at) \
             VALUES ($1, $2, $3, $4, $5)",
        )
        .bind(Uuid::new_v4())
        .bind(user_id)
        .bind(token_hash)
        .bind(now + Duration::days(30))
        .bind(now)
        .execute(&self.pool)
        .await?;
        Ok(Some(token))
    }

    pub async fn authenticate(&self, token: &str) -> Result<bool> {
        if token.is_empty() {
            return Ok(false);
        }
        let valid: bool = sqlx::query_scalar(
            "SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash = $1 AND expires_at > $2)",
        )
        .bind(hash_token(token))
        .bind(Utc::now())
        .fetch_one(&self.pool)
        .await?;
        Ok(valid)
    }

    pub async fn logout(&self, token: &str) -> Result<()> {
        sqlx::query("DELETE FROM sessions WHERE token_hash = $1")
            .bind(hash_token(token))
            .execute(&self.pool)
            .await?;
        Ok(())
    }
}

fn hash_token(token: &str) -> String {
    hex::encode(Sha256::digest(token.as_bytes()))
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
