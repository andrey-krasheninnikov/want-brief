use crate::crypto::TokenBox;
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

const COLUMNS: &str = "id,kind,name,email,base_url,token_sealed,query_filter,last_sync_at,project_id,connected,last_error,insecure_tls,created_at,updated_at,sync_interval_minutes,last_attempt_at";

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Source {
    id: Uuid,
    pub(crate) kind: String,
    name: String,
    pub(crate) email: String,
    pub(crate) base_url: String,
    #[serde(skip_serializing)]
    token_sealed: String,
    #[serde(rename = "query")]
    query_filter: String,
    last_sync_at: Option<DateTime<Utc>>,
    sync_interval_minutes: i32,
    last_attempt_at: Option<DateTime<Utc>>,
    project_id: Option<Uuid>,
    connected: bool,
    #[serde(skip_serializing_if = "String::is_empty")]
    last_error: String,
    pub(crate) insecure_tls: bool,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
    #[sqlx(skip)]
    has_token: bool,
}

impl Source {
    pub(crate) async fn load(state: &AppState, source_id: Uuid) -> Result<Self, Response> {
        let query = format!("SELECT {COLUMNS} FROM sources WHERE id=$1");
        sqlx::query_as::<_, Self>(&query)
            .bind(source_id)
            .fetch_one(&state.pool)
            .await
            .map_err(not_found)
    }

    #[allow(clippy::result_large_err)]
    pub(crate) fn token(&self, token_box: &TokenBox) -> Result<String, Response> {
        if self.token_sealed.is_empty() {
            return Err(error(StatusCode::FORBIDDEN, "source token is empty"));
        }
        token_box
            .open(&self.token_sealed)
            .map_err(|_| error(StatusCode::INTERNAL_SERVER_ERROR, "internal error"))
    }

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
    sync_interval_minutes: Option<i32>,
}

fn valid_interval(minutes: i32) -> bool {
    matches!(minutes, 0 | 15 | 60 | 360 | 1440)
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/sources", get(list).post(create))
        .route(
            "/api/sources/{id}",
            axum::routing::patch(patch).delete(delete),
        )
        .route("/api/sources/{id}/sync", axum::routing::post(sync))
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
        let status = response.status().as_u16();
        return Err(if source.kind == "jira" {
            let body = response_body(response, 4 << 20)
                .await
                .map(|(body, _)| body)
                .unwrap_or_default();
            jira_http_error("jira probe", status, &body)
        } else {
            format!("todoist probe: HTTP {status}")
        });
    }
    if source.kind == "jira" {
        let (raw, _) = response_body(response, 4 << 20)
            .await
            .map_err(|err| err.to_string())?;
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
    let sync_interval_minutes = input.sync_interval_minutes.unwrap_or(0);
    if !valid_interval(sync_interval_minutes) {
        return error(StatusCode::BAD_REQUEST, "invalid: sync interval");
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
        sync_interval_minutes,
        last_attempt_at: None,
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
    let query = format!("INSERT INTO sources ({COLUMNS}) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING {COLUMNS}");
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
        .bind(source.sync_interval_minutes)
        .bind(source.last_attempt_at)
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
    if let Some(minutes) = input.sync_interval_minutes {
        if !valid_interval(minutes) {
            return error(StatusCode::BAD_REQUEST, "invalid: sync interval");
        }
        source.sync_interval_minutes = minutes;
    }
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
    let should_reprobe = reprobe || plain.is_some();
    if should_reprobe {
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
    let query = format!("UPDATE sources SET name=$2,email=$3,base_url=$4,token_sealed=$5,query_filter=$6,project_id=$7,connected=CASE WHEN $11 THEN $8 ELSE connected END,last_error=CASE WHEN $11 THEN $9 ELSE last_error END,insecure_tls=$10,updated_at=$12,sync_interval_minutes=$13 WHERE id=$1 RETURNING {COLUMNS}");
    match sqlx::query_as::<_, Source>(&query)
        .bind(source.id)
        .bind(&source.name)
        .bind(&source.email)
        .bind(&source.base_url)
        .bind(&source.token_sealed)
        .bind(&source.query_filter)
        .bind(source.project_id)
        .bind(source.connected)
        .bind(&source.last_error)
        .bind(source.insecure_tls)
        .bind(should_reprobe)
        .bind(source.updated_at)
        .bind(source.sync_interval_minutes)
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

#[derive(Default, Deserialize)]
#[serde(default)]
struct JiraIssue {
    key: String,
    fields: JiraFields,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct JiraFields {
    summary: String,
    duedate: String,
    created: String,
    priority: Option<JiraName>,
}

#[derive(Default, Deserialize)]
struct JiraName {
    name: String,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct JiraSearch {
    issues: Vec<JiraIssue>,
    next_page_token: String,
    is_last: bool,
    total: usize,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct TodoTask {
    id: String,
    content: String,
    due: Option<TodoDue>,
    priority: i32,
}

#[derive(Deserialize)]
struct TodoDue {
    date: String,
}

struct RemoteItem {
    key: String,
    title: String,
    due_at: Option<DateTime<Utc>>,
    created_at: Option<DateTime<Utc>>,
    urgent: bool,
    important: bool,
    url: String,
}

fn parse_due(raw: &str) -> Option<DateTime<Utc>> {
    chrono::NaiveDate::parse_from_str(raw, "%Y-%m-%d")
        .ok()
        .and_then(|date| date.and_hms_opt(0, 0, 0))
        .map(|date| date.and_utc())
        .or_else(|| {
            DateTime::parse_from_rfc3339(raw)
                .ok()
                .map(|date| date.with_timezone(&Utc))
        })
}

fn jira_time(raw: &str) -> Option<DateTime<Utc>> {
    DateTime::parse_from_rfc3339(raw)
        .or_else(|_| DateTime::parse_from_str(raw, "%Y-%m-%dT%H:%M:%S%.f%z"))
        .ok()
        .map(|date| date.with_timezone(&Utc))
}

fn source_client(source: &Source) -> Result<reqwest::Client, String> {
    reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(25))
        .redirect(if source.kind == "jira" {
            reqwest::redirect::Policy::none()
        } else {
            reqwest::redirect::Policy::default()
        })
        .danger_accept_invalid_certs(source.kind == "jira" && source.insecure_tls)
        .build()
        .map_err(|err| err.to_string())
}

fn jira_auth(
    request: reqwest::RequestBuilder,
    source: &Source,
    token: &str,
) -> reqwest::RequestBuilder {
    if source.base_url.to_lowercase().contains(".atlassian.net") {
        request.basic_auth(&source.email, Some(token))
    } else if let Some((user, password)) = token
        .split_once(':')
        .filter(|(user, password)| !user.is_empty() && !password.is_empty())
    {
        request.basic_auth(user, Some(password))
    } else {
        request.bearer_auth(token)
    }
}

async fn pull(source: &Source, token: &str) -> Result<Vec<RemoteItem>, String> {
    let client = source_client(source)?;
    if source.kind == "todoist" {
        let endpoint = format!("{}/rest/v2/tasks", source.base_url);
        let request = client
            .get(endpoint)
            .bearer_auth(token)
            .header("Accept", "application/json");
        let request = if source.query_filter.trim().is_empty() {
            request
        } else {
            request.query(&[("filter", source.query_filter.trim())])
        };
        let response = request
            .send()
            .await
            .map_err(|err| format!("todoist tasks: {err}"))?;
        if !response.status().is_success() {
            return Err(format!(
                "todoist tasks: HTTP {}",
                response.status().as_u16()
            ));
        }
        let (raw, too_large) = response_body(response, 4 << 20)
            .await
            .map_err(|err| format!("todoist tasks: {err}"))?;
        if too_large {
            return Err("todoist tasks: response too large".to_owned());
        }
        let tasks: Vec<TodoTask> =
            serde_json::from_slice(&raw).map_err(|err| format!("todoist decode: {err}"))?;
        return Ok(tasks
            .into_iter()
            .map(|task| {
                let due_at = task.due.as_ref().and_then(|due| parse_due(&due.date));
                let urgent = task.priority == 4 || due_at.is_some_and(|due| due <= Utc::now());
                RemoteItem {
                    key: task.id,
                    title: task.content,
                    due_at,
                    created_at: None,
                    urgent,
                    important: task.priority >= 3,
                    url: String::new(),
                }
            })
            .collect());
    }
    let cloud = source.base_url.to_lowercase().contains(".atlassian.net");
    let endpoint = if cloud {
        format!("{}/rest/api/3/search/jql", source.base_url)
    } else {
        format!("{}/rest/api/2/search", source.base_url)
    };
    let mut page_token = String::new();
    let mut start_at = 0_usize;
    let mut items = Vec::new();
    for _ in 0..50 {
        let mut body = serde_json::json!({"jql":"assignee = currentUser() AND statusCategory != Done","fields":["summary","status","priority","duedate","created"],"maxResults":100});
        if cloud {
            if !page_token.is_empty() {
                body["nextPageToken"] = page_token.clone().into();
            }
        } else {
            body["startAt"] = start_at.into();
        }
        let request = jira_auth(
            client
                .post(&endpoint)
                .header("Accept", "application/json")
                .json(&body),
            source,
            token,
        );
        let response = request
            .send()
            .await
            .map_err(|err| format!("jira search: {err}"))?;
        if !response.status().is_success() {
            let status = response.status().as_u16();
            let body = response_body(response, 4 << 20)
                .await
                .map(|(body, _)| body)
                .unwrap_or_default();
            return Err(jira_http_error("jira search", status, &body));
        }
        let (raw, too_large) = response_body(response, 4 << 20)
            .await
            .map_err(|err| format!("jira search: {err}"))?;
        if too_large {
            return Err("jira search: response too large".to_owned());
        }
        if raw.iter().find(|byte| !byte.is_ascii_whitespace()) == Some(&b'<') {
            return Err("jira decode: HTML from search".to_owned());
        }
        let page: JiraSearch =
            serde_json::from_slice(&raw).map_err(|err| format!("jira decode: {err}"))?;
        let count = page.issues.len();
        for issue in page.issues {
            let due_at = parse_due(&issue.fields.duedate);
            let priority = issue
                .fields
                .priority
                .map(|value| value.name.to_lowercase())
                .unwrap_or_default();
            items.push(RemoteItem {
                key: issue.key.clone(),
                title: issue.fields.summary,
                due_at,
                created_at: jira_time(&issue.fields.created),
                urgent: priority == "highest"
                    || priority == "blocker"
                    || due_at.is_some_and(|due| due <= Utc::now()),
                important: priority == "highest" || priority == "high",
                url: format!("{}/browse/{}", source.base_url, issue.key),
            });
        }
        if cloud {
            if page.is_last || page.next_page_token.is_empty() {
                break;
            }
            page_token = page.next_page_token;
        } else {
            start_at += count;
            if count == 0 || start_at >= page.total {
                break;
            }
        }
    }
    Ok(items)
}

async fn finish_failure(
    state: &AppState,
    source: &Source,
    claim_id: Uuid,
    message: &str,
) -> Result<(), sqlx::Error> {
    sqlx::query(
        "UPDATE sources SET connected=CASE WHEN updated_at=$4 THEN false ELSE connected END, \
         last_error=CASE WHEN updated_at=$4 THEN $3 ELSE last_error END, \
         updated_at=CASE WHEN updated_at=$4 THEN clock_timestamp() ELSE updated_at END, \
         sync_claim_id=NULL,sync_claim_until=NULL WHERE id=$1 AND sync_claim_id=$2",
    )
    .bind(source.id)
    .bind(claim_id)
    .bind(message)
    .bind(source.updated_at)
    .execute(&state.pool)
    .await?;
    Ok(())
}

pub(crate) fn integration_error(message: String) -> Response {
    let status = if message.contains("HTTP") {
        StatusCode::BAD_GATEWAY
    } else {
        StatusCode::INTERNAL_SERVER_ERROR
    };
    if status == StatusCode::BAD_GATEWAY {
        error(status, message)
    } else {
        error(status, "internal error")
    }
}

pub(crate) fn jira_http_error(operation: &str, status: u16, body: &[u8]) -> String {
    let base = format!("{operation}: HTTP {status}");
    let detail = serde_json::from_slice::<serde_json::Value>(body)
        .ok()
        .and_then(|body| {
            body["errorMessages"][0]
                .as_str()
                .map(str::trim)
                .map(str::to_owned)
        })
        .filter(|detail| !detail.is_empty());
    match detail {
        Some(detail) => format!("{base} ({})", detail.chars().take(200).collect::<String>()),
        None => base,
    }
}

pub(crate) async fn response_body(
    mut response: reqwest::Response,
    limit: usize,
) -> Result<(Vec<u8>, bool), reqwest::Error> {
    let mut body = Vec::new();
    while let Some(chunk) = response.chunk().await? {
        let remaining = limit.saturating_sub(body.len());
        body.extend_from_slice(&chunk[..chunk.len().min(remaining)]);
        if chunk.len() > remaining {
            return Ok((body, true));
        }
    }
    Ok((body, false))
}

async fn sync(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let source_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    sync_source(&state, source_id, false).await
}

async fn sync_source(state: &AppState, source_id: Uuid, scheduled: bool) -> Response {
    if !scheduled {
        let source = match Source::load(state, source_id).await {
            Ok(source) => source,
            Err(response) => return response,
        };
        if source.kind == "manual" {
            return error(StatusCode::FORBIDDEN, "forbidden: manual source");
        }
        if source.project_id.is_none() {
            return error(StatusCode::BAD_REQUEST, "invalid: source project");
        }
        if let Err(response) = source.token(&state.token_box) {
            return response;
        }
    }
    let claim_id = Uuid::new_v4();
    let query = format!(
        "UPDATE sources SET sync_claim_id=$2,sync_claim_until=clock_timestamp()+INTERVAL '10 minutes', \
         last_attempt_at=clock_timestamp() WHERE id=$1 AND kind IN ('jira','todoist') \
         AND project_id IS NOT NULL AND token_sealed<>'' \
         AND (sync_claim_until IS NULL OR sync_claim_until<=clock_timestamp()) \
         AND (NOT $3::boolean OR (sync_interval_minutes>0 AND \
         (last_attempt_at IS NULL OR last_attempt_at + sync_interval_minutes*INTERVAL '1 minute'<=clock_timestamp()))) \
         RETURNING {COLUMNS}"
    );
    let source = match sqlx::query_as::<_, Source>(&query)
        .bind(source_id)
        .bind(claim_id)
        .bind(scheduled)
        .fetch_optional(&state.pool)
        .await
    {
        Ok(Some(source)) => source,
        Ok(None) => return error(StatusCode::CONFLICT, "source sync unavailable"),
        Err(err) => return database_error(err),
    };
    let token = match source.token(&state.token_box) {
        Ok(token) => token,
        Err(response) => {
            if let Err(err) = finish_failure(state, &source, claim_id, "source token invalid").await
            {
                return database_error(err);
            }
            return response;
        }
    };
    let remote = tokio::time::timeout(std::time::Duration::from_secs(300), async {
        probe(&source, &token).await?;
        pull(&source, &token).await
    })
    .await;
    let remote = match remote {
        Ok(Ok(remote)) => remote,
        other => {
            let message = match other {
                Ok(Err(message)) => message,
                Err(_) => "source sync timed out".to_owned(),
                Ok(Ok(_)) => unreachable!(),
            };
            if let Err(err) = finish_failure(state, &source, claim_id, &message).await {
                return database_error(err);
            }
            return integration_error(message);
        }
    };
    match apply_remote(state, &source, claim_id, remote).await {
        Ok(upserted) => Json(serde_json::json!({"upserted":upserted})).into_response(),
        Err(response) => {
            if let Err(err) = finish_failure(state, &source, claim_id, "local sync error").await {
                return database_error(err);
            }
            response
        }
    }
}

async fn apply_remote(
    state: &AppState,
    source: &Source,
    claim_id: Uuid,
    remote: Vec<RemoteItem>,
) -> Result<usize, Response> {
    let source_id = source.id;
    let mut transaction = match state.pool.begin().await {
        Ok(transaction) => transaction,
        Err(err) => return Err(database_error(err)),
    };
    let current: Option<(Uuid, DateTime<Utc>)> =
        match sqlx::query_as("SELECT sync_claim_id,updated_at FROM sources WHERE id=$1 AND sync_claim_until>clock_timestamp() FOR UPDATE")
            .bind(source_id)
            .fetch_optional(&mut *transaction)
            .await
        {
            Ok(current) => current,
            Err(err) => return Err(database_error(err)),
        };
    if current != Some((claim_id, source.updated_at)) {
        return Err(error(StatusCode::CONFLICT, "source changed during sync"));
    }
    let mut upserted = 0;
    for item in remote {
        let now = Utc::now();
        let created_at = item.created_at.unwrap_or(now);
        let links = if item.url.is_empty() {
            serde_json::json!([])
        } else {
            let label = if source.kind == "jira" {
                "Jira"
            } else {
                "Todoist"
            };
            serde_json::json!([{"label":label,"url":item.url}])
        };
        let result = sqlx::query_scalar::<_, Uuid>("INSERT INTO items (id,source_id,external_key,title,status,kind,project_id,urgent,important,due_at,links,created_at,updated_at) VALUES ($1,$2,$3,$4,'backlog','task',$5,$6,$7,$8,$9,$10,$11) ON CONFLICT (source_id,external_key) WHERE external_key <> '' DO UPDATE SET title=CASE WHEN btrim(EXCLUDED.title)='' THEN items.title ELSE EXCLUDED.title END,due_at=EXCLUDED.due_at,project_id=EXCLUDED.project_id,links=EXCLUDED.links,created_at=LEAST(items.created_at,EXCLUDED.created_at),updated_at=EXCLUDED.updated_at WHERE items.deleted_at IS NULL RETURNING id")
            .bind(Uuid::new_v4()).bind(source_id).bind(&item.key).bind(&item.title).bind(source.project_id)
            .bind(item.urgent).bind(item.important).bind(item.due_at).bind(sqlx::types::Json(links))
            .bind(created_at).bind(now)
            .fetch_one(&mut *transaction).await;
        if let Err(err) = result {
            return Err(not_found(err));
        }
        upserted += 1;
    }
    let now = Utc::now();
    if let Err(err) = sqlx::query(
        "UPDATE sources SET last_sync_at=$2,connected=true,last_error='',updated_at=$2, \
         sync_claim_id=NULL,sync_claim_until=NULL WHERE id=$1 AND sync_claim_id=$3",
    )
    .bind(source_id)
    .bind(now)
    .bind(claim_id)
    .execute(&mut *transaction)
    .await
    {
        return Err(database_error(err));
    }
    if let Err(err) = transaction.commit().await {
        return Err(database_error(err));
    }
    Ok(upserted)
}

pub async fn run_scheduler(state: AppState) {
    let mut ticker = tokio::time::interval(std::time::Duration::from_secs(60));
    ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Skip);
    let mut running = tokio::task::JoinSet::new();
    loop {
        ticker.tick().await;
        while running.try_join_next().is_some() {}
        let available = 4usize.saturating_sub(running.len());
        if available == 0 {
            continue;
        }
        let due: Vec<Uuid> = match sqlx::query_scalar(
            "SELECT id FROM sources WHERE kind IN ('jira','todoist') AND project_id IS NOT NULL \
             AND token_sealed<>'' AND sync_interval_minutes>0 \
             AND (sync_claim_until IS NULL OR sync_claim_until<=clock_timestamp()) \
             AND (last_attempt_at IS NULL OR \
             last_attempt_at+sync_interval_minutes*INTERVAL '1 minute'<=clock_timestamp()) \
             ORDER BY last_attempt_at NULLS FIRST,id LIMIT $1",
        )
        .bind(available as i64)
        .fetch_all(&state.pool)
        .await
        {
            Ok(due) => due,
            Err(err) => {
                eprintln!("scheduled source lookup failed: {err}");
                continue;
            }
        };
        for source_id in due {
            let state = state.clone();
            running.spawn(async move {
                let response = sync_source(&state, source_id, true).await;
                if response.status().is_server_error() {
                    eprintln!("scheduled source sync failed: {}", response.status());
                }
            });
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn source_token_preserves_error_status() {
        let token_box =
            TokenBox::new("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
                .unwrap();
        let mut source = Source {
            id: Uuid::nil(),
            kind: "jira".to_owned(),
            name: String::new(),
            email: String::new(),
            base_url: String::new(),
            token_sealed: String::new(),
            query_filter: String::new(),
            last_sync_at: None,
            sync_interval_minutes: 0,
            last_attempt_at: None,
            project_id: None,
            connected: false,
            last_error: String::new(),
            insecure_tls: false,
            created_at: Utc::now(),
            updated_at: Utc::now(),
            has_token: false,
        };
        assert_eq!(
            source.token(&token_box).unwrap_err().status(),
            StatusCode::FORBIDDEN
        );
        source.token_sealed = "invalid".to_owned();
        assert_eq!(
            source.token(&token_box).unwrap_err().status(),
            StatusCode::INTERNAL_SERVER_ERROR
        );
        source.token_sealed = token_box.seal("secret").unwrap();
        assert_eq!(source.token(&token_box).unwrap(), "secret");
    }

    #[test]
    fn jira_error_keeps_remote_detail() {
        let body = br#"{"errorMessages":["  permission denied  "]}"#;
        assert_eq!(
            jira_http_error("jira search", 403, body),
            "jira search: HTTP 403 (permission denied)"
        );
        assert_eq!(
            jira_http_error("jira search", 503, b"bad"),
            "jira search: HTTP 503"
        );
    }

    #[test]
    fn remote_failures_keep_go_error_status() {
        assert_eq!(
            integration_error("jira search: connection refused".to_owned()).status(),
            StatusCode::INTERNAL_SERVER_ERROR
        );
        assert_eq!(
            integration_error("jira search: HTTP 503".to_owned()).status(),
            StatusCode::BAD_GATEWAY
        );
    }

    #[tokio::test]
    async fn remote_response_is_bounded() {
        let app = Router::new().route("/", get(|| async { "123456789" }));
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let url = format!("http://{}", listener.local_addr().unwrap());
        let server = tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
        let response = reqwest::get(&url).await.unwrap();
        assert_eq!(
            response_body(response, 4).await.unwrap(),
            (b"1234".to_vec(), true)
        );
        let response = reqwest::get(&url).await.unwrap();
        assert_eq!(
            response_body(response, 9).await.unwrap(),
            (b"123456789".to_vec(), false)
        );
        server.abort();
    }
}
