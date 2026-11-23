import { Button } from '@want-foundation/desyst-react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api'
import { useAccount } from '../../shared/account'
import { span } from '../../shared/format'
import { openEvent, openTask } from '../../shared/taskOverlay'
import { Window } from '../../shared/Window'
import { workspaceUrl } from '../../shared/workspacePath'
import { schedulePage, shiftYmd, zonedStartOfDay, zonedYmd } from '../../shared/zoned'

export function ScheduleScope() {
  const { workspace, workspaces } = useAccount()
  return <label className="workspace-picker">Planning view
    <select value={workspace.id} onChange={(event) => window.location.assign(workspaceUrl(event.target.value, '/schedule'))}>
      {workspaces.map((space) => <option key={space.id} value={space.id}>
        {space.kind === 'personal' ? 'My schedule' : 'Team · ' + space.name}
      </option>)}
    </select>
  </label>
}

export function TeamScheduleScreen() {
  const { workspace, members } = useAccount()
  const navigate = useNavigate()
  const [page, setPage] = useState(0)
  const schedule = useQuery({ queryKey: ['schedule', 'team', workspace.id, workspace.plan], queryFn: () => api.teamSchedule() })
  const { data, refetch } = schedule
  useEffect(() => {
    if (!data) return
    const tomorrow = shiftYmd(zonedYmd(new Date(), data.timezone), 1)
    const delay = zonedStartOfDay(tomorrow, data.timezone).getTime() - Date.now()
    const timer = window.setTimeout(() => { void refetch() }, Math.max(delay + 1000, 1000))
    return () => window.clearTimeout(timer)
  }, [data, refetch])
  const title = 'Team schedule'
  const controls = <div className="sched-actions">
    <ScheduleScope />
    {workspace.role === 'owner' ? <Button variant="ghost" onClick={() => navigate('/schedule/settings')}>Team calendar settings</Button> : null}
  </div>
  if (!data) return <Window title={title} actions={controls}>
    {schedule.error ? <><p className="error" role="alert">{schedule.error.message}</p><Button onClick={() => void refetch()}>Retry</Button></> : <p role="status">Loading team schedule…</p>}
  </Window>

  const period = schedulePage(zonedYmd(data.from, data.timezone), data.timezone, workspace.plan, page)
  const memberName = (id: string) => members.find((member) => member.id === id)?.label ?? 'Former member'
  const events = data.events.filter((event) => Date.parse(event.startsAt) < Date.parse(period.to) && Date.parse(event.endsAt) > Date.parse(period.from))
  const dayLabel = (day: string) => new Intl.DateTimeFormat('en-GB', { weekday: 'short', day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(day + 'T12:00:00Z'))
  const eventTime = (time: string) => new Intl.DateTimeFormat('en-GB', { dateStyle: 'short', timeStyle: 'short', timeZone: data.timezone }).format(new Date(time))
  const last = period.days[period.days.length - 1]
  return <Window title={title} actions={controls}>
    <div className="team-plan stack">
      <div className="team-plan-range">
        <div><h2>{workspace.name}</h2><p>{dayLabel(period.days[0])}{period.days.length > 1 ? ' to ' + dayLabel(last) : ''} · {data.timezone}</p></div>
        {period.pageCount > 1 ? <div className="row">
          <Button variant="ghost" disabled={period.pageIndex === 0} onClick={() => setPage(period.pageIndex - 1)}>Previous week</Button>
          <Button variant="ghost" disabled={period.pageIndex + 1 >= period.pageCount} onClick={() => setPage(period.pageIndex + 1)}>Next week</Button>
        </div> : null}
      </div>
      <section className="stack" aria-labelledby="team-availability">
        <h3 id="team-availability">Member availability</h3>
        <p className="muted">Planned and available time per day. Personal tasks and appointments stay private.</p>
        {data.members.length === 0 ? <p>No members in this workspace.</p> : null}
        {data.members.map((member) => <article className="team-member" key={member.userId}>
          <h4>{memberName(member.userId)}</h4>
          <dl className="team-days">{member.days.filter((day) => day.date >= period.days[0] && day.date <= last).map((day) =>
            <div key={day.date}><dt>{dayLabel(day.date)}</dt>
              <dd>{span(day.plannedSeconds)} planned</dd><dd>{span(day.availableSeconds)} available</dd>
            </div>)}</dl>
        </article>)}
      </section>
      <section aria-labelledby="team-tasks">
        <h3 id="team-tasks">Team tasks</h3>
        {data.items.length === 0 ? <p>No open team tasks.</p> : <ul className="account-list">{data.items.map((item) =>
          <li key={item.id}><Button variant="ghost" onClick={() => openTask(navigate, item.id)}>{item.title}</Button>
            <span>{item.assigneeUserId ? memberName(item.assigneeUserId) : 'Unassigned'}</span></li>)}</ul>}
      </section>
      <section aria-labelledby="team-events">
        <h3 id="team-events">Team events</h3>
        {events.length === 0 ? <p>No team events in this period.</p> : <ul className="account-list">{events.map((event) =>
          <li key={event.seriesId + ':' + event.originalOn}><div><Button variant="ghost" onClick={() => openEvent(navigate, event.seriesId, event.originalOn)}>{event.title}</Button>
            <small>{eventTime(event.startsAt)} to {eventTime(event.endsAt)}</small></div>
            <span>{event.participantUserIds.length ? event.participantUserIds.map(memberName).join(', ') : 'No participants'}</span></li>)}</ul>}
      </section>
    </div>
  </Window>
}
