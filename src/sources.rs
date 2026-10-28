use crate::http::{database_error, decode, error, id, not_found, AppState};
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sqlx::FromRow;
use uuid::Uuid;

const COLUMNS: &str = "id,kind,name,email,base_url,token_sealed,query_filter,last_sync_at,project_id,connected,last_error,insecure_tls,created_at,updated_at";

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct Source {
    id: Uuid,
    kind: String,
    name: String,
    email: String,
    base_url: String,
    #[serde(skip_serializing)]
    token_sealed: String,
    #[serde(rename = "query")]
    query_filter: String,
    last_sync_at: Option<DateTime<Utc>>,
    project_id: Option<Uuid>,
    connected: bool,
    #[serde(skip_serializing_if = "String::is_empty")]
    last_error: String,
    insecure_tls: bool,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
    #[sqlx(skip)]
    has_token: bool,
}

impl Source {
    fn public(mut self) -> Self {
        self.has_token = !self.token_sealed.is_empty();
        self
    }
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct SourceInput {
    kind: String,
    name: String,
    base_url: String,
    token: String,
    email: String,
    query: String,
    project_id: Option<Uuid>,
    insecure_tls: Option<bool>,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/sources", get(list).post(create))
        .route(
            "/api/sources/{id}",
            axum::routing::patch(patch).delete(delete),
        )
}

async fn project_exists(state: &AppState, project_id: Option<Uuid>) -> Result<(), Response> {
    let Some(project_id) = project_id.filter(|id| !id.is_nil()) else {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: source project"));
    };
    sqlx::query_scalar::<_, Uuid>("SELECT id FROM projects WHERE id=$1")
        .bind(project_id)
        .fetch_one(&state.pool)
        .await
        .map(|_| ())
        .map_err(not_found)
}

async fn probe(source: &Source, token: &str) -> Result<(), String> {
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(25))
        .redirect(if source.kind == "jira" {
            reqwest::redirect::Policy::none()
        } else {
            reqwest::redirect::Policy::default()
        })
        .danger_accept_invalid_certs(source.kind == "jira" && source.insecure_tls)
        .build()
        .map_err(|err| err.to_string())?;
    let endpoint = if source.kind == "jira" {
        let suffix = if source.base_url.to_lowercase().contains(".atlassian.net") {
            "/rest/api/3/myself"
        } else {
            "/rest/api/2/myself"
        };
        format!("{}{suffix}", source.base_url)
    } else {
        format!("{}/rest/v2/projects", source.base_url)
    };
    let mut request = client.get(endpoint).header("Accept", "application/json");
    if source.kind == "jira" {
        if source.base_url.to_lowercase().contains(".atlassian.net") {
            request = request.basic_auth(&source.email, Some(token));
        } else if let Some((user, password)) = token
            .split_once(':')
            .filter(|(user, password)| !user.is_empty() && !password.is_empty())
        {
            request = request.basic_auth(user, Some(password));
        } else {
            request = request.bearer_auth(token);
        }
    } else {
        request = request.bearer_auth(token);
    }
    let response = request
        .send()
        .await
        .map_err(|err| format!("{} probe: {err}", source.kind))?;
    if !response.status().is_success() {
        return Err(format!(
            "{} probe: HTTP {}",
            source.kind,
            response.status().as_u16()
        ));
    }
    if source.kind == "jira" {
        let raw = response.bytes().await.map_err(|err| err.to_string())?;
        if raw.iter().find(|byte| !byte.is_ascii_whitespace()) == Some(&b'<') {
            return Err("jira probe: HTML response".to_owned());
        }
    }
    Ok(())
}

async fn list(State(state): State<AppState>) -> Response {
    let query = format!("SELECT {COLUMNS} FROM sources ORDER BY kind,name");
    match sqlx::query_as::<_, Source>(&query)
        .fetch_all(&state.pool)
        .await
    {
        Ok(sources) => {
            Json(sources.into_iter().map(Source::public).collect::<Vec<_>>()).into_response()
        }
        Err(err) => database_error(err),
    }
}

async fn create(State(state): State<AppState>, body: Bytes) -> Response {
    let input: SourceInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    if !matches!(input.kind.as_str(), "jira" | "todoist" | "manual") {
        return error(StatusCode::BAD_REQUEST, "invalid: source kind");
    }
    if input.kind == "manual" {
        return error(StatusCode::CONFLICT, "conflict: manual source is built-in");
    }
    let name = input.name.trim();
    if name.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: source name");
    }
    let mut base_url = input.base_url.trim().trim_end_matches('/').to_owned();
    if input.kind == "jira" && base_url.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: jira base url");
    }
    let email = if input.kind == "jira" {
        let value = input.email.trim();
        if !value.contains('@') {
            return error(StatusCode::BAD_REQUEST, "invalid: jira email");
        }
        value.to_owned()
    } else {
        String::new()
    };
    if let Err(response) = project_exists(&state, input.project_id).await {
        return response;
    }
    if base_url.is_empty() {
        base_url = "https://api.todoist.com".to_owned();
    }
    let token = input.token.trim();
    let sealed = if token.is_empty() {
        String::new()
    } else {
        match state.token_box.seal(token) {
            Ok(sealed) => sealed,
            Err(_) => return error(StatusCode::INTERNAL_SERVER_ERROR, "internal error"),
        }
    };
    let now = Utc::now();
    let mut source = Source {
        id: Uuid::new_v4(),
        kind: input.kind.clone(),
        name: name.to_owned(),
        email,
        base_url,
        token_sealed: sealed,
        query_filter: if input.kind == "jira" {
            "assignee = currentUser() AND statusCategory != Done".to_owned()
        } else {
            input.query
        },
        last_sync_at: None,
        project_id: input.project_id,
        connected: false,
        last_error: String::new(),
        insecure_tls: input.insecure_tls.unwrap_or(false),
        created_at: now,
        updated_at: now,
        has_token: false,
    };
    if !token.is_empty() {
        match probe(&source, token).await {
            Ok(()) => source.connected = true,
            Err(err) => source.last_error = err,
        }
    }
    let query = format!("INSERT INTO sources ({COLUMNS}) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING {COLUMNS}");
    match sqlx::query_as::<_, Source>(&query)
        .bind(source.id)
        .bind(&source.kind)
        .bind(&source.name)
        .bind(&source.email)
        .bind(&source.base_url)
        .bind(&source.token_sealed)
        .bind(&source.query_filter)
        .bind(source.last_sync_at)
        .bind(source.project_id)
        .bind(source.connected)
        .bind(&source.last_error)
        .bind(source.insecure_tls)
        .bind(now)
        .bind(now)
        .fetch_one(&state.pool)
        .await
    {
        Ok(created) => (StatusCode::CREATED, Json(created.public())).into_response(),
        Err(err) => database_error(err),
    }
}

async fn patch(State(state): State<AppState>, Path(raw): Path<String>, body: Bytes) -> Response {
    let source_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let query = format!("SELECT {COLUMNS} FROM sources WHERE id=$1");
    let mut source = match sqlx::query_as::<_, Source>(&query)
        .bind(source_id)
        .fetch_one(&state.pool)
        .await
    {
        Ok(source) => source,
        Err(err) => return not_found(err),
    };
    if source.kind == "manual" {
        return error(StatusCode::FORBIDDEN, "forbidden: manual source");
    }
    let input: SourceInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    if !input.name.trim().is_empty() {
        source.name = input.name.trim().to_owned();
    }
    let mut reprobe = false;
    if !input.base_url.is_empty() {
        let next = input.base_url.trim().trim_end_matches('/').to_owned();
        reprobe |= next != source.base_url;
        source.base_url = next;
    }
    if source.kind != "jira" && !input.query.is_empty() {
        source.query_filter = input.query;
    }
    if let Some(project_id) = input.project_id {
        if let Err(response) = project_exists(&state, Some(project_id)).await {
            return response;
        }
        source.project_id = Some(project_id);
    }
    if source.kind == "jira" && !input.email.trim().is_empty() {
        if !input.email.contains('@') {
            return error(StatusCode::BAD_REQUEST, "invalid: jira email");
        }
        let next = input.email.trim().to_owned();
        reprobe |= next != source.email;
        source.email = next;
    }
    if let Some(insecure_tls) = input.insecure_tls {
        reprobe |= insecure_tls != source.insecure_tls;
        source.insecure_tls = insecure_tls;
    }
    let token = input.token.trim();
    let plain = if token.is_empty() {
        None
    } else {
        source.token_sealed = match state.token_box.seal(token) {
            Ok(sealed) => sealed,
            Err(_) => return error(StatusCode::INTERNAL_SERVER_ERROR, "internal error"),
        };
        Some(token.to_owned())
    };
    if reprobe || plain.is_some() {
        let plain = match plain {
            Some(plain) => plain,
            None if source.token_sealed.is_empty() => String::new(),
            None => match state.token_box.open(&source.token_sealed) {
                Ok(plain) => plain,
                Err(_) => return error(StatusCode::INTERNAL_SERVER_ERROR, "internal error"),
            },
        };
        if !plain.is_empty() {
            match probe(&source, &plain).await {
                Ok(()) => {
                    source.connected = true;
                    source.last_error.clear();
                }
                Err(err) => {
                    source.connected = false;
                    source.last_error = err;
                }
            }
        }
    }
    source.updated_at = Utc::now();
    let query = format!("UPDATE sources SET name=$2,email=$3,base_url=$4,token_sealed=$5,query_filter=$6,last_sync_at=$7,project_id=$8,connected=$9,last_error=$10,insecure_tls=$11,updated_at=$12 WHERE id=$1 RETURNING {COLUMNS}");
    match sqlx::query_as::<_, Source>(&query)
        .bind(source.id)
        .bind(&source.name)
        .bind(&source.email)
        .bind(&source.base_url)
        .bind(&source.token_sealed)
        .bind(&source.query_filter)
        .bind(source.last_sync_at)
        .bind(source.project_id)
        .bind(source.connected)
        .bind(&source.last_error)
        .bind(source.insecure_tls)
        .bind(source.updated_at)
        .fetch_one(&state.pool)
        .await
    {
        Ok(updated) => Json(updated.public()).into_response(),
        Err(err) => not_found(err),
    }
}

async fn delete(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let source_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let kind: String = match sqlx::query_scalar("SELECT kind FROM sources WHERE id=$1")
        .bind(source_id)
        .fetch_one(&state.pool)
        .await
    {
        Ok(kind) => kind,
        Err(err) => return not_found(err),
    };
    if kind == "manual" {
        return error(StatusCode::FORBIDDEN, "forbidden: manual source");
    }
    match sqlx::query("DELETE FROM sources WHERE id=$1")
        .bind(source_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}
