use crate::http::{database_error, decode, error, id, not_found, AppState};
use crate::{people, schedule_settings};
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::post;
use axum::{Json, Router};
use chrono::{DateTime, NaiveDate, Utc};
use chrono_tz::Tz;
use serde::Deserialize;
use sqlx::{FromRow, Postgres, Transaction};
use uuid::Uuid;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Open {
    other_id: Option<String>,
    kind: String,
    #[serde(default)]
    comment: String,
    started_on: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Change {
    #[serde(default)]
    kind: String,
    comment: Option<String>,
    #[serde(default)]
    action_comment: String,
    started_on: Option<String>,
    changed_on: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct End {
    ended_on: Option<String>,
    #[serde(default)]
    comment: String,
}

#[derive(FromRow)]
struct BondRow {
    kind: String,
    comment: String,
    started_on: NaiveDate,
    changed_on: NaiveDate,
    ended_on: Option<NaiveDate>,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/people/{id}/bonds", post(open))
        .route(
            "/api/people/{id}/bonds/{bondId}",
            axum::routing::patch(change),
        )
        .route("/api/people/{id}/bonds/{bondId}/end", post(end))
}

#[allow(clippy::result_large_err)]
fn date(raw: &str) -> Result<NaiveDate, Response> {
    NaiveDate::parse_from_str(raw, "%Y-%m-%d")
        .or_else(|_| DateTime::parse_from_rfc3339(raw).map(|at| at.date_naive()))
        .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))
}

fn valid_kind(kind: &str) -> bool {
    matches!(
        kind,
        "acquaintance" | "comrade" | "friend" | "relative" | "spouse" | "adversary" | "other"
    )
}

async fn today(state: &AppState) -> Result<(NaiveDate, Tz), Response> {
    let settings = schedule_settings::settings(state).await?;
    let zone: Tz = settings
        .timezone
        .parse()
        .unwrap_or(chrono_tz::Europe::Moscow);
    Ok((Utc::now().with_timezone(&zone).date_naive(), zone))
}

#[allow(clippy::too_many_arguments)]
async fn snapshot(
    tx: &mut Transaction<'_, Postgres>,
    bond: Uuid,
    action: &str,
    kind: &str,
    comment: &str,
    started: NaiveDate,
    changed: NaiveDate,
    ended: Option<NaiveDate>,
) -> Result<(), sqlx::Error> {
    sqlx::query("INSERT INTO person_bond_events (id,bond_id,action,kind,comment,started_on,changed_on,ended_on,at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)")
        .bind(Uuid::new_v4()).bind(bond).bind(action).bind(kind).bind(comment.trim()).bind(started).bind(changed).bind(ended).bind(Utc::now()).execute(&mut **tx).await?;
    Ok(())
}

async fn response(state: &AppState, person: Uuid, bond: Uuid, zone: Tz, created: bool) -> Response {
    match people::fetch(&state.pool, person, zone).await {
        Ok(person) => match person
            .bonds
            .into_iter()
            .find(|entry| entry["id"] == bond.to_string())
        {
            Some(entry) if created => (StatusCode::CREATED, Json(entry)).into_response(),
            Some(entry) => Json(entry).into_response(),
            None => error(StatusCode::NOT_FOUND, "not found"),
        },
        Err(err) => not_found(err),
    }
}

async fn open(State(state): State<AppState>, Path(raw): Path<String>, body: Bytes) -> Response {
    let person = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let write: Open = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let kind = write.kind.trim();
    if !valid_kind(kind) {
        return error(StatusCode::BAD_REQUEST, "invalid: bond kind");
    }
    let other = match write.other_id.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => match id(raw) {
            Ok(id) => Some(id),
            Err(response) => return response,
        },
        None => None,
    };
    if other == Some(person) {
        return error(StatusCode::BAD_REQUEST, "invalid: bond self");
    }
    let (today, zone) = match today(&state).await {
        Ok(value) => value,
        Err(response) => return response,
    };
    let started = match write.started_on.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => match date(raw) {
            Ok(day) => day,
            Err(response) => return response,
        },
        None => today,
    };
    let (person_a, person_b) = match other {
        Some(other) if person.to_string() < other.to_string() => (Some(person), other),
        Some(other) => (Some(other), person),
        None => (None, person),
    };
    let bond = Uuid::new_v4();
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    if let Err(err) = sqlx::query("INSERT INTO person_bonds (id,person_a_id,person_b_id,kind,comment,started_on,changed_on,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$7)")
        .bind(bond).bind(person_a).bind(person_b).bind(kind).bind(write.comment.trim()).bind(started).bind(Utc::now()).execute(&mut *tx).await {
        return if err.as_database_error().is_some_and(|db| db.code().as_deref()==Some("23503")) { error(StatusCode::NOT_FOUND,"not found") } else { database_error(err) };
    }
    if let Err(err) = snapshot(
        &mut tx,
        bond,
        "open",
        kind,
        &write.comment,
        started,
        started,
        None,
    )
    .await
    {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    response(&state, person, bond, zone, true).await
}

async fn load(
    tx: &mut Transaction<'_, Postgres>,
    person: Uuid,
    bond: Uuid,
) -> Result<BondRow, sqlx::Error> {
    sqlx::query_as("SELECT kind,comment,started_on,changed_on,ended_on FROM person_bonds WHERE id=$1 AND (person_a_id=$2 OR person_b_id=$2) FOR UPDATE")
        .bind(bond).bind(person).fetch_one(&mut **tx).await
}

async fn change(
    State(state): State<AppState>,
    Path((raw, bond)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    let person = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let bond = match id(&bond) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let write: Change = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let (today, zone) = match today(&state).await {
        Ok(value) => value,
        Err(response) => return response,
    };
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let old = match load(&mut tx, person, bond).await {
        Ok(value) => value,
        Err(err) => return not_found(err),
    };
    if old.ended_on.is_some() {
        return error(StatusCode::BAD_REQUEST, "invalid: bond ended");
    }
    let kind = if write.kind.trim().is_empty() {
        old.kind.as_str()
    } else {
        write.kind.trim()
    };
    if !valid_kind(kind) {
        return error(StatusCode::BAD_REQUEST, "invalid: bond kind");
    }
    let started = match write.started_on.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => match date(raw) {
            Ok(day) => day,
            Err(response) => return response,
        },
        None => old.started_on,
    };
    let changed = match write.changed_on.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => match date(raw) {
            Ok(day) => day,
            Err(response) => return response,
        },
        None => today,
    };
    if changed < started {
        return error(StatusCode::BAD_REQUEST, "invalid: bond dates");
    }
    let comment = write
        .comment
        .as_deref()
        .map(str::trim)
        .unwrap_or(&old.comment);
    if let Err(err) = sqlx::query("UPDATE person_bonds SET kind=$2,comment=$3,started_on=$4,changed_on=$5,updated_at=$6 WHERE id=$1")
        .bind(bond).bind(kind).bind(comment).bind(started).bind(changed).bind(Utc::now()).execute(&mut *tx).await { return database_error(err); }
    if let Err(err) = snapshot(
        &mut tx,
        bond,
        "change",
        kind,
        &write.action_comment,
        started,
        changed,
        None,
    )
    .await
    {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    response(&state, person, bond, zone, false).await
}

async fn end(
    State(state): State<AppState>,
    Path((raw, bond)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    let person = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let bond = match id(&bond) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let write: End = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let (today, zone) = match today(&state).await {
        Ok(value) => value,
        Err(response) => return response,
    };
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let old = match load(&mut tx, person, bond).await {
        Ok(value) => value,
        Err(err) => return not_found(err),
    };
    if old.ended_on.is_some() {
        return error(StatusCode::BAD_REQUEST, "invalid: bond ended");
    }
    let ended = match write.ended_on.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => match date(raw) {
            Ok(day) => day,
            Err(response) => return response,
        },
        None => today,
    };
    if ended < old.started_on {
        return error(StatusCode::BAD_REQUEST, "invalid: bond dates");
    }
    if let Err(err) = sqlx::query("UPDATE person_bonds SET ended_on=$2,updated_at=$3 WHERE id=$1")
        .bind(bond)
        .bind(ended)
        .bind(Utc::now())
        .execute(&mut *tx)
        .await
    {
        return database_error(err);
    }
    if let Err(err) = snapshot(
        &mut tx,
        bond,
        "end",
        &old.kind,
        &write.comment,
        old.started_on,
        old.changed_on,
        Some(ended),
    )
    .await
    {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    response(&state, person, bond, zone, false).await
}
