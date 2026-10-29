use crate::http::{database_error, error, AppState};
use axum::body::{Body, Bytes};
use axum::extract::{DefaultBodyLimit, Query, State};
use axum::http::{header, HeaderMap, StatusCode};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::Router;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sqlx::types::Json;
use sqlx::Postgres;
use std::collections::{BTreeMap, BTreeSet};

const FORMAT_VERSION: u32 = 1;
const SCHEMA_VERSION: i64 = 48;
const MAX_BYTES: usize = 64 * 1024 * 1024;

// Parent tables precede their references so restored IDs keep every relation.
const TABLES: [&str; 40] = [
    "projects",
    "sources",
    "items",
    "notes",
    "time_intervals",
    "stress_logs",
    "events",
    "project_notes",
    "item_notes",
    "people",
    "person_notes",
    "person_projects",
    "person_events",
    "person_items",
    "person_contacts",
    "person_sites",
    "person_bonds",
    "person_bond_events",
    "person_professions",
    "person_profession_salaries",
    "item_checks",
    "event_overrides",
    "event_notes",
    "schedule_settings",
    "schedule_day_overrides",
    "person_absences",
    "schedule_snapshots",
    "companies",
    "company_projects",
    "company_events",
    "company_people",
    "company_notes",
    "company_titles",
    "company_salaries",
    "company_managers",
    "company_reports",
    "company_contracts",
    "item_events",
    "person_addresses",
    "earn_days",
];

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct Backup {
    format_version: u32,
    schema_version: i64,
    exported_at: DateTime<Utc>,
    tables: BTreeMap<String, Vec<Value>>,
}

#[derive(Default, Deserialize)]
#[serde(default)]
struct RestoreMode {
    mode: String,
}

pub fn router() -> Router<AppState> {
    Router::new().route("/api/backup", get(download)).route(
        "/api/restore",
        post(restore).layer(DefaultBodyLimit::max(MAX_BYTES)),
    )
}

async fn download(State(state): State<AppState>) -> Response {
    match export(&state).await {
        Ok(bytes) => Response::builder()
            .header(header::CONTENT_TYPE, "application/json")
            .header(
                header::CONTENT_DISPOSITION,
                "attachment; filename=want-brief-backup.json",
            )
            .header(header::CACHE_CONTROL, "no-store")
            .header(header::X_CONTENT_TYPE_OPTIONS, "nosniff")
            .body(Body::from(bytes))
            .unwrap(),
        Err(response) => response,
    }
}

async fn export(state: &AppState) -> Result<Vec<u8>, Response> {
    let mut transaction = state.pool.begin().await.map_err(database_error)?;
    sqlx::query("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY")
        .execute(&mut *transaction)
        .await
        .map_err(database_error)?;
    if schema_version(&mut transaction).await? != SCHEMA_VERSION {
        return Err(error(StatusCode::CONFLICT, "unsupported database schema"));
    }
    let mut tables = BTreeMap::new();
    for table in TABLES {
        let query = format!("SELECT to_jsonb(row) FROM {table} row");
        let mut rows: Vec<Value> = sqlx::query_scalar(&query)
            .fetch_all(&mut *transaction)
            .await
            .map_err(database_error)?;
        if table == "sources" {
            for row in &mut rows {
                let Some(fields) = row.as_object_mut() else {
                    return Err(error(
                        StatusCode::INTERNAL_SERVER_ERROR,
                        "invalid source row",
                    ));
                };
                fields.remove("token_sealed");
                fields.remove("sync_claim_id");
                fields.remove("sync_claim_until");
                fields.insert("connected".to_owned(), Value::Bool(false));
                fields.insert("last_error".to_owned(), Value::String(String::new()));
            }
        }
        rows.sort_by_key(Value::to_string);
        tables.insert(table.to_owned(), rows);
    }
    transaction.commit().await.map_err(database_error)?;
    let backup = Backup {
        format_version: FORMAT_VERSION,
        schema_version: SCHEMA_VERSION,
        exported_at: Utc::now(),
        tables,
    };
    let bytes = serde_json::to_vec(&backup)
        .map_err(|_| error(StatusCode::INTERNAL_SERVER_ERROR, "backup encoding failed"))?;
    if bytes.len() > MAX_BYTES {
        return Err(error(
            StatusCode::PAYLOAD_TOO_LARGE,
            "backup exceeds 64 MiB",
        ));
    }
    Ok(bytes)
}

async fn restore(
    State(state): State<AppState>,
    Query(mode): Query<RestoreMode>,
    headers: HeaderMap,
    body: Bytes,
) -> Response {
    if mode.mode != "replace" {
        return error(StatusCode::BAD_REQUEST, "confirm replacement");
    }
    if !headers
        .get(header::CONTENT_TYPE)
        .and_then(|value| value.to_str().ok())
        .is_some_and(|value| {
            value
                .split(';')
                .next()
                .unwrap_or("")
                .trim()
                .eq_ignore_ascii_case("application/json")
        })
    {
        return error(
            StatusCode::UNSUPPORTED_MEDIA_TYPE,
            "expected application/json",
        );
    }
    match import(&state, &body).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(response) => response,
    }
}

async fn import(state: &AppState, body: &[u8]) -> Result<(), Response> {
    let mut backup: Backup = serde_json::from_slice(body)
        .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid backup"))?;
    if backup.format_version != FORMAT_VERSION || !matches!(backup.schema_version, 47 | 48) {
        return Err(error(StatusCode::CONFLICT, "unsupported backup version"));
    }
    if backup.schema_version == 47 {
        for row in backup.tables.get_mut("sources").into_iter().flatten() {
            let Some(fields) = row.as_object_mut() else {
                return Err(error(StatusCode::BAD_REQUEST, "invalid backup row"));
            };
            if fields.contains_key("sync_interval_minutes")
                || fields.contains_key("last_attempt_at")
            {
                return Err(error(StatusCode::BAD_REQUEST, "invalid backup row"));
            }
            fields.insert("sync_interval_minutes".to_owned(), Value::from(0));
            fields.insert("last_attempt_at".to_owned(), Value::Null);
        }
    }
    let columns = table_columns(state).await?;
    if backup.tables.len() != TABLES.len() {
        return Err(error(StatusCode::BAD_REQUEST, "invalid backup tables"));
    }
    for table in TABLES {
        let Some(rows) = backup.tables.get(table) else {
            return Err(error(StatusCode::BAD_REQUEST, "invalid backup tables"));
        };
        let Some(names) = columns.get(table) else {
            return Err(error(StatusCode::CONFLICT, "unsupported database schema"));
        };
        let expected: BTreeSet<&str> = names
            .iter()
            .map(String::as_str)
            .filter(|name| exported_column(table, name))
            .collect();
        if rows.iter().any(|row| {
            row.as_object().is_none_or(|fields| {
                fields.keys().map(String::as_str).collect::<BTreeSet<_>>() != expected
            })
        }) {
            return Err(error(StatusCode::BAD_REQUEST, "invalid backup row"));
        }
    }
    for row in backup.tables.get_mut("sources").unwrap() {
        row["connected"] = Value::Bool(false);
        row["last_error"] = Value::String(String::new());
    }
    let mut transaction = state.pool.begin().await.map_err(database_error)?;
    if schema_version(&mut transaction).await? != SCHEMA_VERSION {
        return Err(error(StatusCode::CONFLICT, "unsupported database schema"));
    }
    let truncate = format!("TRUNCATE TABLE {}", TABLES.join(","));
    sqlx::query(&truncate)
        .execute(&mut *transaction)
        .await
        .map_err(database_error)?;
    for table in TABLES {
        let rows = backup.tables.remove(table).unwrap();
        if rows.is_empty() {
            continue;
        }
        let names = columns.get(table).unwrap();
        let names = names
            .iter()
            .filter(|name| exported_column(table, name))
            .map(|name| format!("\"{}\"", name.replace('"', "\"\"")))
            .collect::<Vec<_>>()
            .join(",");
        let insert = format!(
            "INSERT INTO {table} ({names}) SELECT {names} FROM jsonb_populate_recordset(NULL::{table}, $1::jsonb)"
        );
        sqlx::query(&insert)
            .bind(Json(rows))
            .execute(&mut *transaction)
            .await
            .map_err(import_error)?;
    }
    transaction.commit().await.map_err(database_error)?;
    Ok(())
}

fn exported_column(table: &str, name: &str) -> bool {
    table != "sources" || !matches!(name, "token_sealed" | "sync_claim_id" | "sync_claim_until")
}

async fn schema_version(
    transaction: &mut sqlx::Transaction<'_, Postgres>,
) -> Result<i64, Response> {
    sqlx::query_scalar("SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied")
        .fetch_one(&mut **transaction)
        .await
        .map_err(database_error)
}

async fn table_columns(state: &AppState) -> Result<BTreeMap<String, Vec<String>>, Response> {
    let rows: Vec<(String, String)> = sqlx::query_as(
        "SELECT table_name, column_name FROM information_schema.columns \
         WHERE table_schema = current_schema() ORDER BY table_name, ordinal_position",
    )
    .fetch_all(&state.pool)
    .await
    .map_err(database_error)?;
    let mut columns: BTreeMap<String, Vec<String>> = BTreeMap::new();
    for (table, name) in rows {
        columns.entry(table).or_default().push(name);
    }
    Ok(columns)
}

fn import_error(err: sqlx::Error) -> Response {
    if let sqlx::Error::Database(database) = &err {
        if database
            .code()
            .is_some_and(|code| code.starts_with("22") || code.starts_with("23"))
        {
            return error(StatusCode::BAD_REQUEST, "invalid backup data");
        }
    }
    database_error(err)
}
