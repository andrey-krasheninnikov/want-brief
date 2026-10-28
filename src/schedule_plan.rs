use crate::events::EventOccurrence;
use crate::items::Item;
use crate::schedule_settings::Settings;
use chrono::{
    DateTime, Datelike, Duration, LocalResult, NaiveDate, NaiveDateTime, TimeZone, Timelike, Utc,
};
use chrono_tz::Tz;
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};
use uuid::Uuid;

#[derive(Clone, Copy)]
pub(crate) struct Span {
    pub(crate) start: DateTime<Utc>,
    pub(crate) end: DateTime<Utc>,
}

#[derive(Clone)]
pub(crate) struct DayOverride {
    pub(crate) off: bool,
    pub(crate) start: Option<i32>,
    pub(crate) end: Option<i32>,
}

#[derive(Clone)]
pub(crate) struct Absence {
    pub(crate) person: Uuid,
    pub(crate) start: NaiveDate,
    pub(crate) end: NaiveDate,
}

#[derive(Clone)]
pub(crate) struct Input {
    pub(crate) kind: String,
    pub(crate) items: Vec<Item>,
    pub(crate) events: Vec<EventOccurrence>,
    pub(crate) settings: Settings,
    pub(crate) overrides: HashMap<NaiveDate, DayOverride>,
    pub(crate) absences: Vec<Absence>,
    pub(crate) names: HashMap<Uuid, String>,
    pub(crate) active: HashSet<Uuid>,
    pub(crate) factors: HashMap<String, f64>,
    pub(crate) history: Vec<Span>,
    pub(crate) checkins: Option<(i16, i16)>,
    pub(crate) previous: Vec<Block>,
    pub(crate) now: DateTime<Utc>,
    pub(crate) from: DateTime<Utc>,
    pub(crate) to: DateTime<Utc>,
}

#[derive(Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Block {
    pub(crate) item_id: Uuid,
    pub(crate) title: String,
    pub(crate) external_key: String,
    pub(crate) kind: String,
    pub(crate) starts_at: DateTime<Utc>,
    pub(crate) ends_at: DateTime<Utc>,
    pub(crate) late: bool,
    pub(crate) continued: bool,
    pub(crate) continues: bool,
    pub(crate) lane: usize,
    pub(crate) occupancy: String,
    pub(crate) pinned: bool,
    pub(crate) quadrant: String,
    pub(crate) due_at: DateTime<Utc>,
    pub(crate) stress: Option<i16>,
    pub(crate) remaining_seconds: i64,
    pub(crate) estimate_factor: f64,
    pub(crate) reasons: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) people: Vec<SchedulePerson>,
}

#[derive(Clone, Deserialize, Serialize)]
pub(crate) struct SchedulePerson {
    id: Uuid,
    name: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Lane {
    project_id: Option<Uuid>,
    project_name: String,
    project_color: String,
    blocks: Vec<Block>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Unplanned {
    item: Item,
    missing_dev_due: bool,
    missing_plan: bool,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Busy {
    starts_at: DateTime<Utc>,
    ends_at: DateTime<Utc>,
    title: String,
    soft: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    series_id: Option<Uuid>,
    #[serde(skip_serializing_if = "Option::is_none")]
    original_on: Option<NaiveDate>,
    #[serde(skip_serializing_if = "Option::is_none")]
    active_starts_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    active_ends_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    people: Vec<SchedulePerson>,
    #[serde(skip_serializing_if = "std::ops::Not::not")]
    can_skip: bool,
}

#[derive(Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Overflow {
    pub(crate) item_count: usize,
    pub(crate) seconds: i64,
    pub(crate) first_due_at: Option<DateTime<Utc>>,
}

#[derive(Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Capacity {
    free_seconds: i64,
    packed_seconds: i64,
    busy_seconds: i64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Grid {
    timezone: String,
    start_hour: i32,
    end_hour: i32,
    workdays: Vec<i32>,
    work_start_min: i32,
    work_end_min: i32,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Risk {
    item_id: Uuid,
    key: String,
    title: String,
    slack_seconds: i64,
}

#[derive(Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Score {
    pub(crate) late_seconds: i64,
    pub(crate) fragments: i32,
    pub(crate) switches: i32,
    pub(crate) load_variance: f64,
    pub(crate) total: f64,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct Schedule {
    lanes: Vec<Lane>,
    unplanned: Vec<Unplanned>,
    busy: Vec<Busy>,
    pub(crate) overflow: Overflow,
    capacity: Capacity,
    grid: Grid,
    pub(crate) at_risk: Vec<Risk>,
    pub(crate) score: Score,
    #[serde(skip)]
    pub(crate) horizon: Vec<Block>,
}

#[derive(Clone)]
struct Ranked {
    item: Item,
    due: DateTime<Utc>,
    effective_due: DateTime<Utc>,
    remaining: i64,
    slack: i64,
    bucket: i32,
    factor: f64,
    active: bool,
    in_progress: bool,
    reasons: Vec<String>,
}

fn local_at(zone: Tz, day: NaiveDate, minute: i32) -> DateTime<Utc> {
    let naive =
        NaiveDateTime::new(day, chrono::NaiveTime::MIN) + Duration::minutes(i64::from(minute));
    let mut candidate = naive;
    for _ in 0..180 {
        match zone.from_local_datetime(&candidate) {
            LocalResult::Single(at) => return at.with_timezone(&Utc),
            LocalResult::Ambiguous(a, b) => return a.min(b).with_timezone(&Utc),
            LocalResult::None => candidate += Duration::minutes(1),
        }
    }
    zone.from_utc_datetime(&candidate).with_timezone(&Utc)
}

fn clip(span: Span, from: DateTime<Utc>, to: DateTime<Utc>) -> Option<Span> {
    let start = span.start.max(from);
    let end = span.end.min(to);
    (end > start).then_some(Span { start, end })
}

fn seconds(span: Span) -> i64 {
    (span.end - span.start).num_seconds().max(0)
}

fn subtract(spans: &[Span], hole: Span) -> Vec<Span> {
    let mut out = Vec::new();
    for span in spans {
        if hole.end <= span.start || hole.start >= span.end {
            out.push(*span);
            continue;
        }
        if hole.start > span.start {
            out.push(Span {
                start: span.start,
                end: hole.start,
            });
        }
        if hole.end < span.end {
            out.push(Span {
                start: hole.end,
                end: span.end,
            });
        }
    }
    out
}

fn merge(mut spans: Vec<Span>) -> Vec<Span> {
    spans.sort_by_key(|span| span.start);
    let mut out: Vec<Span> = Vec::new();
    for span in spans {
        if let Some(last) = out.last_mut() {
            if span.start <= last.end {
                last.end = last.end.max(span.end);
                continue;
            }
        }
        out.push(span);
    }
    out
}

fn event_active(event: &EventOccurrence) -> Span {
    if let Some((start, end)) = event.active_start_offset.zip(event.active_end_offset) {
        let active = Span {
            start: event.starts_at + Duration::seconds(i64::from(start)),
            end: event.starts_at + Duration::seconds(i64::from(end)),
        };
        if active.end > active.start {
            return active;
        }
    }
    Span {
        start: event.starts_at,
        end: event.ends_at,
    }
}

fn named(ids: &[Uuid], names: &HashMap<Uuid, String>) -> Vec<SchedulePerson> {
    ids.iter()
        .map(|id| SchedulePerson {
            id: *id,
            name: names
                .get(id)
                .cloned()
                .unwrap_or_else(|| id.to_string()[..8].to_owned()),
        })
        .collect()
}

fn work_window(input: &Input, zone: Tz, day: NaiveDate) -> Vec<Span> {
    let override_row = input.overrides.get(&day);
    if override_row.is_some_and(|row| row.off) {
        return Vec::new();
    }
    let weekday = day.weekday().number_from_sunday() as i32 - 1;
    let custom = override_row.and_then(|row| row.start.zip(row.end));
    if custom.is_none() && !input.settings.workdays.contains(&weekday) {
        return Vec::new();
    }
    let (start, end) =
        custom.unwrap_or((input.settings.work_start_min, input.settings.work_end_min));
    let full = Span {
        start: local_at(zone, day, start),
        end: local_at(zone, day, end),
    };
    if let Some(pause) = &input.settings.break_window {
        subtract(
            &[full],
            Span {
                start: local_at(zone, day, pause.start_min),
                end: local_at(zone, day, pause.end_min),
            },
        )
    } else {
        vec![full]
    }
}

fn work_seconds(input: &Input, zone: Tz, from: DateTime<Utc>, to: DateTime<Utc>) -> i64 {
    if to <= from {
        return 0;
    }
    let mut day = from.with_timezone(&zone).date_naive();
    let end = to.with_timezone(&zone).date_naive();
    let mut total = 0;
    while day <= end {
        for span in work_window(input, zone, day) {
            if let Some(span) = clip(span, from, to) {
                total += seconds(span);
            }
        }
        day = day.succ_opt().unwrap();
    }
    total
}

fn back_workdays(input: &Input, zone: Tz, due: DateTime<Utc>, mut count: i32) -> DateTime<Utc> {
    let mut day = due.with_timezone(&zone).date_naive();
    while count > 0 {
        day = day.pred_opt().unwrap();
        if !work_window(input, zone, day).is_empty() {
            count -= 1;
        }
    }
    let local = due.with_timezone(&zone);
    local_at(zone, day, (local.hour() * 60 + local.minute()) as i32)
        + Duration::seconds(i64::from(local.second()))
}

fn grid_round(zone: Tz, at: DateTime<Utc>, minutes: i32) -> DateTime<Utc> {
    let local = at.with_timezone(&zone);
    let minute = (local.hour() * 60 + local.minute()) as i32;
    let step = minutes.max(1);
    let rounded = if minute % step != 0 || local.second() > 0 || local.nanosecond() > 0 {
        (minute / step + 1) * step
    } else {
        minute
    };
    local_at(zone, local.date_naive(), rounded)
}

fn eligible(item: &Item, kind: &str) -> bool {
    if item.archived_at.is_some() || item.deleted_at.is_some() {
        return false;
    }
    if kind == "followup" {
        return item.kind == "task"
            && matches!(item.status.as_str(), "review" | "qa" | "awaiting_decision");
    }
    matches!(item.kind.as_str(), "task" | "request")
        && matches!(item.status.as_str(), "to_do" | "in_progress")
}

fn due(item: &Item, kind: &str) -> Option<DateTime<Utc>> {
    if kind == "followup" {
        match item.status.as_str() {
            "review" => item.review_due_at,
            "qa" => item.test_due_at,
            _ => item.due_at,
        }
    } else if item.kind == "request" {
        item.due_at
    } else {
        item.dev_due_at
    }
}

fn nominal_day(settings: &Settings) -> i64 {
    let mut minutes = settings.work_end_min - settings.work_start_min;
    if let Some(pause) = &settings.break_window {
        minutes -= pause.end_min - pause.start_min;
    }
    i64::from(minutes.max(1)) * 60
}

fn rank(input: &Input, zone: Tz, item: Item, due: DateTime<Utc>) -> Ranked {
    let project = item.project_id.map(|id| id.to_string()).unwrap_or_default();
    let factor = if input.settings.estimate_buffer {
        input
            .factors
            .get(&project)
            .copied()
            .unwrap_or(1.0)
            .clamp(1.0, input.settings.estimate_max_k)
    } else {
        1.0
    };
    let factor = (factor * 100.0).round() / 100.0;
    let remaining =
        ((f64::from(item.planned_seconds) * factor) as i64 - item.tracked_seconds).max(0);
    let mut effective_due = back_workdays(
        input,
        zone,
        due,
        input.settings.target_lead_workdays + i32::from(item.urgent),
    );
    if let Some(stress) = item.stress {
        if stress > 1 {
            effective_due -= Duration::hours(
                i64::from(stress - 1) * i64::from(input.settings.stress_shift_hours),
            );
        }
    }
    let slack = if effective_due < input.now {
        (effective_due - input.now).num_seconds() - remaining
    } else {
        work_seconds(input, zone, input.now, effective_due) - remaining
    };
    let days = slack / nominal_day(&input.settings);
    let bucket = if slack < 0 {
        0
    } else if days <= 0 {
        1
    } else if days == 1 {
        2
    } else if days <= 3 {
        3
    } else if days <= 7 {
        4
    } else {
        5
    };
    let active = input.active.contains(&item.id);
    let in_progress = item.status == "in_progress" || item.tracked_seconds > 0;
    let mut reasons = Vec::new();
    if active {
        reasons.push("active".to_owned());
    }
    if item.pinned {
        reasons.push("pinned".to_owned());
    }
    if in_progress && !active {
        reasons.push("in_progress".to_owned());
    }
    if bucket == 0 {
        reasons.push("no_slack".to_owned());
    } else if bucket <= 2 {
        reasons.push("due_soon".to_owned());
    }
    if factor > 1.001 {
        reasons.push(format!("estimate_x{factor}"));
    }
    Ranked {
        item,
        due,
        effective_due,
        remaining,
        slack,
        bucket,
        factor,
        active,
        in_progress,
        reasons,
    }
}

fn ranks(input: &Input, zone: Tz) -> (Vec<Ranked>, Vec<Unplanned>) {
    let mut queue = Vec::new();
    let mut unplanned = Vec::new();
    for item in &input.items {
        if !eligible(item, &input.kind) {
            continue;
        }
        let due = due(item, &input.kind);
        let missing_plan = input.kind == "work" && item.planned_seconds <= 0;
        if due.is_none() || missing_plan {
            unplanned.push(Unplanned {
                item: item.clone(),
                missing_dev_due: due.is_none(),
                missing_plan,
            });
            continue;
        }
        let entry = rank(input, zone, item.clone(), due.unwrap());
        if input.kind == "work" && entry.remaining <= 0 {
            continue;
        }
        queue.push(entry);
    }
    queue.sort_by(|a, b| {
        b.active
            .cmp(&a.active)
            .then_with(|| b.item.pinned.cmp(&a.item.pinned))
            .then_with(|| b.in_progress.cmp(&a.in_progress))
            .then_with(|| a.bucket.cmp(&b.bucket))
            .then_with(|| b.item.important.cmp(&a.item.important))
            .then_with(|| b.item.urgent.cmp(&a.item.urgent))
            .then_with(|| a.effective_due.cmp(&b.effective_due))
            .then_with(|| {
                b.item
                    .stress
                    .unwrap_or(-1)
                    .cmp(&a.item.stress.unwrap_or(-1))
            })
            .then_with(|| {
                if input.settings.oldest_first {
                    a.item.created_at.cmp(&b.item.created_at)
                } else {
                    b.item.created_at.cmp(&a.item.created_at)
                }
            })
    });
    unplanned.sort_by(|a, b| {
        b.item
            .pinned
            .cmp(&a.item.pinned)
            .then_with(|| quadrant_rank(&a.item.quadrant).cmp(&quadrant_rank(&b.item.quadrant)))
            .then_with(
                || match (due(&a.item, &input.kind), due(&b.item, &input.kind)) {
                    (Some(a), Some(b)) => a.cmp(&b),
                    (Some(_), None) => std::cmp::Ordering::Less,
                    (None, Some(_)) => std::cmp::Ordering::Greater,
                    (None, None) => std::cmp::Ordering::Equal,
                },
            )
            .then_with(|| {
                b.item
                    .stress
                    .unwrap_or(-1)
                    .cmp(&a.item.stress.unwrap_or(-1))
            })
            .then_with(|| {
                if input.settings.oldest_first {
                    a.item.created_at.cmp(&b.item.created_at)
                } else {
                    b.item.created_at.cmp(&a.item.created_at)
                }
            })
    });
    (queue, unplanned)
}

fn quadrant_rank(value: &str) -> i32 {
    match value {
        "do" => 0,
        "schedule" => 1,
        "delegate" => 2,
        _ => 3,
    }
}

fn buffered(input: &Input, event: &EventOccurrence) -> Span {
    let active = event_active(event);
    let buffer = Duration::minutes(i64::from(input.settings.meeting_buffer_min));
    Span {
        start: active.start - buffer,
        end: active.end + buffer,
    }
}

fn free(input: &Input, zone: Tz) -> (Vec<Span>, Vec<Span>) {
    let mut hard = Vec::new();
    let mut soft = Vec::new();
    for event in &input.events {
        let span = buffered(input, event);
        if input.settings.soft_busy && event.can_skip {
            soft.push(span);
        } else {
            hard.push(span);
        }
    }
    hard = merge(hard);
    let mut day = input.now.with_timezone(&zone).date_naive();
    let mut output = Vec::new();
    for _ in 0..120 {
        for window in work_window(input, zone, day) {
            if let Some(window) = clip(window, input.now, window.end) {
                let mut slots = vec![window];
                for hole in &hard {
                    slots = subtract(&slots, *hole);
                }
                output.extend(slots);
            }
        }
        day = day.succ_opt().unwrap();
    }
    (output, soft)
}

fn grid(input: &Input, zone: Tz) -> Grid {
    let mut start = (input.settings.work_start_min / 60 - 2).max(0);
    let mut end = ((input.settings.work_end_min + 59) / 60).min(24);
    for (day, row) in &input.overrides {
        let stamp = local_at(zone, *day, 0);
        if stamp < local_at(zone, input.from.with_timezone(&zone).date_naive(), 0)
            || stamp >= input.to
        {
            continue;
        }
        if let Some((first, last)) = row.start.zip(row.end) {
            start = start.min(first / 60);
            end = end.max((last + 59) / 60);
        }
    }
    if end <= start {
        end = start + 1;
    }
    Grid {
        timezone: input.settings.timezone.clone(),
        start_hour: start,
        end_hour: end,
        workdays: input.settings.workdays.clone(),
        work_start_min: input.settings.work_start_min,
        work_end_min: input.settings.work_end_min,
    }
}

fn busy(input: &Input, zone: Tz) -> Vec<Busy> {
    let mut out = Vec::new();
    for event in &input.events {
        let active = event_active(event);
        let mut day = event.starts_at.with_timezone(&zone).date_naive();
        let last = event.ends_at.with_timezone(&zone).date_naive();
        while day <= last {
            let mut first = (input.settings.work_start_min / 60 - 2).max(0);
            let mut last_hour = ((input.settings.work_end_min + 59) / 60).min(24);
            if let Some((start, end)) = input
                .overrides
                .get(&day)
                .and_then(|row| row.start.zip(row.end))
            {
                first = first.min(start / 60);
                last_hour = last_hour.max((end + 59) / 60);
            }
            let bounds = Span {
                start: local_at(zone, day, first * 60),
                end: local_at(zone, day, last_hour * 60),
            };
            if let Some(slot) = clip(
                Span {
                    start: event.starts_at,
                    end: event.ends_at,
                },
                bounds.start,
                bounds.end,
            )
            .and_then(|slot| clip(slot, input.from, input.to))
            {
                let visible_active = clip(active, slot.start, slot.end);
                let ids: Vec<Uuid> = event.people.iter().map(|person| person.id).collect();
                out.push(Busy {
                    starts_at: slot.start,
                    ends_at: slot.end,
                    title: event.title.clone(),
                    soft: input.settings.soft_busy && event.can_skip,
                    series_id: Some(event.series_id),
                    original_on: Some(event.original_on),
                    active_starts_at: visible_active.map(|span| span.start),
                    active_ends_at: visible_active.map(|span| span.end),
                    people: named(&ids, &input.names),
                    can_skip: event.can_skip,
                });
            }
            day = day.succ_opt().unwrap();
        }
    }
    if let Some(pause) = &input.settings.break_window {
        let mut day = input.from.with_timezone(&zone).date_naive();
        let last = input.to.with_timezone(&zone).date_naive();
        while day <= last {
            if !work_window(input, zone, day).is_empty() {
                let span = Span {
                    start: local_at(zone, day, pause.start_min),
                    end: local_at(zone, day, pause.end_min),
                };
                if let Some(span) = clip(span, input.from, input.to) {
                    out.push(Busy {
                        starts_at: span.start,
                        ends_at: span.end,
                        title: "Break".to_owned(),
                        soft: false,
                        series_id: None,
                        original_on: None,
                        active_starts_at: None,
                        active_ends_at: None,
                        people: Vec::new(),
                        can_skip: false,
                    });
                }
            }
            day = day.succ_opt().unwrap();
        }
    }
    out.sort_by(|a, b| {
        a.starts_at
            .cmp(&b.starts_at)
            .then_with(|| a.title.cmp(&b.title))
    });
    out
}

fn golden_weights(input: &Input, zone: Tz) -> Option<[f64; 24]> {
    if !input.settings.golden_hours {
        return None;
    }
    let mut weights = [0.0; 24];
    let mut total = 0.0;
    for span in &input.history {
        let mut cursor = span.start;
        while cursor < span.end {
            let local = cursor.with_timezone(&zone);
            let remaining = 3600 - i64::from(local.minute() * 60 + local.second());
            let next = (cursor + Duration::seconds(remaining)).min(span.end);
            let amount = (next - cursor).num_seconds() as f64;
            weights[local.hour() as usize] += amount;
            total += amount;
            cursor = next;
        }
    }
    if total < 3600.0 {
        return None;
    }
    let max = weights.iter().copied().fold(0.0, f64::max);
    if max == 0.0 {
        return None;
    }
    for weight in &mut weights {
        *weight /= max;
    }
    Some(weights)
}

#[derive(Clone)]
struct Pack {
    free: Vec<Span>,
    tracks: Vec<Vec<Span>>,
    soft: Vec<Span>,
    blocks: Vec<Block>,
    left: HashMap<Uuid, i64>,
    focus: HashMap<NaiveDate, i64>,
    tasks: HashMap<NaiveDate, HashSet<Uuid>>,
    last_project: String,
    energy_zone: Option<Span>,
    golden: Option<[f64; 24]>,
}

impl Pack {
    fn new(input: &Input, zone: Tz, free: &[Span], soft: &[Span], queue: &[Ranked]) -> Self {
        let tracks = vec![free.to_vec(); (1 + input.settings.waiting_tracks).max(1) as usize];
        let light = queue.iter().any(|r| {
            r.remaining <= 3600 || matches!(r.item.quadrant.as_str(), "schedule" | "delegate")
        });
        let energy_zone = if input.settings.energy_aware
            && light
            && input
                .checkins
                .is_some_and(|(focus, energy)| focus <= 2 || energy <= 2)
        {
            work_window(input, zone, input.now.with_timezone(&zone).date_naive())
                .first()
                .and_then(|window| {
                    let span = Span {
                        start: window.start,
                        end: window.start + Duration::hours(2),
                    };
                    (input.now < span.end).then_some(span)
                })
        } else {
            None
        };
        let golden = golden_weights(input, zone);
        Self {
            free: free.to_vec(),
            tracks,
            soft: soft.to_vec(),
            blocks: Vec::new(),
            left: queue.iter().map(|r| (r.item.id, r.remaining)).collect(),
            focus: HashMap::new(),
            tasks: HashMap::new(),
            last_project: String::new(),
            energy_zone,
            golden,
        }
    }

    fn occupy(&mut self, occupancy: &str, track: usize, span: Span) {
        self.free = subtract(&self.free, span);
        if occupancy == "solo" {
            for slots in &mut self.tracks {
                *slots = subtract(slots, span);
            }
        } else if let Some(slots) = self.tracks.get_mut(track) {
            *slots = subtract(slots, span);
        }
    }

    fn view(
        &self,
        input: &Input,
        zone: Tz,
        r: &Ranked,
        from: DateTime<Utc>,
        include_soft: bool,
    ) -> (Vec<Span>, usize) {
        let occupancy = r.item.occupancy.as_str();
        let track = if occupancy == "waiting" && self.tracks.len() > 1 {
            (1..self.tracks.len())
                .filter_map(|index| {
                    self.tracks[index]
                        .iter()
                        .find(|span| span.end > from)
                        .map(|span| (index, span.start.max(from)))
                })
                .min_by_key(|(_, start)| *start)
                .map(|(index, _)| index)
                .unwrap_or(1)
        } else {
            0
        };
        let mut view = if occupancy == "solo" {
            self.free.clone()
        } else {
            self.tracks[track].clone()
        };
        if occupancy != "solo" {
            for block in &self.blocks {
                if block.item_id == r.item.id {
                    view = subtract(
                        &view,
                        Span {
                            start: block.starts_at,
                            end: block.ends_at,
                        },
                    );
                }
            }
        }
        if !include_soft {
            for span in &self.soft {
                view = subtract(&view, *span);
            }
        }
        for absence in &input.absences {
            if !r.item.person_ids.contains(&absence.person) {
                continue;
            }
            let mut day = absence.start;
            while day <= absence.end {
                view = subtract(
                    &view,
                    Span {
                        start: local_at(zone, day, 0),
                        end: local_at(zone, day.succ_opt().unwrap(), 0),
                    },
                );
                day = day.succ_opt().unwrap();
            }
        }
        for event in &input.events {
            if input.settings.soft_busy && event.can_skip {
                continue;
            }
            if event
                .people
                .iter()
                .any(|rel| r.item.person_ids.contains(&rel.id))
            {
                view = subtract(&view, buffered(input, event));
            }
        }
        (view, track)
    }

    fn day_allows(&self, input: &Input, day: NaiveDate, r: &Ranked) -> bool {
        if input.settings.max_tasks_per_day > 0
            && !self
                .tasks
                .get(&day)
                .is_some_and(|set| set.contains(&r.item.id))
            && self
                .tasks
                .get(&day)
                .is_some_and(|set| set.len() >= input.settings.max_tasks_per_day as usize)
        {
            return false;
        }
        r.item.occupancy == "waiting"
            || input.settings.daily_focus_min <= 0
            || self.focus.get(&day).copied().unwrap_or(0)
                < i64::from(input.settings.daily_focus_min) * 60
    }

    fn commit(
        &mut self,
        input: &Input,
        zone: Tz,
        r: &Ranked,
        span: Span,
        track: usize,
        mut extra: Vec<String>,
    ) -> i64 {
        let occupancy = match r.item.occupancy.as_str() {
            "parallel" => "parallel",
            "waiting" => "waiting",
            _ => "solo",
        };
        let day = span.start.with_timezone(&zone).date_naive();
        let left = self.left.get(&r.item.id).copied().unwrap_or(0);
        let mut reasons = r.reasons.clone();
        reasons.append(&mut extra);
        if let Some(energy) = self.energy_zone {
            if (r.remaining <= 3600 || matches!(r.item.quadrant.as_str(), "schedule" | "delegate"))
                && energy.start < span.end
                && energy.end > span.start
            {
                reasons.push("light_task".to_owned());
            }
        }
        for event in &input.events {
            if input.settings.soft_busy
                && event.can_skip
                && buffered(input, event).start < span.end
                && buffered(input, event).end > span.start
            {
                reasons.push(format!("soft_busy:{}", event.title));
            }
        }
        let block = Block {
            item_id: r.item.id,
            title: r.item.title.clone(),
            external_key: r.item.external_key.clone(),
            kind: "work".to_owned(),
            starts_at: span.start,
            ends_at: span.end,
            late: span.end > r.due,
            continued: false,
            continues: false,
            lane: track,
            occupancy: occupancy.to_owned(),
            pinned: r.item.pinned,
            quadrant: r.item.quadrant.clone(),
            due_at: r.due,
            stress: r.item.stress,
            remaining_seconds: r.remaining,
            estimate_factor: r.factor,
            reasons,
            people: named(&r.item.person_ids, &input.names),
        };
        self.occupy(occupancy, track, span);
        let used = seconds(span);
        self.tasks.entry(day).or_default().insert(r.item.id);
        if occupancy != "waiting" {
            *self.focus.entry(day).or_default() += used;
        }
        self.left.insert(r.item.id, (left - used).max(0));
        self.last_project = r
            .item
            .project_id
            .map(|id| id.to_string())
            .unwrap_or_default();
        self.blocks.push(block);
        used
    }

    fn better_gap(
        &self,
        input: &Input,
        zone: Tz,
        r: &Ranked,
        view: &[Span],
        start: DateTime<Utc>,
        need: i64,
    ) -> (DateTime<Utc>, i64, Vec<String>) {
        let day = start.with_timezone(&zone).date_naive();
        let day_end = local_at(zone, day.succ_opt().unwrap(), 0);
        let want = if input.settings.max_slice_min > 0 {
            need.min(i64::from(input.settings.max_slice_min) * 60)
        } else {
            need
        };
        let mut candidates = Vec::new();
        for slot in view {
            if slot.end <= start || slot.start >= day_end {
                continue;
            }
            let mut at = grid_round(zone, slot.start.max(start), input.settings.grid_min);
            if let Some(energy) = self.energy_zone {
                if r.remaining > 3600
                    && !matches!(r.item.quadrant.as_str(), "schedule" | "delegate")
                    && at >= energy.start
                    && at < energy.end
                {
                    at = grid_round(zone, energy.end, input.settings.grid_min);
                }
            }
            if at < slot.end {
                candidates.push((at, (slot.end - at).num_seconds()));
            }
        }
        if candidates.is_empty() {
            return (start, 0, Vec::new());
        }
        let mut current = candidates[0];
        if (input.settings.max_slice_min == 0
            || need <= i64::from(input.settings.max_slice_min) * 60)
            && current.1 < need
        {
            if let Some(found) = candidates
                .iter()
                .skip(1)
                .find(|(_, length)| *length >= need)
            {
                current = *found;
            }
        }
        let Some(weights) = self.golden else {
            return (current.0, current.1, Vec::new());
        };
        if r.item.quadrant != "do" {
            return (current.0, current.1, Vec::new());
        }
        let mut best = current;
        let mut weight = weights[current.0.with_timezone(&zone).hour() as usize];
        for (at, length) in candidates {
            let mut starts = vec![at];
            let mut hour = at.with_timezone(&zone).hour() + 1;
            while hour < 24 {
                let candidate = local_at(zone, day, (hour * 60) as i32);
                if candidate + Duration::seconds(want) > at + Duration::seconds(length) {
                    break;
                }
                starts.push(candidate);
                hour += 1;
            }
            for candidate in starts {
                let remaining = (at + Duration::seconds(length) - candidate).num_seconds();
                if remaining < want && remaining < current.1 {
                    continue;
                }
                if input.settings.min_slice_min > 0
                    && remaining < i64::from(input.settings.min_slice_min) * 60
                    && need >= i64::from(input.settings.min_slice_min) * 60
                {
                    continue;
                }
                if candidate + Duration::seconds(remaining.min(want)) > r.due {
                    continue;
                }
                let candidate_weight = weights[candidate.with_timezone(&zone).hour() as usize];
                if candidate_weight > weight + 0.1 {
                    best = (candidate, remaining);
                    weight = candidate_weight;
                }
            }
        }
        (
            best.0,
            best.1,
            if best.0 != current.0 {
                vec!["golden_hour".to_owned()]
            } else {
                Vec::new()
            },
        )
    }

    fn place(
        &mut self,
        input: &Input,
        zone: Tz,
        r: &Ranked,
        from: DateTime<Utc>,
        include_soft: bool,
        extra: &[String],
    ) {
        let mut cursor = from.max(input.now);
        for _ in 0..960 {
            let left = self.left.get(&r.item.id).copied().unwrap_or(0);
            if left <= 0 {
                break;
            }
            let (view, track) = self.view(input, zone, r, cursor, include_soft);
            let mut chosen = None;
            let mut next_day = false;
            for slot in &view {
                if slot.end <= cursor {
                    continue;
                }
                let mut start = grid_round(zone, slot.start.max(cursor), input.settings.grid_min);
                if start >= slot.end {
                    continue;
                }
                let day = start.with_timezone(&zone).date_naive();
                if !self.day_allows(input, day, r) {
                    cursor = local_at(zone, day.succ_opt().unwrap(), 0);
                    next_day = true;
                    break;
                }
                if let Some(energy) = self.energy_zone {
                    if r.remaining > 3600
                        && !matches!(r.item.quadrant.as_str(), "schedule" | "delegate")
                        && start >= energy.start
                        && start < energy.end
                    {
                        start = grid_round(zone, energy.end, input.settings.grid_min);
                    }
                }
                if start >= slot.end {
                    continue;
                }
                let available = (slot.end - start).num_seconds();
                if input.settings.min_slice_min > 0
                    && available < i64::from(input.settings.min_slice_min) * 60
                    && left >= i64::from(input.settings.min_slice_min) * 60
                {
                    continue;
                }
                let (start, available, reasons) =
                    self.better_gap(input, zone, r, &view, start, left);
                let mut take = available.min(left);
                if input.settings.max_slice_min > 0 {
                    take = take.min(i64::from(input.settings.max_slice_min) * 60);
                }
                if r.item.occupancy != "waiting" && input.settings.daily_focus_min > 0 {
                    let remaining = i64::from(input.settings.daily_focus_min) * 60
                        - self.focus.get(&day).copied().unwrap_or(0);
                    if remaining > 0 {
                        let min = i64::from(input.settings.min_slice_min) * 60;
                        if take > remaining && take - remaining >= min {
                            take = remaining.max(min);
                        }
                    }
                }
                if input.settings.min_slice_min > 0
                    && take < i64::from(input.settings.min_slice_min) * 60
                    && left >= i64::from(input.settings.min_slice_min) * 60
                {
                    continue;
                }
                let step = i64::from(input.settings.grid_min.max(1)) * 60;
                take = if take >= left {
                    ((left + step - 1) / step) * step
                } else {
                    (take / step) * step
                };
                take = take.min(available);
                if take <= 0 {
                    continue;
                }
                let continues = take < left && available - take >= step;
                chosen = Some((
                    Span {
                        start,
                        end: start + Duration::seconds(take),
                    },
                    track,
                    continues,
                    reasons,
                ));
                break;
            }
            if next_day {
                continue;
            }
            let Some((span, track, continues, reasons)) = chosen else {
                break;
            };
            let mut all = reasons;
            all.extend_from_slice(extra);
            self.commit(input, zone, r, span, track, all);
            cursor = span.end;
            if continues
                && self.left.get(&r.item.id).copied().unwrap_or(0) > 0
                && input.settings.slice_break_min > 0
            {
                let pause = Span {
                    start: cursor,
                    end: cursor + Duration::minutes(i64::from(input.settings.slice_break_min)),
                };
                self.occupy("solo", 0, pause);
                cursor = pause.end;
            }
        }
    }

    fn late(&self, item: Uuid, due: DateTime<Utc>) -> i64 {
        self.blocks
            .iter()
            .filter(|block| block.item_id == item)
            .map(|block| {
                seconds(Span {
                    start: block.starts_at.max(due),
                    end: block.ends_at,
                })
            })
            .sum()
    }
}

fn pack_with(
    input: &Input,
    zone: Tz,
    queue: &[Ranked],
    free: &[Span],
    soft: &[Span],
    sticky: &[(Uuid, Span)],
) -> Vec<Block> {
    let mut pack = Pack::new(input, zone, free, soft, queue);
    for r in queue {
        if r.active && r.remaining > 0 {
            let max = if input.settings.max_slice_min > 0 {
                i64::from(input.settings.max_slice_min) * 60
            } else {
                r.remaining
            };
            let step = i64::from(input.settings.grid_min.max(1)) * 60;
            let take = ((r.remaining.min(max) + step - 1) / step) * step;
            pack.commit(
                input,
                zone,
                r,
                Span {
                    start: input.now,
                    end: input.now + Duration::seconds(take),
                },
                0,
                vec!["active".to_owned()],
            );
        }
    }
    for (item_id, reservation) in sticky {
        let Some(r) = queue.iter().find(|r| r.item.id == *item_id) else {
            continue;
        };
        let left = pack.left.get(item_id).copied().unwrap_or(0);
        if left <= 0 {
            continue;
        }
        let span = Span {
            start: reservation.start,
            end: reservation
                .end
                .min(reservation.start + Duration::seconds(left)),
        };
        let (view, track) = pack.view(input, zone, r, span.start, false);
        if span.end <= span.start
            || !view
                .iter()
                .any(|slot| slot.start <= span.start && slot.end >= span.end)
            || !pack.day_allows(
                input,
                zone.from_utc_datetime(&span.start.naive_utc()).date_naive(),
                r,
            )
        {
            continue;
        }
        pack.commit(input, zone, r, span, track, vec!["sticky".to_owned()]);
    }
    let mut pinned: Vec<&Ranked> = queue
        .iter()
        .filter(|r| r.item.pinned_at.is_some_and(|at| at >= input.now))
        .collect();
    pinned.sort_by_key(|r| r.item.pinned_at);
    for r in pinned {
        pack.place(
            input,
            zone,
            r,
            r.item.pinned_at.unwrap(),
            false,
            &["pinned_at".to_owned()],
        );
    }
    let mut remaining: Vec<&Ranked> = queue
        .iter()
        .filter(|r| pack.left.get(&r.item.id).copied().unwrap_or(0) > 0)
        .collect();
    pack.last_project.clear();
    while !remaining.is_empty() {
        let head = remaining[0];
        let selected = remaining
            .iter()
            .take_while(|r| {
                r.active == head.active
                    && r.item.pinned == head.item.pinned
                    && r.in_progress == head.in_progress
                    && r.bucket == head.bucket
            })
            .position(|r| {
                r.item
                    .project_id
                    .map(|id| id.to_string())
                    .unwrap_or_default()
                    == pack.last_project
            })
            .unwrap_or(0);
        let r = remaining.remove(selected);
        let extra = if selected > 0 {
            vec!["same_project".to_owned()]
        } else {
            Vec::new()
        };
        let mut trial = pack.clone();
        trial.place(input, zone, r, input.now, false, &extra);
        if input.settings.soft_busy
            && !soft.is_empty()
            && (trial.left.get(&r.item.id).copied().unwrap_or(0) > 0
                || trial.late(r.item.id, r.due) > 0)
        {
            let mut alternative = pack.clone();
            alternative.place(input, zone, r, input.now, true, &extra);
            if alternative.late(r.item.id, r.due) < trial.late(r.item.id, r.due)
                || (alternative.late(r.item.id, r.due) == trial.late(r.item.id, r.due)
                    && alternative.left.get(&r.item.id).copied().unwrap_or(0)
                        < trial.left.get(&r.item.id).copied().unwrap_or(0))
            {
                trial = alternative;
            }
        }
        pack = trial;
    }
    pack.blocks
}

fn pack_work(
    input: &Input,
    zone: Tz,
    queue: &[Ranked],
    free: &[Span],
    soft: &[Span],
) -> Vec<Block> {
    let greedy = pack_with(input, zone, queue, free, soft, &[]);
    if input.previous.is_empty() || input.settings.stability_threshold_min <= 0 {
        return greedy;
    }
    let mut sticky = Vec::new();
    for r in queue {
        if r.active {
            continue;
        }
        let Some(fresh) = greedy
            .iter()
            .filter(|block| block.item_id == r.item.id)
            .map(|block| block.starts_at)
            .min()
        else {
            continue;
        };
        let mut prior: Vec<&Block> = input
            .previous
            .iter()
            .filter(|block| {
                block.item_id == r.item.id && block.kind != "ping" && block.starts_at > input.now
            })
            .collect();
        prior.sort_by_key(|block| block.starts_at);
        let Some(first) = prior.first() else {
            continue;
        };
        if first.ends_at > r.due
            || (first.starts_at - fresh).num_minutes()
                >= i64::from(input.settings.stability_threshold_min)
        {
            continue;
        }
        for block in prior {
            if block.ends_at > r.due {
                break;
            }
            sticky.push((
                r.item.id,
                Span {
                    start: block.starts_at,
                    end: block.ends_at,
                },
            ));
        }
    }
    if sticky.is_empty() {
        return greedy;
    }
    sticky.sort_by_key(|(_, span)| span.start);
    pack_with(input, zone, queue, free, soft, &sticky)
}

fn pack_followup(input: &Input, zone: Tz, queue: &[Ranked], free: &[Span]) -> Vec<Block> {
    let mut slots = free.to_vec();
    let mut blocks = Vec::new();
    let ping = Duration::minutes(i64::from(input.settings.followup_ping_min.max(1)));
    let windows: Vec<i32> = input
        .settings
        .check_windows
        .iter()
        .filter_map(|raw| {
            raw.split_once(':')
                .and_then(|(h, m)| h.parse::<i32>().ok().zip(m.parse::<i32>().ok()))
                .map(|(h, m)| h * 60 + m)
        })
        .collect();
    for r in queue {
        let mut day = input.now.with_timezone(&zone).date_naive();
        let until = if r.due < input.now {
            input.to.max(input.now).with_timezone(&zone).date_naive()
        } else {
            r.due.with_timezone(&zone).date_naive()
        };
        let mut placed = 0;
        let mut carry = Vec::new();
        for search in 0..150 {
            if search >= 30 && placed == 0 {
                break;
            }
            if placed > 0 && day > until {
                break;
            }
            if work_window(input, zone, day).is_empty() {
                day = day.succ_opt().unwrap();
                continue;
            }
            let mut away = false;
            for absence in &input.absences {
                if r.item.person_ids.contains(&absence.person)
                    && day >= absence.start
                    && day <= absence.end
                {
                    let name = input
                        .names
                        .get(&absence.person)
                        .cloned()
                        .unwrap_or_else(|| absence.person.to_string()[..8].to_owned());
                    let reason = format!("person_away:{name}");
                    if !carry.contains(&reason) {
                        carry.push(reason);
                    }
                    away = true;
                }
            }
            if away {
                day = day.succ_opt().unwrap();
                continue;
            }
            let is_today = day == input.now.with_timezone(&zone).date_naive();
            let mut at = windows
                .iter()
                .map(|minute| local_at(zone, day, *minute))
                .find(|at| !is_today || *at >= input.now);
            if at.is_none() && is_today && r.due <= local_at(zone, day.succ_opt().unwrap(), 0) {
                at = Some(grid_round(zone, input.now, input.settings.grid_min));
            }
            if let Some(mut at) = at {
                for _ in 0..8 {
                    let Some(slot) = slots.iter().find(|slot| slot.end > at).copied() else {
                        break;
                    };
                    let start = grid_round(zone, slot.start.max(at), input.settings.grid_min);
                    let end = start + ping;
                    if start.with_timezone(&zone).date_naive() != day {
                        break;
                    }
                    if end > slot.end {
                        at = start + Duration::minutes(1);
                        continue;
                    }
                    let mut person_busy = None;
                    for event in &input.events {
                        if input.settings.soft_busy && event.can_skip {
                            continue;
                        }
                        if let Some(rel) = event
                            .people
                            .iter()
                            .find(|rel| r.item.person_ids.contains(&rel.id))
                        {
                            let span = buffered(input, event);
                            if span.start < end && span.end > start {
                                let name = input
                                    .names
                                    .get(&rel.id)
                                    .cloned()
                                    .unwrap_or_else(|| rel.id.to_string()[..8].to_owned());
                                person_busy = Some((span.end, format!("person_busy:{name}")));
                                break;
                            }
                        }
                    }
                    if let Some((until, reason)) = person_busy {
                        at = until;
                        if !carry.contains(&reason) {
                            carry.push(reason);
                        }
                        continue;
                    }
                    let mut reasons = r.reasons.clone();
                    reasons.append(&mut carry);
                    blocks.push(Block {
                        item_id: r.item.id,
                        title: r.item.title.clone(),
                        external_key: r.item.external_key.clone(),
                        kind: "ping".to_owned(),
                        starts_at: start,
                        ends_at: end,
                        late: end > r.due,
                        continued: false,
                        continues: false,
                        lane: 0,
                        occupancy: "solo".to_owned(),
                        pinned: r.item.pinned,
                        quadrant: r.item.quadrant.clone(),
                        due_at: r.due,
                        stress: r.item.stress,
                        remaining_seconds: 0,
                        estimate_factor: r.factor,
                        reasons,
                        people: named(&r.item.person_ids, &input.names),
                    });
                    slots = subtract(&slots, Span { start, end });
                    placed += 1;
                    break;
                }
            }
            day = day.succ_opt().unwrap();
        }
    }
    blocks
}

fn overflow(queue: &[Ranked], blocks: &[Block], kind: &str) -> Overflow {
    let mut out = Overflow::default();
    for r in queue {
        let mine = blocks.iter().filter(|block| block.item_id == r.item.id);
        let packed: i64 = mine
            .clone()
            .map(|block| {
                seconds(Span {
                    start: block.starts_at,
                    end: block.ends_at,
                })
            })
            .sum();
        let late: i64 = mine
            .map(|block| {
                seconds(Span {
                    start: block.starts_at.max(r.due),
                    end: block.ends_at,
                })
            })
            .sum();
        let left = if kind == "work" {
            (r.remaining - packed).max(0)
        } else {
            0
        };
        if late + left == 0 {
            continue;
        }
        out.item_count += 1;
        out.seconds += late + left;
        out.first_due_at = Some(out.first_due_at.map(|at| at.min(r.due)).unwrap_or(r.due));
    }
    out
}

fn score(blocks: &[Block], overflow: &Overflow, zone: Tz, queue: &[Ranked]) -> Score {
    let mut out = Score {
        late_seconds: overflow.seconds,
        ..Score::default()
    };
    let mut items = HashSet::new();
    let mut previous = String::new();
    let projects: HashMap<Uuid, String> = queue
        .iter()
        .map(|r| {
            (
                r.item.id,
                r.item
                    .project_id
                    .map(|id| id.to_string())
                    .unwrap_or_default(),
            )
        })
        .collect();
    let mut days: HashMap<NaiveDate, f64> = HashMap::new();
    let mut work = 0;
    for block in blocks {
        if block.kind == "ping" {
            continue;
        }
        work += 1;
        items.insert(block.item_id);
        let project = projects.get(&block.item_id).cloned().unwrap_or_default();
        if !previous.is_empty() && project != previous {
            out.switches += 1;
        }
        previous = project;
        if block.occupancy != "waiting" {
            *days
                .entry(block.starts_at.with_timezone(&zone).date_naive())
                .or_default() += seconds(Span {
                start: block.starts_at,
                end: block.ends_at,
            }) as f64
                / 3600.0;
        }
    }
    out.fragments = (work - items.len()).max(0) as i32;
    if days.len() > 1 {
        let mean = days.values().sum::<f64>() / days.len() as f64;
        out.load_variance = ((days
            .values()
            .map(|value| (value - mean).powi(2))
            .sum::<f64>()
            / days.len() as f64)
            * 100.0)
            .round()
            / 100.0;
    }
    out.total = ((out.late_seconds as f64 / 3600.0 * 10.0
        + f64::from(out.fragments)
        + f64::from(out.switches) * 0.5
        + out.load_variance)
        * 100.0)
        .round()
        / 100.0;
    out
}

pub(crate) fn plan(input: &Input) -> Schedule {
    let zone: Tz = input
        .settings
        .timezone
        .parse()
        .unwrap_or(chrono_tz::Europe::Moscow);
    let grid = grid(input, zone);
    if input.to <= input.from {
        return Schedule {
            lanes: Vec::new(),
            unplanned: Vec::new(),
            busy: Vec::new(),
            overflow: Overflow::default(),
            capacity: Capacity::default(),
            grid,
            at_risk: Vec::new(),
            score: Score::default(),
            horizon: Vec::new(),
        };
    }
    let (queue, unplanned) = ranks(input, zone);
    let (free, soft) = free(input, zone);
    let mut horizon = if input.kind == "followup" {
        pack_followup(input, zone, &queue, &free)
    } else {
        pack_work(input, zone, &queue, &free, &soft)
    };
    horizon.sort_by(|a, b| {
        a.starts_at
            .cmp(&b.starts_at)
            .then_with(|| a.lane.cmp(&b.lane))
            .then_with(|| a.item_id.cmp(&b.item_id))
    });
    let mut counts = HashMap::new();
    for block in &horizon {
        *counts.entry(block.item_id).or_insert(0usize) += 1;
    }
    let mut seen = HashMap::new();
    for block in &mut horizon {
        let count = seen.entry(block.item_id).or_insert(0usize);
        *count += 1;
        block.continued = *count > 1;
        block.continues = *count < counts[&block.item_id];
    }
    let rank_of: HashMap<Uuid, usize> = queue
        .iter()
        .enumerate()
        .map(|(index, r)| (r.item.id, index))
        .collect();
    let placed = horizon.clone();
    let pause = Duration::minutes(i64::from(input.settings.slice_break_min));
    for block in &mut horizon {
        if block.kind == "ping"
            || block
                .reasons
                .iter()
                .any(|reason| matches!(reason.as_str(), "active" | "pinned_at" | "sticky"))
        {
            continue;
        }
        if let Some(earlier) = placed.iter().find(|other| {
            other.item_id != block.item_id
                && other.kind != "ping"
                && (other.ends_at == block.starts_at || other.ends_at + pause == block.starts_at)
                && rank_of.get(&other.item_id) < rank_of.get(&block.item_id)
        }) {
            let label = if earlier.external_key.is_empty() {
                &earlier.title
            } else {
                &earlier.external_key
            };
            let reason = format!("pushed_by:{label}");
            if !block.reasons.contains(&reason) {
                block.reasons.push(reason);
            }
        }
    }
    let overflow = overflow(&queue, &horizon, &input.kind);
    let mut at_risk: Vec<Risk> = queue
        .iter()
        .filter(|r| r.slack < 0)
        .map(|r| Risk {
            item_id: r.item.id,
            key: r.item.external_key.clone(),
            title: r.item.title.clone(),
            slack_seconds: r.slack,
        })
        .collect();
    at_risk.sort_by_key(|risk| risk.slack_seconds);
    let score = score(&horizon, &overflow, zone, &queue);
    let visible: Vec<Block> = horizon
        .iter()
        .filter(|block| block.ends_at > input.from && block.starts_at < input.to)
        .cloned()
        .collect();
    let mut lanes: HashMap<Option<Uuid>, Lane> = HashMap::new();
    let by_id: HashMap<Uuid, &Item> = queue.iter().map(|r| (r.item.id, &r.item)).collect();
    for block in &visible {
        let item = by_id[&block.item_id];
        lanes
            .entry(item.project_id)
            .or_insert_with(|| Lane {
                project_id: item.project_id,
                project_name: if item.project_name.is_empty() {
                    "No project".to_owned()
                } else {
                    item.project_name.clone()
                },
                project_color: item.project_color.clone(),
                blocks: Vec::new(),
            })
            .blocks
            .push(block.clone());
    }
    let mut lanes: Vec<Lane> = lanes.into_values().collect();
    lanes.sort_by(|a, b| {
        a.project_id
            .is_none()
            .cmp(&b.project_id.is_none())
            .then_with(|| a.project_name.cmp(&b.project_name))
            .then_with(|| a.project_id.cmp(&b.project_id))
    });
    let mut capacity = Capacity::default();
    for slot in &free {
        if let Some(span) = clip(*slot, input.from, input.to) {
            capacity.free_seconds += seconds(span);
        }
    }
    for block in &visible {
        if let Some(span) = clip(
            Span {
                start: block.starts_at,
                end: block.ends_at,
            },
            input.from,
            input.to,
        ) {
            capacity.packed_seconds += seconds(span);
        }
    }
    for event in &input.events {
        if let Some(span) = clip(event_active(event), input.from, input.to) {
            capacity.busy_seconds += seconds(span);
        }
    }
    Schedule {
        lanes,
        unplanned,
        busy: busy(input, zone),
        overflow,
        capacity,
        grid,
        at_risk,
        score,
        horizon,
    }
}
