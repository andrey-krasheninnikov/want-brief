import { Button } from '@want-foundation/desyst-react'
import { useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChartBar } from '@phosphor-icons/react'
import { api } from '../../api'
import { zonedMonth, zonedWeek, zonedYmd, shiftMonths, shiftWeeks } from '../../shared/zoned'
import { useTimezone } from '../../shared/timezone'
import { openEvent } from '../../shared/taskOverlay'
import { Window } from '../../shared/Window'
import { TaskSheet } from '../tasks/TaskSheet'
import { EventDossier } from './EventDossier'
import { EventStats } from './EventStats'
import { eventLookups } from './eventMeta'
import { MonthGrid } from './MonthGrid'
import { WeekGrid } from './WeekGrid'
import './events.css'

export function EventsScreen() {
  const tz = useTimezone()
  const [params, setParams] = useSearchParams()
  const creating = params.get('new') === '1'
  const selectedId = params.get('event') ?? undefined
  const originalOn = params.get('on') ?? undefined
  const navigate = useNavigate()
  const [anchor, setAnchor] = useState(() => new Date())
  const [view, setView] = useState<'week' | 'month'>('week')
  const [stats, setStats] = useState(false)
  const week = useMemo(() => zonedWeek(anchor, tz), [anchor, tz])
  const month = useMemo(() => zonedMonth(anchor, tz), [anchor, tz])
  const today = zonedYmd(new Date(), tz)
  const range = view === 'week' ? week : month
  const events = useQuery({
    queryKey: ['events', range.from, range.to, tz],
    queryFn: () => api.events(range.from, range.to),
  })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const companies = useQuery({ queryKey: ['companies'], queryFn: api.companies })
  const rows = events.data ?? []
  const lookups = useMemo(
    () => eventLookups(projects.data ?? [], companies.data ?? []),
    [projects.data, companies.data],
  )

  function clearOverlay() {
    const next = new URLSearchParams(params)
    next.delete('event')
    next.delete('on')
    next.delete('new')
    setParams(next, { replace: true })
  }

  function closeSheet() {
    setStats(false)
    clearOverlay()
  }

  function openStats() {
    setStats(true)
    clearOverlay()
  }

  function pick(seriesId: string, on: string) {
    setStats(false)
    openEvent(navigate, seriesId, on)
  }

  return (
    <Window
      className="events-window"
      kicker={view === 'week' ? 'Week' : month.label}
      title="Events"
      actions={
        <div className="events-actions">
          <div className="people-tabs">
            <Button type="button" className={view === 'week' ? 'on' : ''} onClick={() => setView('week')}>
              Week
            </Button>
            <Button type="button" className={view === 'month' ? 'on' : ''} onClick={() => setView('month')}>
              Month
            </Button>
          </div>
          <Button
            type="button"
            className="ghost"
            aria-label="Previous"
            onClick={() => setAnchor((current) => (view === 'week' ? shiftWeeks(current, -1, tz) : shiftMonths(current, -1, tz)))}
          >
            ‹
          </Button>
          <Button
            type="button"
            className="ghost"
            aria-label="Next"
            onClick={() => setAnchor((current) => (view === 'week' ? shiftWeeks(current, 1, tz) : shiftMonths(current, 1, tz)))}
          >
            ›
          </Button>
          <Button
            type="button"
            className={stats && !creating ? 'ghost on' : 'ghost'}
            aria-label="Stats"
            aria-pressed={stats && !creating}
            onClick={openStats}
          >
            <ChartBar size={18} weight="light" aria-hidden />
          </Button>
          <Button
            type="button"
            className="tasks-plus"
            aria-label="Add event"
            onClick={() => {
              setStats(false)
              navigate('/events?new=1')
            }}
          >
            +
          </Button>
        </div>
      }
    >
      <div className="events-pane">
        <div className="events-core">
          {events.isLoading ? <p className="muted">Loading…</p> : null}
          {events.isError ? <p className="error">{events.error.message}</p> : null}
          {!events.isLoading && !events.isError && view === 'week' ? (
            <WeekGrid
              timeZone={tz}
              days={week.days}
              rows={rows}
              lookups={lookups}
              today={today}
              selectedId={selectedId}
              selectedOn={originalOn}
              onPick={pick}
            />
          ) : null}
          {!events.isLoading && !events.isError && view === 'month' ? (
            <MonthGrid
              timeZone={tz}
              weeks={month.weeks}
              month={month.month}
              rows={rows}
              lookups={lookups}
              today={today}
              selectedId={selectedId}
              selectedOn={originalOn}
              onPick={pick}
            />
          ) : null}
        </div>
      </div>
      <TaskSheet
        open={stats || creating}
        kicker={creating ? 'Series' : view === 'week' ? 'Week' : month.label}
        title={creating ? 'New event' : 'Stats'}
        onClose={closeSheet}
      >
        {creating ? (
          <EventDossier
            onCreated={(next) => openEvent(navigate, next)}
            onDeleted={closeSheet}
          />
        ) : (
          <EventStats rows={rows} lookups={lookups} timeZone={tz} />
        )}
      </TaskSheet>
    </Window>
  )
}
