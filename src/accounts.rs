use crate::auth::{
    create_workspace, hash_token, new_token, password_hash, password_matches, Auth, Session,
};
use crate::http::{error, AppState};
use axum::extract::{ConnectInfo, Path, State};
use axum::http::{header, HeaderValue, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::routing::{delete, get, post};
use axum::{Extension, Json, Router};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sqlx::{Postgres, Transaction};
use std::net::SocketAddr;
use uuid::Uuid;

pub fn public_router() -> Router<AppState> {
    Router::new()
        .route("/api/login", post(login))
        .route("/api/auth/register", post(register))
        .route("/api/auth/confirm", post(confirm))
        .route("/api/auth/forgot", post(forgot))
        .route("/api/auth/reset", post(reset))
}

pub fn protected_router() -> Router<AppState> {
    Router::new()
        .route("/api/account/email", post(bind_email))
        .route("/api/account/sessions", get(sessions))
        .route("/api/account/sessions/{id}", delete(revoke_session))
        .route("/api/workspaces", post(create_team))
        .route("/api/workspaces/{id}/members", get(members))
        .route(
            "/api/workspaces/{id}/invitations",
            get(invitations).post(invite),
        )
        .route(
            "/api/workspaces/{id}/invitations/{invite}",
            delete(revoke_invitation),
        )
        .route("/api/invitations/accept", post(accept))
}

#[derive(Debug)]
struct AccountError(StatusCode, &'static str);

impl IntoResponse for AccountError {
    fn into_response(self) -> Response {
        error(self.0, self.1)
    }
}

impl From<sqlx::Error> for AccountError {
    fn from(err: sqlx::Error) -> Self {
        match err {
            sqlx::Error::Database(db) if db.is_unique_violation() => {
                Self(StatusCode::CONFLICT, "conflict")
            }
            _ => Self(StatusCode::INTERNAL_SERVER_ERROR, "internal error"),
        }
    }
}

type Outcome<T> = Result<T, AccountError>;
type Peer = Option<Extension<ConnectInfo<SocketAddr>>>;
const INVALID_LINK: AccountError = AccountError(StatusCode::BAD_REQUEST, "invalid or expired link");
const NOT_FOUND: AccountError = AccountError(StatusCode::NOT_FOUND, "not found");
const NO_MAIL: AccountError = AccountError(
    StatusCode::SERVICE_UNAVAILABLE,
    "email is unavailable; try again later",
);
const UNAUTHORIZED: AccountError =
    AccountError(StatusCode::UNAUTHORIZED, "invalid sign-in details");

fn ok() -> Json<Value> {
    Json(json!({"ok":true}))
}

#[derive(Deserialize)]
struct Login {
    email: Option<String>,
    password: String,
}
#[derive(Deserialize)]
struct Email {
    email: String,
}
#[derive(Deserialize)]
struct Bind {
    email: String,
    password: String,
}
#[derive(Deserialize)]
struct Link {
    token: String,
}
#[derive(Deserialize)]
struct Confirmation {
    token: String,
    password: Option<String>,
}
#[derive(Deserialize)]
struct Reset {
    token: String,
    password: String,
}
#[derive(Deserialize)]
struct NewTeam {
    name: String,
}

fn address(raw: &str) -> Outcome<String> {
    let email = raw.trim().to_ascii_lowercase();
    if !email.is_ascii()
        || email.len() > 254
        || email.contains(char::is_control)
        || email.parse::<lettre::Address>().is_err()
    {
        return Err(AccountError(
            StatusCode::BAD_REQUEST,
            "invalid email address",
        ));
    }
    Ok(email)
}

fn link_hash(raw: &str) -> Outcome<String> {
    if raw.len() != 64 || !raw.bytes().all(|c| c.is_ascii_hexdigit()) {
        return Err(INVALID_LINK);
    }
    Ok(hash_token(raw))
}

async fn new_password(value: String) -> Outcome<String> {
    if value.len() > 72 || value.contains('\0') || value.chars().count() < 12 {
        return Err(AccountError(
            StatusCode::BAD_REQUEST,
            "password must have at least 12 characters and at most 72 bytes",
        ));
    }
    password_hash(value, bcrypt::Version::TwoB)
        .await
        .map_err(|_| AccountError(StatusCode::INTERNAL_SERVER_ERROR, "password unavailable"))
}

async fn limit(auth: &Auth, action: &str, subject: &str, max: i32) -> Outcome<()> {
    if !auth.allow_attempt(action, subject, max).await? {
        return Err(AccountError(
            StatusCode::TOO_MANY_REQUESTS,
            "too many requests; try again later",
        ));
    }
    Ok(())
}

async fn limit_peer(auth: &Auth, peer: Peer) -> Outcome<()> {
    let address = peer
        .map(|Extension(ConnectInfo(addr))| addr.ip().to_string())
        .unwrap_or_else(|| "local".into());
    limit(auth, "peer", &address, 120).await
}

async fn login(
    State(state): State<AppState>,
    peer: Peer,
    Json(body): Json<Login>,
) -> Outcome<Response> {
    limit_peer(&state.auth, peer).await?;
    let email = body.email.as_deref().map(address).transpose()?;
    let auth = &state.auth;
    limit(auth, "login", email.as_deref().unwrap_or("legacy"), 20).await?;
    let token = auth
        .login_identity(email.as_deref(), &body.password)
        .await
        .map_err(|_| AccountError(StatusCode::INTERNAL_SERVER_ERROR, "internal error"))?
        .ok_or(UNAUTHORIZED)?;
    let mut response = ok().into_response();
    let cookie =
        HeaderValue::from_str(&auth.cookie(&token, 30 * 24 * 60 * 60)).expect("hex session token");
    response.headers_mut().insert(header::SET_COOKIE, cookie);
    Ok(response)
}

async fn issue_link(
    auth: &Auth,
    purpose: &str,
    email: &str,
    identity: Option<(Uuid, i32)>,
) -> Outcome<()> {
    if !auth.email_available() {
        return Err(NO_MAIL);
    }
    let token = new_token();
    let (id, version) = identity.unzip();
    let expiry = if purpose == "reset" { 30 } else { 24 * 60 };
    let hash = hash_token(&token);
    sqlx::query("DELETE FROM account_tokens WHERE expires_at<=clock_timestamp()")
        .execute(auth.pool())
        .await?;
    sqlx::query(
        "INSERT INTO account_tokens (token_hash, purpose, email, user_id, credential_version, expires_at)
         VALUES ($1, $2, $3, $4, $5, now()+make_interval(mins=>$6))",
    )
        .bind(&hash)
        .bind(purpose)
        .bind(email)
        .bind(id)
        .bind(version)
        .bind(expiry)
        .execute(auth.pool())
        .await?;
    let action = if purpose == "signup" {
        "verify"
    } else {
        purpose
    };
    if auth.send_link(email, action, &token).await.is_err() {
        sqlx::query("DELETE FROM account_tokens WHERE token_hash=$1")
            .bind(&hash)
            .execute(auth.pool())
            .await?;
        return Err(NO_MAIL);
    }
    Ok(())
}

async fn register(
    State(state): State<AppState>,
    peer: Peer,
    Json(body): Json<Email>,
) -> Outcome<Json<Value>> {
    limit_peer(&state.auth, peer).await?;
    let email = address(&body.email)?;
    limit(&state.auth, "register", &email, 5).await?;
    issue_link(&state.auth, "signup", &email, None).await?;
    Ok(ok())
}

async fn bind_email(
    State(state): State<AppState>,
    Extension(session): Extension<Session>,
    Json(body): Json<Bind>,
) -> Outcome<Json<Value>> {
    let auth = &state.auth;
    limit(auth, "binding", &session.user_id.to_string(), 8).await?;
    let email = address(&body.email)?;
    let identity: Option<(String, i32)> = sqlx::query_as(
        "SELECT password_hash, credential_version FROM users
         WHERE id=$1 AND is_legacy AND email IS NULL",
    )
    .bind(session.user_id)
    .fetch_optional(auth.pool())
    .await?;
    let (hash, version) = identity.ok_or(AccountError(
        StatusCode::CONFLICT,
        "account already has an email",
    ))?;
    if !password_matches(body.password, hash)
        .await
        .map_err(|_| UNAUTHORIZED)?
    {
        return Err(UNAUTHORIZED);
    }
    issue_link(auth, "bind", &email, Some((session.user_id, version))).await?;
    Ok(ok())
}

async fn forgot(
    State(state): State<AppState>,
    peer: Peer,
    Json(body): Json<Email>,
) -> Outcome<Json<Value>> {
    let auth = &state.auth;
    limit_peer(auth, peer).await?;
    let email = address(&body.email)?;
    limit(auth, "forgot", &email, 5).await?;
    if !auth.email_available() {
        return Err(NO_MAIL);
    }
    let identity: Option<(Uuid, i32)> =
        sqlx::query_as("SELECT id,credential_version FROM users WHERE email=$1")
            .bind(&email)
            .fetch_optional(auth.pool())
            .await?;
    if let Some(identity) = identity {
        issue_link(auth, "reset", &email, Some(identity)).await?;
    } else {
        // The HTTP result and mail transport do not disclose which addresses are registered.
        auth.send_link(&email, "absent", "")
            .await
            .map_err(|_| NO_MAIL)?;
    }
    Ok(ok())
}

async fn confirm(
    State(state): State<AppState>,
    peer: Peer,
    Json(body): Json<Confirmation>,
) -> Outcome<Json<Value>> {
    let hash = link_hash(&body.token)?;
    let auth = &state.auth;
    limit_peer(auth, peer).await?;
    let purpose: Option<String> = sqlx::query_scalar(
        "SELECT purpose FROM account_tokens
         WHERE token_hash=$1 AND purpose IN ('signup','bind') AND expires_at>clock_timestamp()",
    )
    .bind(&hash)
    .fetch_optional(auth.pool())
    .await?;
    let purpose = purpose.ok_or(INVALID_LINK)?;
    let password = if purpose == "signup" {
        Some(new_password(body.password.unwrap_or_default()).await?)
    } else {
        None
    };
    let mut tx = auth.pool().begin().await?;
    let identity: Option<(String, Option<Uuid>, Option<i32>)> = sqlx::query_as(
        "DELETE FROM account_tokens WHERE token_hash=$1 AND purpose=$2
         AND expires_at>clock_timestamp() RETURNING email, user_id, credential_version",
    )
    .bind(&hash)
    .bind(&purpose)
    .fetch_optional(&mut *tx)
    .await?;
    let (email, user, version) = identity.ok_or(INVALID_LINK)?;
    if let Some(password) = password {
        let user: Option<Uuid> = sqlx::query_scalar(
            "INSERT INTO users (email, password_hash) VALUES ($1, $2)
             ON CONFLICT (email) DO NOTHING RETURNING id",
        )
        .bind(&email)
        .bind(password)
        .fetch_optional(&mut *tx)
        .await?;
        let user = user.ok_or(AccountError(
            StatusCode::CONFLICT,
            "email already registered; sign in or reset your password",
        ))?;
        create_workspace(&mut tx, user, "personal", "Personal", false).await?;
    } else {
        let user = user.ok_or(INVALID_LINK)?;
        let updated = sqlx::query(
            "UPDATE users SET email=$1, credential_version=credential_version+1
             WHERE id=$2 AND credential_version=$3 AND email IS NULL AND is_legacy",
        )
        .bind(&email)
        .bind(user)
        .bind(version)
        .execute(&mut *tx)
        .await?;
        if updated.rows_affected() != 1 {
            return Err(INVALID_LINK);
        }
        sqlx::query("DELETE FROM sessions WHERE user_id=$1")
            .bind(user)
            .execute(&mut *tx)
            .await?;
    }
    tx.commit().await?;
    Ok(ok())
}

async fn reset(
    State(state): State<AppState>,
    peer: Peer,
    Json(body): Json<Reset>,
) -> Outcome<Json<Value>> {
    let hash = link_hash(&body.token)?;
    let auth = &state.auth;
    limit_peer(auth, peer).await?;
    let active: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT FROM account_tokens
         WHERE token_hash=$1 AND purpose='reset' AND expires_at>clock_timestamp())",
    )
    .bind(&hash)
    .fetch_one(auth.pool())
    .await?;
    if !active {
        return Err(INVALID_LINK);
    }
    let password = new_password(body.password).await?;
    let mut tx = auth.pool().begin().await?;
    let row: Option<(Uuid, i32)> = sqlx::query_as(
        "DELETE FROM account_tokens WHERE token_hash=$1 AND purpose='reset'
         AND expires_at>clock_timestamp() RETURNING user_id, credential_version",
    )
    .bind(&hash)
    .fetch_optional(&mut *tx)
    .await?;
    let (user, version) = row.ok_or(INVALID_LINK)?;
    let updated = sqlx::query(
        "UPDATE users SET password_hash=$1, credential_version=credential_version+1
         WHERE id=$2 AND credential_version=$3",
    )
    .bind(password)
    .bind(user)
    .bind(version)
    .execute(&mut *tx)
    .await?;
    if updated.rows_affected() != 1 {
        return Err(INVALID_LINK);
    }
    sqlx::query("DELETE FROM sessions WHERE user_id=$1")
        .bind(user)
        .execute(&mut *tx)
        .await?;
    tx.commit().await?;
    Ok(ok())
}

#[derive(Serialize, sqlx::FromRow)]
#[serde(rename_all = "camelCase")]
struct SessionInfo {
    id: Uuid,
    created_at: DateTime<Utc>,
    expires_at: DateTime<Utc>,
    current: bool,
}

async fn sessions(
    State(state): State<AppState>,
    Extension(session): Extension<Session>,
) -> Outcome<Json<Vec<SessionInfo>>> {
    let rows = sqlx::query_as(
        "SELECT id, created_at, expires_at, id=$2 AS current FROM sessions
         WHERE user_id=$1 AND expires_at>clock_timestamp() ORDER BY created_at DESC, id",
    )
    .bind(session.user_id)
    .bind(session.id)
    .fetch_all(state.auth.pool())
    .await?;
    Ok(Json(rows))
}

async fn revoke_session(
    State(state): State<AppState>,
    Extension(session): Extension<Session>,
    Path(id): Path<Uuid>,
) -> Outcome<StatusCode> {
    let result = sqlx::query("DELETE FROM sessions WHERE user_id=$1 AND id=$2")
        .bind(session.user_id)
        .bind(id)
        .execute(state.auth.pool())
        .await?;
    if result.rows_affected() == 0 {
        return Err(NOT_FOUND);
    }
    Ok(StatusCode::NO_CONTENT)
}

async fn create_team(
    State(state): State<AppState>,
    Extension(session): Extension<Session>,
    Json(body): Json<NewTeam>,
) -> Outcome<(StatusCode, Json<Value>)> {
    let name = body.name.trim();
    if name.is_empty() || name.chars().count() > 100 || name.contains(char::is_control) {
        return Err(AccountError(
            StatusCode::BAD_REQUEST,
            "workspace name must have 1 to 100 characters",
        ));
    }
    let mut tx = state.auth.pool().begin().await?;
    let id = create_workspace(&mut tx, session.user_id, "team", name, false).await?;
    tx.commit().await?;
    Ok((StatusCode::CREATED, Json(json!({"id":id}))))
}

#[derive(Serialize, sqlx::FromRow)]
#[serde(rename_all = "camelCase")]
struct Member {
    user_id: Uuid,
    label: String,
    role: String,
}

async fn members(
    State(state): State<AppState>,
    Extension(session): Extension<Session>,
    Path(id): Path<Uuid>,
) -> Outcome<Json<Vec<Member>>> {
    let rows = sqlx::query_as(
        "SELECT m.user_id, coalesce(split_part(u.email,'@',1),'Owner') AS label, m.role
         FROM workspace_members m JOIN users u ON u.id=m.user_id
         WHERE m.workspace_id=$1 AND EXISTS (
             SELECT FROM workspace_members WHERE workspace_id=$1 AND user_id=$2
         )
         ORDER BY m.role='owner' DESC, u.email, m.user_id",
    )
    .bind(id)
    .bind(session.user_id)
    .fetch_all(state.auth.pool())
    .await?;
    if rows.is_empty() {
        return Err(NOT_FOUND);
    }
    Ok(Json(rows))
}

async fn owned_team(tx: &mut Transaction<'_, Postgres>, id: Uuid, user: Uuid) -> Outcome<String> {
    let plan: Option<String> = sqlx::query_scalar(
        "SELECT plan FROM workspaces WHERE id=$1 AND owner_id=$2 AND kind='team' FOR UPDATE",
    )
    .bind(id)
    .bind(user)
    .fetch_optional(&mut **tx)
    .await?;
    plan.ok_or(NOT_FOUND)
}

#[derive(Serialize, sqlx::FromRow)]
#[serde(rename_all = "camelCase")]
struct Invitation {
    id: Uuid,
    email: String,
    expires_at: DateTime<Utc>,
}

async fn invitations(
    State(state): State<AppState>,
    Extension(session): Extension<Session>,
    Path(id): Path<Uuid>,
) -> Outcome<Json<Vec<Invitation>>> {
    let mut tx = state.auth.pool().begin().await?;
    owned_team(&mut tx, id, session.user_id).await?;
    let rows = sqlx::query_as(
        "SELECT id, email, expires_at FROM workspace_invitations
         WHERE workspace_id=$1 AND expires_at>now() ORDER BY email",
    )
    .bind(id)
    .fetch_all(&mut *tx)
    .await?;
    tx.commit().await?;
    Ok(Json(rows))
}

async fn invite(
    State(state): State<AppState>,
    Extension(session): Extension<Session>,
    Path(id): Path<Uuid>,
    Json(body): Json<Email>,
) -> Outcome<Json<Value>> {
    let email = address(&body.email)?;
    let auth = &state.auth;
    limit(auth, "invite", &session.user_id.to_string(), 30).await?;
    let mut tx = auth.pool().begin().await?;
    if owned_team(&mut tx, id, session.user_id).await? != "paid" {
        return Err(AccountError(
            StatusCode::FORBIDDEN,
            "invitations require a paid workspace",
        ));
    }
    if !auth.email_available() {
        return Err(NO_MAIL);
    }
    let already_member: bool = sqlx::query_scalar(
        "SELECT EXISTS (SELECT FROM workspace_members m JOIN users u ON u.id=m.user_id
         WHERE m.workspace_id=$1 AND u.email=$2)",
    )
    .bind(id)
    .bind(&email)
    .fetch_one(&mut *tx)
    .await?;
    if already_member {
        return Err(AccountError(StatusCode::CONFLICT, "already a member"));
    }
    let token = new_token();
    let hash = hash_token(&token);
    let invitation: Option<Uuid> = sqlx::query_scalar(
        "INSERT INTO workspace_invitations (workspace_id, email, token_hash, expires_at)
         VALUES ($1, $2, $3, clock_timestamp()+interval '7 days')
         ON CONFLICT (workspace_id, email) DO UPDATE SET id=gen_random_uuid(),
         token_hash=EXCLUDED.token_hash, expires_at=EXCLUDED.expires_at
         WHERE workspace_invitations.expires_at<=clock_timestamp() RETURNING id",
    )
    .bind(id)
    .bind(&email)
    .bind(&hash)
    .fetch_optional(&mut *tx)
    .await?;
    let invitation = invitation.ok_or(AccountError(
        StatusCode::CONFLICT,
        "an invitation is already pending",
    ))?;
    tx.commit().await?;
    if auth.send_link(&email, "invite", &token).await.is_err() {
        sqlx::query("DELETE FROM workspace_invitations WHERE id=$1 AND token_hash=$2")
            .bind(invitation)
            .bind(&hash)
            .execute(auth.pool())
            .await?;
        return Err(NO_MAIL);
    }
    Ok(ok())
}

async fn revoke_invitation(
    State(state): State<AppState>,
    Extension(session): Extension<Session>,
    Path((id, invitation)): Path<(Uuid, Uuid)>,
) -> Outcome<StatusCode> {
    let mut tx = state.auth.pool().begin().await?;
    owned_team(&mut tx, id, session.user_id).await?;
    let result = sqlx::query("DELETE FROM workspace_invitations WHERE id=$1 AND workspace_id=$2")
        .bind(invitation)
        .bind(id)
        .execute(&mut *tx)
        .await?;
    if result.rows_affected() == 0 {
        return Err(NOT_FOUND);
    }
    tx.commit().await?;
    Ok(StatusCode::NO_CONTENT)
}

async fn accept(
    State(state): State<AppState>,
    Extension(session): Extension<Session>,
    Json(body): Json<Link>,
) -> Outcome<Json<Value>> {
    let hash = link_hash(&body.token)?;
    let mut tx = state.auth.pool().begin().await?;
    let team: Option<(Uuid, String)> = sqlx::query_as(
        "SELECT w.id, w.plan FROM workspace_invitations i
         JOIN workspaces w ON w.id=i.workspace_id JOIN users u ON u.email=i.email
         WHERE u.id=$1 AND i.token_hash=$2 AND i.expires_at>clock_timestamp() FOR UPDATE OF w",
    )
    .bind(session.user_id)
    .bind(&hash)
    .fetch_optional(&mut *tx)
    .await?;
    let (id, plan) = team.ok_or(INVALID_LINK)?;
    if plan != "paid" {
        return Err(AccountError(
            StatusCode::FORBIDDEN,
            "invitations require a paid workspace",
        ));
    }
    let claimed = sqlx::query(
        "DELETE FROM workspace_invitations WHERE workspace_id=$1
         AND token_hash=$2 AND expires_at>clock_timestamp()",
    )
    .bind(id)
    .bind(hash)
    .execute(&mut *tx)
    .await?;
    if claimed.rows_affected() != 1 {
        return Err(INVALID_LINK);
    }
    sqlx::query(
        "INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'editor')
         ON CONFLICT DO NOTHING",
    )
    .bind(id)
    .bind(session.user_id)
    .execute(&mut *tx)
    .await?;
    tx.commit().await?;
    Ok(Json(json!({"workspaceId":id})))
}
