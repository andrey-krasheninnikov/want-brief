mod workspaces;

use crate::http::{database_error, decode, error, AppState};
use crate::schedule_plan::{self, Absence, DayOverride, Input, Reservation};
use axum::body::Bytes;
use axum::extract::{Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Json, Router};
use chrono::{DateTime, Duration, NaiveDate, Utc};
use chrono_tz::Tz;
use serde::{Deserialize, Serialize};
use sqlx::types::Json as DbJson;
use std::collections::{HashMap, HashSet};
use uuid::Uuid;

#[derive(Default, Deserialize)]
#[serde(default)]
struct Range {
    from: String,
    to: String,
    kind: String,
    view: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct DueMove {
    item_id: Uuid,
    due_at: DateTime<Utc>,
}

#[derive(Default, Deserialize)]
#[serde(default, rename_all = "camelCase")]
struct Scenario {
    from: String,
    to: String,
    kind: String,
    view: String,
    move_due: Vec<DueMove>,
    drop_items: Vec<Uuid>,
    skip_events: Vec<Uuid>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct Summary {
    late_seconds: i64,
    overflow_items: i64,
    overflow_seconds: i64,
    at_risk: i64,
    fragments: i64,
    switches: i64,
    score: f64,
}

#[derive(Serialize)]
struct Comparison {
    base: Summary,
    variant: Summary,
    delta: Summary,
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/api/schedule", get(get_schedule))
        .route("/api/schedule/whatif", post(what_if))
}

#[allow(clippy::result_large_err)]
fn parse_range(
    from: &str,
    to: &str,
    kind: &str,
    view: &str,
) -> Result<(DateTime<Utc>, DateTime<Utc>, String), Response> {
    let parse = |raw: &str| {
        DateTime::parse_from_rfc3339(raw)
            .map(|stamp| stamp.with_timezone(&Utc))
            .map_err(|_| error(StatusCode::BAD_REQUEST, "invalid"))
    };
    let (from, to) = if view == "team" && from.is_empty() && to.is_empty() {
        (DateTime::<Utc>::MIN_UTC, DateTime::<Utc>::MAX_UTC)
    } else {
        (parse(from)?, parse(to)?)
    };
    if to <= from {
        return Err(error(StatusCode::BAD_REQUEST, "invalid"));
    }
    let kind = kind.trim();
    if !matches!(kind, "" | "work" | "followup") {
        return Err(error(StatusCode::BAD_REQUEST, "invalid: schedule kind"));
    }
    Ok((
        from,
        to,
        if kind.is_empty() { "work" } else { kind }.to_owned(),
    ))
}

async fn running_intervals(
    state: &AppState,
    items: &mut [crate::items::Item],
    now: DateTime<Utc>,
) -> Result<HashSet<Uuid>, Response> {
    let open: Vec<(Uuid, DateTime<Utc>)> =
        sqlx::query_as("SELECT item_id,started_at FROM time_intervals WHERE ended_at IS NULL")
            .fetch_all(&state.pool)
            .await
            .map_err(database_error)?;
    let mut active = HashSet::new();
    let mut elapsed = HashMap::<Uuid, i64>::new();
    for (item_id, started_at) in open {
        active.insert(item_id);
        let seconds = (now - started_at).num_seconds();
        if seconds >= 0 {
            *elapsed.entry(item_id).or_default() += seconds;
        }
    }
    for item in items {
        item.tracked_seconds += elapsed.get(&item.id).copied().unwrap_or(0);
    }
    Ok(active)
}

async fn input(
    state: &AppState,
    window: workspaces::Window,
    kind: String,
    settings: crate::schedule_settings::Settings,
) -> Result<Input, Response> {
    let workspaces::Window {
        from,
        to,
        now,
        horizon_end: horizon,
    } = window;
    let zone: Tz = settings
        .timezone
        .parse()
        .map_err(|_| error(StatusCode::INTERNAL_SERVER_ERROR, "internal error"))?;
    let event_from = now.min(from);
    let mut items = crate::items::open(&state.pool)
        .await
        .map_err(database_error)?;
    let events = crate::events::occurrences(&state.pool, event_from, horizon)
        .await
        .map_err(database_error)?;
    let active = running_intervals(state, &mut items, now).await?;
    let rows: Vec<(NaiveDate, bool, Option<i32>, Option<i32>)> = sqlx::query_as("SELECT day,\"off\",work_start_min,work_end_min FROM schedule_day_overrides WHERE day BETWEEN $1 AND $2")
        .bind(event_from.with_timezone(&zone).date_naive())
        .bind(horizon.with_timezone(&zone).date_naive())
        .fetch_all(&state.pool).await.map_err(database_error)?;
    let overrides = rows
        .into_iter()
        .map(|(day, off, start, end)| (day, DayOverride { off, start, end }))
        .collect();
    let rows: Vec<(Uuid, NaiveDate, NaiveDate)> = sqlx::query_as("SELECT person_id,starts_on,ends_on FROM person_absences WHERE starts_on <= $2 AND ends_on >= $1")
        .bind(event_from.with_timezone(&zone).date_naive())
        .bind(horizon.with_timezone(&zone).date_naive())
        .fetch_all(&state.pool).await.map_err(database_error)?;
    let absences = rows
        .into_iter()
        .map(|(person, start, end)| Absence { person, start, end })
        .collect();
    let names: HashMap<Uuid, String> =
        sqlx::query_as::<_, (Uuid, String)>("SELECT id,name FROM people")
            .fetch_all(&state.pool)
            .await
            .map_err(database_error)?
            .into_iter()
            .collect();
    let mut factors = HashMap::new();
    if settings.estimate_buffer {
        let rows: Vec<(Option<Uuid>, i32, i64)> = sqlx::query_as("SELECT i.project_id,i.planned_seconds,COALESCE(t.seconds,0)::bigint FROM items i LEFT JOIN (SELECT item_id,EXTRACT(EPOCH FROM SUM(ended_at-started_at))::bigint AS seconds FROM time_intervals WHERE ended_at IS NOT NULL GROUP BY item_id) t ON t.item_id=i.id WHERE i.kind='task' AND i.status='done' AND i.deleted_at IS NULL")
            .fetch_all(&state.pool).await.map_err(database_error)?;
        let mut ratios: HashMap<String, Vec<f64>> = HashMap::new();
        for (project, planned, tracked) in rows {
            if planned > 0 && tracked > 0 {
                ratios
                    .entry(project.map(|id| id.to_string()).unwrap_or_default())
                    .or_default()
                    .push(tracked as f64 / f64::from(planned));
            }
        }
        for (project, mut values) in ratios {
            if values.len() < 3 {
                continue;
            }
            values.sort_by(f64::total_cmp);
            let mid = values.len() / 2;
            let median = if values.len() % 2 == 0 {
                (values[mid - 1] + values[mid]) / 2.0
            } else {
                values[mid]
            };
            if median > 1.0 {
                factors.insert(project, median);
            }
        }
    }
    let history = if settings.golden_hours {
        let rows: Vec<(DateTime<Utc>, DateTime<Utc>)> = sqlx::query_as("SELECT started_at,ended_at FROM time_intervals WHERE ended_at IS NOT NULL AND ended_at>$1 AND started_at<$2")
            .bind(now - Duration::weeks(8)).bind(now).fetch_all(&state.pool).await.map_err(database_error)?;
        rows.into_iter()
            .map(|(start, end)| schedule_plan::Span { start, end })
            .collect()
    } else {
        Vec::new()
    };
    let checkins = if settings.energy_aware {
        let rows: Vec<(String, i16)> = sqlx::query_as("SELECT DISTINCT ON (kind) kind,level FROM stress_logs WHERE kind IN ('focus','energy') ORDER BY kind,logged_at DESC")
            .fetch_all(&state.pool).await.map_err(database_error)?;
        let mut focus = 3;
        let mut energy = 3;
        for (kind, level) in rows {
            if kind == "focus" {
                focus = level;
            } else {
                energy = level;
            }
        }
        Some((focus, energy))
    } else {
        None
    };
    Ok(Input {
        kind,
        items,
        events,
        settings,
        overrides,
        absences,
        names,
        active,
        factors,
        history,
        checkins,
        previous: Vec::new(),
        horizon_end: horizon,
        now,
        from,
        to,
    })
}

async fn get_schedule(State(state): State<AppState>, Query(range): Query<Range>) -> Response {
    let (from, to, kind) = match parse_range(&range.from, &range.to, &range.kind, &range.view) {
        Ok(range) => range,
        Err(response) => return response,
    };
    let Some(actor) = crate::workspace::current() else {
        return error(StatusCode::UNAUTHORIZED, "unauthorized");
    };
    let result = match range.view.as_str() {
        "team" if actor.kind == "team" => workspaces::team(&state, &actor, from, to, kind).await,
        "team" => Err(error(StatusCode::BAD_REQUEST, "select a team workspace")),
        "" | "personal" => {
            let personal =
                match crate::workspace::access(state.auth.pool(), actor.user_id, None).await {
                    Ok(scope) => scope,
                    Err(err) => return crate::http::not_found(err),
                };
            crate::workspace::in_scope(personal.clone(), async {
                let input = workspaces::personal(&state, &personal, from, to, kind).await?;
                snapshot(&state, &personal, input).await
            })
            .await
        }
        _ => Err(error(StatusCode::BAD_REQUEST, "invalid: schedule view")),
    };
    result.unwrap_or_else(|err| err)
}

async fn snapshot(
    state: &AppState,
    personal: &crate::workspace::Scope,
    mut input: Input,
) -> Result<Response, Response> {
    let kind = input.kind.clone();
    let previous: Option<DbJson<serde_json::Value>> =
        sqlx::query_scalar("SELECT blocks FROM schedule_snapshots WHERE kind=$1")
            .bind(&kind)
            .fetch_optional(&state.pool)
            .await
            .map_err(database_error)?;
    input.previous = previous
        .and_then(|DbJson(blocks)| serde_json::from_value::<Vec<Reservation>>(blocks).ok())
        .unwrap_or_default();
    let report = schedule_plan::plan(&input);
    let own: Vec<_> = report
        .horizon
        .iter()
        .filter(|block| block.workspace_id == personal.workspace_id)
        .map(Reservation::from)
        .collect();
    sqlx::query("INSERT INTO schedule_snapshots(kind,taken_at,blocks) VALUES($1,$2,$3) ON CONFLICT(workspace_id,owner_id,kind) DO UPDATE SET taken_at=EXCLUDED.taken_at,blocks=EXCLUDED.blocks")
        .bind(&kind).bind(input.now).bind(DbJson(own)).execute(&state.pool).await.map_err(database_error)?;
    Ok(Json(report).into_response())
}

fn summarize(report: &schedule_plan::Schedule) -> Summary {
    Summary {
        late_seconds: report.score.late_seconds,
        overflow_items: report.overflow.item_count as i64,
        overflow_seconds: report.overflow.seconds,
        at_risk: report.at_risk.len() as i64,
        fragments: i64::from(report.score.fragments),
        switches: i64::from(report.score.switches),
        score: report.score.total,
    }
}

async fn what_if(State(state): State<AppState>, body: Bytes) -> Response {
    let scenario: Scenario = match decode(&body) {
        Ok(value) => value,
        Err(response) => return response,
    };
    let (from, to, kind) =
        match parse_range(&scenario.from, &scenario.to, &scenario.kind, &scenario.view) {
            Ok(range) => range,
            Err(response) => return response,
        };
    if !matches!(scenario.view.as_str(), "" | "personal") {
        return error(
            StatusCode::BAD_REQUEST,
            "what-if uses the personal schedule",
        );
    }
    let Some(actor) = crate::workspace::current() else {
        return error(StatusCode::UNAUTHORIZED, "unauthorized");
    };
    let personal = match crate::workspace::access(state.auth.pool(), actor.user_id, None).await {
        Ok(scope) => scope,
        Err(err) => return crate::http::not_found(err),
    };
    let input = match crate::workspace::in_scope(
        personal.clone(),
        workspaces::personal(&state, &personal, from, to, kind),
    )
    .await
    {
        Ok(input) => input,
        Err(response) => return response,
    };
    let base = summarize(&schedule_plan::plan(&input));
    let mut changed = input;
    let drop: HashSet<Uuid> = scenario.drop_items.into_iter().collect();
    let moves: HashMap<Uuid, DateTime<Utc>> = scenario
        .move_due
        .into_iter()
        .map(|change| (change.item_id, change.due_at))
        .collect();
    changed.items.retain(|item| !drop.contains(&item.id));
    for item in &mut changed.items {
        if let Some(due) = moves.get(&item.id).copied() {
            if changed.kind == "followup" {
                match item.status.as_str() {
                    "review" => item.review_due_at = Some(due),
                    "qa" => item.test_due_at = Some(due),
                    _ => item.due_at = Some(due),
                }
            } else if item.kind == "request" {
                item.due_at = Some(due);
            } else {
                item.dev_due_at = Some(due);
            }
        }
    }
    let skip: HashSet<Uuid> = scenario.skip_events.into_iter().collect();
    changed
        .events
        .retain(|event| !skip.contains(&event.series_id) && !skip.contains(&event.id));
    let variant = summarize(&schedule_plan::plan(&changed));
    let delta = Summary {
        late_seconds: variant.late_seconds - base.late_seconds,
        overflow_items: variant.overflow_items - base.overflow_items,
        overflow_seconds: variant.overflow_seconds - base.overflow_seconds,
        at_risk: variant.at_risk - base.at_risk,
        fragments: variant.fragments - base.fragments,
        switches: variant.switches - base.switches,
        score: variant.score - base.score,
    };
    Json(Comparison {
        base,
        variant,
        delta,
    })
    .into_response()
}
