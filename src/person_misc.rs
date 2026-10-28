use crate::http::{database_error, decode, error, id, AppState};
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::post;
use axum::{Json, Router};
use chrono::{DateTime, NaiveDate, Utc};
use serde::Deserialize;
use serde_json::Value;
use sqlx::PgPool;
use uuid::Uuid;

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/people/{id}/sites", post(create_site))
        .route(
            "/api/people/{id}/sites/{siteId}",
            axum::routing::patch(update_site).delete(delete_site),
        )
        .route("/api/people/{id}/absences", post(create_absence))
        .route(
            "/api/people/{id}/absences/{absenceId}",
            axum::routing::patch(update_absence).delete(delete_absence),
        )
}

#[derive(Deserialize)]
struct SiteWrite {
    kind: String,
    url: String,
    #[serde(default)]
    comment: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct AbsenceWrite {
    starts_on: String,
    ends_on: String,
    #[serde(default)]
    note: String,
}

#[allow(clippy::result_large_err)]
fn date(raw: &str) -> Result<NaiveDate, Response> {
    NaiveDate::parse_from_str(raw, "%Y-%m-%d")
        .or_else(|_| DateTime::parse_from_rfc3339(raw).map(|at| at.date_naive()))
        .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))
}

async fn get(pool: &PgPool, person: Uuid, entry: Uuid, site: bool) -> Result<Value, sqlx::Error> {
    let sql = if site {
        "SELECT to_jsonb(s) FROM (SELECT id,person_id AS \"personId\",kind,url,comment,created_at AS \"createdAt\",updated_at AS \"updatedAt\" FROM person_sites WHERE person_id=$1 AND id=$2) s"
    } else {
        "SELECT to_jsonb(a) FROM (SELECT id,person_id AS \"personId\",starts_on::timestamp AT TIME ZONE 'UTC' AS \"startsOn\",ends_on::timestamp AT TIME ZONE 'UTC' AS \"endsOn\",note,created_at AS \"createdAt\",updated_at AS \"updatedAt\" FROM person_absences WHERE person_id=$1 AND id=$2) a"
    };
    sqlx::query_scalar(sql)
        .bind(person)
        .bind(entry)
        .fetch_one(pool)
        .await
}

async fn save_site(
    state: AppState,
    person: String,
    entry: Option<String>,
    body: Bytes,
) -> Response {
    let person_id = match id(&person) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let updating = entry.is_some();
    let entry_id = match entry {
        Some(raw) => match id(&raw) {
            Ok(id) => id,
            Err(response) => return response,
        },
        None => Uuid::new_v4(),
    };
    let mut write: SiteWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    write.kind = write.kind.trim().to_owned();
    write.url = write.url.trim().to_owned();
    write.comment = write.comment.trim().to_owned();
    if !matches!(
        write.kind.as_str(),
        "personal_site"
            | "company_site"
            | "github"
            | "linkedin"
            | "youtube"
            | "telegram_channel"
            | "other"
    ) {
        return error(StatusCode::BAD_REQUEST, "invalid: site kind");
    }
    let url = match reqwest::Url::parse(&write.url) {
        Ok(url) if matches!(url.scheme(), "http" | "https") && url.host().is_some() => url,
        _ => return error(StatusCode::BAD_REQUEST, "invalid: link url"),
    };
    let sql = if updating {
        "UPDATE person_sites SET kind=$3,url=$4,comment=$5,updated_at=$6 WHERE person_id=$1 AND id=$2"
    } else {
        "INSERT INTO person_sites (person_id,id,kind,url,comment,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$6)"
    };
    match sqlx::query(sql)
        .bind(person_id)
        .bind(entry_id)
        .bind(&write.kind)
        .bind(url.as_str())
        .bind(&write.comment)
        .bind(Utc::now())
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => {
            match get(&state.pool, person_id, entry_id, true).await {
                Ok(value) if !updating => (StatusCode::CREATED, Json(value)).into_response(),
                Ok(value) => Json(value).into_response(),
                Err(err) => database_error(err),
            }
        }
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err)
            if err
                .as_database_error()
                .is_some_and(|db| db.code().as_deref() == Some("23503")) =>
        {
            error(StatusCode::NOT_FOUND, "not found")
        }
        Err(err) => database_error(err),
    }
}

async fn save_absence(
    state: AppState,
    person: String,
    entry: Option<String>,
    body: Bytes,
) -> Response {
    let person_id = match id(&person) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let updating = entry.is_some();
    let entry_id = match entry {
        Some(raw) => match id(&raw) {
            Ok(id) => id,
            Err(response) => return response,
        },
        None => Uuid::new_v4(),
    };
    let write: AbsenceWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let start = match date(&write.starts_on) {
        Ok(day) => day,
        Err(response) => return response,
    };
    let end = match date(&write.ends_on) {
        Ok(day) => day,
        Err(response) => return response,
    };
    if end < start {
        return error(
            StatusCode::BAD_REQUEST,
            "invalid: absence ends before start",
        );
    }
    let sql = if updating {
        "UPDATE person_absences SET starts_on=$3,ends_on=$4,note=$5,updated_at=$6 WHERE person_id=$1 AND id=$2"
    } else {
        "INSERT INTO person_absences (person_id,id,starts_on,ends_on,note,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$6)"
    };
    match sqlx::query(sql)
        .bind(person_id)
        .bind(entry_id)
        .bind(start)
        .bind(end)
        .bind(write.note.trim())
        .bind(Utc::now())
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => {
            match get(&state.pool, person_id, entry_id, false).await {
                Ok(value) if !updating => (StatusCode::CREATED, Json(value)).into_response(),
                Ok(value) => Json(value).into_response(),
                Err(err) => database_error(err),
            }
        }
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err)
            if err
                .as_database_error()
                .is_some_and(|db| db.code().as_deref() == Some("23503")) =>
        {
            error(StatusCode::NOT_FOUND, "not found")
        }
        Err(err) => database_error(err),
    }
}

async fn remove(state: AppState, person: String, entry: String, site: bool) -> Response {
    let person_id = match id(&person) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let entry_id = match id(&entry) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let sql = if site {
        "DELETE FROM person_sites WHERE person_id=$1 AND id=$2"
    } else {
        "DELETE FROM person_absences WHERE person_id=$1 AND id=$2"
    };
    match sqlx::query(sql)
        .bind(person_id)
        .bind(entry_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}

async fn create_site(
    State(state): State<AppState>,
    Path(person): Path<String>,
    body: Bytes,
) -> Response {
    save_site(state, person, None, body).await
}
async fn update_site(
    State(state): State<AppState>,
    Path((person, entry)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    save_site(state, person, Some(entry), body).await
}
async fn delete_site(
    State(state): State<AppState>,
    Path((person, entry)): Path<(String, String)>,
) -> Response {
    remove(state, person, entry, true).await
}
async fn create_absence(
    State(state): State<AppState>,
    Path(person): Path<String>,
    body: Bytes,
) -> Response {
    save_absence(state, person, None, body).await
}
async fn update_absence(
    State(state): State<AppState>,
    Path((person, entry)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    save_absence(state, person, Some(entry), body).await
}
async fn delete_absence(
    State(state): State<AppState>,
    Path((person, entry)): Path<(String, String)>,
) -> Response {
    remove(state, person, entry, false).await
}
