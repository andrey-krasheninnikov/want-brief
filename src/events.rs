use crate::http::{database_error, decode, error, id, not_found, AppState};
use crate::projects::{normalize_links, normalize_rels, PersonRel, ProjectLink};
use axum::body::Bytes;
use axum::extract::{Path, Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{
    DateTime, Datelike, Duration, LocalResult, NaiveDate, NaiveDateTime, TimeZone, Timelike, Utc,
};
use chrono_tz::Tz;
use serde::{Deserialize, Serialize};
use sqlx::types::Json as DbJson;
use sqlx::{FromRow, PgPool, Postgres, Transaction};
use std::collections::HashMap;
use uuid::Uuid;

const COLUMNS: &str = "id,title,description,agenda,kind,type,project_id,starts_at,timezone,duration_seconds,recurrence,repeat_until,weekdays,links,meet_url,involvement,active_start_offset,active_end_offset,can_skip,created_at,updated_at";

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Event {
    id: Uuid,
    title: String,
    description: String,
    agenda: String,
    kind: String,
    #[serde(rename = "type")]
    #[sqlx(rename = "type")]
    event_type: String,
    project_id: Option<Uuid>,
    starts_at: DateTime<Utc>,
    timezone: String,
    duration_seconds: i32,
    recurrence: String,
    repeat_until: Option<NaiveDate>,
    weekdays: Vec<i16>,
    links: DbJson<Vec<ProjectLink>>,
    meet_url: String,
    involvement: i32,
    active_start_offset: Option<i32>,
    active_end_offset: Option<i32>,
    can_skip: bool,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
    #[sqlx(skip)]
    people: Vec<PersonRel>,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct EventWrite {
    title: String,
    description: String,
    agenda: String,
    kind: String,
    #[serde(rename = "type")]
    event_type: String,
    project_id: Option<Uuid>,
    starts_at: Option<DateTime<Utc>>,
    timezone: String,
    duration_seconds: i32,
    recurrence: String,
    repeat_until: Option<String>,
    weekdays: Vec<i16>,
    links: Vec<ProjectLink>,
    meet_url: String,
    involvement: Option<i32>,
    active_start_offset: Option<i32>,
    active_end_offset: Option<i32>,
    can_skip: Option<bool>,
    people: Option<Vec<PersonRel>>,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/event-series", get(list_series))
        .route("/api/events", get(list_occurrences).post(create))
        .route("/api/events/{id}/notes", get(list_notes).post(create_note))
        .route(
            "/api/events/{id}/notes/{noteId}",
            axum::routing::delete(delete_note),
        )
        .route(
            "/api/events/{id}/occurrence",
            get(get_occurrence)
                .put(put_occurrence)
                .delete(delete_occurrence),
        )
        .route("/api/events/{id}", get(show).patch(replace).delete(delete))
}

async fn attach_people(pool: &PgPool, event: &mut Event) -> Result<(), sqlx::Error> {
    event.people = sqlx::query_as::<_, PersonRel>(
        "SELECT person_id AS id,comment FROM person_events WHERE event_id=$1 ORDER BY person_id",
    )
    .bind(event.id)
    .fetch_all(pool)
    .await?;
    Ok(())
}

pub(crate) async fn fetch(pool: &PgPool, event_id: Uuid) -> Result<Event, sqlx::Error> {
    let query = format!("SELECT {COLUMNS} FROM events WHERE id=$1");
    let mut event = sqlx::query_as::<_, Event>(&query)
        .bind(event_id)
        .fetch_one(pool)
        .await?;
    attach_people(pool, &mut event).await?;
    Ok(event)
}

pub(crate) async fn all(pool: &PgPool) -> Result<Vec<Event>, sqlx::Error> {
    let query = format!("SELECT {COLUMNS} FROM events ORDER BY starts_at");
    let mut events = sqlx::query_as::<_, Event>(&query).fetch_all(pool).await?;
    for event in &mut events {
        attach_people(pool, event).await?;
    }
    Ok(events)
}

async fn list_series(State(state): State<AppState>) -> Response {
    match all(&state.pool).await {
        Ok(events) => Json(events).into_response(),
        Err(err) => database_error(err),
    }
}

async fn show(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let event_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match fetch(&state.pool, event_id).await {
        Ok(event) => Json(event).into_response(),
        Err(err) => not_found(err),
    }
}

async fn default_timezone(pool: &PgPool) -> Result<String, sqlx::Error> {
    let value: Option<String> =
        sqlx::query_scalar("SELECT data->>'timezone' FROM schedule_settings WHERE id=1")
            .fetch_optional(pool)
            .await?
            .flatten();
    Ok(value
        .filter(|value| !value.trim().is_empty())
        .unwrap_or_else(|| "Europe/Moscow".to_owned()))
}

#[allow(clippy::result_large_err)]
fn validate(
    write: &mut EventWrite,
    old_timezone: Option<&str>,
    default_timezone: &str,
) -> Result<(), Response> {
    write.title = write.title.trim().to_owned();
    if write.title.is_empty() {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: title"));
    }
    if write.kind.is_empty() {
        write.kind = "call".to_owned();
    }
    if !matches!(write.kind.as_str(), "event" | "call") {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: event kind"));
    }
    if write.event_type.is_empty() {
        write.event_type = "sync".to_owned();
    }
    if !matches!(
        write.event_type.as_str(),
        "sync"
            | "grooming"
            | "lesson"
            | "mentorship"
            | "planning"
            | "daily"
            | "retro"
            | "one_on_one"
            | "global"
            | "team_building"
            | "external"
    ) {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: event type"));
    }
    let Some(starts_at) = write.starts_at else {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: startsAt"));
    };
    write.timezone = if write.timezone.trim().is_empty() {
        old_timezone.unwrap_or(default_timezone).to_owned()
    } else {
        write.timezone.trim().to_owned()
    };
    let timezone: Tz = write
        .timezone
        .parse()
        .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid: timezone"))?;
    if write.duration_seconds < 1 {
        write.duration_seconds = 1800;
    }
    if write.recurrence.is_empty() {
        write.recurrence = "once".to_owned();
    }
    if !matches!(write.recurrence.as_str(), "once" | "weekly" | "monthly") {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: event recurrence"));
    }
    write.weekdays.sort_unstable();
    write.weekdays.dedup();
    if write.weekdays.iter().any(|day| !(0..14).contains(day)) {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: weekday"));
    }
    if write.recurrence == "weekly" && write.weekdays.is_empty() {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: weekdays"));
    }
    if write.recurrence != "weekly" {
        write.weekdays.clear();
    }
    if let Some(until) = write
        .repeat_until
        .as_deref()
        .filter(|until| !until.trim().is_empty())
    {
        let parsed = NaiveDate::parse_from_str(until, "%Y-%m-%d")
            .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid: repeatUntil"))?;
        if parsed < starts_at.with_timezone(&timezone).date_naive() {
            return Err(error(StatusCode::BAD_REQUEST, "invalid: repeatUntil"));
        }
        write.repeat_until = Some(parsed.to_string());
    } else {
        write.repeat_until = None;
    }
    let involvement = write.involvement.unwrap_or(5);
    if !(0..=10).contains(&involvement) {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: involvement"));
    }
    write.involvement = Some(involvement);
    match (write.active_start_offset, write.active_end_offset) {
        (None, None) => {}
        (Some(start), Some(end)) if start >= 0 && end > start && end <= write.duration_seconds => {}
        _ => return Err(error(StatusCode::BAD_REQUEST, "invalid: active interval")),
    }
    write.meet_url = write.meet_url.trim().to_owned();
    if !write.meet_url.is_empty() {
        let url = reqwest::Url::parse(&write.meet_url)
            .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid: meet url"))?;
        if !matches!(url.scheme(), "http" | "https") || url.host().is_none() {
            return Err(error(StatusCode::BAD_REQUEST, "invalid: meet url"));
        }
        write.meet_url = url.to_string();
    }
    normalize_links(&mut write.links)?;
    if let Some(people) = write.people.take() {
        write.people = Some(normalize_rels(people)?);
    }
    write.description = write.description.trim().to_owned();
    write.agenda = write.agenda.trim().to_owned();
    Ok(())
}

async fn save_people(
    tx: &mut Transaction<'_, Postgres>,
    event_id: Uuid,
    people: &[PersonRel],
) -> Result<(), sqlx::Error> {
    sqlx::query("DELETE FROM person_events WHERE event_id=$1")
        .bind(event_id)
        .execute(&mut **tx)
        .await?;
    for person in people {
        sqlx::query("INSERT INTO person_events (event_id,person_id,comment) VALUES ($1,$2,$3)")
            .bind(event_id)
            .bind(person.id)
            .bind(&person.comment)
            .execute(&mut **tx)
            .await?;
    }
    Ok(())
}

async fn create(State(state): State<AppState>, body: Bytes) -> Response {
    let mut write: EventWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let default_timezone = match default_timezone(&state.pool).await {
        Ok(timezone) => timezone,
        Err(err) => return database_error(err),
    };
    if let Err(response) = validate(&mut write, None, &default_timezone) {
        return response;
    }
    let event_id = Uuid::new_v4();
    let starts_at = write.starts_at.unwrap();
    let until = write
        .repeat_until
        .as_deref()
        .map(|value| NaiveDate::parse_from_str(value, "%Y-%m-%d").unwrap());
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let result = sqlx::query("INSERT INTO events (id,title,description,agenda,kind,type,project_id,starts_at,ends_at,timezone,duration_seconds,recurrence,repeat_until,weekdays,links,meet_url,involvement,active_start_offset,active_end_offset,can_skip) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)")
        .bind(event_id).bind(&write.title).bind(&write.description).bind(&write.agenda).bind(&write.kind).bind(&write.event_type)
        .bind(write.project_id).bind(starts_at).bind(starts_at + chrono::Duration::seconds(write.duration_seconds.into()))
        .bind(&write.timezone).bind(write.duration_seconds).bind(&write.recurrence).bind(until).bind(&write.weekdays)
        .bind(DbJson(write.links)).bind(&write.meet_url).bind(write.involvement.unwrap()).bind(write.active_start_offset)
        .bind(write.active_end_offset).bind(write.can_skip.unwrap_or(true)).execute(&mut *tx).await;
    if let Err(err) = result {
        return database_error(err);
    }
    if let Err(err) = save_people(&mut tx, event_id, write.people.as_deref().unwrap_or(&[])).await {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    match fetch(&state.pool, event_id).await {
        Ok(event) => (StatusCode::CREATED, Json(event)).into_response(),
        Err(err) => database_error(err),
    }
}

async fn replace(State(state): State<AppState>, Path(raw): Path<String>, body: Bytes) -> Response {
    let event_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let old = match fetch(&state.pool, event_id).await {
        Ok(event) => event,
        Err(err) => return not_found(err),
    };
    let mut write: EventWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    if let Err(response) = validate(&mut write, Some(&old.timezone), &old.timezone) {
        return response;
    }
    let starts_at = write.starts_at.unwrap();
    let until = write
        .repeat_until
        .as_deref()
        .map(|value| NaiveDate::parse_from_str(value, "%Y-%m-%d").unwrap());
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let result = sqlx::query("UPDATE events SET title=$2,description=$3,agenda=$4,kind=$5,type=$6,project_id=$7,starts_at=$8,ends_at=$9,timezone=$10,duration_seconds=$11,recurrence=$12,repeat_until=$13,weekdays=$14,links=$15,meet_url=$16,involvement=$17,active_start_offset=$18,active_end_offset=$19,can_skip=$20,updated_at=$21 WHERE id=$1")
        .bind(event_id).bind(&write.title).bind(&write.description).bind(&write.agenda).bind(&write.kind).bind(&write.event_type)
        .bind(write.project_id).bind(starts_at).bind(starts_at + chrono::Duration::seconds(write.duration_seconds.into()))
        .bind(&write.timezone).bind(write.duration_seconds).bind(&write.recurrence).bind(until).bind(&write.weekdays)
        .bind(DbJson(write.links)).bind(&write.meet_url).bind(write.involvement.unwrap()).bind(write.active_start_offset)
        .bind(write.active_end_offset).bind(write.can_skip.unwrap_or(true)).bind(Utc::now()).execute(&mut *tx).await;
    if let Err(err) = result {
        return database_error(err);
    }
    if let Some(people) = write.people.as_deref() {
        if let Err(err) = save_people(&mut tx, event_id, people).await {
            return database_error(err);
        }
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    match fetch(&state.pool, event_id).await {
        Ok(event) => Json(event).into_response(),
        Err(err) => database_error(err),
    }
}

async fn delete(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let event_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query("DELETE FROM events WHERE id=$1")
        .bind(event_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}

#[derive(Clone, FromRow)]
struct EventOverride {
    series_id: Uuid,
    original_on: NaiveDate,
    starts_at: Option<DateTime<Utc>>,
    duration_seconds: Option<i32>,
    skipped: bool,
}

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct EventOccurrence {
    pub(crate) id: Uuid,
    pub(crate) series_id: Uuid,
    pub(crate) title: String,
    description: String,
    agenda: String,
    kind: String,
    #[serde(rename = "type")]
    event_type: String,
    project_id: Option<Uuid>,
    pub(crate) starts_at: DateTime<Utc>,
    pub(crate) ends_at: DateTime<Utc>,
    duration_seconds: i32,
    recurrence: String,
    pub(crate) original_on: NaiveDate,
    overridden: bool,
    #[serde(skip_serializing_if = "std::ops::Not::not")]
    skipped: bool,
    links: Vec<ProjectLink>,
    meet_url: String,
    involvement: i32,
    pub(crate) active_start_offset: Option<i32>,
    pub(crate) active_end_offset: Option<i32>,
    pub(crate) can_skip: bool,
    pub(crate) people: Vec<PersonRel>,
    created_at: DateTime<Utc>,
}

#[cfg(test)]
pub(crate) fn schedule_fixture(
    title: &str,
    start: DateTime<Utc>,
    minutes: i64,
    can_skip: bool,
    people: &[Uuid],
) -> EventOccurrence {
    EventOccurrence {
        id: Uuid::new_v4(),
        series_id: Uuid::new_v4(),
        title: title.to_owned(),
        description: String::new(),
        agenda: String::new(),
        kind: String::new(),
        event_type: String::new(),
        project_id: None,
        starts_at: start,
        ends_at: start + Duration::minutes(minutes),
        duration_seconds: (minutes * 60) as i32,
        recurrence: String::new(),
        original_on: start.date_naive(),
        overridden: false,
        skipped: false,
        links: Vec::new(),
        meet_url: String::new(),
        involvement: 0,
        active_start_offset: None,
        active_end_offset: None,
        can_skip,
        people: people
            .iter()
            .map(|id| PersonRel {
                id: *id,
                comment: String::new(),
            })
            .collect(),
        created_at: start,
    }
}

#[derive(Default, Deserialize)]
struct OccurrenceFilter {
    from: Option<DateTime<Utc>>,
    to: Option<DateTime<Utc>>,
}

#[derive(Default, Deserialize)]
struct OnFilter {
    on: String,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct OverrideWrite {
    original_on: String,
    starts_at: Option<DateTime<Utc>>,
    duration_seconds: Option<i32>,
    skipped: bool,
}

#[allow(clippy::result_large_err)]
fn date(raw: &str) -> Result<NaiveDate, Response> {
    NaiveDate::parse_from_str(raw, "%Y-%m-%d")
        .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid: date"))
}

fn timezone(event: &Event) -> Tz {
    event.timezone.parse().unwrap_or(chrono_tz::Europe::Moscow)
}

fn local_day(event: &Event) -> NaiveDate {
    event.starts_at.with_timezone(&timezone(event)).date_naive()
}

fn add_month(day: NaiveDate) -> Option<NaiveDate> {
    let (year, month) = if day.month() == 12 {
        (day.year() + 1, 1)
    } else {
        (day.year(), day.month() + 1)
    };
    NaiveDate::from_ymd_opt(year, month, 1)?
        .checked_add_signed(Duration::days(i64::from(day.day() - 1)))
}

fn has_slot(event: &Event, day: NaiveDate) -> bool {
    let start = local_day(event);
    if day < start || event.repeat_until.is_some_and(|until| day > until) {
        return false;
    }
    match event.recurrence.as_str() {
        "once" => day == start,
        "weekly" => {
            let monday = start - Duration::days(start.weekday().num_days_from_monday().into());
            let slot = (day - monday).num_days().rem_euclid(14) as i16;
            event.weekdays.contains(&slot)
        }
        "monthly" => {
            let mut cursor = start;
            while cursor < day {
                let Some(next) = add_month(cursor) else {
                    return false;
                };
                if next <= cursor {
                    return false;
                }
                cursor = next;
            }
            cursor == day
        }
        _ => false,
    }
}

fn local_start(event: &Event, day: NaiveDate) -> DateTime<Utc> {
    let zone = timezone(event);
    let local = event.starts_at.with_timezone(&zone);
    let time =
        chrono::NaiveTime::from_hms_opt(local.hour(), local.minute(), local.second()).unwrap();
    let mut naive = NaiveDateTime::new(day, time);
    for _ in 0..10_800 {
        match zone.from_local_datetime(&naive) {
            LocalResult::Single(start) => return start.with_timezone(&Utc),
            LocalResult::Ambiguous(first, second) => return first.min(second).with_timezone(&Utc),
            LocalResult::None => naive += Duration::seconds(1),
        }
    }
    event.starts_at
}

fn occurrence(
    event: &Event,
    day: NaiveDate,
    override_row: Option<&EventOverride>,
) -> EventOccurrence {
    let start = if event.recurrence == "once" {
        event.starts_at
    } else {
        local_start(event, day)
    };
    let start = override_row.and_then(|row| row.starts_at).unwrap_or(start);
    let duration = override_row
        .and_then(|row| row.duration_seconds)
        .unwrap_or(event.duration_seconds);
    EventOccurrence {
        id: event.id,
        series_id: event.id,
        title: event.title.clone(),
        description: event.description.clone(),
        agenda: event.agenda.clone(),
        kind: event.kind.clone(),
        event_type: event.event_type.clone(),
        project_id: event.project_id,
        starts_at: start,
        ends_at: start + Duration::seconds(duration.into()),
        duration_seconds: duration,
        recurrence: event.recurrence.clone(),
        original_on: day,
        overridden: override_row.is_some(),
        skipped: override_row.is_some_and(|row| row.skipped),
        links: event.links.0.clone(),
        meet_url: event.meet_url.clone(),
        involvement: event.involvement,
        active_start_offset: event.active_start_offset,
        active_end_offset: event.active_end_offset,
        can_skip: event.can_skip,
        people: event.people.clone(),
        created_at: event.created_at,
    }
}

async fn overrides(pool: &PgPool, ids: &[Uuid]) -> Result<Vec<EventOverride>, sqlx::Error> {
    sqlx::query_as("SELECT series_id,original_on,starts_at,duration_seconds,skipped FROM event_overrides WHERE series_id=ANY($1)")
        .bind(ids).fetch_all(pool).await
}

fn range(filter: OccurrenceFilter) -> (DateTime<Utc>, DateTime<Utc>) {
    let from = filter.from.unwrap_or_else(Utc::now);
    let to = filter
        .to
        .filter(|to| *to >= from)
        .unwrap_or(from + Duration::days(56));
    (from, to.min(from + Duration::days(56)))
}

async fn list_occurrences(
    State(state): State<AppState>,
    Query(filter): Query<OccurrenceFilter>,
) -> Response {
    let (from, to) = range(filter);
    match occurrences(&state.pool, from, to).await {
        Ok(result) => Json(result).into_response(),
        Err(err) => database_error(err),
    }
}

pub(crate) async fn occurrences(
    pool: &PgPool,
    from: DateTime<Utc>,
    to: DateTime<Utc>,
) -> Result<Vec<EventOccurrence>, sqlx::Error> {
    let events = all(pool).await?;
    let ids: Vec<Uuid> = events.iter().map(|event| event.id).collect();
    let rows = overrides(pool, &ids).await?;
    let mut by_override: HashMap<(Uuid, NaiveDate), EventOverride> = rows
        .into_iter()
        .map(|row| ((row.series_id, row.original_on), row))
        .collect();
    let mut result = Vec::new();
    for event in &events {
        let zone = timezone(event);
        let mut day = from.with_timezone(&zone).date_naive();
        let end_day = to
            .with_timezone(&zone)
            .date_naive()
            .succ_opt()
            .unwrap_or(day);
        let start_day = local_day(event);
        if day < start_day {
            day = start_day;
        }
        while day <= end_day {
            if has_slot(event, day) {
                let row = by_override.remove(&(event.id, day));
                let entry = occurrence(event, day, row.as_ref());
                if !entry.skipped && entry.starts_at >= from && entry.starts_at < to {
                    result.push(entry);
                }
            }
            let Some(next) = day.succ_opt() else {
                break;
            };
            day = next;
        }
        for ((series_id, on), row) in by_override
            .iter()
            .filter(|((series_id, _), _)| *series_id == event.id)
        {
            if *series_id == event.id && has_slot(event, *on) && !row.skipped {
                let entry = occurrence(event, *on, Some(row));
                if entry.starts_at >= from && entry.starts_at < to {
                    result.push(entry);
                }
            }
        }
    }
    result.sort_by(|left, right| {
        left.starts_at
            .cmp(&right.starts_at)
            .then_with(|| left.series_id.cmp(&right.series_id))
    });
    Ok(result)
}

async fn get_occurrence(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    Query(filter): Query<OnFilter>,
) -> Response {
    let event_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let on = match date(&filter.on) {
        Ok(on) => on,
        Err(response) => return response,
    };
    let event = match fetch(&state.pool, event_id).await {
        Ok(event) => event,
        Err(err) => return not_found(err),
    };
    if !has_slot(&event, on) {
        return error(StatusCode::NOT_FOUND, "not found");
    }
    let row: Option<EventOverride> = match sqlx::query_as("SELECT series_id,original_on,starts_at,duration_seconds,skipped FROM event_overrides WHERE series_id=$1 AND original_on=$2")
        .bind(event_id).bind(on).fetch_optional(&state.pool).await { Ok(row) => row, Err(err) => return database_error(err) };
    Json(occurrence(&event, on, row.as_ref())).into_response()
}

async fn put_occurrence(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    body: Bytes,
) -> Response {
    let event_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let write: OverrideWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let on = match date(&write.original_on) {
        Ok(on) => on,
        Err(response) => return response,
    };
    let event = match fetch(&state.pool, event_id).await {
        Ok(event) => event,
        Err(err) => return not_found(err),
    };
    if !has_slot(&event, on) {
        return error(StatusCode::NOT_FOUND, "not found");
    }
    if write.duration_seconds.is_some_and(|duration| duration < 1)
        || (!write.skipped && write.starts_at.is_none() && write.duration_seconds.is_none())
    {
        return error(StatusCode::BAD_REQUEST, "invalid: occurrence override");
    }
    let row: EventOverride = match sqlx::query_as("INSERT INTO event_overrides (series_id,original_on,starts_at,duration_seconds,skipped) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (series_id,original_on) DO UPDATE SET starts_at=EXCLUDED.starts_at,duration_seconds=EXCLUDED.duration_seconds,skipped=EXCLUDED.skipped RETURNING series_id,original_on,starts_at,duration_seconds,skipped")
        .bind(event_id).bind(on).bind(write.starts_at).bind(write.duration_seconds).bind(write.skipped)
        .fetch_one(&state.pool).await { Ok(row) => row, Err(err) => return database_error(err) };
    Json(occurrence(&event, on, Some(&row))).into_response()
}

async fn delete_occurrence(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    Query(filter): Query<OnFilter>,
) -> Response {
    let event_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let on = match date(&filter.on) {
        Ok(on) => on,
        Err(response) => return response,
    };
    if let Err(err) = fetch(&state.pool, event_id).await {
        return not_found(err);
    }
    match sqlx::query("DELETE FROM event_overrides WHERE series_id=$1 AND original_on=$2")
        .bind(event_id)
        .bind(on)
        .execute(&state.pool)
        .await
    {
        Ok(_) => StatusCode::NO_CONTENT.into_response(),
        Err(err) => database_error(err),
    }
}

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct EventNote {
    id: Uuid,
    series_id: Uuid,
    original_on: NaiveDate,
    body: String,
    created_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct NoteWrite {
    body: String,
    original_on: String,
}

async fn list_notes(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    Query(filter): Query<OnFilter>,
) -> Response {
    let event_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(err) = fetch(&state.pool, event_id).await {
        return not_found(err);
    }
    let on = if filter.on.trim().is_empty() {
        None
    } else {
        match date(filter.on.trim()) {
            Ok(on) => Some(on),
            Err(response) => return response,
        }
    };
    match sqlx::query_as::<_, EventNote>("SELECT id,series_id,original_on,body,created_at FROM event_notes WHERE series_id=$1 AND ($2::date IS NULL OR original_on=$2) ORDER BY created_at DESC")
        .bind(event_id).bind(on).fetch_all(&state.pool).await {
        Ok(notes) => Json(notes).into_response(),
        Err(err) => database_error(err),
    }
}

async fn create_note(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    body: Bytes,
) -> Response {
    let event_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let input: NoteWrite = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    let on = match date(&input.original_on) {
        Ok(on) => on,
        Err(response) => return response,
    };
    let event = match fetch(&state.pool, event_id).await {
        Ok(event) => event,
        Err(err) => return not_found(err),
    };
    if !has_slot(&event, on) {
        return error(StatusCode::BAD_REQUEST, "invalid: originalOn");
    }
    let body = input.body.trim();
    if body.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: note body");
    }
    match sqlx::query_as::<_, EventNote>("INSERT INTO event_notes (id,series_id,original_on,body,created_at) VALUES ($1,$2,$3,$4,$5) RETURNING id,series_id,original_on,body,created_at")
        .bind(Uuid::new_v4()).bind(event_id).bind(on).bind(body).bind(Utc::now())
        .fetch_one(&state.pool).await {
        Ok(note) => (StatusCode::CREATED,Json(note)).into_response(),
        Err(err) => database_error(err),
    }
}

async fn delete_note(
    State(state): State<AppState>,
    Path((raw, note_raw)): Path<(String, String)>,
) -> Response {
    let event_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let note_id = match id(&note_raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(err) = fetch(&state.pool, event_id).await {
        return not_found(err);
    }
    match sqlx::query("DELETE FROM event_notes WHERE id=$1 AND series_id=$2")
        .bind(note_id)
        .bind(event_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}
