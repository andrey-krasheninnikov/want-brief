use crate::http::{database_error, error, id, not_found, AppState};
use crate::sources::Source;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::post;
use axum::{Json, Router};
use chrono::{DateTime, Utc};
use serde::Deserialize;
use serde_json::Value;
use sqlx::FromRow;
use std::collections::HashSet;
use uuid::Uuid;

#[derive(FromRow)]
struct ItemRef {
    id: Uuid,
    source_id: Uuid,
    external_key: String,
    kind: String,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct JiraIssue {
    fields: JiraFields,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct JiraFields {
    summary: String,
    status: JiraStatus,
    description: Value,
    comment: JiraCommentPage,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct JiraStatus {
    name: String,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct JiraCommentPage {
    comments: Vec<JiraComment>,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct JiraComment {
    id: String,
    body: Value,
    author: JiraAuthor,
    created: String,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct JiraAuthor {
    display_name: String,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct TodoTask {
    content: String,
    description: String,
    is_completed: bool,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct TodoComment {
    id: String,
    content: String,
    posted_at: String,
}

struct RemoteComment {
    id: String,
    body: String,
    author: String,
    url: String,
    created_at: Option<DateTime<Utc>>,
}

struct RemoteDetail {
    title: String,
    status: String,
    description: String,
    comments: Vec<RemoteComment>,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/items/sync-active", post(sync_active))
        .route("/api/items/{id}/sync", post(sync_item))
}

fn jira_text(value: &Value) -> String {
    if let Some(text) = value.as_str() {
        return text.trim().to_owned();
    }
    if let Some(text) = value.get("text").and_then(Value::as_str) {
        return text.to_owned();
    }
    value
        .get("content")
        .and_then(Value::as_array)
        .map(|content| {
            content
                .iter()
                .map(jira_text)
                .filter(|text| !text.is_empty())
                .collect::<Vec<_>>()
                .join("\n")
        })
        .unwrap_or_default()
        .trim()
        .to_owned()
}

fn timestamp(raw: &str) -> Option<DateTime<Utc>> {
    DateTime::parse_from_rfc3339(raw)
        .or_else(|_| DateTime::parse_from_str(raw, "%Y-%m-%dT%H:%M:%S%.f%z"))
        .ok()
        .map(|time| time.with_timezone(&Utc))
}

async fn get_json<T: for<'de> Deserialize<'de>>(
    request: reqwest::RequestBuilder,
    label: &str,
) -> Result<T, String> {
    let response = request
        .send()
        .await
        .map_err(|err| format!("{label}: {err}"))?;
    if !response.status().is_success() {
        let status = response.status().as_u16();
        return Err(if label.starts_with("jira ") {
            let body = crate::sources::response_body(response, 4 << 20)
                .await
                .map(|(body, _)| body)
                .unwrap_or_default();
            crate::sources::jira_http_error(label, status, &body)
        } else {
            format!("{label}: HTTP {status}")
        });
    }
    let (body, too_large) = crate::sources::response_body(response, 4 << 20)
        .await
        .map_err(|err| format!("{label}: {err}"))?;
    if too_large {
        return Err(format!("{label}: response too large"));
    }
    if body.iter().find(|byte| !byte.is_ascii_whitespace()) == Some(&b'<') {
        return Err(format!("{label}: HTML response"));
    }
    serde_json::from_slice(&body).map_err(|err| format!("{label} decode: {err}"))
}

async fn fetch_remote(source: &Source, token: &str, key: &str) -> Result<RemoteDetail, String> {
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
    if source.kind == "todoist" {
        let mut task_url = reqwest::Url::parse(&source.base_url).map_err(|err| err.to_string())?;
        task_url
            .path_segments_mut()
            .map_err(|_| "invalid todoist URL")?
            .extend(["rest", "v2", "tasks", key]);
        let task: TodoTask = get_json(
            client
                .get(task_url)
                .bearer_auth(token)
                .header("Accept", "application/json"),
            "todoist task",
        )
        .await?;
        let mut comment_url =
            reqwest::Url::parse(&source.base_url).map_err(|err| err.to_string())?;
        comment_url
            .path_segments_mut()
            .map_err(|_| "invalid todoist URL")?
            .extend(["rest", "v2", "comments"]);
        comment_url.query_pairs_mut().append_pair("task_id", key);
        let comments: Vec<TodoComment> = get_json(
            client
                .get(comment_url)
                .bearer_auth(token)
                .header("Accept", "application/json"),
            "todoist comments",
        )
        .await?;
        return Ok(RemoteDetail {
            title: task.content,
            status: if task.is_completed {
                "Completed"
            } else {
                "Open"
            }
            .to_owned(),
            description: task.description,
            comments: comments
                .into_iter()
                .map(|comment| RemoteComment {
                    id: comment.id.trim().to_owned(),
                    body: comment.content.trim().to_owned(),
                    author: String::new(),
                    url: String::new(),
                    created_at: timestamp(&comment.posted_at),
                })
                .collect(),
        });
    }
    let cloud = source.base_url.to_lowercase().contains(".atlassian.net");
    let mut issue_url = reqwest::Url::parse(&source.base_url).map_err(|err| err.to_string())?;
    issue_url
        .path_segments_mut()
        .map_err(|_| "invalid jira URL")?
        .extend(["rest", "api", if cloud { "3" } else { "2" }, "issue", key]);
    issue_url.query_pairs_mut().append_pair(
        "fields",
        "summary,status,priority,duedate,created,description,comment",
    );
    let mut request = client.get(issue_url).header("Accept", "application/json");
    if cloud {
        request = request.basic_auth(&source.email, Some(token));
    } else if let Some((user, password)) = token
        .split_once(':')
        .filter(|(user, password)| !user.is_empty() && !password.is_empty())
    {
        request = request.basic_auth(user, Some(password));
    } else {
        request = request.bearer_auth(token);
    }
    let issue: JiraIssue = get_json(request, "jira issue").await?;
    let browse = format!("{}/browse/{key}", source.base_url.trim_end_matches('/'));
    Ok(RemoteDetail {
        title: issue.fields.summary,
        status: issue.fields.status.name.trim().to_owned(),
        description: jira_text(&issue.fields.description),
        comments: issue
            .fields
            .comment
            .comments
            .into_iter()
            .map(|comment| {
                let id = comment.id.trim().to_owned();
                RemoteComment {
                    url: if id.is_empty() {
                        String::new()
                    } else {
                        format!("{browse}?focusedCommentId={id}")
                    },
                    id,
                    body: jira_text(&comment.body),
                    author: comment.author.display_name.trim().to_owned(),
                    created_at: timestamp(&comment.created),
                }
            })
            .collect(),
    })
}

async fn sync_one(state: &AppState, item_id: Uuid) -> Result<Value, Response> {
    let item: ItemRef = sqlx::query_as("SELECT i.id,i.source_id,i.external_key,s.kind FROM items i JOIN sources s ON s.id=i.source_id WHERE i.id=$1 AND i.deleted_at IS NULL")
        .bind(item_id).fetch_one(&state.pool).await.map_err(not_found)?;
    if item.kind != "jira" && item.kind != "todoist" {
        return Err(error(StatusCode::FORBIDDEN, "forbidden: sync"));
    }
    if item.external_key.trim().is_empty() {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: external key"));
    }
    let source = Source::load(state, item.source_id).await?;
    let token = source.token(&state.token_box)?;
    let remote = fetch_remote(&source, &token, &item.external_key)
        .await
        .map_err(crate::sources::integration_error)?;
    let mut tx = state.pool.begin().await.map_err(database_error)?;
    sqlx::query("UPDATE items SET external_status=$2,title=CASE WHEN btrim($3::text)='' THEN title ELSE btrim($3) END,description=CASE WHEN btrim($4::text)='' THEN description ELSE btrim($4) END,updated_at=$5 WHERE id=$1")
        .bind(item.id).bind(remote.status.trim()).bind(&remote.title).bind(&remote.description).bind(Utc::now())
        .execute(&mut *tx).await.map_err(database_error)?;
    let notes: Vec<(String, String)> =
        sqlx::query_as("SELECT external_id,body FROM item_notes WHERE item_id=$1")
            .bind(item.id)
            .fetch_all(&mut *tx)
            .await
            .map_err(database_error)?;
    let mut seen_ids: HashSet<String> = notes
        .iter()
        .filter(|(external, _)| !external.is_empty())
        .map(|(external, _)| external.clone())
        .collect();
    let mut seen_bodies: HashSet<String> = notes
        .iter()
        .map(|(_, body)| body.trim().to_owned())
        .collect();
    for comment in remote.comments {
        if comment.body.is_empty()
            || seen_ids.contains(&comment.id)
            || seen_bodies.contains(&comment.body)
        {
            continue;
        }
        sqlx::query("INSERT INTO item_notes (id,item_id,body,source_kind,external_id,author_name,url,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)")
            .bind(Uuid::new_v4()).bind(item.id).bind(&comment.body).bind(&source.kind).bind(&comment.id)
            .bind(&comment.author).bind(&comment.url).bind(comment.created_at.unwrap_or_else(Utc::now))
            .execute(&mut *tx).await.map_err(database_error)?;
        seen_ids.insert(comment.id);
        seen_bodies.insert(comment.body);
    }
    tx.commit().await.map_err(database_error)?;
    let updated = crate::items::fetch(&state.pool, item.id)
        .await
        .map_err(not_found)?;
    serde_json::to_value(updated)
        .map_err(|_| error(StatusCode::INTERNAL_SERVER_ERROR, "internal error"))
}

async fn sync_item(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sync_one(&state, item_id).await {
        Ok(item) => Json(item).into_response(),
        Err(response) => response,
    }
}

async fn sync_active(State(state): State<AppState>) -> Response {
    let items: Vec<(Uuid,)> = match sqlx::query_as("SELECT i.id FROM items i JOIN sources s ON s.id=i.source_id WHERE i.deleted_at IS NULL AND i.archived_at IS NULL AND i.status NOT IN ('done','cancelled') AND s.kind IN ('jira','todoist') AND btrim(i.external_key)<>'' ORDER BY i.updated_at DESC")
        .fetch_all(&state.pool).await {
        Ok(items) => items,
        Err(err) => return database_error(err),
    };
    let mut synced = 0;
    let mut failed = 0;
    for (item_id,) in items {
        if sync_one(&state, item_id).await.is_ok() {
            synced += 1;
        } else {
            failed += 1;
        }
    }
    Json(serde_json::json!({"synced": synced, "failed": failed})).into_response()
}
