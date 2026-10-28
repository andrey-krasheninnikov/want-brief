use crate::http::{database_error, decode, error, id, not_found, AppState};
use crate::projects::{normalize_links, normalize_rels, PersonRel, ProjectLink};
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
use sqlx::types::Json as DbJson;
use sqlx::{FromRow, PgPool, Postgres, Transaction};
use std::collections::HashSet;
use uuid::Uuid;

#[derive(FromRow)]
struct CompanyRow {
    id: Uuid,
    name: String,
    description: String,
    links: DbJson<Vec<ProjectLink>>,
    started_on: Option<DateTime<Utc>>,
    ended_on: Option<DateTime<Utc>>,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
}

#[derive(Serialize)]
struct Tenure {
    years: i32,
    months: i32,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Company {
    id: Uuid,
    name: String,
    description: String,
    links: Vec<ProjectLink>,
    projects: Vec<PersonRel>,
    events: Vec<PersonRel>,
    people: Vec<PersonRel>,
    started_on: Option<DateTime<Utc>>,
    ended_on: Option<DateTime<Utc>>,
    tenure: Option<Tenure>,
    titles: Vec<Value>,
    salaries: Vec<Value>,
    managers: Vec<Value>,
    reports: Vec<Value>,
    contracts: Vec<Value>,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct Write {
    name: String,
    description: String,
    links: Vec<ProjectLink>,
    projects: Vec<PersonRel>,
    events: Vec<PersonRel>,
    people: Vec<PersonRel>,
    started_on: Option<String>,
    ended_on: Option<String>,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/companies", get(list).post(create))
        .route(
            "/api/companies/{id}",
            get(show).patch(replace).delete(delete),
        )
}

#[allow(clippy::result_large_err)]
fn date(raw: &str) -> Result<NaiveDate, Response> {
    NaiveDate::parse_from_str(raw, "%Y-%m-%d")
        .or_else(|_| DateTime::parse_from_rfc3339(raw).map(|at| at.date_naive()))
        .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))
}

async fn zone(state: &AppState) -> Result<Tz, Response> {
    let settings = schedule_settings::settings(state).await?;
    Ok(settings
        .timezone
        .parse()
        .unwrap_or(chrono_tz::Europe::Moscow))
}

async fn rels(
    pool: &PgPool,
    table: &str,
    other: &str,
    company: Uuid,
) -> Result<Vec<PersonRel>, sqlx::Error> {
    let query =
        format!("SELECT {other} AS id,comment FROM {table} WHERE company_id=$1 ORDER BY {other}");
    sqlx::query_as(&query).bind(company).fetch_all(pool).await
}

async fn rows(pool: &PgPool, query: &str, company: Uuid) -> Result<Vec<Value>, sqlx::Error> {
    let value: Value = sqlx::query_scalar(query)
        .bind(company)
        .fetch_one(pool)
        .await?;
    Ok(value.as_array().cloned().unwrap_or_default())
}

pub(crate) async fn fetch(pool: &PgPool, company: Uuid, zone: Tz) -> Result<Company, sqlx::Error> {
    let row: CompanyRow = sqlx::query_as("SELECT id,name,description,links,started_on::timestamp AT TIME ZONE 'UTC' AS started_on,ended_on::timestamp AT TIME ZONE 'UTC' AS ended_on,created_at,updated_at FROM companies WHERE id=$1").bind(company).fetch_one(pool).await?;
    let projects = rels(pool, "company_projects", "project_id", company).await?;
    let events = rels(pool, "company_events", "event_id", company).await?;
    let people = rels(pool, "company_people", "person_id", company).await?;
    let titles = rows(pool,"SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.\"startedOn\"),'[]'::jsonb) FROM (SELECT id,company_id AS \"companyId\",title,started_on::timestamp AT TIME ZONE 'UTC' AS \"startedOn\",ended_on::timestamp AT TIME ZONE 'UTC' AS \"endedOn\" FROM company_titles WHERE company_id=$1) t",company).await?;
    let salaries = rows(pool,"SELECT COALESCE(jsonb_agg(to_jsonb(s) ORDER BY s.\"startedOn\"),'[]'::jsonb) FROM (SELECT id,company_id AS \"companyId\",currency,amount::float8,comment,started_on::timestamp AT TIME ZONE 'UTC' AS \"startedOn\",ended_on::timestamp AT TIME ZONE 'UTC' AS \"endedOn\" FROM company_salaries WHERE company_id=$1) s",company).await?;
    let managers = rows(pool,"SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY m.\"startedOn\"),'[]'::jsonb) FROM (SELECT id,company_id AS \"companyId\",person_id AS \"personId\",started_on::timestamp AT TIME ZONE 'UTC' AS \"startedOn\",ended_on::timestamp AT TIME ZONE 'UTC' AS \"endedOn\" FROM company_managers WHERE company_id=$1) m",company).await?;
    let reports = rows(pool,"SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.\"startedOn\"),'[]'::jsonb) FROM (SELECT id,company_id AS \"companyId\",person_id AS \"personId\",started_on::timestamp AT TIME ZONE 'UTC' AS \"startedOn\",ended_on::timestamp AT TIME ZONE 'UTC' AS \"endedOn\" FROM company_reports WHERE company_id=$1) r",company).await?;
    let contracts = rows(pool,"SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY c.\"startedOn\"),'[]'::jsonb) FROM (SELECT id,company_id AS \"companyId\",person_id AS \"personId\",kind,started_on::timestamp AT TIME ZONE 'UTC' AS \"startedOn\",ended_on::timestamp AT TIME ZONE 'UTC' AS \"endedOn\" FROM company_contracts WHERE company_id=$1) c",company).await?;
    let tenure = row.started_on.map(|start| {
        let end = row
            .ended_on
            .map(|at| at.date_naive())
            .unwrap_or_else(|| Utc::now().with_timezone(&zone).date_naive());
        let start = start.date_naive();
        let mut years = end.year() - start.year();
        let mut months = end.month() as i32 - start.month() as i32;
        if end.day() < start.day() {
            months -= 1;
        }
        if months < 0 {
            years -= 1;
            months += 12;
        }
        if years < 0 {
            years = 0;
            months = 0;
        }
        Tenure { years, months }
    });
    Ok(Company {
        id: row.id,
        name: row.name,
        description: row.description,
        links: row.links.0,
        projects,
        events,
        people,
        started_on: row.started_on,
        ended_on: row.ended_on,
        tenure,
        titles,
        salaries,
        managers,
        reports,
        contracts,
        created_at: row.created_at,
        updated_at: row.updated_at,
    })
}

async fn list(State(state): State<AppState>) -> Response {
    let zone = match zone(&state).await {
        Ok(zone) => zone,
        Err(response) => return response,
    };
    let ids: Vec<Uuid> = match sqlx::query_scalar("SELECT id FROM companies ORDER BY name,id")
        .fetch_all(&state.pool)
        .await
    {
        Ok(ids) => ids,
        Err(err) => return database_error(err),
    };
    let mut companies = Vec::with_capacity(ids.len());
    for company in ids {
        match fetch(&state.pool, company, zone).await {
            Ok(value) => companies.push(value),
            Err(err) => return database_error(err),
        }
    }
    Json(companies).into_response()
}

async fn show(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let company = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let zone = match zone(&state).await {
        Ok(zone) => zone,
        Err(response) => return response,
    };
    match fetch(&state.pool, company, zone).await {
        Ok(value) => Json(value).into_response(),
        Err(err) => not_found(err),
    }
}

#[allow(clippy::result_large_err)]
fn normalize(write: &mut Write) -> Result<(Option<NaiveDate>, Option<NaiveDate>), Response> {
    write.name = write.name.trim().to_owned();
    write.description = write.description.trim().to_owned();
    if write.name.is_empty() {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: company name"));
    }
    normalize_links(&mut write.links)?;
    write.projects = normalize_rels(std::mem::take(&mut write.projects))?;
    write.events = normalize_rels(std::mem::take(&mut write.events))?;
    let mut seen = HashSet::new();
    write
        .people
        .retain(|rel| !rel.id.is_nil() && seen.insert(rel.id));
    for rel in &mut write.people {
        rel.comment = rel.comment.trim().to_owned();
    }
    let start = match write.started_on.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => Some(date(raw)?),
        None => None,
    };
    let end = match write.ended_on.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => Some(date(raw)?),
        None => None,
    };
    if end.is_some() && (start.is_none() || end < start) {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: company dates"));
    }
    Ok((start, end))
}

async fn replace_links(
    tx: &mut Transaction<'_, Postgres>,
    company: Uuid,
    write: &Write,
    today: NaiveDate,
) -> Result<(), sqlx::Error> {
    for (table, other, rels) in [
        ("company_projects", "project_id", &write.projects),
        ("company_events", "event_id", &write.events),
        ("company_people", "person_id", &write.people),
    ] {
        let delete = format!("DELETE FROM {table} WHERE company_id=$1");
        sqlx::query(&delete)
            .bind(company)
            .execute(&mut **tx)
            .await?;
        let insert = format!("INSERT INTO {table} (company_id,{other},comment) VALUES ($1,$2,$3)");
        for rel in rels {
            sqlx::query(&insert)
                .bind(company)
                .bind(rel.id)
                .bind(&rel.comment)
                .execute(&mut **tx)
                .await?;
        }
    }
    sqlx::query("UPDATE company_contracts SET ended_on=GREATEST(started_on,$2) WHERE company_id=$1 AND ended_on IS NULL AND person_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM company_people WHERE company_id=$1 AND person_id=company_contracts.person_id)")
        .bind(company).bind(today).execute(&mut **tx).await?;
    Ok(())
}

async fn save(state: AppState, company: Option<String>, body: Bytes) -> Response {
    let updating = company.is_some();
    let company_id = match company {
        Some(raw) => match id(&raw) {
            Ok(id) => id,
            Err(response) => return response,
        },
        None => Uuid::new_v4(),
    };
    let mut write: Write = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let (start, end) = match normalize(&mut write) {
        Ok(dates) => dates,
        Err(response) => return response,
    };
    let zone = match zone(&state).await {
        Ok(zone) => zone,
        Err(response) => return response,
    };
    let today = Utc::now().with_timezone(&zone).date_naive();
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let sql = if updating {
        "UPDATE companies SET name=$2,description=$3,links=$4,started_on=$5,ended_on=$6,updated_at=$7 WHERE id=$1"
    } else {
        "INSERT INTO companies (id,name,description,links,started_on,ended_on,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$7)"
    };
    let result = match sqlx::query(sql)
        .bind(company_id)
        .bind(&write.name)
        .bind(&write.description)
        .bind(DbJson(&write.links))
        .bind(start)
        .bind(end)
        .bind(Utc::now())
        .execute(&mut *tx)
        .await
    {
        Ok(result) => result,
        Err(err) => return database_error(err),
    };
    if result.rows_affected() == 0 {
        return error(StatusCode::NOT_FOUND, "not found");
    }
    if let Err(err) = replace_links(&mut tx, company_id, &write, today).await {
        return database_error(err);
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    match fetch(&state.pool, company_id, zone).await {
        Ok(value) if !updating => (StatusCode::CREATED, Json(value)).into_response(),
        Ok(value) => Json(value).into_response(),
        Err(err) => database_error(err),
    }
}

async fn create(State(state): State<AppState>, body: Bytes) -> Response {
    save(state, None, body).await
}
async fn replace(
    State(state): State<AppState>,
    Path(company): Path<String>,
    body: Bytes,
) -> Response {
    save(state, Some(company), body).await
}
async fn delete(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let company = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query("DELETE FROM companies WHERE id=$1")
        .bind(company)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}
