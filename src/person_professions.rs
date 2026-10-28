use crate::http::{database_error, decode, error, id, AppState};
use crate::schedule_settings;
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::post;
use axum::{Json, Router};
use chrono::{DateTime, NaiveDate, Utc};
use chrono_tz::Tz;
use serde::Deserialize;
use serde_json::Value;
use sqlx::PgPool;
use uuid::Uuid;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Write {
    title: String,
    #[serde(default)]
    comment: String,
    started_on: Option<String>,
    ended_on: Option<String>,
    #[serde(default)]
    monthly_salary_usd: f64,
    #[serde(default)]
    monthly_salary_rub: f64,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/people/{id}/professions", post(create))
        .route(
            "/api/people/{id}/professions/{professionId}",
            axum::routing::patch(update).delete(delete),
        )
}

#[allow(clippy::result_large_err)]
fn date(raw: &str) -> Result<NaiveDate, Response> {
    NaiveDate::parse_from_str(raw, "%Y-%m-%d")
        .or_else(|_| DateTime::parse_from_rfc3339(raw).map(|at| at.date_naive()))
        .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))
}

async fn today(state: &AppState) -> Result<NaiveDate, Response> {
    let settings = schedule_settings::settings(state).await?;
    let zone: Tz = settings
        .timezone
        .parse()
        .unwrap_or(chrono_tz::Europe::Moscow);
    Ok(Utc::now().with_timezone(&zone).date_naive())
}

async fn get(pool: &PgPool, person: Uuid, profession: Uuid) -> Result<Value, sqlx::Error> {
    sqlx::query_scalar(r#"
        SELECT to_jsonb(p) || jsonb_build_object('salaries',COALESCE((
            SELECT jsonb_agg(to_jsonb(s) ORDER BY s."startedOn") FROM (
                SELECT id,profession_id AS "professionId",started_on::timestamp AT TIME ZONE 'UTC' AS "startedOn",
                    ended_on::timestamp AT TIME ZONE 'UTC' AS "endedOn",monthly_salary_usd::float8 AS "monthlySalaryUsd",
                    monthly_salary_rub::float8 AS "monthlySalaryRub"
                FROM person_profession_salaries WHERE profession_id=p.id
            ) s
        ),'[]'::jsonb)) FROM (
            SELECT id,person_id AS "personId",title,comment,started_on::timestamp AT TIME ZONE 'UTC' AS "startedOn",
                ended_on::timestamp AT TIME ZONE 'UTC' AS "endedOn",created_at AS "createdAt",updated_at AS "updatedAt"
            FROM person_professions WHERE person_id=$1 AND id=$2
        ) p
    "#).bind(person).bind(profession).fetch_one(pool).await
}

async fn save(
    state: AppState,
    person: String,
    profession: Option<String>,
    body: Bytes,
) -> Response {
    let person_id = match id(&person) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let updating = profession.is_some();
    let profession_id = match profession {
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
    write.title = write.title.trim().to_owned();
    write.comment = write.comment.trim().to_owned();
    if write.title.is_empty()
        || !write.monthly_salary_usd.is_finite()
        || !write.monthly_salary_rub.is_finite()
        || write.monthly_salary_usd < 0.0
        || write.monthly_salary_rub < 0.0
    {
        return error(StatusCode::BAD_REQUEST, "invalid: profession");
    }
    let today = match today(&state).await {
        Ok(day) => day,
        Err(response) => return response,
    };
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    type Previous = Option<(NaiveDate, Option<(f64, f64, NaiveDate)>)>;
    let old: Previous = if updating {
        let start: NaiveDate = match sqlx::query_scalar(
            "SELECT started_on FROM person_professions WHERE person_id=$1 AND id=$2 FOR UPDATE",
        )
        .bind(person_id)
        .bind(profession_id)
        .fetch_one(&mut *tx)
        .await
        {
            Ok(value) => value,
            Err(sqlx::Error::RowNotFound) => return error(StatusCode::NOT_FOUND, "not found"),
            Err(err) => return database_error(err),
        };
        let salary: Option<(f64,f64,NaiveDate)> = match sqlx::query_as("SELECT monthly_salary_usd::float8,monthly_salary_rub::float8,started_on FROM person_profession_salaries WHERE profession_id=$1 AND ended_on IS NULL ORDER BY started_on DESC LIMIT 1").bind(profession_id).fetch_optional(&mut *tx).await {
            Ok(value) => value, Err(err) => return database_error(err)
        };
        Some((start, salary))
    } else {
        None
    };
    let start = match write
        .started_on
        .as_deref()
        .filter(|value| !value.is_empty())
    {
        Some(value) => match date(value) {
            Ok(day) => day,
            Err(response) => return response,
        },
        None => old.as_ref().map(|(start, _)| *start).unwrap_or(today),
    };
    let end = match write.ended_on.as_deref().filter(|value| !value.is_empty()) {
        Some(value) => match date(value) {
            Ok(day) => Some(day),
            Err(response) => return response,
        },
        None => None,
    };
    if end.is_some_and(|end| end < start) {
        return error(StatusCode::BAD_REQUEST, "invalid: profession dates");
    }
    let sql = if updating {
        "UPDATE person_professions SET title=$3,comment=$4,started_on=$5,ended_on=$6,updated_at=$7 WHERE person_id=$1 AND id=$2"
    } else {
        "INSERT INTO person_professions (person_id,id,title,comment,started_on,ended_on,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$7)"
    };
    if let Err(err) = sqlx::query(sql)
        .bind(person_id)
        .bind(profession_id)
        .bind(&write.title)
        .bind(&write.comment)
        .bind(start)
        .bind(end)
        .bind(Utc::now())
        .execute(&mut *tx)
        .await
    {
        return if err
            .as_database_error()
            .is_some_and(|db| db.code().as_deref() == Some("23503"))
        {
            error(StatusCode::NOT_FOUND, "not found")
        } else {
            database_error(err)
        };
    }
    let changed = old
        .as_ref()
        .and_then(|(_, salary)| *salary)
        .is_none_or(|(usd, rub, _)| {
            usd != write.monthly_salary_usd || rub != write.monthly_salary_rub
        });
    if changed {
        if let Some((_, Some((_, _, salary_start)))) = old {
            if today < salary_start {
                return error(StatusCode::BAD_REQUEST, "invalid: salary dates");
            }
            if let Err(err) = sqlx::query("UPDATE person_profession_salaries SET ended_on=$2 WHERE profession_id=$1 AND ended_on IS NULL").bind(profession_id).bind(today).execute(&mut *tx).await { return database_error(err); }
        }
        let salary_start = if updating { today } else { start };
        if let Err(err) = sqlx::query("INSERT INTO person_profession_salaries (id,profession_id,started_on,monthly_salary_usd,monthly_salary_rub) VALUES ($1,$2,$3,$4,$5)")
            .bind(Uuid::new_v4()).bind(profession_id).bind(salary_start).bind(write.monthly_salary_usd).bind(write.monthly_salary_rub).execute(&mut *tx).await { return database_error(err); }
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    match get(&state.pool, person_id, profession_id).await {
        Ok(value) if !updating => (StatusCode::CREATED, Json(value)).into_response(),
        Ok(value) => Json(value).into_response(),
        Err(err) => database_error(err),
    }
}

async fn create(
    State(state): State<AppState>,
    Path(person): Path<String>,
    body: Bytes,
) -> Response {
    save(state, person, None, body).await
}
async fn update(
    State(state): State<AppState>,
    Path((person, profession)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    save(state, person, Some(profession), body).await
}
async fn delete(
    State(state): State<AppState>,
    Path((person, profession)): Path<(String, String)>,
) -> Response {
    let person_id = match id(&person) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let profession_id = match id(&profession) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query("DELETE FROM person_professions WHERE person_id=$1 AND id=$2")
        .bind(person_id)
        .bind(profession_id)
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}
