use crate::http::{database_error, AppState};
use axum::extract::State;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, Utc};
use serde::Serialize;
use sqlx::FromRow;
use uuid::Uuid;

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct JournalEntry {
    id: String,
    source: String,
    owner_id: Option<Uuid>,
    owner_name: String,
    body: String,
    created_at: DateTime<Utc>,
}

pub fn router() -> Router<AppState> {
    Router::new().route("/api/journal", get(list))
}

async fn list(State(state): State<AppState>) -> Response {
    match sqlx::query_as::<_, JournalEntry>("SELECT id::text AS id,source,owner_id,owner_name,body,created_at FROM (SELECT n.id,'loose'::text AS source,n.item_id AS owner_id,COALESCE(i.title,'Loose') AS owner_name,n.body,n.created_at FROM notes n LEFT JOIN items i ON i.id=n.item_id UNION ALL SELECT n.id,'project',n.project_id,p.name,n.body,n.created_at FROM project_notes n JOIN projects p ON p.id=n.project_id UNION ALL SELECT n.id,'item',n.item_id,i.title,n.body,n.created_at FROM item_notes n JOIN items i ON i.id=n.item_id AND i.deleted_at IS NULL UNION ALL SELECT n.id,'person',n.person_id,pe.name,n.body,n.created_at FROM person_notes n JOIN people pe ON pe.id=n.person_id) feed ORDER BY created_at DESC")
        .fetch_all(&state.pool)
        .await
    {
        Ok(entries) => Json(entries).into_response(),
        Err(err) => database_error(err),
    }
}
