use crate::http::{database_error, decode, error, id, not_found, AppState};
use crate::projects::{normalize_links, ProjectLink};
use axum::body::Bytes;
use axum::extract::{Path, Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, SecondsFormat, Utc};
use serde::{Deserialize, Serialize};
use sqlx::types::Json as DbJson;
use sqlx::{FromRow, PgPool};
use std::collections::HashSet;
use uuid::Uuid;

const SELECT: &str = "SELECT i.id,i.source_id,i.external_key,i.title,i.status,i.occupancy,i.external_status,i.kind,i.project_id,i.urgent,i.important,i.pinned,i.pinned_at,i.stress,i.due_at,i.dev_due_at,i.review_due_at,i.test_due_at,i.description,i.planned_seconds,i.links,COALESCE(t.tracked_seconds,0)::bigint AS tracked_seconds,i.archived_at,i.deleted_at,i.created_at,i.updated_at,s.name AS source_name,s.kind AS source_kind,COALESCE(p.name,'') AS project_name,COALESCE(p.color,'') AS project_color,COALESCE((SELECT COUNT(*)::int FROM item_checks c WHERE c.item_id=i.id),0) AS check_total,COALESCE((SELECT COUNT(*)::int FROM item_checks c WHERE c.item_id=i.id AND c.done),0) AS check_done,COALESCE((SELECT array_agg(person_id) FROM person_items WHERE item_id=i.id),ARRAY[]::uuid[]) AS person_ids FROM items i JOIN sources s ON s.id=i.source_id LEFT JOIN projects p ON p.id=i.project_id LEFT JOIN (SELECT item_id,EXTRACT(EPOCH FROM SUM(ended_at-started_at))::bigint AS tracked_seconds FROM time_intervals WHERE ended_at IS NOT NULL GROUP BY item_id) t ON t.item_id=i.id";

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct Item {
    id: Uuid,
    source_id: Uuid,
    external_key: String,
    title: String,
    status: String,
    occupancy: String,
    external_status: String,
    kind: String,
    project_id: Option<Uuid>,
    urgent: bool,
    important: bool,
    pinned: bool,
    pinned_at: Option<DateTime<Utc>>,
    stress: Option<i16>,
    due_at: Option<DateTime<Utc>>,
    dev_due_at: Option<DateTime<Utc>>,
    review_due_at: Option<DateTime<Utc>>,
    test_due_at: Option<DateTime<Utc>>,
    description: String,
    planned_seconds: i32,
    links: DbJson<Vec<ProjectLink>>,
    tracked_seconds: i64,
    archived_at: Option<DateTime<Utc>>,
    deleted_at: Option<DateTime<Utc>>,
    created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
    source_name: String,
    source_kind: String,
    project_name: String,
    project_color: String,
    check_total: i32,
    check_done: i32,
    person_ids: Vec<Uuid>,
    #[sqlx(skip)]
    quadrant: String,
}

impl Item {
    fn with_quadrant(mut self) -> Self {
        self.quadrant = match (self.urgent, self.important) {
            (true, true) => "do",
            (false, true) => "schedule",
            (true, false) => "delegate",
            (false, false) => "drop",
        }
        .to_owned();
        self
    }
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct ItemInput {
    title: String,
    kind: String,
    project_id: Option<String>,
    urgent: bool,
    important: bool,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct ItemFilter {
    source_id: Option<String>,
    project_id: Option<String>,
    kind: Option<String>,
    status: Option<String>,
    open_only: Option<String>,
    include_archived: Option<String>,
    archived_only: Option<String>,
}

fn valid_kind(kind: &str) -> bool {
    matches!(
        kind,
        "task" | "request" | "note" | "agreement" | "obligation" | "initiative" | "life"
    )
}

fn valid_status(status: &str) -> bool {
    matches!(
        status,
        "backlog"
            | "needs_grooming"
            | "to_do"
            | "in_progress"
            | "blocked"
            | "review"
            | "qa"
            | "awaiting_decision"
            | "release_candidate"
            | "done"
            | "cancelled"
    )
}

fn flag(value: Option<&str>) -> bool {
    matches!(value, Some("true" | "1"))
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/items", get(list).post(create))
        .route("/api/items/{id}", get(show).patch(patch).delete(delete))
        .route("/api/items/{id}/undelete", axum::routing::post(undelete))
}

async fn fetch(pool: &PgPool, item_id: Uuid) -> Result<Item, sqlx::Error> {
    let query = format!("{SELECT} WHERE i.id=$1");
    sqlx::query_as::<_, Item>(&query)
        .bind(item_id)
        .fetch_one(pool)
        .await
        .map(Item::with_quadrant)
}

async fn show(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match fetch(&state.pool, item_id).await {
        Ok(item) if item.deleted_at.is_none() => Json(item).into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => not_found(err),
    }
}

async fn list(State(state): State<AppState>, Query(filter): Query<ItemFilter>) -> Response {
    let source_id = match filter.source_id.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => match id(raw) {
            Ok(id) => Some(id),
            Err(response) => return response,
        },
        None => None,
    };
    let project_id = match filter.project_id.as_deref().filter(|raw| !raw.is_empty()) {
        Some(raw) => match id(raw) {
            Ok(id) => Some(id),
            Err(response) => return response,
        },
        None => None,
    };
    let kind = filter.kind.filter(|raw| !raw.is_empty());
    if kind.as_deref().is_some_and(|kind| !valid_kind(kind)) {
        return error(StatusCode::BAD_REQUEST, "invalid: item kind");
    }
    let status = filter.status.filter(|raw| !raw.is_empty());
    if status
        .as_deref()
        .is_some_and(|status| !valid_status(status))
    {
        return error(StatusCode::BAD_REQUEST, "invalid: item status");
    }
    let open_only = flag(filter.open_only.as_deref());
    let include_archived = flag(filter.include_archived.as_deref());
    let archived_only = flag(filter.archived_only.as_deref());
    let query = format!("{SELECT} WHERE i.deleted_at IS NULL AND ($1::uuid IS NULL OR i.source_id=$1) AND ($2::uuid IS NULL OR i.project_id=$2) AND ($3::text IS NULL OR i.kind=$3) AND ($4::text IS NULL OR i.status=$4) AND (NOT $5 OR i.status NOT IN ('done','cancelled')) AND ($6 OR i.archived_at IS NULL) AND (NOT $7 OR i.archived_at IS NOT NULL) ORDER BY i.updated_at DESC");
    match sqlx::query_as::<_, Item>(&query)
        .bind(source_id)
        .bind(project_id)
        .bind(kind)
        .bind(status)
        .bind(open_only)
        .bind(include_archived || archived_only)
        .bind(archived_only)
        .fetch_all(&state.pool)
        .await
    {
        Ok(items) => Json(
            items
                .into_iter()
                .map(Item::with_quadrant)
                .collect::<Vec<_>>(),
        )
        .into_response(),
        Err(err) => database_error(err),
    }
}

async fn create(State(state): State<AppState>, body: Bytes) -> Response {
    let input: ItemInput = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    let title = input.title.trim();
    if title.is_empty() {
        return error(StatusCode::BAD_REQUEST, "invalid: title");
    }
    let kind = if input.kind.is_empty() {
        "task"
    } else {
        input.kind.as_str()
    };
    if !valid_kind(kind) {
        return error(StatusCode::BAD_REQUEST, "invalid: item kind");
    }
    let project_id = match input
        .project_id
        .as_deref()
        .filter(|value| !value.is_empty())
    {
        Some(raw) => match id(raw) {
            Ok(id) => Some(id),
            Err(response) => return response,
        },
        None => None,
    };
    let source_id: Uuid = match sqlx::query_scalar(
        "SELECT id FROM sources WHERE kind='manual' ORDER BY created_at LIMIT 1",
    )
    .fetch_one(&state.pool)
    .await
    {
        Ok(id) => id,
        Err(err) => return not_found(err),
    };
    let now = Utc::now();
    let item_id = Uuid::new_v4();
    let occupancy = if kind == "request" { "waiting" } else { "solo" };
    if let Err(err) = sqlx::query("INSERT INTO items (id,source_id,title,status,kind,project_id,urgent,important,occupancy,created_at,updated_at) VALUES ($1,$2,$3,'backlog',$4,$5,$6,$7,$8,$9,$9)")
        .bind(item_id).bind(source_id).bind(title).bind(kind).bind(project_id).bind(input.urgent).bind(input.important).bind(occupancy).bind(now)
        .execute(&state.pool).await {
        return database_error(err);
    }
    match fetch(&state.pool, item_id).await {
        Ok(item) => (StatusCode::CREATED, Json(item)).into_response(),
        Err(err) => database_error(err),
    }
}

async fn delete(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query(
        "UPDATE items SET deleted_at=$2,updated_at=$2 WHERE id=$1 AND deleted_at IS NULL",
    )
    .bind(item_id)
    .bind(Utc::now())
    .execute(&state.pool)
    .await
    {
        Ok(result) if result.rows_affected() > 0 => StatusCode::NO_CONTENT.into_response(),
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}

async fn undelete(State(state): State<AppState>, Path(raw): Path<String>) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    match sqlx::query(
        "UPDATE items SET deleted_at=NULL,updated_at=$2 WHERE id=$1 AND deleted_at IS NOT NULL",
    )
    .bind(item_id)
    .bind(Utc::now())
    .execute(&state.pool)
    .await
    {
        Ok(result) if result.rows_affected() > 0 => match fetch(&state.pool, item_id).await {
            Ok(item) => Json(item).into_response(),
            Err(err) => database_error(err),
        },
        Ok(_) => error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => database_error(err),
    }
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct ItemPatch {
    title: Option<String>,
    status: Option<String>,
    kind: Option<String>,
    project_id: Option<String>,
    urgent: Option<bool>,
    important: Option<bool>,
    pinned: Option<bool>,
    pinned_at: Option<String>,
    stress: Option<i32>,
    due_at: Option<String>,
    dev_due_at: Option<String>,
    review_due_at: Option<String>,
    test_due_at: Option<String>,
    description: Option<String>,
    planned_seconds: Option<i32>,
    links: Option<Vec<ProjectLink>>,
    person_ids: Option<Vec<String>>,
    external_key: Option<String>,
    occupancy: Option<String>,
    external_status: Option<String>,
    archived: Option<bool>,
}

fn status_allowed(kind: &str, status: &str) -> bool {
    valid_status(status)
        && (!matches!(
            status,
            "review" | "qa" | "awaiting_decision" | "release_candidate"
        ) || kind == "task")
}

#[allow(clippy::result_large_err)]
fn set_date(target: &mut Option<DateTime<Utc>>, raw: Option<String>) -> Result<(), Response> {
    if let Some(raw) = raw {
        *target = if raw.is_empty() {
            None
        } else {
            Some(
                DateTime::parse_from_rfc3339(&raw)
                    .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))?
                    .with_timezone(&Utc),
            )
        };
    }
    Ok(())
}

fn snapshot_time(value: Option<DateTime<Utc>>) -> String {
    value
        .map(|value| value.to_rfc3339_opts(SecondsFormat::Secs, true))
        .unwrap_or_default()
}

fn snapshots(item: &Item) -> Vec<(&'static str, String)> {
    vec![
        ("title", item.title.clone()),
        ("kind", item.kind.clone()),
        ("status", item.status.clone()),
        (
            "projectId",
            item.project_id.map(|id| id.to_string()).unwrap_or_default(),
        ),
        ("urgent", item.urgent.to_string()),
        ("important", item.important.to_string()),
        ("pinned", item.pinned.to_string()),
        ("pinnedAt", snapshot_time(item.pinned_at)),
        (
            "stress",
            item.stress
                .map(|value| value.to_string())
                .unwrap_or_default(),
        ),
        ("dueAt", snapshot_time(item.due_at)),
        ("devDueAt", snapshot_time(item.dev_due_at)),
        ("reviewDueAt", snapshot_time(item.review_due_at)),
        ("testDueAt", snapshot_time(item.test_due_at)),
        ("description", item.description.clone()),
        ("plannedSeconds", item.planned_seconds.to_string()),
        (
            "links",
            serde_json::to_string(&item.links.0).unwrap_or_default(),
        ),
        (
            "personIds",
            item.person_ids
                .iter()
                .map(Uuid::to_string)
                .collect::<Vec<_>>()
                .join(","),
        ),
        ("occupancy", item.occupancy.clone()),
        ("archived", snapshot_time(item.archived_at)),
        ("externalKey", item.external_key.clone()),
        ("externalStatus", item.external_status.clone()),
    ]
}

async fn patch(State(state): State<AppState>, Path(raw): Path<String>, body: Bytes) -> Response {
    let item_id = match id(&raw) {
        Ok(id) => id,
        Err(response) => return response,
    };
    let mut item = match fetch(&state.pool, item_id).await {
        Ok(item) if item.deleted_at.is_none() => item,
        Ok(_) => return error(StatusCode::NOT_FOUND, "not found"),
        Err(err) => return not_found(err),
    };
    let before = snapshots(&item);
    let input: ItemPatch = match decode(&body) {
        Ok(input) => input,
        Err(response) => return response,
    };
    if let Some(title) = input.title {
        let title = title.trim();
        if title.is_empty() {
            return error(StatusCode::BAD_REQUEST, "invalid: title");
        }
        item.title = title.to_owned();
    }
    if let Some(kind) = input.kind {
        if !valid_kind(&kind) {
            return error(StatusCode::BAD_REQUEST, "invalid: item kind");
        }
        item.kind = kind;
    }
    if let Some(status) = input.status {
        if !valid_status(&status) {
            return error(StatusCode::BAD_REQUEST, "invalid: item status");
        }
        if !status_allowed(&item.kind, &status) {
            return error(StatusCode::BAD_REQUEST, "invalid: item status");
        }
        item.status = status;
    } else if !status_allowed(&item.kind, &item.status) {
        return error(StatusCode::BAD_REQUEST, "invalid: item status");
    }
    if let Some(project_id) = input.project_id {
        item.project_id = if project_id.is_empty() {
            None
        } else {
            match id(&project_id) {
                Ok(id) => Some(id),
                Err(response) => return response,
            }
        };
    }
    if let Some(urgent) = input.urgent {
        item.urgent = urgent;
    }
    if let Some(important) = input.important {
        item.important = important;
    }
    if let Some(pinned) = input.pinned {
        item.pinned = pinned;
    }
    if let Err(response) = set_date(&mut item.pinned_at, input.pinned_at) {
        return response;
    }
    if let Some(stress) = input.stress {
        item.stress = if stress == 0 {
            None
        } else if (1..=5).contains(&stress) {
            Some(stress as i16)
        } else {
            return error(StatusCode::BAD_REQUEST, "invalid: stress must be 1-5");
        };
    }
    for (target, raw) in [
        (&mut item.due_at, input.due_at),
        (&mut item.dev_due_at, input.dev_due_at),
        (&mut item.review_due_at, input.review_due_at),
        (&mut item.test_due_at, input.test_due_at),
    ] {
        if let Err(response) = set_date(target, raw) {
            return response;
        }
    }
    if let Some(description) = input.description {
        item.description = description.trim().to_owned();
    }
    if let Some(planned) = input.planned_seconds {
        if planned < 0 {
            return error(StatusCode::BAD_REQUEST, "invalid: planned seconds");
        }
        item.planned_seconds = planned;
    }
    if let Some(mut links) = input.links {
        if let Err(response) = normalize_links(&mut links) {
            return response;
        }
        item.links.0 = links;
    }
    let mut replace_people = false;
    if let Some(ids) = input.person_ids {
        let mut seen = HashSet::new();
        let mut people = Vec::new();
        for raw in ids {
            let person_id = match id(&raw) {
                Ok(id) => id,
                Err(response) => return response,
            };
            if !person_id.is_nil() && seen.insert(person_id) {
                people.push(person_id);
            }
        }
        item.person_ids = people;
        replace_people = true;
    }
    if let Some(external_key) = input.external_key {
        if item.source_kind != "manual" {
            return error(StatusCode::FORBIDDEN, "forbidden: external key");
        }
        item.external_key = external_key.trim().to_owned();
    }
    if let Some(occupancy) = input.occupancy {
        let occupancy = occupancy.trim();
        if !matches!(occupancy, "" | "solo" | "parallel" | "waiting") {
            return error(StatusCode::BAD_REQUEST, "invalid: occupancy");
        }
        item.occupancy = if occupancy.is_empty() {
            "solo"
        } else {
            occupancy
        }
        .to_owned();
    }
    if let Some(status) = input.external_status {
        item.external_status = status.trim().to_owned();
    }
    let now = Utc::now();
    if let Some(archived) = input.archived {
        if archived && item.archived_at.is_none() {
            item.archived_at = Some(now);
        }
        if !archived {
            item.archived_at = None;
        }
    }
    item.updated_at = now;
    let after = snapshots(&item);
    let mut tx = match state.pool.begin().await {
        Ok(tx) => tx,
        Err(err) => return database_error(err),
    };
    let result = sqlx::query("UPDATE items SET title=$2,status=$3,kind=$4,project_id=$5,urgent=$6,important=$7,pinned=$8,stress=$9,due_at=$10,dev_due_at=$11,review_due_at=$12,test_due_at=$13,description=$14,planned_seconds=$15,links=$16,external_key=$17,archived_at=$18,deleted_at=$19,updated_at=$20,occupancy=$21,external_status=$22,pinned_at=$23 WHERE id=$1")
        .bind(item.id).bind(&item.title).bind(&item.status).bind(&item.kind).bind(item.project_id)
        .bind(item.urgent).bind(item.important).bind(item.pinned).bind(item.stress)
        .bind(item.due_at).bind(item.dev_due_at).bind(item.review_due_at).bind(item.test_due_at)
        .bind(&item.description).bind(item.planned_seconds).bind(DbJson(item.links.0.clone()))
        .bind(&item.external_key).bind(item.archived_at).bind(item.deleted_at).bind(item.updated_at)
        .bind(&item.occupancy).bind(&item.external_status).bind(item.pinned_at)
        .execute(&mut *tx).await;
    if let Err(err) = result {
        return database_error(err);
    }
    if replace_people {
        if let Err(err) = sqlx::query("DELETE FROM person_items WHERE item_id=$1")
            .bind(item.id)
            .execute(&mut *tx)
            .await
        {
            return database_error(err);
        }
        for person_id in &item.person_ids {
            if let Err(err) =
                sqlx::query("INSERT INTO person_items (item_id,person_id) VALUES ($1,$2)")
                    .bind(item.id)
                    .bind(person_id)
                    .execute(&mut *tx)
                    .await
            {
                return database_error(err);
            }
        }
    }
    for ((field, from), (_, to)) in before.iter().zip(after.iter()) {
        if from == to {
            continue;
        }
        let kind = if *field == "status" {
            "status"
        } else {
            "field"
        };
        if let Err(err) = sqlx::query("INSERT INTO item_events (id,item_id,kind,field,from_value,to_value,note,created_at) VALUES ($1,$2,$3,$4,$5,$6,'',$7)")
            .bind(Uuid::new_v4()).bind(item.id).bind(kind).bind(field).bind(from).bind(to).bind(now)
            .execute(&mut *tx).await { return database_error(err); }
    }
    if let Err(err) = tx.commit().await {
        return database_error(err);
    }
    match fetch(&state.pool, item.id).await {
        Ok(item) => Json(item).into_response(),
        Err(err) => database_error(err),
    }
}
