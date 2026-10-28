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

#[derive(Clone, FromRow, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Item {
    pub(crate) id: Uuid,
    source_id: Uuid,
    pub(crate) external_key: String,
    pub(crate) title: String,
    pub(crate) status: String,
    pub(crate) occupancy: String,
    external_status: String,
    pub(crate) kind: String,
    pub(crate) project_id: Option<Uuid>,
    pub(crate) urgent: bool,
    pub(crate) important: bool,
    pub(crate) pinned: bool,
    pub(crate) pinned_at: Option<DateTime<Utc>>,
    pub(crate) stress: Option<i16>,
    pub(crate) due_at: Option<DateTime<Utc>>,
    pub(crate) dev_due_at: Option<DateTime<Utc>>,
    pub(crate) review_due_at: Option<DateTime<Utc>>,
    pub(crate) test_due_at: Option<DateTime<Utc>>,
    description: String,
    pub(crate) planned_seconds: i32,
    links: DbJson<Vec<ProjectLink>>,
    pub(crate) tracked_seconds: i64,
    pub(crate) archived_at: Option<DateTime<Utc>>,
    pub(crate) deleted_at: Option<DateTime<Utc>>,
    pub(crate) created_at: DateTime<Utc>,
    updated_at: DateTime<Utc>,
    source_name: String,
    source_kind: String,
    pub(crate) project_name: String,
    pub(crate) project_color: String,
    check_total: i32,
    check_done: i32,
    pub(crate) person_ids: Vec<Uuid>,
    #[sqlx(skip)]
    pub(crate) quadrant: String,
}

#[cfg(test)]
mod schedule_tests {
    use super::*;
    use crate::schedule_plan::{plan, Input};
    use crate::schedule_settings::Settings;
    use chrono::TimeZone;
    use serde_json::{json, Map, Value};
    use std::collections::{HashMap, HashSet};

    fn at(day: u32, hour: u32, minute: u32) -> DateTime<Utc> {
        chrono_tz::Europe::Moscow
            .with_ymd_and_hms(2026, 9, day, hour, minute, 0)
            .unwrap()
            .with_timezone(&Utc)
    }

    fn item(key: &str, index: i64) -> Item {
        Item {
            id: Uuid::new_v4(),
            source_id: Uuid::nil(),
            external_key: key.to_owned(),
            title: key.to_owned(),
            status: "to_do".to_owned(),
            occupancy: "solo".to_owned(),
            external_status: String::new(),
            kind: "task".to_owned(),
            project_id: Some(Uuid::nil()),
            urgent: key == "OV-4",
            important: false,
            pinned: false,
            pinned_at: None,
            stress: if key == "OV-3" { Some(5) } else { None },
            due_at: None,
            dev_due_at: Some(at(16, 18, 0)),
            review_due_at: None,
            test_due_at: None,
            description: String::new(),
            planned_seconds: 8 * 3600,
            links: DbJson(Vec::new()),
            tracked_seconds: 0,
            archived_at: None,
            deleted_at: None,
            created_at: at(1, 12, 0) + chrono::Duration::hours(index),
            updated_at: at(1, 12, 0),
            source_name: String::new(),
            source_kind: "manual".to_owned(),
            project_name: "Overload".to_owned(),
            project_color: "#555".to_owned(),
            check_total: 0,
            check_done: 0,
            person_ids: Vec::new(),
            quadrant: if key == "OV-4" { "delegate" } else { "drop" }.to_owned(),
        }
    }

    fn projection(input: &Input) -> Value {
        let output = serde_json::to_value(plan(input)).unwrap();
        let mut blocks: Vec<Value> = output["lanes"]
            .as_array()
            .unwrap()
            .iter()
            .flat_map(|lane| lane["blocks"].as_array().unwrap().iter().cloned())
            .collect();
        blocks.sort_by(|a, b| {
            a["startsAt"]
                .as_str()
                .cmp(&b["startsAt"].as_str())
                .then_with(|| a["lane"].as_u64().cmp(&b["lane"].as_u64()))
                .then_with(|| a["itemId"].as_str().cmp(&b["itemId"].as_str()))
        });
        let stamp = |value: &Value| {
            DateTime::parse_from_rfc3339(value.as_str().unwrap())
                .unwrap()
                .with_timezone(&chrono_tz::Europe::Moscow)
                .format("%a %d %H:%M")
                .to_string()
        };
        let blocks: Vec<Value> = blocks
            .iter()
            .map(|block| {
                let mut row = Map::new();
                row.insert("key".to_owned(), block["externalKey"].clone());
                row.insert("kind".to_owned(), block["kind"].clone());
                row.insert("start".to_owned(), json!(stamp(&block["startsAt"])));
                row.insert("end".to_owned(), json!(stamp(&block["endsAt"])));
                row.insert("lane".to_owned(), block["lane"].clone());
                if block["late"] == true {
                    row.insert("late".to_owned(), json!(true));
                }
                if !block["reasons"].as_array().unwrap().is_empty() {
                    row.insert("reasons".to_owned(), block["reasons"].clone());
                }
                let continuation = match (block["continued"] == true, block["continues"] == true) {
                    (true, true) => "both",
                    (true, false) => "from",
                    (false, true) => "to",
                    _ => "",
                };
                if !continuation.is_empty() {
                    row.insert("continue".to_owned(), json!(continuation));
                }
                Value::Object(row)
            })
            .collect();
        let unplanned: Vec<Value> = output["unplanned"]
            .as_array()
            .unwrap()
            .iter()
            .map(|row| row["item"]["externalKey"].clone())
            .collect();
        let busy: Vec<String> = output["busy"]
            .as_array()
            .unwrap()
            .iter()
            .map(|row| {
                format!(
                    "{} {}-{}",
                    row["title"].as_str().unwrap(),
                    stamp(&row["startsAt"]),
                    stamp(&row["endsAt"])
                )
            })
            .collect();
        let mut score = output["score"].clone();
        for field in ["loadVariance", "total"] {
            if let Some(value) = score[field].as_f64().filter(|value| value.fract() == 0.0) {
                score[field] = json!(value as i64);
            }
        }
        json!({"blocks":blocks,"unplanned":unplanned,"atRisk":output["atRisk"].as_array().unwrap().iter().map(|risk|risk["key"].clone()).collect::<Vec<_>>(),
            "busy":busy,"overflow":output["overflow"],"score":score})
    }

    fn base(items: Vec<Item>) -> Input {
        Input {
            kind: "work".to_owned(),
            items,
            events: Vec::new(),
            settings: Settings::default(),
            overrides: HashMap::new(),
            absences: Vec::new(),
            names: HashMap::new(),
            active: HashSet::new(),
            factors: HashMap::new(),
            history: Vec::new(),
            checkins: None,
            previous: Vec::new(),
            now: at(15, 9, 30),
            from: at(14, 0, 0),
            to: at(21, 0, 0),
        }
    }

    #[test]
    fn unplanned_items_with_deadlines_come_first() {
        let mut without_due = item("NO-DUE", 0);
        without_due.dev_due_at = None;
        let mut without_plan = item("NO-PLAN", 1);
        without_plan.planned_seconds = 0;
        let output = serde_json::to_value(plan(&base(vec![without_due, without_plan]))).unwrap();
        let keys: Vec<&str> = output["unplanned"]
            .as_array()
            .unwrap()
            .iter()
            .map(|entry| entry["item"]["externalKey"].as_str().unwrap())
            .collect();
        assert_eq!(keys, ["NO-PLAN", "NO-DUE"]);
    }

    #[test]
    fn berlin_settings_keep_local_work_hours() {
        let berlin = chrono_tz::Europe::Berlin;
        let local = |day, hour| {
            berlin
                .with_ymd_and_hms(2026, 9, day, hour, 0, 0)
                .unwrap()
                .with_timezone(&Utc)
        };
        let mut task = item("Berlin", 0);
        task.dev_due_at = Some(local(20, 18));
        task.planned_seconds = 3600;
        let mut input = base(vec![task.clone()]);
        input.settings.timezone = "Europe/Berlin".to_owned();
        input.now = local(15, 8);
        input.from = input.now;
        input.to = local(16, 8);
        let report = plan(&input);
        assert_eq!(
            report
                .horizon
                .iter()
                .find(|block| block.item_id == task.id)
                .unwrap()
                .starts_at,
            local(15, 10)
        );
        let grid = serde_json::to_value(report).unwrap()["grid"].clone();
        assert_eq!(grid["timezone"], "Europe/Berlin");
        assert_eq!(grid["startHour"], 8);
        assert_eq!(grid["endHour"], 19);
    }

    #[test]
    fn active_interval_stays_now_before_work_hours() {
        let mut first = item("First", 0);
        let mut tracked = item("Tracked", 1);
        for task in [&mut first, &mut tracked] {
            task.dev_due_at = Some(at(20, 18, 0));
        }
        first.planned_seconds = 3600;
        tracked.planned_seconds = 3 * 3600;
        let mut input = base(vec![first.clone(), tracked.clone()]);
        input.now = at(15, 4, 30);
        input.from = at(15, 0, 0);
        input.to = at(16, 0, 0);
        input.active.insert(tracked.id);
        let report = plan(&input);
        let blocks: Vec<_> = report
            .horizon
            .iter()
            .filter(|block| block.item_id == tracked.id)
            .collect();
        assert_eq!(blocks.len(), 2);
        assert_eq!(blocks[0].starts_at, input.now);
        assert_eq!(blocks[0].ends_at, at(15, 6, 30));
        assert!(blocks[0].reasons.iter().any(|reason| reason == "active"));
        assert_eq!(blocks[1].starts_at, at(15, 10, 0));
        assert_eq!(
            report
                .horizon
                .iter()
                .find(|block| block.item_id == first.id)
                .unwrap()
                .starts_at,
            at(15, 11, 0)
        );
    }

    #[test]
    fn third_waiting_task_queues_on_waiting_lane() {
        let mut tasks = Vec::new();
        for index in 0..3 {
            let mut task = item(&format!("Wait{index}"), index);
            task.occupancy = "waiting".to_owned();
            task.dev_due_at = Some(at(20, 18, 0));
            task.planned_seconds = 3600;
            tasks.push(task);
        }
        let mut input = base(tasks);
        input.now = at(15, 8, 0);
        input.from = input.now;
        input.to = at(16, 8, 0);
        let report = plan(&input);
        let blocks: Vec<_> = report
            .horizon
            .iter()
            .filter(|block| block.external_key.starts_with("Wait"))
            .collect();
        assert_eq!(blocks.len(), 3);
        assert!(blocks.iter().all(|block| block.lane > 0));
        assert_eq!(
            blocks
                .iter()
                .filter(|block| block.starts_at == at(15, 11, 0))
                .count(),
            1
        );
    }

    #[test]
    fn previous_block_is_sticky_only_when_close() {
        let mut first = item("A", 0);
        let mut second = item("B", 1);
        for task in [&mut first, &mut second] {
            task.dev_due_at = Some(at(20, 18, 0));
            task.planned_seconds = 3600;
        }
        let mut input = base(vec![first, second.clone()]);
        input.now = at(15, 8, 0);
        input.from = input.now;
        input.to = at(16, 8, 0);
        let mut prior = plan(&input)
            .horizon
            .into_iter()
            .find(|block| block.item_id == second.id)
            .unwrap();
        prior.starts_at = at(15, 11, 20);
        prior.ends_at = at(15, 12, 20);
        input.previous = vec![prior.clone()];
        let near = plan(&input);
        let block = near
            .horizon
            .iter()
            .find(|block| block.item_id == second.id)
            .unwrap();
        assert_eq!(block.starts_at, prior.starts_at);
        assert!(block.reasons.iter().any(|reason| reason == "sticky"));
        prior.starts_at = at(16, 10, 0);
        prior.ends_at = at(16, 11, 0);
        input.previous = vec![prior];
        let far = plan(&input);
        assert_eq!(
            far.horizon
                .iter()
                .find(|block| block.item_id == second.id)
                .unwrap()
                .starts_at,
            at(15, 11, 0)
        );
    }

    #[test]
    fn dropping_late_task_and_skipping_meeting_remove_lateness() {
        let mut late = item("Late", 0);
        late.dev_due_at = Some(at(15, 9, 0));
        late.planned_seconds = 2 * 3600;
        let mut input = base(vec![late]);
        input.now = at(15, 8, 0);
        input.from = input.now;
        input.to = at(16, 8, 0);
        assert!(plan(&input).score.late_seconds > 0);
        input.items.clear();
        assert_eq!(plan(&input).score.late_seconds, 0);
        let mut today = item("Today", 0);
        today.dev_due_at = Some(at(15, 18, 0));
        today.planned_seconds = 3600;
        input.items = vec![today];
        input.events = vec![crate::events::schedule_fixture(
            "Blocker",
            at(15, 10, 0),
            9 * 60,
            false,
            &[],
        )];
        assert!(plan(&input).score.late_seconds > 0);
        input.events.clear();
        assert_eq!(plan(&input).score.late_seconds, 0);
    }

    #[test]
    fn overloaded_week_preserves_priority_and_deadline() {
        let items = ["OV-1", "OV-2", "OV-3", "OV-4", "OV-5"]
            .iter()
            .enumerate()
            .map(|(index, key)| item(key, index as i64))
            .collect();
        let projection = projection(&base(items));
        let expected: Value = serde_json::from_str(include_str!(
            "../tests/fixtures/schedule/overloaded_week.golden.json"
        ))
        .unwrap();
        assert_eq!(projection, expected);
    }

    #[test]
    fn current_week_preserves_meetings_and_tracks() {
        let cases = [
            ("HB-101", "Hamkor", 6, 18, "parallel", "in_progress"),
            ("HB-102", "Hamkor", 4, 18, "parallel", "to_do"),
            ("HB-103", "Hamkor", 3, 17, "waiting", "to_do"),
            ("AL-7", "Alpha", 5, 17, "solo", "to_do"),
            ("AL-9", "Alpha", 2, 25, "solo", "to_do"),
        ];
        let mut items: Vec<Item> = cases
            .iter()
            .map(|(key, project, hours, due, occupancy, status)| {
                let mut entry = item(key, 0);
                entry.project_id = Some(if *project == "Hamkor" {
                    Uuid::nil()
                } else {
                    Uuid::from_u128(1)
                });
                entry.project_name = (*project).to_owned();
                entry.project_color = if *project == "Hamkor" { "#0a7" } else { "#c33" }.to_owned();
                entry.planned_seconds = hours * 3600;
                entry.dev_due_at = Some(at(*due, 18, 0));
                entry.occupancy = (*occupancy).to_owned();
                entry.status = (*status).to_owned();
                entry.urgent = false;
                entry.quadrant = "drop".to_owned();
                entry
            })
            .collect();
        items[0].tracked_seconds = 3600;
        items[4].important = true;
        items[4].quadrant = "schedule".to_owned();
        let mut input = base(items);
        input.events = vec![
            crate::events::schedule_fixture("Sprint review", at(16, 12, 0), 60, false, &[]),
            crate::events::schedule_fixture("Optional sync", at(17, 15, 0), 60, true, &[]),
        ];
        input.factors.insert(Uuid::nil().to_string(), 1.25);
        let expected: Value = serde_json::from_str(include_str!(
            "../tests/fixtures/schedule/current_week.golden.json"
        ))
        .unwrap();
        assert_eq!(projection(&input), expected);
    }

    #[test]
    fn followup_week_preserves_people_constraints() {
        let ann = Uuid::from_u128(2);
        let bob = Uuid::from_u128(3);
        let mut review = item("FU-1", 0);
        review.status = "review".to_owned();
        review.review_due_at = Some(at(17, 18, 0));
        let mut waiting = item("FU-2", 0);
        waiting.status = "awaiting_decision".to_owned();
        waiting.project_id = Some(Uuid::from_u128(1));
        waiting.due_at = Some(at(16, 18, 0));
        waiting.person_ids = vec![ann];
        let mut overdue = item("FU-3", 0);
        overdue.status = "awaiting_decision".to_owned();
        overdue.project_id = Some(Uuid::from_u128(1));
        overdue.due_at = Some(at(14, 18, 0));
        overdue.person_ids = vec![bob];
        let mut input = base(vec![review, waiting, overdue]);
        input.kind = "followup".to_owned();
        input.names = HashMap::from([(ann, "Ann".to_owned()), (bob, "Bob".to_owned())]);
        input.events = vec![crate::events::schedule_fixture(
            "Ann 1:1",
            at(16, 10, 0),
            30,
            false,
            &[ann],
        )];
        input.absences = vec![crate::schedule_plan::Absence {
            person: bob,
            start: at(15, 12, 0).date_naive(),
            end: at(16, 12, 0).date_naive(),
        }];
        let expected: Value = serde_json::from_str(include_str!(
            "../tests/fixtures/schedule/followup_week.golden.json"
        ))
        .unwrap();
        assert_eq!(projection(&input), expected);
    }

    #[test]
    fn overrides_preserve_custom_windows_and_breaks() {
        let mut first = item("OR-1", 0);
        first.project_name = "Alpha".to_owned();
        first.planned_seconds = 4 * 3600;
        first.dev_due_at = Some(at(18, 18, 0));
        let mut second = item("OR-2", 0);
        second.project_id = Some(Uuid::from_u128(1));
        second.project_name = "Beta".to_owned();
        second.planned_seconds = 3 * 3600;
        second.dev_due_at = Some(at(18, 18, 0));
        second.pinned_at = Some(at(17, 14, 0));
        let mut input = base(vec![first, second]);
        input.settings.break_window = Some(crate::schedule_settings::BreakWindow {
            start_min: 780,
            end_min: 840,
        });
        input.overrides.insert(
            at(15, 12, 0).date_naive(),
            crate::schedule_plan::DayOverride {
                off: true,
                start: None,
                end: None,
            },
        );
        input.overrides.insert(
            at(16, 12, 0).date_naive(),
            crate::schedule_plan::DayOverride {
                off: false,
                start: Some(720),
                end: Some(960),
            },
        );
        input.checkins = Some((2, 2));
        let expected: Value = serde_json::from_str(include_str!(
            "../tests/fixtures/schedule/overrides_absences.golden.json"
        ))
        .unwrap();
        assert_eq!(projection(&input), expected);
    }
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

pub(crate) async fn fetch(pool: &PgPool, item_id: Uuid) -> Result<Item, sqlx::Error> {
    let query = format!("{SELECT} WHERE i.id=$1");
    sqlx::query_as::<_, Item>(&query)
        .bind(item_id)
        .fetch_one(pool)
        .await
        .map(Item::with_quadrant)
}

pub(crate) async fn open(pool: &PgPool) -> Result<Vec<Item>, sqlx::Error> {
    let query = format!("{SELECT} WHERE i.archived_at IS NULL AND i.deleted_at IS NULL");
    let rows = sqlx::query_as::<_, Item>(&query).fetch_all(pool).await?;
    Ok(rows.into_iter().map(Item::with_quadrant).collect())
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
