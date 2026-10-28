use crate::http::{database_error, decode, error, id, AppState};
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::Utc;
use serde::Deserialize;
use serde_json::Value;
use sqlx::PgPool;
use uuid::Uuid;

#[derive(Deserialize)]
struct NoteWrite {
    body: String,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/companies/{id}/notes", get(list).post(create))
        .route(
            "/api/companies/{id}/notes/{noteId}",
            axum::routing::patch(replace).delete(delete),
        )
}

async fn get_note(pool: &PgPool, company: Uuid, note: Uuid) -> Result<Value, sqlx::Error> {
    sqlx::query_scalar("SELECT to_jsonb(n) FROM (SELECT id,company_id AS \"companyId\",body,created_at AS \"createdAt\" FROM company_notes WHERE company_id=$1 AND id=$2) n")
        .bind(company).bind(note).fetch_one(pool).await
}

async fn list(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let company = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let exists: bool =
        match sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM companies WHERE id=$1)")
            .bind(company)
            .fetch_one(&state.pool)
            .await
        {
            Ok(value) => value,
            Err(err) => return database_error(err),
        };
    if !exists {
        return error(StatusCode::NOT_FOUND, "not found");
    }
    let result:Result<Value,_>=sqlx::query_scalar("SELECT COALESCE(jsonb_agg(to_jsonb(n) ORDER BY n.\"createdAt\" DESC),'[]'::jsonb) FROM (SELECT id,company_id AS \"companyId\",body,created_at AS \"createdAt\" FROM company_notes WHERE company_id=$1) n")
        .bind(company).fetch_one(&state.pool).await;
    match result {
        Ok(value) => Json(value).into_response(),
        Err(err) => database_error(err),
    }
}

async fn save(state: AppState, company: String, note: Option<String>, body: Bytes) -> Response {
    let company = match id(&company) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let updating = note.is_some();
    let note = match note {
        Some(raw) => match id(&raw) {
            Ok(id) => id,
            Err(response) => return response,
        },
        None => Uuid::new_v4(),
    };
    let write: NoteWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let text = write.body.trim();
    if text.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: note body");
    }
    let result = if updating {
        sqlx::query("UPDATE company_notes SET body=$3 WHERE company_id=$1 AND id=$2")
            .bind(company)
            .bind(note)
            .bind(text)
            .execute(&state.pool)
            .await
    } else {
        sqlx::query(
            "INSERT INTO company_notes (company_id,id,body,created_at) VALUES ($1,$2,$3,$4)",
        )
        .bind(company)
        .bind(note)
        .bind(text)
        .bind(Utc::now())
        .execute(&state.pool)
        .await
    };
    match result {
        Ok(result) if result.rows_affected() > 0 => {
            match get_note(&state.pool, company, note).await {
                Ok(value) if !updating => (StatusCode::CREATED, Json(value)).into_response(),
                Ok(value) => Json(value).into_response(),
                Err(err) => database_error(err),
            }
        }
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err)
            if err
                .as_database_error()
                .is_some_and(|db| db.code().as_deref() == Some("23503")) =>
        {
            error(StatusCode::NOT_FOUND, "not found")
        }
        Err(err) => database_error(err),
    }
}

async fn create(
    State(state): State<AppState>,
    Path(company): Path<String>,
    body: Bytes,
) -> Response {
    save(state, company, None, body).await
}
async fn replace(
    State(state): State<AppState>,
    Path((company, note)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    save(state, company, Some(note), body).await
}
async fn delete(
    State(state): State<AppState>,
    Path((company, note)): Path<(String, String)>,
) -> Response {
    let company = match id(&company) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let note = match id(&note) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query("DELETE FROM company_notes WHERE company_id=$1 AND id=$2")
        .bind(company)
        .bind(note)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}
