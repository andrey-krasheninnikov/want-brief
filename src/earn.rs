use crate::http::{database_error, decode, error, AppState};
use axum::body::Bytes;
use axum::extract::{Path, Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::{get, put};
use axum::{Json, Router};
use chrono::{DateTime, NaiveDate, Utc};
use serde::{Deserialize, Serialize};
use sqlx::FromRow;

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct EarnDay {
    day: String,
    usd: f64,
    rub: f64,
    rate: f64,
    updated_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct EarnInput {
    usd: f64,
    rub: f64,
    rate: f64,
}

#[derive(Deserialize)]
struct EarnQuery {
    from: String,
    to: String,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/earn/days", get(list))
        .route("/api/earn/days/{day}", put(upsert))
}

#[allow(clippy::result_large_err)]
fn date(raw: &str) -> Result<String, Response> {
    let day = raw.trim();
    if NaiveDate::parse_from_str(day, "%Y-%m-%d").is_err() || day.len() != 10 {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: date"));
    }
    Ok(day.to_owned())
}

async fn list(State(state): State<AppState>, Query(query): Query<EarnQuery>) -> Response {
    let mut from = match date(&query.from) {
        Ok(day) => day,
        Err(response) => return response,
    };
    let mut to = match date(&query.to) {
        Ok(day) => day,
        Err(response) => return response,
    };
    if from > to {
        std::mem::swap(&mut from, &mut to);
    }
    match sqlx::query_as::<_, EarnDay>("SELECT day::text AS day,usd::double precision AS usd,rub::double precision AS rub,rate::double precision AS rate,updated_at FROM earn_days WHERE day >= $1::date AND day <= $2::date ORDER BY day")
        .bind(from)
        .bind(to)
        .fetch_all(&state.pool)
        .await
    {
        Ok(days) => Json(days).into_response(),
        Err(err) => database_error(err),
    }
}

async fn upsert(State(state): State<AppState>, Path(raw): Path<String>, body: Bytes) -> Response {
    let day = match date(&raw) {
        Ok(day) => day,
        Err(response) => return response,
    };
    let input: EarnInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    let rate = if input.rate <= 0.0 { 85.0 } else { input.rate };
    if !input.usd.is_finite()
        || !input.rub.is_finite()
        || !rate.is_finite()
        || input.usd < 0.0
        || input.rub < 0.0
    {
        return error(StatusCode::BAD_REQUEST, "invalid: earn");
    }
    match sqlx::query_as::<_, EarnDay>("INSERT INTO earn_days (day,usd,rub,rate,updated_at) VALUES ($1::date,$2::double precision::numeric,$3::double precision::numeric,$4::double precision::numeric,$5) ON CONFLICT (day) DO UPDATE SET usd=EXCLUDED.usd,rub=EXCLUDED.rub,rate=EXCLUDED.rate,updated_at=EXCLUDED.updated_at RETURNING day::text AS day,usd::double precision AS usd,rub::double precision AS rub,rate::double precision AS rate,updated_at")
        .bind(day)
        .bind(input.usd)
        .bind(input.rub)
        .bind(rate)
        .bind(Utc::now())
        .fetch_one(&state.pool)
        .await
    {
        Ok(day) => Json(day).into_response(),
        Err(err) => database_error(err),
    }
}
