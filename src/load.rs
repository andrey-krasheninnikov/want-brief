use crate::http::{database_error, AppState};
use axum::extract::{Query, State};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, Duration, LocalResult, NaiveDate, TimeZone, Utc};
use chrono_tz::Tz;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sqlx::FromRow;
use std::collections::HashMap;
use uuid::Uuid;

#[derive(Default, Deserialize)]
struct Range {
    from: Option<DateTime<Utc>>,
    to: Option<DateTime<Utc>>,
}

#[derive(FromRow)]
struct Interval {
    item_id: Uuid,
    started_at: DateTime<Utc>,
    ended_at: Option<DateTime<Utc>>,
}

#[derive(FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
struct StressLog {
    id: Uuid,
    item_id: Option<Uuid>,
    kind: String,
    level: i16,
    logged_at: DateTime<Utc>,
}

#[derive(FromRow)]
struct Item {
    id: Uuid,
    title: String,
    kind: String,
    project_id: Option<Uuid>,
    project_name: String,
}

#[derive(FromRow)]
struct Project {
    id: Uuid,
    name: String,
    color: String,
    target_hours_week: f64,
    archived_at: Option<DateTime<Utc>>,
}

pub fn router() -> Router<AppState> {
    Router::new().route("/api/load", get(load))
}

fn clipped(
    interval: &Interval,
    from: DateTime<Utc>,
    to: DateTime<Utc>,
    now: DateTime<Utc>,
) -> Option<(DateTime<Utc>, DateTime<Utc>)> {
    let start = interval.started_at.max(from);
    let end = interval.ended_at.unwrap_or(now).min(to);
    (end > start).then_some((start, end))
}

fn allocated(
    intervals: &[Interval],
    from: DateTime<Utc>,
    to: DateTime<Utc>,
    now: DateTime<Utc>,
) -> i64 {
    let total: Duration = intervals
        .iter()
        .filter_map(|interval| clipped(interval, from, to, now))
        .map(|(start, end)| end - start)
        .sum();
    total.num_seconds()
}

fn wall(intervals: &[Interval], from: DateTime<Utc>, to: DateTime<Utc>, now: DateTime<Utc>) -> i64 {
    let mut spans: Vec<_> = intervals
        .iter()
        .filter_map(|interval| clipped(interval, from, to, now))
        .collect();
    spans.sort_by_key(|(start, _)| *start);
    let Some((mut start, mut end)) = spans.first().copied() else {
        return 0;
    };
    let mut total = Duration::zero();
    for (next_start, next_end) in spans.into_iter().skip(1) {
        if next_start > end {
            total += end - start;
            start = next_start;
            end = next_end;
        } else if next_end > end {
            end = next_end;
        }
    }
    (total + (end - start)).num_seconds()
}

fn midnight(zone: Tz, day: NaiveDate) -> DateTime<Utc> {
    let mut naive = day.and_hms_opt(0, 0, 0).unwrap();
    for _ in 0..172_800 {
        match zone.from_local_datetime(&naive) {
            LocalResult::Single(time) => return time.with_timezone(&Utc),
            LocalResult::Ambiguous(first, second) => return first.min(second).with_timezone(&Utc),
            LocalResult::None => naive += Duration::seconds(1),
        }
    }
    Utc.from_utc_datetime(&naive)
}

async fn load(State(state): State<AppState>, Query(range): Query<Range>) -> Response {
    let now = Utc::now();
    let mut to = range.to.unwrap_or(now);
    let from = range.from.unwrap_or(to - Duration::days(7));
    if to <= from {
        to = from + Duration::hours(1);
    }
    let intervals: Vec<Interval> = match sqlx::query_as("SELECT item_id,started_at,ended_at FROM time_intervals WHERE started_at<$2 AND (ended_at IS NULL OR ended_at>$1) ORDER BY started_at")
        .bind(from).bind(to).fetch_all(&state.pool).await { Ok(rows) => rows, Err(err) => return database_error(err) };
    let stress: Vec<StressLog> = match sqlx::query_as("SELECT id,item_id,kind,level,logged_at FROM stress_logs WHERE logged_at>=$1 AND logged_at<$2 ORDER BY logged_at")
        .bind(from).bind(to).fetch_all(&state.pool).await { Ok(rows) => rows, Err(err) => return database_error(err) };
    let items: Vec<Item> = match sqlx::query_as("SELECT i.id,i.title,i.kind,i.project_id,COALESCE(p.name,'') AS project_name FROM items i LEFT JOIN projects p ON p.id=i.project_id WHERE i.deleted_at IS NULL AND i.archived_at IS NULL")
        .fetch_all(&state.pool).await { Ok(rows) => rows, Err(err) => return database_error(err) };
    let projects: Vec<Project> = match sqlx::query_as("SELECT id,name,color,target_hours_week::float8 AS target_hours_week,archived_at FROM projects ORDER BY name")
        .fetch_all(&state.pool).await { Ok(rows) => rows, Err(err) => return database_error(err) };
    let zone_name: Option<String> =
        match sqlx::query_scalar("SELECT data->>'timezone' FROM schedule_settings WHERE id=1")
            .fetch_optional(&state.pool)
            .await
        {
            Ok(value) => value.flatten(),
            Err(err) => return database_error(err),
        };
    let zone: Tz = zone_name
        .as_deref()
        .unwrap_or("Europe/Moscow")
        .parse()
        .unwrap_or(chrono_tz::Europe::Moscow);
    let by_id: HashMap<Uuid, &Item> = items.iter().map(|item| (item.id, item)).collect();
    let projects_by_id: HashMap<Uuid, &Project> = projects
        .iter()
        .map(|project| (project.id, project))
        .collect();
    let mut allocated_by_item: HashMap<Uuid, i64> = HashMap::new();
    for interval in &intervals {
        *allocated_by_item.entry(interval.item_id).or_default() +=
            allocated(std::slice::from_ref(interval), from, to, now);
    }
    let mut project_seconds: HashMap<Uuid, i64> = HashMap::new();
    let mut unassigned = 0;
    let mut by_item = Vec::new();
    for (item_id, seconds) in allocated_by_item {
        if seconds == 0 {
            continue;
        }
        let item = by_id.get(&item_id).copied();
        let project_id = item.and_then(|item| item.project_id);
        if let Some(project_id) = project_id {
            *project_seconds.entry(project_id).or_default() += seconds;
        } else {
            unassigned += seconds;
        }
        let project_name = project_id
            .and_then(|id| projects_by_id.get(&id).map(|project| project.name.as_str()))
            .or_else(|| item.map(|item| item.project_name.as_str()))
            .unwrap_or("");
        let title = item
            .map(|item| item.title.as_str())
            .filter(|title| !title.is_empty())
            .map(str::to_owned)
            .unwrap_or_else(|| item_id.to_string());
        by_item.push(json!({"itemId":item_id,"title":title,"kind":item.map(|item| item.kind.as_str()).unwrap_or(""),"projectName":project_name,"allocatedSeconds":seconds}));
    }
    by_item.sort_by(|left, right| left["itemId"].as_str().cmp(&right["itemId"].as_str()));
    let mut by_project = Vec::new();
    for project in projects {
        let seconds = project_seconds.get(&project.id).copied().unwrap_or(0);
        if project.archived_at.is_some() && seconds == 0 {
            continue;
        }
        by_project.push(json!({"projectId":project.id,"name":project.name,"color":project.color,"targetHoursWeek":project.target_hours_week,"allocatedSeconds":seconds}));
    }
    if unassigned > 0 {
        by_project.push(json!({"projectId":null,"name":"Unassigned","color":"#6B6B8C","targetHoursWeek":0,"allocatedSeconds":unassigned}));
    }
    let mut totals: HashMap<&str, (i64, i64)> = HashMap::new();
    for log in &stress {
        let kind = if log.kind.is_empty() {
            "stress"
        } else {
            log.kind.as_str()
        };
        if let Some((sum, count)) = totals.get_mut(kind) {
            *sum += i64::from(log.level);
            *count += 1;
        } else if matches!(
            kind,
            "stress" | "focus" | "energy" | "interest" | "happiness"
        ) {
            totals.insert(kind, (i64::from(log.level), 1));
        }
    }
    let mean = |kind: &str| -> Option<f64> {
        totals
            .get(kind)
            .map(|(sum, count)| *sum as f64 / *count as f64)
    };
    let averages = json!({"stress":mean("stress"),"focus":mean("focus"),"energy":mean("energy"),"interest":mean("interest"),"happiness":mean("happiness")});
    let mut by_day = Vec::new();
    let mut day = from.with_timezone(&zone).date_naive();
    while midnight(zone, day) < to {
        let Some(next_day) = day.succ_opt() else {
            break;
        };
        let day_from = midnight(zone, day).max(from);
        let day_to = midnight(zone, next_day).min(to);
        if day_to > day_from {
            by_day.push(json!({"date":day.to_string(),"allocatedSeconds":allocated(&intervals,day_from,day_to,now),"wallSeconds":wall(&intervals,day_from,day_to,now)}));
        }
        day = next_day;
    }
    let report: Value = json!({"from":from,"to":to,"allocatedSeconds":allocated(&intervals,from,to,now),"wallSeconds":wall(&intervals,from,to,now),"byProject":by_project,"byItem":by_item,"byDay":by_day,"stress":stress,"averages":averages,"averageStress":mean("stress")});
    Json(report).into_response()
}
