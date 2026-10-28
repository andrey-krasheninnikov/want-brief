use crate::http::{database_error, decode, error, id, not_found, AppState};
use axum::body::Bytes;
use axum::extract::{Path, Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sqlx::FromRow;
use uuid::Uuid;

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct Note {
    id: Uuid,
    item_id: Option<Uuid>,
    body: String,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct NoteWrite {
    body: String,
    item_id: Option<String>,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct NoteQuery {
    item_id: Option<String>,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/notes", get(list).post(create))
        .route(
            "/api/notes/{id}",
            axum::routing::patch(patch).delete(delete),
        )
}

async fn list(State(state): State<AppState>, Query(query): Query<NoteQuery>) -> Response {
    let item_id = match query.item_id.filter(|value| !value.is_empty()) {
        Some(raw) => match id(&raw) {
            Ok(id) => Some(id),
            Err(response) => return response,
        },
        None => None,
    };
    let notes = if let Some(item_id) = item_id {
        sqlx::query_as::<_, Note>(
            "SELECT id, item_id, body, created_at, updated_at FROM notes WHERE item_id=$1 ORDER BY updated_at DESC",
        )
        .bind(item_id)
        .fetch_all(&state.pool)
        .await
    } else {
        sqlx::query_as::<_, Note>(
            "SELECT id, item_id, body, created_at, updated_at FROM notes WHERE item_id IS NULL ORDER BY updated_at DESC",
        )
        .fetch_all(&state.pool)
        .await
    };
    match notes {
        Ok(notes) => Json(notes).into_response(),
        Err(err) => database_error(err),
    }
}

async fn create(State(state): State<AppState>, body: Bytes) -> Response {
    let input: NoteWrite = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    let item_id = match input.item_id.filter(|value| !value.is_empty()) {
        Some(raw) => match id(&raw) {
            Ok(id) => Some(id),
            Err(response) => return response,
        },
        None => None,
    };
    let body = input.body.trim();
    if body.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: note body");
    }
    let now = Utc::now();
    match sqlx::query_as::<_, Note>(
        "INSERT INTO notes (id, item_id, body, created_at, updated_at) VALUES ($1,$2,$3,$4,$5) RETURNING id, item_id, body, created_at, updated_at",
    )
    .bind(Uuid::new_v4())
    .bind(item_id)
    .bind(body)
    .bind(now)
    .bind(now)
    .fetch_one(&state.pool)
    .await
    {
        Ok(note) => (StatusCode::CREATED, Json(note)).into_response(),
        Err(err) => database_error(err),
    }
}

async fn patch(State(state): State<AppState>, Path(raw): Path<String>, body: Bytes) -> Response {
    let note_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(err) = sqlx::query_scalar::<_, Uuid>("SELECT id FROM notes WHERE id=$1")
        .bind(note_id)
        .fetch_one(&state.pool)
        .await
    {
        return not_found(err);
    }
    let input: NoteWrite = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    let body = input.body.trim();
    if body.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: note body");
    }
    match sqlx::query_as::<_, Note>(
        "UPDATE notes SET body=$2, updated_at=$3 WHERE id=$1 RETURNING id, item_id, body, created_at, updated_at",
    )
    .bind(note_id)
    .bind(body)
    .bind(Utc::now())
    .fetch_one(&state.pool)
    .await
    {
        Ok(note) => Json(note).into_response(),
        Err(err) => not_found(err),
    }
}

async fn delete(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let note_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query("DELETE FROM notes WHERE id=$1")
        .bind(note_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}
