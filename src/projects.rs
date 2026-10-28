use crate::http::{database_error, decode, error, id, not_found, AppState};
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use sqlx::types::Json as DbJson;
use sqlx::{FromRow, PgPool, Postgres, Transaction};
use std::collections::HashSet;
use uuid::Uuid;

const COLUMNS: &str = "id, name, color, description, monthly_income_usd::float8 AS monthly_income_usd, monthly_income_rub::float8 AS monthly_income_rub, target_hours_day::float8 AS target_hours_day, target_hours_week::float8 AS target_hours_week, links, archived_at, created_at, updated_at";

#[derive(Clone, Serialize, Deserialize)]
struct ProjectLink {
    label: String,
    url: String,
}

#[derive(Clone, Serialize, Deserialize, FromRow)]
struct PersonRel {
    id: Uuid,
    comment: String,
}

#[derive(FromRow)]
struct ProjectRow {
    id: Uuid,
    name: String,
    color: String,
    description: String,
    monthly_income_usd: f64,
    monthly_income_rub: f64,
    target_hours_day: f64,
    target_hours_week: f64,
    links: DbJson<Vec<ProjectLink>>,
    archived_at: Option<DateTime<Utc>>,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct Project {
    id: Uuid,
    name: String,
    color: String,
    description: String,
    monthly_income_usd: f64,
    monthly_income_rub: f64,
    target_hours_day: f64,
    target_hours_week: f64,
    links: Vec<ProjectLink>,
    people: Vec<PersonRel>,
    companies: Vec<PersonRel>,
    archived_at: Option<DateTime<Utc>>,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct ProjectWrite {
    name: String,
    color: String,
    description: String,
    monthly_income_usd: f64,
    monthly_income_rub: f64,
    target_hours_day: f64,
    links: Vec<ProjectLink>,
    people: Vec<PersonRel>,
    companies: Vec<PersonRel>,
}

#[derive(Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct ProjectPatch {
    name: Option<String>,
    color: Option<String>,
    description: Option<String>,
    monthly_income_usd: Option<f64>,
    monthly_income_rub: Option<f64>,
    target_hours_day: Option<f64>,
    links: Option<Vec<ProjectLink>>,
    people: Option<Vec<PersonRel>>,
    companies: Option<Vec<PersonRel>>,
    archived: Option<bool>,
}

#[derive(Serialize, FromRow)]
#[serde(rename_all = "camelCase")]
struct ProjectNote {
    id: Uuid,
    project_id: Uuid,
    body: String,
    created_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct NoteWrite {
    body: String,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/projects", get(list).post(create))
        .route("/api/projects/{id}", get(show).patch(patch).delete(delete))
        .route(
            "/api/projects/{id}/notes",
            get(list_notes).post(create_note),
        )
        .route(
            "/api/projects/{id}/notes/{noteId}",
            axum::routing::delete(delete_note),
        )
}

#[allow(clippy::result_large_err)]
fn normalize_links(links: &mut [ProjectLink]) -> Result<(), Response> {
    for link in links {
        let trimmed = link.url.trim();
        let parsed = reqwest::Url::parse(trimmed)
            .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid: link url"))?;
        if !matches!(parsed.scheme(), "http" | "https") || parsed.host().is_none() {
            return Err(error(StatusCode::BAD_REQUEST, "invalid: link url"));
        }
        link.label = link.label.trim().to_owned();
        link.url = parsed.to_string();
    }
    Ok(())
}

#[allow(clippy::result_large_err)]
fn normalize_rels(rels: Vec<PersonRel>) -> Result<Vec<PersonRel>, Response> {
    let mut seen = HashSet::new();
    let mut output = Vec::with_capacity(rels.len());
    for mut rel in rels {
        if rel.id.is_nil() || !seen.insert(rel.id) {
            continue;
        }
        rel.comment = rel.comment.trim().to_owned();
        if rel.comment.is_empty() {
            return Err(error(StatusCode::BAD_REQUEST, "invalid: link comment"));
        }
        output.push(rel);
    }
    Ok(output)
}

async fn read(pool: &PgPool, project_id: Uuid) -> Result<Project, Response> {
    let row: ProjectRow = sqlx::query_as(&format!("SELECT {COLUMNS} FROM projects WHERE id=$1"))
        .bind(project_id)
        .fetch_one(pool)
        .await
        .map_err(not_found)?;
    assemble(pool, row).await
}

async fn assemble(pool: &PgPool, row: ProjectRow) -> Result<Project, Response> {
    let people = sqlx::query_as::<_, PersonRel>(
        "SELECT person_id AS id, comment FROM person_projects WHERE project_id=$1",
    )
    .bind(row.id)
    .fetch_all(pool)
    .await
    .map_err(database_error)?;
    let companies = sqlx::query_as::<_, PersonRel>(
        "SELECT company_id AS id, comment FROM company_projects WHERE project_id=$1",
    )
    .bind(row.id)
    .fetch_all(pool)
    .await
    .map_err(database_error)?;
    Ok(Project {
        id: row.id,
        name: row.name,
        color: row.color,
        description: row.description,
        monthly_income_usd: row.monthly_income_usd,
        monthly_income_rub: row.monthly_income_rub,
        target_hours_day: row.target_hours_day,
        target_hours_week: row.target_hours_week,
        links: row.links.0,
        people,
        companies,
        archived_at: row.archived_at,
        created_at: row.created_at,
        updated_at: row.updated_at,
    })
}

async fn replace_rels(
    transaction: &mut Transaction<'_, Postgres>,
    project_id: Uuid,
    people: &[PersonRel],
    companies: &[PersonRel],
) -> Result<(), Response> {
    sqlx::query("DELETE FROM person_projects WHERE project_id=$1")
        .bind(project_id)
        .execute(&mut **transaction)
        .await
        .map_err(database_error)?;
    for rel in people {
        sqlx::query(
            "INSERT INTO person_projects (project_id, person_id, comment) VALUES ($1,$2,$3)",
        )
        .bind(project_id)
        .bind(rel.id)
        .bind(&rel.comment)
        .execute(&mut **transaction)
        .await
        .map_err(database_error)?;
    }
    sqlx::query("DELETE FROM company_projects WHERE project_id=$1")
        .bind(project_id)
        .execute(&mut **transaction)
        .await
        .map_err(database_error)?;
    for rel in companies {
        sqlx::query(
            "INSERT INTO company_projects (project_id, company_id, comment) VALUES ($1,$2,$3)",
        )
        .bind(project_id)
        .bind(rel.id)
        .bind(&rel.comment)
        .execute(&mut **transaction)
        .await
        .map_err(database_error)?;
    }
    Ok(())
}

async fn list(State(state): State<AppState>) -> Response {
    let rows: Vec<ProjectRow> =
        match sqlx::query_as(&format!("SELECT {COLUMNS} FROM projects ORDER BY name"))
            .fetch_all(&state.pool)
            .await
        {
            Ok(rows) => rows,
            Err(err) => return database_error(err),
        };
    let mut projects = Vec::with_capacity(rows.len());
    for row in rows {
        match assemble(&state.pool, row).await {
            Ok(project) => projects.push(project),
            Err(response) => return response,
        }
    }
    Json(projects).into_response()
}

async fn show(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let project_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match read(&state.pool, project_id).await {
        Ok(project) => Json(project).into_response(),
        Err(response) => response,
    }
}

async fn create(State(state): State<AppState>, body: Bytes) -> Response {
    let mut input: ProjectWrite = match decode(&body) {
        Ok(value) => value,
        Err(response) => return response,
    };
    input.name = input.name.trim().to_owned();
    if input.name.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: project name");
    }
    if input.target_hours_day < 0.0 {
        return error(StatusCode::BAD_REQUEST, "invalid: target hours");
    }
    if input.monthly_income_usd < 0.0 {
        return error(StatusCode::BAD_REQUEST, "invalid: monthly income usd");
    }
    if input.monthly_income_rub < 0.0 {
        return error(StatusCode::BAD_REQUEST, "invalid: monthly income rub");
    }
    if let Err(response) = normalize_links(&mut input.links) {
        return response;
    }
    let people = match normalize_rels(input.people) {
        Ok(value) => value,
        Err(response) => return response,
    };
    let companies = match normalize_rels(input.companies) {
        Ok(value) => value,
        Err(response) => return response,
    };
    let project_id = Uuid::new_v4();
    let now = Utc::now();
    let mut transaction = match state.pool.begin().await {
        Ok(transaction) => transaction,
        Err(err) => return database_error(err),
    };
    let result = sqlx::query(
        "INSERT INTO projects (id, name, color, description, monthly_income_usd, monthly_income_rub, target_hours_day, target_hours_week, links, archived_at, created_at, updated_at) \
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)",
    )
    .bind(project_id)
    .bind(input.name)
    .bind(if input.color.is_empty() { "#6152ED" } else { &input.color })
    .bind(input.description.trim())
    .bind(input.monthly_income_usd)
    .bind(input.monthly_income_rub)
    .bind(input.target_hours_day)
    .bind(input.target_hours_day * 7.0)
    .bind(DbJson(input.links))
    .bind(Option::<DateTime<Utc>>::None)
    .bind(now)
    .bind(now)
    .execute(&mut *transaction)
    .await;
    if let Err(err) = result {
        return database_error(err);
    }
    if let Err(response) = replace_rels(&mut transaction, project_id, &people, &companies).await {
        return response;
    }
    if let Err(err) = transaction.commit().await {
        return database_error(err);
    }
    match read(&state.pool, project_id).await {
        Ok(project) => (StatusCode::CREATED, Json(project)).into_response(),
        Err(response) => response,
    }
}

async fn patch(State(state): State<AppState>, Path(raw): Path<String>, body: Bytes) -> Response {
    let project_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let input: ProjectPatch = match decode(&body) {
        Ok(value) => value,
        Err(response) => return response,
    };
    let mut project = match read(&state.pool, project_id).await {
        Ok(project) => project,
        Err(response) => return response,
    };
    if let Some(name) = input.name {
        let name = name.trim();
        if name.is_empty() {
            return error(StatusCode::BAD_REQUEST, "invalid: project name");
        }
        project.name = name.to_owned();
    }
    if let Some(color) = input.color.filter(|value| !value.is_empty()) {
        project.color = color;
    }
    if let Some(description) = input.description {
        project.description = description.trim().to_owned();
    }
    if let Some(income) = input.monthly_income_usd {
        if income < 0.0 {
            return error(StatusCode::BAD_REQUEST, "invalid: monthly income usd");
        }
        project.monthly_income_usd = income;
    }
    if let Some(income) = input.monthly_income_rub {
        if income < 0.0 {
            return error(StatusCode::BAD_REQUEST, "invalid: monthly income rub");
        }
        project.monthly_income_rub = income;
    }
    if let Some(hours) = input.target_hours_day {
        if hours < 0.0 {
            return error(StatusCode::BAD_REQUEST, "invalid: target hours");
        }
        project.target_hours_day = hours;
        project.target_hours_week = hours * 7.0;
    }
    if let Some(mut links) = input.links {
        if let Err(response) = normalize_links(&mut links) {
            return response;
        }
        project.links = links;
    }
    if let Some(people) = input.people {
        project.people = match normalize_rels(people) {
            Ok(value) => value,
            Err(response) => return response,
        };
    }
    if let Some(companies) = input.companies {
        project.companies = match normalize_rels(companies) {
            Ok(value) => value,
            Err(response) => return response,
        };
    }
    let now = Utc::now();
    if let Some(archived) = input.archived {
        if archived && project.archived_at.is_none() {
            project.archived_at = Some(now);
        }
        if !archived {
            project.archived_at = None;
        }
    }
    let mut transaction = match state.pool.begin().await {
        Ok(transaction) => transaction,
        Err(err) => return database_error(err),
    };
    let result = sqlx::query(
        "UPDATE projects SET name=$2, color=$3, description=$4, monthly_income_usd=$5, monthly_income_rub=$6, target_hours_day=$7, target_hours_week=$8, links=$9, archived_at=$10, updated_at=$11 WHERE id=$1",
    )
    .bind(project_id)
    .bind(project.name)
    .bind(project.color)
    .bind(project.description)
    .bind(project.monthly_income_usd)
    .bind(project.monthly_income_rub)
    .bind(project.target_hours_day)
    .bind(project.target_hours_week)
    .bind(DbJson(project.links))
    .bind(project.archived_at)
    .bind(now)
    .execute(&mut *transaction)
    .await;
    if let Err(err) = result {
        return database_error(err);
    }
    if let Err(response) = replace_rels(
        &mut transaction,
        project_id,
        &project.people,
        &project.companies,
    )
    .await
    {
        return response;
    }
    if let Err(err) = transaction.commit().await {
        return database_error(err);
    }
    match read(&state.pool, project_id).await {
        Ok(project) => Json(project).into_response(),
        Err(response) => response,
    }
}

async fn delete(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let project_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query("DELETE FROM projects WHERE id=$1")
        .bind(project_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}

async fn list_notes(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let project_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(response) = read(&state.pool, project_id).await {
        return response;
    }
    match sqlx::query_as::<_, ProjectNote>(
        "SELECT id, project_id, body, created_at FROM project_notes WHERE project_id=$1 ORDER BY created_at DESC",
    )
    .bind(project_id)
    .fetch_all(&state.pool)
    .await
    {
        Ok(notes) => Json(notes).into_response(),
        Err(err) => database_error(err),
    }
}

async fn create_note(
    State(state): State<AppState>,
    Path(raw): Path<String>,
    body: Bytes,
) -> Response {
    let project_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    if let Err(response) = read(&state.pool, project_id).await {
        return response;
    }
    let input: NoteWrite = match decode(&body) {
        Ok(value) => value,
        Err(response) => return response,
    };
    let body = input.body.trim();
    if body.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: note body");
    }
    match sqlx::query_as::<_, ProjectNote>(
        "INSERT INTO project_notes (id, project_id, body, created_at) VALUES ($1,$2,$3,$4) RETURNING id, project_id, body, created_at",
    )
    .bind(Uuid::new_v4())
    .bind(project_id)
    .bind(body)
    .bind(Utc::now())
    .fetch_one(&state.pool)
    .await
    {
        Ok(note) => (StatusCode::CREATED, Json(note)).into_response(),
        Err(err) => database_error(err),
    }
}

async fn delete_note(
    State(state): State<AppState>,
    Path((raw_project, raw_note)): Path<(String, String)>,
) -> Response {
    let project_id = match id(&raw_project) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let note_id = match id(&raw_note) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let belongs: Option<Uuid> =
        match sqlx::query_scalar("SELECT project_id FROM project_notes WHERE id=$1")
            .bind(note_id)
            .fetch_optional(&state.pool)
            .await
        {
            Ok(value) => value,
            Err(err) => return database_error(err),
        };
    if belongs != Some(project_id) {
        return error(StatusCode::NOT_FOUND, "not found");
    }
    match sqlx::query("DELETE FROM project_notes WHERE id=$1")
        .bind(note_id)
        .execute(&state.pool)
        .await
    {
        Ok(_) => StatusCode::NO_CONTENT.into_response(),
        Err(err) => database_error(err),
    }
}
