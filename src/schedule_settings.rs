use crate::http::{database_error, decode, error, not_found, AppState};
use axum::body::Bytes;
use axum::extract::{Path, Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, NaiveDate, Utc};
use chrono_tz::Tz;
use serde::{Deserialize, Serialize};
use sqlx::types::Json as DbJson;
use sqlx::FromRow;

#[derive(Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct BreakWindow {
    pub(crate) start_min: i32,
    pub(crate) end_min: i32,
}

#[derive(Clone, Serialize, Deserialize)]
#[serde(default, rename_all = "camelCase")]
pub(crate) struct Settings {
    pub(crate) timezone: String,
    pub(crate) work_start_min: i32,
    pub(crate) work_end_min: i32,
    pub(crate) workdays: Vec<i32>,
    #[serde(rename = "break")]
    pub(crate) break_window: Option<BreakWindow>,
    pub(crate) meeting_buffer_min: i32,
    pub(crate) daily_focus_min: i32,
    pub(crate) max_tasks_per_day: i32,
    pub(crate) min_slice_min: i32,
    pub(crate) max_slice_min: i32,
    pub(crate) slice_break_min: i32,
    pub(crate) grid_min: i32,
    pub(crate) estimate_buffer: bool,
    pub(crate) estimate_max_k: f64,
    pub(crate) oldest_first: bool,
    pub(crate) stress_shift_hours: i32,
    pub(crate) target_lead_workdays: i32,
    pub(crate) soft_busy: bool,
    pub(crate) waiting_tracks: i32,
    pub(crate) followup_ping_min: i32,
    pub(crate) check_windows: Vec<String>,
    pub(crate) energy_aware: bool,
    pub(crate) golden_hours: bool,
    pub(crate) stability_threshold_min: i32,
}

impl Default for Settings {
    fn default() -> Self {
        Self {
            timezone: "Europe/Moscow".to_owned(),
            work_start_min: 600,
            work_end_min: 1140,
            workdays: vec![1, 2, 3, 4, 5],
            break_window: None,
            meeting_buffer_min: 10,
            daily_focus_min: 360,
            max_tasks_per_day: 5,
            min_slice_min: 45,
            max_slice_min: 120,
            slice_break_min: 10,
            grid_min: 5,
            estimate_buffer: true,
            estimate_max_k: 2.0,
            oldest_first: true,
            stress_shift_hours: 6,
            target_lead_workdays: 1,
            soft_busy: true,
            waiting_tracks: 2,
            followup_ping_min: 20,
            check_windows: vec!["10:00".to_owned(), "14:00".to_owned()],
            energy_aware: true,
            golden_hours: true,
            stability_threshold_min: 30,
        }
    }
}

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct DayOverride {
    day: NaiveDate,
    #[sqlx(rename = "off")]
    off: bool,
    work_start_min: Option<i32>,
    work_end_min: Option<i32>,
    note: String,
    updated_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct DayWrite {
    off: bool,
    work_start_min: Option<i32>,
    work_end_min: Option<i32>,
    note: String,
}

#[derive(Default, Deserialize)]
struct DayRange {
    from: String,
    to: String,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route(
            "/api/schedule/settings",
            get(get_settings).put(put_settings),
        )
        .route("/api/schedule/days", get(list_days))
        .route(
            "/api/schedule/days/{date}",
            axum::routing::put(put_day).delete(delete_day),
        )
}

pub(crate) async fn settings(state: &AppState) -> Result<Settings, Response> {
    let row: Option<DbJson<serde_json::Value>> =
        sqlx::query_scalar("SELECT data FROM schedule_settings WHERE id=1")
            .fetch_optional(&state.pool)
            .await
            .map_err(database_error)?;
    match row {
        Some(DbJson(value)) => serde_json::from_value(value)
            .map_err(|_| error(StatusCode::INTERNAL_SERVER_ERROR, "internal error")),
        None => Ok(Settings::default()),
    }
}

fn invalid(name: &str) -> Response {
    error(StatusCode::BAD_REQUEST, format!("invalid: {name}"))
}

#[allow(clippy::result_large_err)]
fn normalize(settings: &mut Settings) -> Result<(), Response> {
    settings.timezone = settings.timezone.trim().to_owned();
    if settings.timezone.parse::<Tz>().is_err() {
        return Err(invalid("timezone"));
    }
    if settings.work_start_min < 0
        || settings.work_end_min > 1440
        || settings.work_start_min >= settings.work_end_min
    {
        return Err(invalid("work window"));
    }
    settings.workdays.sort_unstable();
    settings.workdays.dedup();
    if settings.workdays.is_empty() || settings.workdays.iter().any(|day| !(0..=6).contains(day)) {
        return Err(invalid("workdays"));
    }
    if let Some(window) = &settings.break_window {
        if window.start_min < settings.work_start_min
            || window.end_min > settings.work_end_min
            || window.start_min >= window.end_min
        {
            return Err(invalid("break window"));
        }
    }
    if [
        settings.meeting_buffer_min,
        settings.daily_focus_min,
        settings.max_tasks_per_day,
        settings.min_slice_min,
        settings.max_slice_min,
        settings.slice_break_min,
        settings.stress_shift_hours,
        settings.target_lead_workdays,
        settings.waiting_tracks,
        settings.followup_ping_min,
        settings.stability_threshold_min,
    ]
    .iter()
    .any(|value| *value < 0)
    {
        return Err(invalid("settings"));
    }
    if settings.min_slice_min > 0
        && settings.max_slice_min > 0
        && settings.min_slice_min > settings.max_slice_min
    {
        return Err(invalid("min slice above max slice"));
    }
    if !matches!(settings.grid_min, 1 | 5 | 10 | 15 | 30) {
        return Err(invalid("grid"));
    }
    if !(1.0..=5.0).contains(&settings.estimate_max_k) {
        return Err(invalid("estimate max factor"));
    }
    if settings.waiting_tracks > 5 {
        return Err(invalid("waiting tracks"));
    }
    if settings.check_windows.is_empty() || settings.check_windows.len() > 6 {
        return Err(invalid("check windows"));
    }
    for window in &mut settings.check_windows {
        *window = window.trim().to_owned();
        let Some((hour, minute)) = window.split_once(':') else {
            return Err(invalid("clock"));
        };
        let hour: i32 = hour.parse().map_err(|_| invalid("clock"))?;
        let minute: i32 = minute.parse().map_err(|_| invalid("clock"))?;
        if !(0..=23).contains(&hour)
            || !(0..=59).contains(&minute)
            || hour * 60 + minute < settings.work_start_min
            || hour * 60 + minute >= settings.work_end_min
        {
            return Err(invalid("check window outside work hours"));
        }
    }
    settings.check_windows.sort();
    Ok(())
}

async fn get_settings(State(state): State<AppState>) -> Response {
    match settings(&state).await {
        Ok(settings) => Json(settings).into_response(),
        Err(response) => response,
    }
}

async fn put_settings(State(state): State<AppState>, body: Bytes) -> Response {
    let mut settings: Settings = match decode(&body) {
        Ok(settings) => settings,
        Err(response) => return response,
    };
    if let Err(response) = normalize(&mut settings) {
        return response;
    }
    match sqlx::query("INSERT INTO schedule_settings (id,data,updated_at) VALUES (1,$1,now()) ON CONFLICT (id) DO UPDATE SET data=EXCLUDED.data,updated_at=now()")
        .bind(DbJson(&settings)).execute(&state.pool).await {
        Ok(_) => Json(settings).into_response(),
        Err(err) => database_error(err),
    }
}

#[allow(clippy::result_large_err)]
fn parse_day(raw: &str) -> Result<NaiveDate, Response> {
    NaiveDate::parse_from_str(raw.trim(), "%Y-%m-%d").map_err(|_| invalid("date"))
}

async fn list_days(State(state): State<AppState>, Query(range): Query<DayRange>) -> Response {
    let mut from = match parse_day(&range.from) {
        Ok(day) => day,
        Err(response) => return response,
    };
    let mut to = match parse_day(&range.to) {
        Ok(day) => day,
        Err(response) => return response,
    };
    if from > to {
        std::mem::swap(&mut from, &mut to);
    }
    match sqlx::query_as::<_, DayOverride>("SELECT day,\"off\",work_start_min,work_end_min,note,updated_at FROM schedule_day_overrides WHERE day>=$1 AND day<=$2 ORDER BY day")
        .bind(from).bind(to).fetch_all(&state.pool).await {
        Ok(days) => Json(days).into_response(),
        Err(err) => database_error(err),
    }
}

async fn put_day(State(state): State<AppState>, Path(raw): Path<String>, body: Bytes) -> Response {
    let day = match parse_day(&raw) {
        Ok(day) => day,
        Err(response) => return response,
    };
    let mut write: DayWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    write.note = write.note.trim().to_owned();
    if write.off {
        write.work_start_min = None;
        write.work_end_min = None;
    } else {
        match (write.work_start_min, write.work_end_min) {
            (None, None) if !write.note.is_empty() => {}
            (Some(start), Some(end)) if start >= 0 && end <= 1440 && start < end => {}
            _ => return invalid("day window"),
        }
    }
    match sqlx::query_as::<_, DayOverride>("INSERT INTO schedule_day_overrides (day,\"off\",work_start_min,work_end_min,note,updated_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (day) DO UPDATE SET \"off\"=EXCLUDED.\"off\",work_start_min=EXCLUDED.work_start_min,work_end_min=EXCLUDED.work_end_min,note=EXCLUDED.note,updated_at=EXCLUDED.updated_at RETURNING day,\"off\",work_start_min,work_end_min,note,updated_at")
        .bind(day).bind(write.off).bind(write.work_start_min).bind(write.work_end_min).bind(write.note).bind(Utc::now())
        .fetch_one(&state.pool).await {
        Ok(day) => Json(day).into_response(),
        Err(err) => database_error(err),
    }
}

async fn delete_day(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let day = match parse_day(&raw) {
        Ok(day) => day,
        Err(response) => return response,
    };
    match sqlx::query("DELETE FROM schedule_day_overrides WHERE day=$1")
        .bind(day)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => not_found(sqlx::Error::RowNotFound),
        Err(err) => database_error(err),
    }
}
