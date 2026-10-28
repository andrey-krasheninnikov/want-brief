import { clock } from '../schedule/now'
import type { AgendaEntry } from './todayModel'

type Props = {
  rows: AgendaEntry[]
  loading: boolean
  error?: string
  tz: string
  onOpenWork: (itemId: string) => void
  onOpenMeeting: (seriesId: string, originalOn?: string) => void
}

export function TodayAgenda({ rows, loading, error, tz, onOpenWork, onOpenMeeting }: Props) {
  const empty = !loading && !error && rows.length === 0

  return (
    <section className="agenda">
      <h3>Rest of day</h3>
      {loading ? (
        <div className="agenda-skels" aria-hidden>
          <div className="agenda-skel" />
          <div className="agenda-skel" />
        </div>
      ) : null}
      {error ? <p className="error">{error}</p> : null}
      {empty ? <p className="muted">Clear rest of day.</p> : null}
      {rows.map((row) => {
        const work = row.kind === 'work'
        const clickable = work ? Boolean(row.itemId) : Boolean(row.seriesId)
        const title = row.externalKey ? `[${row.externalKey}] ${row.title}` : row.title
        return (
          <article
            key={`${row.kind}-${row.itemId ?? row.seriesId ?? row.title}-${row.startsAt}`}
            className={clickable ? 'agenda-row' : 'agenda-row static'}
            onClick={
              !clickable
                ? undefined
                : () => {
                    if (work && row.itemId) onOpenWork(row.itemId)
                    else if (row.seriesId) onOpenMeeting(row.seriesId, row.originalOn)
                  }
            }
          >
            <div className="today-agenda-core">
              <time className="mono" dateTime={row.startsAt}>
                {clock(row.startsAt, tz)}
                <small>{clock(row.endsAt, tz)}</small>
              </time>
              <div className="agenda-body">
                <strong>{title}</strong>
                <span className="agenda-chip">{work ? 'Work' : 'Meeting'}</span>
                {row.late ? <span className="agenda-chip late">late</span> : null}
              </div>
            </div>
          </article>
        )
      })}
    </section>
  )
}
