use crate::http::{database_error, decode, error, id, AppState};
use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::post;
use axum::{Json, Router};
use chrono::Utc;
use serde::Deserialize;
use serde_json::Value;
use sqlx::PgPool;
use uuid::Uuid;

#[derive(Deserialize)]
struct FieldWrite {
    kind: String,
    label: String,
    value: String,
    #[serde(default)]
    note: String,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/people/{id}/contacts", post(create_contact))
        .route(
            "/api/people/{id}/contacts/{contactId}",
            axum::routing::patch(update_contact).delete(delete_contact),
        )
        .route("/api/people/{id}/addresses", post(create_address))
        .route(
            "/api/people/{id}/addresses/{addressId}",
            axum::routing::patch(update_address).delete(delete_address),
        )
}

fn table(address: bool) -> &'static str {
    if address {
        "person_addresses"
    } else {
        "person_contacts"
    }
}

#[allow(clippy::result_large_err)]
fn valid(write: &mut FieldWrite, address: bool) -> Result<(), Response> {
    write.kind = write.kind.trim().to_owned();
    write.label = write.label.trim().to_owned();
    write.value = write.value.trim().to_owned();
    write.note = write.note.trim().to_owned();
    let kind_ok = if address {
        matches!(write.kind.as_str(), "home" | "work" | "other")
    } else {
        matches!(write.kind.as_str(), "phone" | "telegram" | "url")
    };
    if !kind_ok || write.label.is_empty() || write.value.is_empty() {
        return Err(error(StatusCode::BAD_REQUEST, "invalid"));
    }
    if !address && write.kind == "url" {
        let url = reqwest::Url::parse(&write.value)
            .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid: contact value"))?;
        if !matches!(url.scheme(), "http" | "https") || url.host().is_none() {
            return Err(error(StatusCode::BAD_REQUEST, "invalid: contact value"));
        }
    }
    Ok(())
}

async fn get(
    pool: &PgPool,
    address: bool,
    person_id: Uuid,
    entry_id: Uuid,
) -> Result<Value, sqlx::Error> {
    let sql = format!("SELECT to_jsonb(e) FROM (SELECT id,person_id AS \"personId\",kind,label,value,note,created_at AS \"createdAt\",updated_at AS \"updatedAt\" FROM {} WHERE person_id=$1 AND id=$2) e", table(address));
    sqlx::query_scalar(&sql)
        .bind(person_id)
        .bind(entry_id)
        .fetch_one(pool)
        .await
}

async fn save(
    state: AppState,
    person: String,
    entry: Option<String>,
    body: Bytes,
    address: bool,
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
    let mut write: FieldWrite = match decode(&body) {
        Ok(write) => write,
        Err(response) => return response,
    };
    if let Err(response) = valid(&mut write, address) {
        return response;
    }
    let sql = if updating {
        format!("UPDATE {} SET kind=$3,label=$4,value=$5,note=$6,updated_at=$7 WHERE person_id=$1 AND id=$2", table(address))
    } else {
        format!("INSERT INTO {} (person_id,id,kind,label,value,note,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$7)", table(address))
    };
    match sqlx::query(&sql)
        .bind(person_id)
        .bind(entry_id)
        .bind(&write.kind)
        .bind(&write.label)
        .bind(&write.value)
        .bind(&write.note)
        .bind(Utc::now())
        .execute(&state.pool)
        .await
    {
        Ok(result) if result.rows_affected() > 0 => {
            match get(&state.pool, address, person_id, entry_id).await {
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

async fn remove(state: AppState, person: String, entry: String, address: bool) -> Response {
    let person_id = match id(&person) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let entry_id = match id(&entry) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let sql = format!(
        "DELETE FROM {} WHERE person_id=$1 AND id=$2",
        table(address)
    );
    match sqlx::query(&sql)
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

async fn create_contact(
    State(state): State<AppState>,
    Path(person): Path<String>,
    body: Bytes,
) -> Response {
    save(state, person, None, body, false).await
}
async fn update_contact(
    State(state): State<AppState>,
    Path((person, entry)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    save(state, person, Some(entry), body, false).await
}
async fn delete_contact(
    State(state): State<AppState>,
    Path((person, entry)): Path<(String, String)>,
) -> Response {
    remove(state, person, entry, false).await
}
async fn create_address(
    State(state): State<AppState>,
    Path(person): Path<String>,
    body: Bytes,
) -> Response {
    save(state, person, None, body, true).await
}
async fn update_address(
    State(state): State<AppState>,
    Path((person, entry)): Path<(String, String)>,
    body: Bytes,
) -> Response {
    save(state, person, Some(entry), body, true).await
}
async fn delete_address(
    State(state): State<AppState>,
    Path((person, entry)): Path<(String, String)>,
) -> Response {
    remove(state, person, entry, true).await
}
