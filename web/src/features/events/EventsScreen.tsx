import { useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { moscowMonth, moscowWeek, moscowYmd, shiftMonths, shiftWeeks } from '../../shared/moscow'
import { openEvent } from '../../shared/taskOverlay'
import { Window } from '../../shared/Window'
import { TaskSheet } from '../tasks/TaskSheet'
import { EventDossier } from './EventDossier'
import { eventLookups } from './eventMeta'
import { MonthGrid } from './MonthGrid'
import { WeekGrid } from './WeekGrid'
import './events.css'

export function EventsScreen() {
  const { id } = useParams()
  const [params] = useSearchParams()
  const creating = !id && params.get('new') === '1'
  const originalOn = params.get('on') ?? undefined
  const navigate = useNavigate()
  const [anchor, setAnchor] = useState(() => new Date())
  const [view, setView] = useState<'week' | 'month'>('week')
  const week = useMemo(() => moscowWeek(anchor), [anchor])
  const month = useMemo(() => moscowMonth(anchor), [anchor])
  const today = moscowYmd()
  const range = view === 'week' ? week : month
  const events = useQuery({
    queryKey: ['events', range.from, range.to],
    queryFn: () => api.events(range.from, range.to),
  })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const companies = useQuery({ queryKey: ['companies'], queryFn: api.companies })
  const rows = events.data ?? []
  const lookups = useMemo(
    () => eventLookups(projects.data ?? [], companies.data ?? []),
    [projects.data, companies.data],
  )

  return (
    <Window
      className="events-window"
      kicker={view === 'week' ? 'Week' : month.label}
      title="Events"
      actions={
        <div className="events-actions">
          <div className="people-tabs">
            <button type="button" className={view === 'week' ? 'on' : ''} onClick={() => setView('week')}>
              Week
            </button>
            <button type="button" className={view === 'month' ? 'on' : ''} onClick={() => setView('month')}>
              Month
            </button>
          </div>
          <button
            type="button"
            className="ghost"
            aria-label="Previous"
            onClick={() => setAnchor((current) => (view === 'week' ? shiftWeeks(current, -1) : shiftMonths(current, -1)))}
          >
            ‹
          </button>
          <button
            type="button"
            className="ghost"
            aria-label="Next"
            onClick={() => setAnchor((current) => (view === 'week' ? shiftWeeks(current, 1) : shiftMonths(current, 1)))}
          >
            ›
          </button>
          <button type="button" className="tasks-plus" aria-label="Add event" onClick={() => navigate('/events?new=1')}>
            +
          </button>
        </div>
      }
    >
      <div className="events-pane">
        <div className="events-core">
          {events.isLoading ? <p className="muted">Loading…</p> : null}
          {events.isError ? <p className="error">{events.error.message}</p> : null}
          {!events.isLoading && !events.isError && view === 'week' ? (
            <WeekGrid
              days={week.days}
              rows={rows}
              lookups={lookups}
              today={today}
              selectedId={id}
              selectedOn={originalOn}
              onPick={(seriesId, on) => openEvent(navigate, seriesId, on)}
            />
          ) : null}
          {!events.isLoading && !events.isError && view === 'month' ? (
            <MonthGrid
              weeks={month.weeks}
              month={month.month}
              rows={rows}
              lookups={lookups}
              today={today}
              selectedId={id}
              selectedOn={originalOn}
              onPick={(seriesId, on) => openEvent(navigate, seriesId, on)}
            />
          ) : null}
        </div>
      </div>
      <TaskSheet
        open={Boolean(id) || creating}
        kicker={originalOn ? 'Occurrence' : 'Series'}
        title={id ? 'Dossier' : 'New event'}
        onClose={() => navigate('/events')}
      >
        <EventDossier
          seriesId={id}
          originalOn={originalOn}
          onCreated={(next) => navigate(`/events/${next}`)}
          onDeleted={() => navigate('/events')}
        />
      </TaskSheet>
    </Window>
  )
}
