use crate::http::{database_error, decode, error, id, not_found, AppState};
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Json, Router};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sqlx::{FromRow, PgPool, Postgres, Transaction};
use uuid::Uuid;

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct Interval {
    id: Uuid,
    item_id: Uuid,
    started_at: DateTime<Utc>,
    ended_at: Option<DateTime<Utc>>,
}

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct StressLog {
    id: Uuid,
    item_id: Option<Uuid>,
    kind: String,
    level: i16,
    logged_at: DateTime<Utc>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct IntervalInput {
    item_id: String,
    started_at: Option<String>,
    ended_at: Option<String>,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct StressInput {
    level: i32,
    item_id: Option<String>,
    at: Option<String>,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct CheckinInput {
    kind: String,
    level: i32,
}

#[derive(Serialize)]
struct LatestCheckins {
    stress: i32,
    focus: i32,
    energy: i32,
    interest: i32,
    happiness: i32,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/intervals", get(list_intervals).post(start_interval))
        .route("/api/intervals/{id}/stop", post(stop_interval))
        .route("/api/stress", post(create_stress))
        .route("/api/checkins", post(create_checkin))
        .route("/api/checkins/latest", get(latest_checkins))
}

async fn item_exists(pool: &PgPool, item_id: Uuid) -> Result<(), Response> {
    sqlx::query_scalar::<_, Uuid>("SELECT id FROM items WHERE id=$1 AND deleted_at IS NULL")
        .bind(item_id)
        .fetch_one(pool)
        .await
        .map(|_| ())
        .map_err(not_found)
}

#[allow(clippy::result_large_err)]
fn timestamp(value: &str) -> Result<DateTime<Utc>, Response> {
    DateTime::parse_from_rfc3339(value)
        .map(|value| value.with_timezone(&Utc))
        .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))
}

async fn record_timer_event(
    tx: &mut Transaction<'_, Postgres>,
    item_id: Uuid,
    kind: &str,
    duration: &str,
) -> Result<(), sqlx::Error> {
    sqlx::query("INSERT INTO item_events (id,item_id,kind,field,from_value,to_value,note,created_at) VALUES ($1,$2,$3,'','',$4,'',$5)")
        .bind(Uuid::new_v4())
        .bind(item_id)
        .bind(kind)
        .bind(duration)
        .bind(Utc::now())
        .execute(&mut **tx)
        .await?;
    Ok(())
}

async fn list_intervals(State(state): State<AppState>) -> Response {
    match sqlx::query_as::<_, Interval>("SELECT id,item_id,started_at,ended_at FROM time_intervals WHERE ended_at IS NULL ORDER BY started_at")
        .fetch_all(&state.pool)
        .await
    {
        Ok(intervals) => Json(intervals).into_response(),
        Err(err) => database_error(err),
    }
}

async fn start_interval(State(state): State<AppState>, body: Bytes) -> Response {
    let input: IntervalInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    let item_id = match id(&input.item_id) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let has_start = input
        .started_at
        .as_deref()
        .is_some_and(|value| !value.is_empty());
    let has_end = input
        .ended_at
        .as_deref()
        .is_some_and(|value| !value.is_empty());
    if has_start != has_end {
        return error(StatusCode::BAD_REQUEST, "invalid");
    }
    let started_at = if has_start {
        match timestamp(input.started_at.as_deref().unwrap_or_default()) {
            Ok(value) => value,
            Err(response) => return response,
        }
    } else {
        Utc::now()
    };
    let ended_at = if has_end {
        match timestamp(input.ended_at.as_deref().unwrap_or_default()) {
            Ok(value) if value > started_at => Some(value),
            Ok(_) => return error(StatusCode::BAD_REQUEST, "invalid: end before start"),
            Err(response) => return response,
        }
    } else {
        None
    };
    if let Err(response) = item_exists(&state.pool, item_id).await {
        return response;
    }
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let result = sqlx::query_as::<_, Interval>("INSERT INTO time_intervals (id,item_id,started_at,ended_at) VALUES ($1,$2,$3,$4) RETURNING id,item_id,started_at,ended_at")
        .bind(Uuid::new_v4())
        .bind(item_id)
        .bind(started_at)
        .bind(ended_at)
        .fetch_one(&mut *tx)
        .await;
    let interval = match result {
        Ok(interval) => interval,
        Err(err) => return database_error(err),
    };
    let duration = ended_at
        .map(|end| (end - started_at).num_seconds().to_string())
        .unwrap_or_default();
    let kind = if ended_at.is_some() {
        "timer_log"
    } else {
        "timer_start"
    };
    if let Err(err) = record_timer_event(&mut tx, item_id, kind, &duration).await {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    (StatusCode::CREATED, Json(interval)).into_response()
}

async fn stop_interval(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let interval_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let interval = match sqlx::query_as::<_, Interval>(
        "SELECT id,item_id,started_at,ended_at FROM time_intervals WHERE id=$1 FOR UPDATE",
    )
    .bind(interval_id)
    .fetch_one(&mut *tx)
    .await
    {
        Ok(interval) => interval,
        Err(err) => return not_found(err),
    };
    if interval.ended_at.is_some() {
        return error(StatusCode::CONFLICT, "conflict: interval already stopped");
    }
    let ended_at = Utc::now();
    if ended_at < interval.started_at {
        return error(StatusCode::BAD_REQUEST, "invalid: end before start");
    }
    let updated = match sqlx::query_as::<_, Interval>("UPDATE time_intervals SET ended_at=$2 WHERE id=$1 RETURNING id,item_id,started_at,ended_at")
        .bind(interval_id)
        .bind(ended_at)
        .fetch_one(&mut *tx)
        .await
    {
        Ok(interval) => interval,
        Err(err) => return database_error(err),
    };
    let duration = (ended_at - interval.started_at).num_seconds().to_string();
    if let Err(err) = record_timer_event(&mut tx, interval.item_id, "timer_stop", &duration).await {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    Json(updated).into_response()
}

async fn create_log(
    pool: &PgPool,
    kind: &str,
    level: i32,
    item_id: Option<Uuid>,
    at: DateTime<Utc>,
) -> Result<StressLog, Response> {
    if !(1..=5).contains(&level) {
        return Err(error(
            StatusCode::BAD_REQUEST,
            "invalid: stress must be 1-5",
        ));
    }
    let log = sqlx::query_as::<_, StressLog>("INSERT INTO stress_logs (id,item_id,kind,level,logged_at) VALUES ($1,$2,$3,$4,$5) RETURNING id,item_id,kind,level,logged_at")
        .bind(Uuid::new_v4())
        .bind(item_id)
        .bind(kind)
        .bind(level as i16)
        .bind(at)
        .fetch_one(pool)
        .await
        .map_err(database_error)?;
    if kind == "stress" {
        if let Some(item_id) = item_id {
            let _ = sqlx::query(
                "UPDATE items SET stress=$2,updated_at=$3 WHERE id=$1 AND deleted_at IS NULL",
            )
            .bind(item_id)
            .bind(level)
            .bind(Utc::now())
            .execute(pool)
            .await;
        }
    }
    Ok(log)
}

async fn create_stress(State(state): State<AppState>, body: Bytes) -> Response {
    let input: StressInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    let item_id = match input.item_id.as_deref().filter(|value| !value.is_empty()) {
        Some(raw) => match id(raw) {
            Ok(id) => Some(id),
            Err(response) => return response,
        },
        None => None,
    };
    let at = match input.at.as_deref().filter(|value| !value.is_empty()) {
        Some(raw) => match timestamp(raw) {
            Ok(at) => at,
            Err(response) => return response,
        },
        None => Utc::now(),
    };
    match create_log(&state.pool, "stress", input.level, item_id, at).await {
        Ok(log) => (StatusCode::CREATED, Json(log)).into_response(),
        Err(response) => response,
    }
}

async fn create_checkin(State(state): State<AppState>, body: Bytes) -> Response {
    let input: CheckinInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    if !matches!(
        input.kind.as_str(),
        "stress" | "focus" | "energy" | "interest" | "happiness"
    ) {
        return error(StatusCode::BAD_REQUEST, "invalid: checkin kind");
    }
    match create_log(&state.pool, &input.kind, input.level, None, Utc::now()).await {
        Ok(log) => (StatusCode::CREATED, Json(log)).into_response(),
        Err(response) => response,
    }
}

async fn latest_checkins(State(state): State<AppState>) -> Response {
    let logs = match sqlx::query_as::<_, StressLog>("SELECT DISTINCT ON (kind) id,item_id,kind,level,logged_at FROM stress_logs ORDER BY kind,logged_at DESC")
        .fetch_all(&state.pool)
        .await
    {
        Ok(logs) => logs,
        Err(err) => return database_error(err),
    };
    let mut latest = LatestCheckins {
        stress: 3,
        focus: 3,
        energy: 3,
        interest: 3,
        happiness: 3,
    };
    for log in logs {
        match log.kind.as_str() {
            "stress" => latest.stress = log.level.into(),
            "focus" => latest.focus = log.level.into(),
            "energy" => latest.energy = log.level.into(),
            "interest" => latest.interest = log.level.into(),
            "happiness" => latest.happiness = log.level.into(),
            _ => {}
        }
    }
    Json(latest).into_response()
}
