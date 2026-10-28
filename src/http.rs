use crate::auth::Auth;
use crate::crypto::TokenBox;
use axum::body::Bytes;
use axum::extract::{Request, State};
use axum::http::{header, HeaderValue, Method, StatusCode};
use axum::middleware::{self, Next};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Json, Router};
use serde::{Deserialize, Serialize};
use sqlx::PgPool;
use std::sync::Arc;
use uuid::Uuid;

const SESSION_COOKIE: &str = "wb_session";

#[derive(Clone)]
pub struct AppState {
    pub pool: PgPool,
    pub auth: Auth,
    pub cors_origin: String,
    pub token_box: Arc<TokenBox>,
}

#[derive(Serialize)]
struct ErrorBody {
    error: String,
}

#[derive(Serialize)]
struct OkBody {
    ok: bool,
}

#[derive(Deserialize)]
struct LoginInput {
    #[serde(default)]
    password: String,
}

pub fn router(state: AppState) -> Router {
    let protected = Router::new()
        .route("/api/logout", post(logout))
        .route("/api/me", get(me))
        .merge(crate::projects::router())
        .merge(crate::notes::router())
        .merge(crate::track::router())
        .merge(crate::earn::router())
        .merge(crate::events::router())
        .merge(crate::journal::router())
        .merge(crate::load::router())
        .merge(crate::sources::router())
        .merge(crate::schedule_settings::router())
        .merge(crate::items::router())
        .merge(crate::item_details::router())
        .merge(crate::item_sync::router())
        .route_layer(middleware::from_fn_with_state(state.clone(), require_auth));
    Router::new()
        .route("/api/health", get(health))
        .route("/api/login", post(login))
        .merge(protected)
        .layer(middleware::from_fn_with_state(state.clone(), cors))
        .with_state(state)
}

async fn cors(State(state): State<AppState>, request: Request, next: Next) -> Response {
    let mut response = if request.method() == Method::OPTIONS {
        StatusCode::NO_CONTENT.into_response()
    } else {
        next.run(request).await
    };
    if !state.cors_origin.is_empty() {
        let headers = response.headers_mut();
        if let Ok(origin) = HeaderValue::from_str(&state.cors_origin) {
            headers.insert(header::ACCESS_CONTROL_ALLOW_ORIGIN, origin);
        }
        headers.insert(
            header::ACCESS_CONTROL_ALLOW_CREDENTIALS,
            HeaderValue::from_static("true"),
        );
        headers.insert(
            header::ACCESS_CONTROL_ALLOW_HEADERS,
            HeaderValue::from_static("Content-Type"),
        );
        headers.insert(
            header::ACCESS_CONTROL_ALLOW_METHODS,
            HeaderValue::from_static("GET,POST,PUT,PATCH,DELETE,OPTIONS"),
        );
    }
    response
}

async fn require_auth(State(state): State<AppState>, request: Request, next: Next) -> Response {
    let token = session_token(&request).unwrap_or_default();
    match state.auth.authenticate(token).await {
        Ok(true) => next.run(request).await,
        Ok(false) => error(StatusCode::UNAUTHORIZED, "unauthorized"),
        Err(_) => error(StatusCode::INTERNAL_SERVER_ERROR, "internal error"),
    }
}

fn session_token(request: &Request) -> Option<&str> {
    request
        .headers()
        .get_all(header::COOKIE)
        .iter()
        .filter_map(|header| header.to_str().ok())
        .flat_map(|header| header.split(';'))
        .filter_map(|part| part.trim().split_once('='))
        .find(|(name, _)| *name == SESSION_COOKIE)
        .map(|(_, value)| value)
}

pub(crate) fn error(status: StatusCode, message: impl Into<String>) -> Response {
    (
        status,
        Json(ErrorBody {
            error: message.into(),
        }),
    )
        .into_response()
}

#[allow(clippy::result_large_err)]
pub(crate) fn decode<T: for<'de> Deserialize<'de>>(body: &Bytes) -> Result<T, Response> {
    let mut decoder = serde_json::Deserializer::from_slice(body);
    T::deserialize(&mut decoder).map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))
}

#[allow(clippy::result_large_err)]
pub(crate) fn id(raw: &str) -> Result<Uuid, Response> {
    Uuid::parse_str(raw).map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))
}

pub(crate) fn database_error(err: sqlx::Error) -> Response {
    if let sqlx::Error::Database(database) = &err {
        if database.code().as_deref() == Some("23505") {
            return error(StatusCode::CONFLICT, "conflict");
        }
    }
    error(StatusCode::INTERNAL_SERVER_ERROR, "internal error")
}

pub(crate) fn not_found(err: sqlx::Error) -> Response {
    if matches!(err, sqlx::Error::RowNotFound) {
        error(StatusCode::NOT_FOUND, "not found")
    } else {
        database_error(err)
    }
}

async fn health() -> impl IntoResponse {
    Json(serde_json::json!({"status": "ok"}))
}

async fn login(State(state): State<AppState>, body: Bytes) -> Response {
    let mut decoder = serde_json::Deserializer::from_slice(&body);
    let Ok(input) = LoginInput::deserialize(&mut decoder) else {
        return error(StatusCode::BAD_REQUEST, "invalid");
    };
    match state.auth.login(&input.password).await {
        Ok(Some(token)) => {
            let mut response = Json(OkBody { ok: true }).into_response();
            let cookie = format!(
                "{SESSION_COOKIE}={token}; Path=/; Max-Age=2592000; HttpOnly; SameSite=Lax"
            );
            if let Ok(value) = HeaderValue::from_str(&cookie) {
                response.headers_mut().insert(header::SET_COOKIE, value);
            }
            response
        }
        Ok(None) => error(StatusCode::UNAUTHORIZED, "unauthorized"),
        Err(_) => error(StatusCode::INTERNAL_SERVER_ERROR, "internal error"),
    }
}

async fn logout(State(state): State<AppState>, request: Request) -> Response {
    if let Some(token) = session_token(&request) {
        let _ = state.auth.logout(token).await;
    }
    let mut response = Json(OkBody { ok: true }).into_response();
    response.headers_mut().insert(
        header::SET_COOKIE,
        HeaderValue::from_static("wb_session=; Path=/; Max-Age=0"),
    );
    response
}

async fn me() -> impl IntoResponse {
    Json(OkBody { ok: true })
}
