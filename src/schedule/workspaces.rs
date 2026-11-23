use super::{database_error, error, input, running_intervals, schedule_plan, AppState, Input};
use crate::http::not_found;
use crate::schedule_settings::{self, Settings};
use crate::workspace::{self, Scope};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::Json;
use chrono::{DateTime, Days, NaiveDate, Timelike, Utc};
use chrono_tz::Tz;
use serde::Serialize;
use uuid::Uuid;

#[derive(Clone, Copy)]
pub(super) struct Window {
    pub from: DateTime<Utc>,
    pub to: DateTime<Utc>,
    pub now: DateTime<Utc>,
    pub horizon_end: DateTime<Utc>,
}

#[allow(clippy::result_large_err)]
fn timezone(settings: &Settings) -> Result<Tz, Response> {
    settings.timezone.parse().map_err(|_| {
        error(
            StatusCode::INTERNAL_SERVER_ERROR,
            "invalid schedule timezone",
        )
    })
}

#[allow(clippy::result_large_err)]
fn window(
    zone: Tz,
    plan: &str,
    from: DateTime<Utc>,
    to: DateTime<Utc>,
    now: DateTime<Utc>,
) -> Result<Window, Response> {
    let today = now.with_timezone(&zone).date_naive();
    let last = today + Days::new(if plan == "paid" { 28 } else { 1 });
    let horizon_end = schedule_plan::local_at(zone, last, 0);
    let from = from.max(schedule_plan::local_at(zone, today, 0));
    let to = to.min(horizon_end);
    if to <= from {
        return Err(error(StatusCode::FORBIDDEN, "dates outside workspace plan"));
    }
    Ok(Window {
        from,
        to,
        now,
        horizon_end,
    })
}

pub(super) async fn personal(
    state: &AppState,
    scope: &Scope,
    from: DateTime<Utc>,
    to: DateTime<Utc>,
    kind: String,
) -> Result<Input, Response> {
    let settings = schedule_settings::settings(state).await?;
    let range = window(
        timezone(&settings)?,
        &scope.plan,
        from,
        to,
        Utc::now().with_nanosecond(0).unwrap(),
    )?;
    member_input(state, scope, range, kind, settings).await
}

// The caller supplies the personal scope; team data is fetched under each verified membership.
async fn member_input(
    state: &AppState,
    personal: &Scope,
    window: Window,
    kind: String,
    settings: Settings,
) -> Result<Input, Response> {
    let mut result = input(state, window, kind, settings).await?;
    let team_ids: Vec<Uuid> = sqlx::query_scalar(
        "SELECT m.workspace_id FROM workspace_members m JOIN workspaces w ON w.id=m.workspace_id
         WHERE m.user_id=$1 AND w.kind='team' ORDER BY m.workspace_id",
    )
    .bind(personal.user_id)
    .fetch_all(state.auth.pool())
    .await
    .map_err(database_error)?;
    for team_id in team_ids {
        let team = workspace::access(state.auth.pool(), personal.user_id, Some(team_id))
            .await
            .map_err(not_found)?;
        workspace::in_scope(team, add_assignments(state, personal.user_id, &mut result)).await?;
    }
    Ok(result)
}

async fn add_assignments(
    state: &AppState,
    user_id: Uuid,
    result: &mut Input,
) -> Result<(), Response> {
    let mut items = crate::items::open_for(&state.pool, Some(user_id))
        .await
        .map_err(database_error)?;
    if !items.is_empty() {
        result
            .active
            .extend(running_intervals(state, &mut items, result.now).await?);
    }
    let start = result.now.min(result.from);
    let events =
        crate::events::occurrences_for(&state.pool, start, result.horizon_end, Some(user_id))
            .await
            .map_err(database_error)?;
    let contacts: Vec<Uuid> = items
        .iter()
        .flat_map(|item| item.person_ids.iter().copied())
        .chain(
            events
                .iter()
                .flat_map(|event| event.people.iter().map(|person| person.id)),
        )
        .collect();
    if !contacts.is_empty() {
        let names =
            sqlx::query_as::<_, (Uuid, String)>("SELECT id,name FROM people WHERE id=ANY($1)")
                .bind(&contacts)
                .fetch_all(&state.pool)
                .await
                .map_err(database_error)?;
        result.names.extend(names);
        let zone = timezone(&result.settings)?;
        let absences = sqlx::query_as::<_, (Uuid, NaiveDate, NaiveDate)>(
            "SELECT person_id,starts_on,ends_on FROM person_absences
             WHERE person_id=ANY($1) AND starts_on <= $2 AND ends_on >= $3",
        )
        .bind(&contacts)
        .bind(result.horizon_end.with_timezone(&zone).date_naive())
        .bind(start.with_timezone(&zone).date_naive())
        .fetch_all(&state.pool)
        .await
        .map_err(database_error)?;
        result.absences.extend(
            absences
                .into_iter()
                .map(|(person, start, end)| schedule_plan::Absence { person, start, end }),
        );
    }
    result.items.extend(items);
    result.events.extend(events);
    Ok(())
}

#[derive(Serialize, sqlx::FromRow)]
#[serde(rename_all = "camelCase")]
struct TeamItem {
    id: Uuid,
    title: String,
    assignee_user_id: Option<Uuid>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct TeamEvent {
    id: Uuid,
    series_id: Uuid,
    original_on: NaiveDate,
    title: String,
    starts_at: DateTime<Utc>,
    ends_at: DateTime<Utc>,
    participant_user_ids: Vec<Uuid>,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct MemberLoad {
    user_id: Uuid,
    days: Vec<schedule_plan::DayLoad>,
}

#[derive(Serialize)]
struct TeamSchedule {
    from: DateTime<Utc>,
    to: DateTime<Utc>,
    timezone: String,
    members: Vec<MemberLoad>,
    items: Vec<TeamItem>,
    events: Vec<TeamEvent>,
}

pub(super) async fn team(
    state: &AppState,
    team: &Scope,
    from: DateTime<Utc>,
    to: DateTime<Utc>,
    kind: String,
) -> Result<Response, Response> {
    let members = sqlx::query_as::<_, (Uuid, String)>(
        "SELECT user_id,role FROM workspace_members WHERE workspace_id=$1 ORDER BY user_id",
    )
    .bind(team.workspace_id)
    .fetch_all(state.auth.pool())
    .await
    .map_err(database_error)?;
    let owner = members
        .iter()
        .find(|(_, role)| role == "owner")
        .ok_or_else(|| error(StatusCode::INTERNAL_SERVER_ERROR, "workspace has no owner"))?
        .0;
    let owner_scope = workspace::access(state.auth.pool(), owner, Some(team.workspace_id))
        .await
        .map_err(not_found)?;
    let settings = workspace::in_scope(owner_scope, schedule_settings::settings(state)).await?;
    let zone = timezone(&settings)?;
    let window = window(
        zone,
        &team.plan,
        from,
        to,
        Utc::now().with_nanosecond(0).unwrap(),
    )?;
    let items = sqlx::query_as::<_, TeamItem>(
        "SELECT id,title,assignee_user_id FROM items
         WHERE archived_at IS NULL AND deleted_at IS NULL AND kind IN ('task','request')
           AND status NOT IN ('done','cancelled') ORDER BY title,id",
    )
    .fetch_all(&state.pool)
    .await
    .map_err(database_error)?;
    let events = crate::events::occurrences(&state.pool, window.from, window.to)
        .await
        .map_err(database_error)?
        .into_iter()
        .map(|event| TeamEvent {
            id: event.id,
            series_id: event.series_id,
            original_on: event.original_on,
            title: event.title,
            starts_at: event.starts_at,
            ends_at: event.ends_at,
            participant_user_ids: event.participant_user_ids,
        })
        .collect();
    // A shorter request must not reveal when someone's private appointment takes place.
    let last_date = (window.to - chrono::Duration::nanoseconds(1))
        .with_timezone(&zone)
        .date_naive();
    let whole_days = Window {
        from: schedule_plan::local_at(zone, window.from.with_timezone(&zone).date_naive(), 0),
        to: schedule_plan::local_at(zone, last_date + Days::new(1), 0),
        ..window
    };
    let mut loads = Vec::with_capacity(members.len());
    for (user_id, _) in members {
        let personal = workspace::access(state.auth.pool(), user_id, None)
            .await
            .map_err(not_found)?;
        let days = workspace::in_scope(personal.clone(), async {
            let settings = schedule_settings::settings(state).await?;
            let input = member_input(state, &personal, whole_days, kind.clone(), settings).await?;
            Ok::<_, Response>(schedule_plan::day_loads(input, team.workspace_id, zone))
        })
        .await?;
        loads.push(MemberLoad { user_id, days });
    }
    Ok(Json(TeamSchedule {
        from: window.from,
        to: window.to,
        timezone: zone.to_string(),
        members: loads,
        items,
        events,
    })
    .into_response())
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::TimeZone;

    #[test]
    fn tariff_uses_calendar_days_during_dst_change() {
        let zone = chrono_tz::America::New_York;
        let now = Utc.with_ymd_and_hms(2026, 11, 1, 12, 0, 0).unwrap();
        let from = Utc.with_ymd_and_hms(2026, 10, 29, 0, 0, 0).unwrap();
        let to = Utc.with_ymd_and_hms(2026, 12, 4, 0, 0, 0).unwrap();
        let free = window(zone, "free", from, to, now).unwrap();
        assert_eq!(
            free.from,
            Utc.with_ymd_and_hms(2026, 11, 1, 4, 0, 0).unwrap()
        );
        assert_eq!(free.to, Utc.with_ymd_and_hms(2026, 11, 2, 5, 0, 0).unwrap());
        assert_eq!(
            window(zone, "paid", from, to, now).unwrap().to,
            Utc.with_ymd_and_hms(2026, 11, 29, 5, 0, 0).unwrap()
        );
        assert!(window(zone, "free", free.to, to, now).is_err());
    }
}
