use crate::http::{database_error, decode, error, id, not_found, AppState};
use crate::track::item_exists;
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sqlx::{FromRow, PgPool, Postgres, Transaction};
use std::collections::HashSet;
use uuid::Uuid;

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct ItemNote {
    id: Uuid,
    item_id: Uuid,
    body: String,
    source_kind: String,
    external_id: String,
    author_name: String,
    url: String,
    created_at: DateTime<Utc>,
}

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct ItemCheck {
    id: Uuid,
    item_id: Uuid,
    body: String,
    done: bool,
    position: i32,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct BodyInput {
    body: String,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct CheckPatch {
    body: Option<String>,
    done: Option<bool>,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct CheckOrder {
    ids: Vec<String>,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/items/{id}/notes", get(list_notes).post(create_note))
        .route(
            "/api/items/{id}/notes/{noteId}",
            axum::routing::delete(delete_note),
        )
        .route(
            "/api/items/{id}/checks",
            get(list_checks).post(create_check),
        )
        .route(
            "/api/items/{id}/checks/order",
            axum::routing::put(reorder_checks),
        )
        .route("/api/items/{id}/events", get(list_events))
        .route(
            "/api/items/{id}/events/annotate",
            axum::routing::post(annotate_event),
        )
        .route(
            "/api/items/{id}/checks/{checkId}",
            axum::routing::patch(patch_check).delete(delete_check),
        )
}

async fn event(
    tx: &mut Transaction<'_, Postgres>,
    item_id: Uuid,
    field: &str,
    from: &str,
    to: &str,
) -> Result<(), sqlx::Error> {
    sqlx::query("INSERT INTO item_events (id,item_id,kind,field,from_value,to_value,note,created_at) VALUES ($1,$2,'check',$3,$4,$5,'',$6)")
        .bind(Uuid::new_v4()).bind(item_id).bind(field).bind(from).bind(to).bind(Utc::now())
        .execute(&mut **tx).await?;
    Ok(())
}

async fn list_notes(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    match sqlx::query_as::<_, ItemNote>("SELECT id,item_id,body,source_kind,external_id,author_name,url,created_at FROM item_notes WHERE item_id=$1 ORDER BY created_at DESC")
        .bind(item_id).fetch_all(&state.pool).await {
        Ok(notes) => Json(notes).into_response(), Err(err) => database_error(err),
    }
}

async fn create_note(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    body: Bytes,
) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let input: BodyInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    let body = input.body.trim();
    if body.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: note body");
    }
    let now = Utc::now();
    match sqlx::query_as::<_, ItemNote>("INSERT INTO item_notes (id,item_id,body,source_kind,external_id,author_name,url,created_at) VALUES ($1,$2,$3,'manual','','','',$4) RETURNING id,item_id,body,source_kind,external_id,author_name,url,created_at")
        .bind(Uuid::new_v4()).bind(item_id).bind(body).bind(now).fetch_one(&state.pool).await {
        Ok(note) => (StatusCode::CREATED, Json(note)).into_response(),
        Err(err) => database_error(err),
    }
}

async fn delete_note(
    State(state): State<AppState>,
    Path((raw_item, raw_note)): Path<(String, String)>,
) -> Response {
    let item_id = match id(&raw_item) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let note_id = match id(&raw_note) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    match sqlx::query("DELETE FROM item_notes WHERE id=$1 AND item_id=$2")
        .bind(note_id)
        .bind(item_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}

async fn checks(pool: &PgPool, item_id: Uuid) -> Result<Vec<ItemCheck>, sqlx::Error> {
    sqlx::query_as::<_, ItemCheck>("SELECT id,item_id,body,done,position,created_at,updated_at FROM item_checks WHERE item_id=$1 ORDER BY position,created_at,id")
        .bind(item_id).fetch_all(pool).await
}

async fn list_checks(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    match checks(&state.pool, item_id).await {
        Ok(checks) => Json(checks).into_response(),
        Err(err) => database_error(err),
    }
}

async fn create_check(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    body: Bytes,
) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let input: BodyInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    let body = input.body.trim();
    if body.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: check body");
    }
    let position: i32 = match sqlx::query_scalar(
        "SELECT COALESCE(MAX(position)+1,0) FROM item_checks WHERE item_id=$1",
    )
    .bind(item_id)
    .fetch_one(&state.pool)
    .await
    {
        Ok(position) => position,
        Err(err) => return database_error(err),
    };
    let now = Utc::now();
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let check = match sqlx::query_as::<_, ItemCheck>("INSERT INTO item_checks (id,item_id,body,done,position,created_at,updated_at) VALUES ($1,$2,$3,false,$4,$5,$5) RETURNING id,item_id,body,done,position,created_at,updated_at")
        .bind(Uuid::new_v4()).bind(item_id).bind(body).bind(position).bind(now)
        .fetch_one(&mut *tx).await {
        Ok(check) => check, Err(err) => return database_error(err),
    };
    if let Err(err) = event(&mut tx, item_id, "create", "", &check.body).await {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    (StatusCode::CREATED, Json(check)).into_response()
}

async fn owned_check(pool: &PgPool, item_id: Uuid, check_id: Uuid) -> Result<ItemCheck, Response> {
    sqlx::query_as::<_, ItemCheck>("SELECT id,item_id,body,done,position,created_at,updated_at FROM item_checks WHERE id=$1 AND item_id=$2")
        .bind(check_id).bind(item_id).fetch_one(pool).await.map_err(not_found)
}

async fn patch_check(
    State(state): State<AppState>,
    Path((raw_item, raw_check)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    let item_id = match id(&raw_item) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let check_id = match id(&raw_check) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let input: CheckPatch = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    let mut check = match owned_check(&state.pool, item_id, check_id).await {
        Ok(check) => check,
        Err(response) => return response,
    };
    let old_body = check.body.clone();
    let old_done = check.done;
    if let Some(body) = input.body {
        let body = body.trim();
        if body.is_empty() {
            return error(StatusCode::BAD_REQUEST, "invalid: check body");
        }
        check.body = body.to_owned();
    }
    if let Some(done) = input.done {
        check.done = done;
    }
    let now = Utc::now();
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let updated = match sqlx::query_as::<_, ItemCheck>("UPDATE item_checks SET body=$2,done=$3,updated_at=$4 WHERE id=$1 RETURNING id,item_id,body,done,position,created_at,updated_at")
        .bind(check_id).bind(&check.body).bind(check.done).bind(now).fetch_one(&mut *tx).await {
        Ok(check) => check, Err(err) => return database_error(err),
    };
    if old_body != updated.body {
        if let Err(err) = event(&mut tx, item_id, "body", &old_body, &updated.body).await {
            return database_error(err);
        }
    }
    if old_done != updated.done {
        if let Err(err) = event(
            &mut tx,
            item_id,
            "done",
            &old_done.to_string(),
            &updated.done.to_string(),
        )
        .await
        {
            return database_error(err);
        }
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    Json(updated).into_response()
}

async fn delete_check(
    State(state): State<AppState>,
    Path((raw_item, raw_check)): Path<(String, String)>,
) -> Response {
    let item_id = match id(&raw_item) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let check_id = match id(&raw_check) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    let check = match owned_check(&state.pool, item_id, check_id).await {
        Ok(check) => check,
        Err(response) => return response,
    };
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    if let Err(err) = sqlx::query("DELETE FROM item_checks WHERE id=$1")
        .bind(check_id)
        .execute(&mut *tx)
        .await
    {
        return database_error(err);
    }
    if let Err(err) = event(&mut tx, item_id, "delete", &check.body, "").await {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    StatusCode::NO_CONTENT.into_response()
}

async fn reorder_checks(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    body: Bytes,
) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let input: CheckOrder = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    let existing = match checks(&state.pool, item_id).await {
        Ok(checks) => checks,
        Err(err) => return database_error(err),
    };
    if existing.len() != input.ids.len() {
        return error(StatusCode::BAD_REQUEST, "invalid: check order");
    }
    let expected: HashSet<Uuid> = existing.iter().map(|check| check.id).collect();
    let mut seen = HashSet::new();
    let mut ordered = Vec::new();
    for raw in input.ids {
        let check_id = match id(&raw) {
            Ok(id) => id,
            Err(response) => return response,
        };
        if !expected.contains(&check_id) || !seen.insert(check_id) {
            return error(StatusCode::BAD_REQUEST, "invalid: check order");
        }
        ordered.push(check_id);
    }
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let now = Utc::now();
    for (position, check_id) in ordered.iter().enumerate() {
        if let Err(err) =
            sqlx::query("UPDATE item_checks SET position=$2,updated_at=$3 WHERE id=$1")
                .bind(check_id)
                .bind(position as i32)
                .bind(now)
                .execute(&mut *tx)
                .await
        {
            return database_error(err);
        }
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    match checks(&state.pool, item_id).await {
        Ok(checks) => Json(checks).into_response(),
        Err(err) => database_error(err),
    }
}

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct ItemEvent {
    id: Uuid,
    item_id: Uuid,
    kind: String,
    field: String,
    #[serde(rename = "from")]
    from_value: String,
    #[serde(rename = "to")]
    to_value: String,
    note: String,
    stress: Option<i32>,
    created_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct AnnotateInput {
    kinds: Vec<String>,
    note: String,
    stress: Option<i32>,
}

async fn list_events(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    match sqlx::query_as::<_, ItemEvent>("SELECT id,item_id,kind,field,from_value,to_value,note,stress,created_at FROM item_events WHERE item_id=$1 ORDER BY created_at DESC,id DESC")
        .bind(item_id).fetch_all(&state.pool).await {
        Ok(events) => Json(events).into_response(), Err(err) => database_error(err),
    }
}

async fn annotate_event(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    body: Bytes,
) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let input: AnnotateInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    if input.kinds.is_empty()
        || input.kinds.iter().any(|kind| {
            !matches!(
                kind.as_str(),
                "status" | "field" | "timer_start" | "timer_stop" | "timer_log" | "check"
            )
        })
    {
        return error(StatusCode::BAD_REQUEST, "invalid: item event kind");
    }
    if input.stress.is_some_and(|level| !(1..=5).contains(&level)) {
        return error(StatusCode::BAD_REQUEST, "invalid: stress must be 1-5");
    }
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let event = sqlx::query_as::<_, ItemEvent>("SELECT id,item_id,kind,field,from_value,to_value,note,stress,created_at FROM item_events WHERE item_id=$1 AND kind=ANY($2) ORDER BY created_at DESC,id DESC LIMIT 1")
        .bind(item_id).bind(&input.kinds).fetch_optional(&mut *tx).await;
    let event = match event {
        Ok(event) => event,
        Err(err) => return database_error(err),
    };
    let note = input.note.trim();
    let event = if let Some(event) = event {
        let next_note = if note.is_empty() {
            event.note
        } else {
            note.to_owned()
        };
        let next_stress = input.stress.or(event.stress);
        match sqlx::query_as::<_, ItemEvent>("UPDATE item_events SET note=$2,stress=$3 WHERE id=$1 RETURNING id,item_id,kind,field,from_value,to_value,note,stress,created_at")
            .bind(event.id).bind(next_note).bind(next_stress).fetch_one(&mut *tx).await {
            Ok(event) => Some(event), Err(err) => return database_error(err),
        }
    } else {
        None
    };
    if let Some(level) = input.stress {
        if let Err(err) = sqlx::query("INSERT INTO stress_logs (id,item_id,kind,level,logged_at) VALUES ($1,$2,'stress',$3,$4)")
            .bind(Uuid::new_v4()).bind(item_id).bind(level as i16).bind(Utc::now())
            .execute(&mut *tx).await { return database_error(err); }
        if let Err(err) = sqlx::query("UPDATE items SET stress=$2,updated_at=$3 WHERE id=$1")
            .bind(item_id)
            .bind(level as i16)
            .bind(Utc::now())
            .execute(&mut *tx)
            .await
        {
            return database_error(err);
        }
    }
    if !note.is_empty() {
        if let Err(err) = sqlx::query("INSERT INTO item_notes (id,item_id,body,source_kind,external_id,author_name,url,created_at) VALUES ($1,$2,$3,'manual','','','',$4)")
            .bind(Uuid::new_v4()).bind(item_id).bind(note).bind(Utc::now()).execute(&mut *tx).await {
            return database_error(err);
        }
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    match event {
        Some(event) => Json(event).into_response(),
        None => Json(serde_json::json!({"id":Uuid::nil(),"itemId":Uuid::nil(),"kind":"","field":"","from":"","to":"","note":"","stress":null,"createdAt":"0001-01-01T00:00:00Z"})).into_response(),
    }
}
