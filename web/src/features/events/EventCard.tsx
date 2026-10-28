import type { CSSProperties } from 'react'
import { eventTypeLabel, type EventOccurrence } from '../../types'

type Props = {
  row: EventOccurrence
  selected: boolean
  delay: number
  onPick: (seriesId: string, originalOn: string) => void
}

function formatTime(iso: string): string {
  return new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(iso))
}

export function EventCard({ row, selected, delay, onPick }: Props) {
  return (
    <article className={selected ? 'events-slot on' : 'events-slot'} style={{ '--d': delay } as CSSProperties}>
      <div className="events-slot-core" onClick={() => onPick(row.seriesId, row.originalOn)}>
        <time dateTime={row.startsAt}>
          {formatTime(row.startsAt)}
          <small>{formatTime(row.endsAt)}</small>
        </time>
        <div>
          <h3>{row.title}</h3>
          <div className="events-slot-meta">
            <span className="events-chip">{row.kind === 'call' ? 'Call' : 'Event'}</span>
            <span className="events-chip">{eventTypeLabel(row.type)}</span>
            {row.meetUrl ? (
              <a className="events-meet" href={row.meetUrl} target="_blank" rel="noreferrer" onClick={(e) => e.stopPropagation()}>
                Meet
              </a>
            ) : null}
          </div>
          <div className="events-bar" style={{ '--inv': row.involvement } as CSSProperties} aria-label={`Involvement ${row.involvement}`}>
            <i />
          </div>
        </div>
      </div>
    </article>
  )
}
