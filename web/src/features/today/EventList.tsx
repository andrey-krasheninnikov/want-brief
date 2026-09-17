import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { moscowDayRange } from '../../shared/moscow'
import { openEvent, openTask } from '../../shared/taskOverlay'
import { eventTypeLabel, type CalendarEvent, type ScheduleBlock } from '../../types'

function formatTime(iso: string): string {
  return new Intl.DateTimeFormat('en-GB', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(iso))
}

function AgendaRow({
  row,
  featured,
  onOpen,
}: {
  row: CalendarEvent
  featured?: boolean
  onOpen: (row: CalendarEvent) => void
}) {
  return (
    <article className={featured ? 'agenda-next' : 'agenda-row'} onClick={() => onOpen(row)}>
      <div className="today-agenda-core">
        <time className="mono" dateTime={row.startsAt}>
          {formatTime(row.startsAt)}
        </time>
        <div className="agenda-body">
          <strong>{row.title}</strong>
          <span className="agenda-chip">{eventTypeLabel(row.type)}</span>
        </div>
      </div>
    </article>
  )
}

function WorkRow({ row, onOpen }: { row: ScheduleBlock; onOpen: (id: string) => void }) {
  return (
    <article className="agenda-next" onClick={() => onOpen(row.itemId)}>
      <div className="today-agenda-core">
        <time className="mono" dateTime={row.startsAt}>
          {formatTime(row.startsAt)}
        </time>
        <div className="agenda-body">
          <strong>{row.externalKey ? `[${row.externalKey}] ${row.title}` : row.title}</strong>
          <span className="agenda-chip">Work</span>
        </div>
      </div>
    </article>
  )
}

export function EventList() {
  const navigate = useNavigate()
  const bounds = moscowDayRange()
  const events = useQuery({
    queryKey: ['events', bounds.from, bounds.to],
    queryFn: () => api.events(bounds.from, bounds.to),
  })
  const schedule = useQuery({
    queryKey: ['schedule', bounds.from, bounds.to],
    queryFn: () => api.schedule(bounds.from, bounds.to),
  })
  const sorted = [...(events.data ?? [])].sort(
    (a, b) => new Date(a.startsAt).getTime() - new Date(b.startsAt).getTime(),
  )
  const now = Date.now()
  const nextWork = (schedule.data?.lanes ?? [])
    .flatMap((lane) => lane.blocks)
    .filter((row) => new Date(row.endsAt).getTime() > now)
    .sort((a, b) => new Date(a.startsAt).getTime() - new Date(b.startsAt).getTime())[0]
  const next = sorted[0]
  const rest = sorted.slice(1)
  const empty = !events.isLoading && !schedule.isLoading && !nextWork && sorted.length === 0

  return (
    <section className="panel agenda">
      <div className="tasks-head">
        <h3>Schedule</h3>
        <button type="button" className="tasks-plus" aria-label="Add event" onClick={() => navigate('/events?new=1')}>
          +
        </button>
      </div>
      {events.isLoading || schedule.isLoading ? (
        <div className="agenda-skels" aria-hidden>
          <div className="agenda-skel" />
          <div className="agenda-skel" />
        </div>
      ) : null}
      {events.isError ? <p className="error">{events.error.message}</p> : null}
      {empty ? <p className="muted">Nothing scheduled.</p> : null}
      {nextWork ? <WorkRow row={nextWork} onOpen={(id) => openTask(navigate, id)} /> : null}
      {next ? (
        <AgendaRow row={next} featured={!nextWork} onOpen={(row) => openEvent(navigate, row.seriesId, row.originalOn)} />
      ) : null}
      {rest.map((row) => (
        <AgendaRow
          key={`${row.seriesId}-${row.originalOn}`}
          row={row}
          onOpen={(item) => openEvent(navigate, item.seriesId, item.originalOn)}
        />
      ))}
    </section>
  )
}
