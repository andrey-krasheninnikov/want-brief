use crate::companies;
use crate::http::{database_error, decode, error, id, not_found, AppState};
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
use sqlx::{Postgres, Transaction};
use uuid::Uuid;

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct Write {
    title: String,
    currency: String,
    amount: f64,
    comment: String,
    started_on: Option<String>,
    person_id: Option<String>,
    kind: String,
}

#[derive(Clone, Copy)]
enum Action {
    Title,
    Salary,
    Manager,
    Report,
    Contract,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/companies/{id}/titles", post(title))
        .route("/api/companies/{id}/salaries", post(salary))
        .route("/api/companies/{id}/manager", post(manager))
        .route("/api/companies/{id}/reports", post(report))
        .route(
            "/api/companies/{id}/reports/{reportId}",
            axum::routing::delete(end_report),
        )
        .route("/api/companies/{id}/contracts", post(contract))
}

#[allow(clippy::result_large_err)]
fn date(raw: &str) -> Result<NaiveDate, Response> {
    NaiveDate::parse_from_str(raw, "%Y-%m-%d")
        .or_else(|_| DateTime::parse_from_rfc3339(raw).map(|at| at.date_naive()))
        .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))
}

async fn today(state: &AppState) -> Result<(NaiveDate, Tz), Response> {
    let settings = schedule_settings::settings(state).await?;
    let zone: Tz = settings
        .timezone
        .parse()
        .unwrap_or(chrono_tz::Europe::Moscow);
    Ok((Utc::now().with_timezone(&zone).date_naive(), zone))
}

async fn response(state: &AppState, company: Uuid, zone: Tz, created: bool) -> Response {
    match companies::fetch(&state.pool, company, zone).await {
        Ok(value) if created => (StatusCode::CREATED, Json(value)).into_response(),
        Ok(value) => Json(value).into_response(),
        Err(err) => not_found(err),
    }
}

#[allow(clippy::result_large_err)]
fn person(write: &Write) -> Result<Option<Uuid>, Response> {
    match write.person_id.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => id(raw).map(Some),
        None => Ok(None),
    }
}

#[allow(clippy::result_large_err)]
async fn apply(
    tx: &mut Transaction<'_, Postgres>,
    company: Uuid,
    write: &Write,
    action: Action,
    day: NaiveDate,
) -> Result<bool, Response> {
    match action {
        Action::Title => {
            let title = write.title.trim();
            if title.is_empty() {
                return Err(error(StatusCode::BAD_REQUEST, "invalid: title"));
            }
            let old:Option<(String,NaiveDate)>=sqlx::query_as("SELECT title,started_on FROM company_titles WHERE company_id=$1 AND ended_on IS NULL FOR UPDATE")
                .bind(company).fetch_optional(&mut **tx).await.map_err(database_error)?;
            if let Some((old_title, old_day)) = old {
                if old_title == title {
                    return Ok(false);
                }
                if day < old_day {
                    return Err(error(StatusCode::BAD_REQUEST, "invalid: title dates"));
                }
                sqlx::query("UPDATE company_titles SET ended_on=$2 WHERE company_id=$1 AND ended_on IS NULL").bind(company).bind(day).execute(&mut **tx).await.map_err(database_error)?;
            }
            sqlx::query(
                "INSERT INTO company_titles (id,company_id,title,started_on) VALUES ($1,$2,$3,$4)",
            )
            .bind(Uuid::new_v4())
            .bind(company)
            .bind(title)
            .bind(day)
            .execute(&mut **tx)
            .await
            .map_err(database_error)?;
        }
        Action::Salary => {
            let currency = write.currency.trim().to_lowercase();
            if !matches!(currency.as_str(), "usd" | "rub")
                || !write.amount.is_finite()
                || write.amount < 0.0
            {
                return Err(error(StatusCode::BAD_REQUEST, "invalid: salary"));
            }
            let old:Option<(String,f64,NaiveDate)>=sqlx::query_as("SELECT currency,amount::float8,started_on FROM company_salaries WHERE company_id=$1 AND ended_on IS NULL FOR UPDATE")
                .bind(company).fetch_optional(&mut **tx).await.map_err(database_error)?;
            let comment = write.comment.trim();
            if let Some((old_currency, old_amount, old_day)) = old {
                if old_currency == currency && old_amount == write.amount {
                    return Ok(false);
                }
                if comment.is_empty() {
                    return Err(error(StatusCode::BAD_REQUEST, "invalid: salary comment"));
                }
                if day < old_day {
                    return Err(error(StatusCode::BAD_REQUEST, "invalid: salary dates"));
                }
                sqlx::query("UPDATE company_salaries SET ended_on=$2 WHERE company_id=$1 AND ended_on IS NULL").bind(company).bind(day).execute(&mut **tx).await.map_err(database_error)?;
            }
            sqlx::query("INSERT INTO company_salaries (id,company_id,currency,amount,comment,started_on) VALUES ($1,$2,$3,$4,$5,$6)")
                .bind(Uuid::new_v4()).bind(company).bind(currency).bind(write.amount).bind(comment).bind(day).execute(&mut **tx).await.map_err(database_error)?;
        }
        Action::Manager => {
            let person = person(write)?;
            if let Some(person) = person {
                let report:bool=sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM company_reports WHERE company_id=$1 AND person_id=$2 AND ended_on IS NULL)")
                    .bind(company).bind(person).fetch_one(&mut **tx).await.map_err(database_error)?;
                if report {
                    return Err(error(StatusCode::BAD_REQUEST, "invalid: manager is report"));
                }
            }
            let old:Option<(Uuid,NaiveDate)>=sqlx::query_as("SELECT person_id,started_on FROM company_managers WHERE company_id=$1 AND ended_on IS NULL FOR UPDATE")
                .bind(company).fetch_optional(&mut **tx).await.map_err(database_error)?;
            if let Some((old_person, old_day)) = old {
                if person == Some(old_person) {
                    return Ok(false);
                }
                if day < old_day {
                    return Err(error(StatusCode::BAD_REQUEST, "invalid: manager dates"));
                }
                sqlx::query("UPDATE company_managers SET ended_on=$2 WHERE company_id=$1 AND ended_on IS NULL").bind(company).bind(day).execute(&mut **tx).await.map_err(database_error)?;
            }
            if let Some(person) = person {
                let result=sqlx::query("INSERT INTO company_managers (id,company_id,person_id,started_on) VALUES ($1,$2,$3,$4)").bind(Uuid::new_v4()).bind(company).bind(person).bind(day).execute(&mut **tx).await;
                if let Err(err) = result {
                    return if err
                        .as_database_error()
                        .is_some_and(|db| db.code().as_deref() == Some("23503"))
                    {
                        Err(error(StatusCode::NOT_FOUND, "not found"))
                    } else {
                        Err(database_error(err))
                    };
                }
            }
        }
        Action::Report => {
            let person = person(write)?
                .ok_or_else(|| error(StatusCode::BAD_REQUEST, "invalid: report person"))?;
            let manager:bool=sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM company_managers WHERE company_id=$1 AND person_id=$2 AND ended_on IS NULL)")
                .bind(company).bind(person).fetch_one(&mut **tx).await.map_err(database_error)?;
            if manager {
                return Err(error(StatusCode::BAD_REQUEST, "invalid: report is manager"));
            }
            let exists:bool=sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM company_reports WHERE company_id=$1 AND person_id=$2 AND ended_on IS NULL)")
                .bind(company).bind(person).fetch_one(&mut **tx).await.map_err(database_error)?;
            if exists {
                return Ok(false);
            }
            let result=sqlx::query("INSERT INTO company_reports (id,company_id,person_id,started_on) VALUES ($1,$2,$3,$4)").bind(Uuid::new_v4()).bind(company).bind(person).bind(day).execute(&mut **tx).await;
            if let Err(err) = result {
                return if err
                    .as_database_error()
                    .is_some_and(|db| db.code().as_deref() == Some("23503"))
                {
                    Err(error(StatusCode::NOT_FOUND, "not found"))
                } else {
                    Err(database_error(err))
                };
            }
        }
        Action::Contract => {
            let kind = write.kind.trim().to_lowercase();
            if !matches!(
                kind.as_str(),
                "informal" | "gph" | "ip" | "labor" | "contract"
            ) {
                return Err(error(StatusCode::BAD_REQUEST, "invalid: contract kind"));
            }
            let person = person(write)?;
            if let Some(person) = person {
                let member:bool=sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM company_people WHERE company_id=$1 AND person_id=$2)")
                    .bind(company).bind(person).fetch_one(&mut **tx).await.map_err(database_error)?;
                if !member {
                    return Err(error(StatusCode::BAD_REQUEST, "invalid: contract person"));
                }
            }
            let old:Option<(String,NaiveDate)>=sqlx::query_as("SELECT kind,started_on FROM company_contracts WHERE company_id=$1 AND person_id IS NOT DISTINCT FROM $2 AND ended_on IS NULL FOR UPDATE")
                .bind(company).bind(person).fetch_optional(&mut **tx).await.map_err(database_error)?;
            if let Some((old_kind, old_day)) = old {
                if old_kind == kind {
                    return Ok(false);
                }
                if day < old_day {
                    return Err(error(StatusCode::BAD_REQUEST, "invalid: contract dates"));
                }
                sqlx::query("UPDATE company_contracts SET ended_on=$3 WHERE company_id=$1 AND person_id IS NOT DISTINCT FROM $2 AND ended_on IS NULL")
                    .bind(company).bind(person).bind(day).execute(&mut **tx).await.map_err(database_error)?;
            }
            sqlx::query("INSERT INTO company_contracts (id,company_id,person_id,kind,started_on) VALUES ($1,$2,$3,$4,$5)")
                .bind(Uuid::new_v4()).bind(company).bind(person).bind(kind).bind(day).execute(&mut **tx).await.map_err(database_error)?;
        }
    }
    Ok(true)
}

async fn mutate(state: AppState, raw: String, body: Bytes, action: Action) -> Response {
    let company = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let write: Write = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    let (today, zone) = match today(&state).await {
        Ok(value) => value,
        Err(response) => return response,
    };
    let day = match write.started_on.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => match date(raw) {
            Ok(day) => day,
            Err(response) => return response,
        },
        None => today,
    };
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let exists: Result<Uuid, _> =
        sqlx::query_scalar("SELECT id FROM companies WHERE id=$1 FOR UPDATE")
            .bind(company)
            .fetch_one(&mut *tx)
            .await;
    if let Err(err) = exists {
        return not_found(err);
    }
    let changed = match apply(&mut tx, company, &write, action, day).await {
        Ok(value) => value,
        Err(response) => return response,
    };
    if changed {
        if let Err(err) = sqlx::query("UPDATE companies SET updated_at=$2 WHERE id=$1")
            .bind(company)
            .bind(Utc::now())
            .execute(&mut *tx)
            .await
        {
            return database_error(err);
        }
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    response(&state, company, zone, matches!(action, Action::Report)).await
}

async fn end_report(
    State(state): State<AppState>,
    Path((raw, report)): Path<(String, String)>,
) -> Response {
    let company = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let report = match id(&report) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let (day, zone) = match today(&state).await {
        Ok(value) => value,
        Err(response) => return response,
    };
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let old: Result<(NaiveDate, Option<NaiveDate>), _> = sqlx::query_as(
        "SELECT started_on,ended_on FROM company_reports WHERE company_id=$1 AND id=$2 FOR UPDATE",
    )
    .bind(company)
    .bind(report)
    .fetch_one(&mut *tx)
    .await;
    let (started, ended) = match old {
        Ok(value) => value,
        Err(err) => return not_found(err),
    };
    if ended.is_none() {
        if day < started {
            return error(StatusCode::BAD_REQUEST, "invalid: report dates");
        }
        if let Err(err) =
            sqlx::query("UPDATE company_reports SET ended_on=$3 WHERE company_id=$1 AND id=$2")
                .bind(company)
                .bind(report)
                .bind(day)
                .execute(&mut *tx)
                .await
        {
            return database_error(err);
        }
        if let Err(err) = sqlx::query("UPDATE companies SET updated_at=$2 WHERE id=$1")
            .bind(company)
            .bind(Utc::now())
            .execute(&mut *tx)
            .await
        {
            return database_error(err);
        }
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    response(&state, company, zone, false).await
}

async fn title(
    State(state): State<AppState>,
    Path(company): Path<String>,
    body: Bytes,
) -> Response {
    mutate(state, company, body, Action::Title).await
}
async fn salary(
    State(state): State<AppState>,
    Path(company): Path<String>,
    body: Bytes,
) -> Response {
    mutate(state, company, body, Action::Salary).await
}
async fn manager(
    State(state): State<AppState>,
    Path(company): Path<String>,
    body: Bytes,
) -> Response {
    mutate(state, company, body, Action::Manager).await
}
async fn report(
    State(state): State<AppState>,
    Path(company): Path<String>,
    body: Bytes,
) -> Response {
    mutate(state, company, body, Action::Report).await
}
async fn contract(
    State(state): State<AppState>,
    Path(company): Path<String>,
    body: Bytes,
) -> Response {
    mutate(state, company, body, Action::Contract).await
}
