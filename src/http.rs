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

pub fn router(state: AppState) -> Router {
    let mut data_state = state.clone();
    data_state.pool = crate::workspace::data_pool(&state.pool);
    let data = Router::new()
        .merge(crate::projects::router())
        .merge(crate::companies::router())
        .merge(crate::company_notes::router())
        .merge(crate::company_career::router())
        .merge(crate::notes::router())
        .merge(crate::people::router())
        .merge(crate::person_fields::router())
        .merge(crate::person_misc::router())
        .merge(crate::person_professions::router())
        .merge(crate::person_bonds::router())
        .merge(crate::track::router())
        .merge(crate::earn::router())
        .merge(crate::events::router())
        .merge(crate::journal::router())
        .merge(crate::load::router())
        .merge(crate::sources::router())
        .merge(crate::backup::router())
        .merge(crate::schedule::router())
        .merge(crate::schedule_settings::router())
        .merge(crate::items::router())
        .merge(crate::item_details::router())
        .merge(crate::item_sync::router())
        .with_state(data_state);
    let protected = Router::new()
        .route("/api/logout", post(logout))
        .route("/api/me", get(crate::workspace::me))
        .merge(crate::accounts::protected_router())
        .with_state(state.clone())
        .merge(data)
        .route_layer(middleware::from_fn_with_state(state.clone(), require_auth));
    Router::new()
        .route("/api/health", get(health))
        .merge(crate::accounts::public_router())
        .with_state(state.clone())
        .merge(protected)
        .layer(middleware::from_fn_with_state(state, cors))
}

async fn cors(State(state): State<AppState>, request: Request, next: Next) -> Response {
    if !matches!(
        *request.method(),
        Method::GET | Method::HEAD | Method::OPTIONS
    ) {
        let origin = request.headers().get(header::ORIGIN);
        let public = state.auth.public_origin();
        if origin.is_some_and(|v| {
            v.to_str()
                .ok()
                .is_none_or(|o| o != state.cors_origin && o != public)
        }) || (origin.is_none()
            && request
                .headers()
                .get("sec-fetch-site")
                .is_some_and(|v| v == "cross-site"))
        {
            return error(StatusCode::FORBIDDEN, "invalid origin");
        }
    }
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
            HeaderValue::from_static("Content-Type, X-Workspace-Id"),
        );
        headers.insert(
            header::ACCESS_CONTROL_ALLOW_METHODS,
            HeaderValue::from_static("GET,POST,PUT,PATCH,DELETE,OPTIONS"),
        );
    }
    response
}

async fn require_auth(State(state): State<AppState>, mut request: Request, next: Next) -> Response {
    let token = session_token(&request).unwrap_or_default();
    let session = match state.auth.authenticate_session(token).await {
        Ok(Some(session)) => session,
        Ok(None) => return error(StatusCode::UNAUTHORIZED, "unauthorized"),
        Err(err) => {
            eprintln!("session lookup failed: {err}");
            return error(StatusCode::INTERNAL_SERVER_ERROR, "internal error");
        }
    };
    let requested = match request.headers().get("x-workspace-id") {
        None => None,
        Some(raw) => match raw.to_str().ok().and_then(|s| Uuid::parse_str(s).ok()) {
            Some(id) => Some(id),
            None => return error(StatusCode::BAD_REQUEST, "invalid workspace"),
        },
    };
    let scope = match crate::workspace::access(state.auth.pool(), session.user_id, requested).await
    {
        Ok(scope) => scope,
        Err(err) => return not_found(err),
    };
    let path = request.uri().path();
    let changes = !matches!(
        *request.method(),
        Method::GET | Method::HEAD | Method::OPTIONS
    );
    let account = path == "/api/logout"
        || path == "/api/workspaces"
        || path.starts_with("/api/workspaces/")
        || path.starts_with("/api/account/")
        || path.starts_with("/api/invitations/");
    let route = request
        .extensions()
        .get::<axum::extract::MatchedPath>()
        .map(|path| path.as_str())
        .unwrap_or("");
    if changes && !account && !scope.can_edit_shared() && !personal_write(route, request.method()) {
        return error(
            StatusCode::FORBIDDEN,
            "shared editing requires a paid workspace",
        );
    }
    if scope.role != "owner"
        && (matches!(path, "/api/backup" | "/api/restore")
            || (changes
                && (path == "/api/sources"
                    || path.starts_with("/api/sources/")
                    || path == "/api/items/sync-active"
                    || (path.starts_with("/api/items/") && path.ends_with("/sync")))))
    {
        return error(StatusCode::FORBIDDEN, "workspace owner required");
    }
    request.extensions_mut().insert(session);
    crate::workspace::in_scope(scope, next.run(request)).await
}

fn personal_write(route: &str, method: &Method) -> bool {
    match route {
        // These handlers reject shared fields and write only the actor's private row.
        "/api/people/{id}" | "/api/items/{id}" | "/api/companies/{id}" | "/api/projects/{id}" => {
            *method == Method::PATCH
        }
        "/api/notes"
        | "/api/notes/{id}"
        | "/api/stress"
        | "/api/checkins"
        | "/api/intervals"
        | "/api/intervals/{id}/stop"
        | "/api/items/{id}/events/annotate"
        | "/api/schedule/whatif"
        | "/api/schedule/settings"
        | "/api/schedule/days/{date}"
        | "/api/earn/days/{day}" => true,
        "/api/companies/{id}/contracts"
        | "/api/companies/{id}/manager"
        | "/api/companies/{id}/notes"
        | "/api/companies/{id}/notes/{noteId}"
        | "/api/companies/{id}/reports"
        | "/api/companies/{id}/reports/{reportId}"
        | "/api/companies/{id}/salaries"
        | "/api/companies/{id}/titles"
        | "/api/people/{id}/absences"
        | "/api/people/{id}/absences/{absenceId}"
        | "/api/people/{id}/addresses"
        | "/api/people/{id}/addresses/{addressId}"
        | "/api/people/{id}/bonds"
        | "/api/people/{id}/bonds/{bondId}"
        | "/api/people/{id}/bonds/{bondId}/end"
        | "/api/people/{id}/contacts"
        | "/api/people/{id}/contacts/{contactId}"
        | "/api/people/{id}/notes"
        | "/api/people/{id}/notes/{noteId}"
        | "/api/people/{id}/professions"
        | "/api/people/{id}/professions/{professionId}"
        | "/api/people/{id}/sites"
        | "/api/people/{id}/sites/{siteId}" => true,
        _ => false,
    }
}

#[allow(clippy::result_large_err)]
pub(crate) fn private_patch(body: &[u8], allowed: &[&str]) -> Result<(), Response> {
    let fields: serde_json::Map<String, serde_json::Value> =
        serde_json::from_slice(body).map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))?;
    if fields.is_empty() || fields.keys().any(|key| !allowed.contains(&key.as_str())) {
        return Err(error(
            StatusCode::FORBIDDEN,
            "shared editing requires a paid workspace",
        ));
    }
    Ok(())
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
        match database.code().as_deref() {
            Some("23503") => return error(StatusCode::NOT_FOUND, "not found"),
            Some("42501") => return error(StatusCode::FORBIDDEN, "forbidden"),
            Some("WB002") => return error(StatusCode::CONFLICT, "external source limit reached"),
            Some("WB003") => {
                return error(StatusCode::CONFLICT, "change would affect private records")
            }
            _ => {}
        }
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

async fn logout(State(state): State<AppState>, request: Request) -> Response {
    if let Some(token) = session_token(&request) {
        if state.auth.logout(token).await.is_err() {
            return error(StatusCode::INTERNAL_SERVER_ERROR, "could not end session");
        }
    }
    let mut response = Json(OkBody { ok: true }).into_response();
    response.headers_mut().insert(
        header::SET_COOKIE,
        HeaderValue::from_str(&state.auth.cookie("", 0))
            .expect("session cookie uses fixed attributes"),
    );
    response
}
