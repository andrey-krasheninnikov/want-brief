use crate::http::{database_error, decode, error, id, not_found, AppState};
use crate::projects::{normalize_rels, PersonRel};
use crate::schedule_settings;
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, Datelike, NaiveDate, Utc};
use chrono_tz::Tz;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sqlx::{FromRow, PgPool, Postgres, Transaction};
use std::collections::HashSet;
use uuid::Uuid;

const COLUMNS: &str =
    "id,name,born_on::timestamp AT TIME ZONE 'UTC' AS born_on,created_at,updated_at";

#[derive(FromRow)]
struct PersonRow {
    id: Uuid,
    name: String,
    born_on: Option<DateTime<Utc>>,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Person {
    id: Uuid,
    name: String,
    born_on: Option<DateTime<Utc>>,
    age: Option<i32>,
    projects: Vec<PersonRel>,
    events: Vec<PersonRel>,
    companies: Vec<PersonRel>,
    item_ids: Vec<Uuid>,
    contacts: Vec<Value>,
    addresses: Vec<Value>,
    sites: Vec<Value>,
    pub(crate) bonds: Vec<Value>,
    professions: Vec<Value>,
    absences: Vec<Value>,
    me_bond: Option<Value>,
    last_note_at: Option<DateTime<Utc>>,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct PersonWrite {
    name: String,
    born_on: Option<String>,
    age_years: Option<i32>,
    projects: Vec<PersonRel>,
    events: Vec<PersonRel>,
    companies: Vec<PersonRel>,
    item_ids: Vec<String>,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/people", get(list).post(create))
        .route("/api/people/{id}", get(show).patch(replace).delete(delete))
        .route("/api/people/{id}/notes", get(list_notes).post(create_note))
        .route(
            "/api/people/{id}/notes/{noteId}",
            axum::routing::patch(replace_note).delete(delete_note),
        )
}

#[derive(Deserialize)]
struct NoteWrite {
    body: String,
}

async fn note(pool: &PgPool, person_id: Uuid, note_id: Uuid) -> Result<Value, sqlx::Error> {
    sqlx::query_scalar("SELECT to_jsonb(n) FROM (SELECT id,person_id AS \"personId\",body,created_at AS \"createdAt\" FROM person_notes WHERE person_id=$1 AND id=$2) n")
        .bind(person_id).bind(note_id).fetch_one(pool).await
}

async fn list_notes(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let person_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let exists: bool = match sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM people WHERE id=$1)")
        .bind(person_id)
        .fetch_one(&state.pool)
        .await
    {
        Ok(value) => value,
        Err(err) => return database_error(err),
    };
    if !exists {
        return error(StatusCode::NOT_FOUND, "not found");
    }
    match rows(&state.pool, "SELECT COALESCE(jsonb_agg(to_jsonb(n) ORDER BY n.\"createdAt\" DESC),'[]'::jsonb) FROM (SELECT id,person_id AS \"personId\",body,created_at AS \"createdAt\" FROM person_notes WHERE person_id=$1) n", person_id).await {
        Ok(notes) => Json(notes).into_response(), Err(err) => database_error(err)
    }
}

async fn create_note(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    body: Bytes,
) -> Response {
    let person_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let write: NoteWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let text = write.body.trim();
    if text.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: note body");
    }
    let note_id = Uuid::new_v4();
    match sqlx::query(
        "INSERT INTO person_notes (id,person_id,body,created_at) VALUES ($1,$2,$3,$4)",
    )
    .bind(note_id)
    .bind(person_id)
    .bind(text)
    .bind(Utc::now())
    .execute(&state.pool)
    .await
    {
        Ok(_) => match note(&state.pool, person_id, note_id).await {
            Ok(value) => (StatusCode::CREATED, Json(value)).into_response(),
            Err(err) => database_error(err),
        },
        Err(err) => database_error(err),
    }
}

async fn replace_note(
    State(state): State<AppState>,
    Path((person, note_id)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    let person_id = match id(&person) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let note_id = match id(&note_id) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let write: NoteWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let text = write.body.trim();
    if text.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: note body");
    }
    match sqlx::query("UPDATE person_notes SET body=$3 WHERE person_id=$1 AND id=$2")
        .bind(person_id)
        .bind(note_id)
        .bind(text)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => {
            match note(&state.pool, person_id, note_id).await {
                Ok(value) => Json(value).into_response(),
                Err(err) => database_error(err),
            }
        }
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}

async fn delete_note(
    State(state): State<AppState>,
    Path((person, note_id)): Path<(String, String)>,
) -> Response {
    let person_id = match id(&person) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let note_id = match id(&note_id) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query("DELETE FROM person_notes WHERE person_id=$1 AND id=$2")
        .bind(person_id)
        .bind(note_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}

async fn rels(
    pool: &PgPool,
    table: &str,
    other: &str,
    person_id: Uuid,
) -> Result<Vec<PersonRel>, sqlx::Error> {
    let query =
        format!("SELECT {other} AS id,comment FROM {table} WHERE person_id=$1 ORDER BY {other}");
    sqlx::query_as(&query).bind(person_id).fetch_all(pool).await
}

async fn rows(pool: &PgPool, query: &str, person_id: Uuid) -> Result<Vec<Value>, sqlx::Error> {
    let value: Value = sqlx::query_scalar(query)
        .bind(person_id)
        .fetch_one(pool)
        .await?;
    Ok(value.as_array().cloned().unwrap_or_default())
}

pub(crate) async fn fetch(pool: &PgPool, person_id: Uuid, zone: Tz) -> Result<Person, sqlx::Error> {
    let query = format!("SELECT {COLUMNS} FROM people WHERE id=$1");
    let row: PersonRow = sqlx::query_as(&query)
        .bind(person_id)
        .fetch_one(pool)
        .await?;
    let projects = rels(pool, "person_projects", "project_id", row.id).await?;
    let events = rels(pool, "person_events", "event_id", row.id).await?;
    let companies = rels(pool, "company_people", "company_id", row.id).await?;
    let item_ids: Vec<Uuid> =
        sqlx::query_scalar("SELECT item_id FROM person_items WHERE person_id=$1 ORDER BY item_id")
            .bind(row.id)
            .fetch_all(pool)
            .await?;
    let last_note_at: Option<DateTime<Utc>> =
        sqlx::query_scalar("SELECT max(created_at) FROM person_notes WHERE person_id=$1")
            .bind(row.id)
            .fetch_one(pool)
            .await?;
    let contacts = rows(pool, "SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY c.\"createdAt\"),'[]'::jsonb) FROM (SELECT id,person_id AS \"personId\",kind,label,value,note,created_at AS \"createdAt\",updated_at AS \"updatedAt\" FROM person_contacts WHERE person_id=$1) c", row.id).await?;
    let addresses = rows(pool, "SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.\"createdAt\"),'[]'::jsonb) FROM (SELECT id,person_id AS \"personId\",kind,label,value,note,created_at AS \"createdAt\",updated_at AS \"updatedAt\" FROM person_addresses WHERE person_id=$1) a", row.id).await?;
    let sites = rows(pool, "SELECT COALESCE(jsonb_agg(to_jsonb(s) ORDER BY s.\"createdAt\"),'[]'::jsonb) FROM (SELECT id,person_id AS \"personId\",kind,url,comment,created_at AS \"createdAt\",updated_at AS \"updatedAt\" FROM person_sites WHERE person_id=$1) s", row.id).await?;
    let absences = rows(pool, "SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.\"startsOn\"),'[]'::jsonb) FROM (SELECT id,person_id AS \"personId\",starts_on::timestamp AT TIME ZONE 'UTC' AS \"startsOn\",ends_on::timestamp AT TIME ZONE 'UTC' AS \"endsOn\",note,created_at AS \"createdAt\",updated_at AS \"updatedAt\" FROM person_absences WHERE person_id=$1) a", row.id).await?;
    let professions = rows(pool, "SELECT COALESCE(jsonb_agg(to_jsonb(p) || jsonb_build_object('salaries',COALESCE((SELECT jsonb_agg(to_jsonb(s) ORDER BY s.\"startedOn\") FROM (SELECT id,profession_id AS \"professionId\",started_on::timestamp AT TIME ZONE 'UTC' AS \"startedOn\",ended_on::timestamp AT TIME ZONE 'UTC' AS \"endedOn\",monthly_salary_usd::float8 AS \"monthlySalaryUsd\",monthly_salary_rub::float8 AS \"monthlySalaryRub\" FROM person_profession_salaries WHERE profession_id=p.id) s),'[]'::jsonb)) ORDER BY p.\"startedOn\"),'[]'::jsonb) FROM (SELECT id,person_id AS \"personId\",title,comment,started_on::timestamp AT TIME ZONE 'UTC' AS \"startedOn\",ended_on::timestamp AT TIME ZONE 'UTC' AS \"endedOn\",created_at AS \"createdAt\",updated_at AS \"updatedAt\" FROM person_professions WHERE person_id=$1) p", row.id).await?;
    let bonds = rows(pool, "SELECT COALESCE(jsonb_agg(to_jsonb(b) || jsonb_build_object('events',COALESCE((SELECT jsonb_agg(to_jsonb(e) ORDER BY e.at) FROM (SELECT id,bond_id AS \"bondId\",action,kind,comment,started_on::timestamp AT TIME ZONE 'UTC' AS \"startedOn\",changed_on::timestamp AT TIME ZONE 'UTC' AS \"changedOn\",ended_on::timestamp AT TIME ZONE 'UTC' AS \"endedOn\",at FROM person_bond_events WHERE bond_id=b.id) e),'[]'::jsonb)) ORDER BY b.\"changedOn\" DESC),'[]'::jsonb) FROM (SELECT pb.id,pb.person_a_id AS \"personAId\",pb.person_b_id AS \"personBId\",CASE WHEN pb.person_a_id IS NULL THEN NULL WHEN pb.person_a_id=$1 THEN pb.person_b_id ELSE pb.person_a_id END AS \"otherId\",CASE WHEN pb.person_a_id IS NULL THEN 'Me' ELSE other.name END AS \"otherName\",pb.kind,pb.comment,pb.started_on::timestamp AT TIME ZONE 'UTC' AS \"startedOn\",pb.changed_on::timestamp AT TIME ZONE 'UTC' AS \"changedOn\",pb.ended_on::timestamp AT TIME ZONE 'UTC' AS \"endedOn\",pb.created_at AS \"createdAt\",pb.updated_at AS \"updatedAt\" FROM person_bonds pb LEFT JOIN people other ON other.id=CASE WHEN pb.person_a_id=$1 THEN pb.person_b_id ELSE pb.person_a_id END WHERE pb.person_a_id=$1 OR pb.person_b_id=$1) b", row.id).await?;
    let me_bond = bonds
        .iter()
        .find(|bond| bond["personAId"].is_null() && bond["endedOn"].is_null())
        .map(|bond| serde_json::json!({"id":bond["id"],"kind":bond["kind"]}));
    let age = row.born_on.map(|born| {
        let today = Utc::now().with_timezone(&zone).date_naive();
        let birth = born.date_naive();
        let years = today.year()
            - birth.year()
            - i32::from((today.month(), today.day()) < (birth.month(), birth.day()));
        years.max(0)
    });
    Ok(Person {
        id: row.id,
        name: row.name,
        born_on: row.born_on,
        age,
        projects,
        events,
        companies,
        item_ids,
        contacts,
        addresses,
        sites,
        bonds,
        professions,
        absences,
        me_bond,
        last_note_at,
        created_at: row.created_at,
        updated_at: row.updated_at,
    })
}

async fn zone(state: &AppState) -> Result<Tz, Response> {
    let settings = schedule_settings::settings(state).await?;
    Ok(settings
        .timezone
        .parse()
        .unwrap_or(chrono_tz::Europe::Moscow))
}

async fn list(State(state): State<AppState>) -> Response {
    let zone = match zone(&state).await {
        Ok(zone) => zone,
        Err(response) => return response,
    };
    let ids: Vec<Uuid> = match sqlx::query_scalar("SELECT id FROM people ORDER BY name")
        .fetch_all(&state.pool)
        .await
    {
        Ok(ids) => ids,
        Err(err) => return database_error(err),
    };
    let mut people = Vec::with_capacity(ids.len());
    for person_id in ids {
        match fetch(&state.pool, person_id, zone).await {
            Ok(person) => people.push(person),
            Err(err) => return database_error(err),
        }
    }
    Json(people).into_response()
}

async fn show(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let person_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let zone = match zone(&state).await {
        Ok(zone) => zone,
        Err(response) => return response,
    };
    match fetch(&state.pool, person_id, zone).await {
        Ok(person) => Json(person).into_response(),
        Err(err) => not_found(err),
    }
}

#[allow(clippy::result_large_err)]
fn normalize(write: &mut PersonWrite, zone: Tz) -> Result<Option<NaiveDate>, Response> {
    write.name = write.name.trim().to_owned();
    if write.name.is_empty() {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: person name"));
    }
    if write.age_years.is_some_and(|age| !(0..=150).contains(&age)) {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: ageYears"));
    }
    let born_on = match write.born_on.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => Some(
            NaiveDate::parse_from_str(raw, "%Y-%m-%d")
                .or_else(|_| DateTime::parse_from_rfc3339(raw).map(|stamp| stamp.date_naive()))
                .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid: bornOn"))?,
        ),
        None => write.age_years.map(|age| {
            let today = Utc::now().with_timezone(&zone).date_naive();
            let year = today.year() - age;
            let first = NaiveDate::from_ymd_opt(year, today.month(), 1).unwrap();
            first + chrono::Duration::days(i64::from(today.day() - 1))
        }),
    };
    write.projects = normalize_rels(std::mem::take(&mut write.projects))?;
    write.events = normalize_rels(std::mem::take(&mut write.events))?;
    let mut seen = HashSet::new();
    write
        .companies
        .retain(|rel| !rel.id.is_nil() && seen.insert(rel.id));
    for rel in &mut write.companies {
        rel.comment = rel.comment.trim().to_owned();
    }
    let mut ids = HashSet::new();
    let mut normalized = Vec::new();
    for raw in &write.item_ids {
        if raw.is_empty() {
            continue;
        }
        let item_id = id(raw)?;
        if !item_id.is_nil() && ids.insert(item_id) {
            normalized.push(item_id.to_string());
        }
    }
    write.item_ids = normalized;
    Ok(born_on)
}

async fn replace_links(
    tx: &mut Transaction<'_, Postgres>,
    person_id: Uuid,
    write: &PersonWrite,
) -> Result<(), sqlx::Error> {
    sqlx::query("DELETE FROM person_projects WHERE person_id=$1")
        .bind(person_id)
        .execute(&mut **tx)
        .await?;
    for rel in &write.projects {
        sqlx::query("INSERT INTO person_projects (person_id,project_id,comment) VALUES ($1,$2,$3)")
            .bind(person_id)
            .bind(rel.id)
            .bind(&rel.comment)
            .execute(&mut **tx)
            .await?;
    }
    sqlx::query("DELETE FROM person_events WHERE person_id=$1")
        .bind(person_id)
        .execute(&mut **tx)
        .await?;
    for rel in &write.events {
        sqlx::query("INSERT INTO person_events (person_id,event_id,comment) VALUES ($1,$2,$3)")
            .bind(person_id)
            .bind(rel.id)
            .bind(&rel.comment)
            .execute(&mut **tx)
            .await?;
    }
    sqlx::query("DELETE FROM company_people WHERE person_id=$1")
        .bind(person_id)
        .execute(&mut **tx)
        .await?;
    for rel in &write.companies {
        sqlx::query("INSERT INTO company_people (person_id,company_id,comment) VALUES ($1,$2,$3)")
            .bind(person_id)
            .bind(rel.id)
            .bind(&rel.comment)
            .execute(&mut **tx)
            .await?;
    }
    sqlx::query("DELETE FROM person_items WHERE person_id=$1")
        .bind(person_id)
        .execute(&mut **tx)
        .await?;
    for raw in &write.item_ids {
        let item_id = Uuid::parse_str(raw).unwrap();
        sqlx::query("INSERT INTO person_items (person_id,item_id) VALUES ($1,$2)")
            .bind(person_id)
            .bind(item_id)
            .execute(&mut **tx)
            .await?;
    }
    Ok(())
}

async fn create(State(state): State<AppState>, body: Bytes) -> Response {
    let mut write: PersonWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let zone = match zone(&state).await {
        Ok(zone) => zone,
        Err(response) => return response,
    };
    let born_on = match normalize(&mut write, zone) {
        Ok(born_on) => born_on,
        Err(response) => return response,
    };
    let person_id = Uuid::new_v4();
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    if let Err(err) = sqlx::query(
        "INSERT INTO people (id,name,born_on,created_at,updated_at) VALUES ($1,$2,$3,$4,$4)",
    )
    .bind(person_id)
    .bind(&write.name)
    .bind(born_on)
    .bind(Utc::now())
    .execute(&mut *tx)
    .await
    {
        return database_error(err);
    }
    if let Err(err) = replace_links(&mut tx, person_id, &write).await {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    match fetch(&state.pool, person_id, zone).await {
        Ok(person) => (StatusCode::CREATED, Json(person)).into_response(),
        Err(err) => database_error(err),
    }
}

async fn replace(State(state): State<AppState>, Path(raw): Path<String>, body: Bytes) -> Response {
    let person_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let zone = match zone(&state).await {
        Ok(zone) => zone,
        Err(response) => return response,
    };
    if let Err(err) = fetch(&state.pool, person_id, zone).await {
        return not_found(err);
    }
    let mut write: PersonWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let born_on = match normalize(&mut write, zone) {
        Ok(born_on) => born_on,
        Err(response) => return response,
    };
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    if let Err(err) = sqlx::query("UPDATE people SET name=$2,born_on=$3,updated_at=$4 WHERE id=$1")
        .bind(person_id)
        .bind(&write.name)
        .bind(born_on)
        .bind(Utc::now())
        .execute(&mut *tx)
        .await
    {
        return database_error(err);
    }
    if let Err(err) = replace_links(&mut tx, person_id, &write).await {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    match fetch(&state.pool, person_id, zone).await {
        Ok(person) => Json(person).into_response(),
        Err(err) => database_error(err),
    }
}

async fn delete(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let person_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query("DELETE FROM people WHERE id=$1")
        .bind(person_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}
