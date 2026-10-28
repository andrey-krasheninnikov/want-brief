import type { EventOccurrence } from '../../types'
import { EventCard } from './EventCard'

type Props = {
  rows: EventOccurrence[]
  selectedId?: string
  selectedOn?: string
  onPick: (seriesId: string, originalOn: string) => void
}

export function DayColumn({ rows, selectedId, selectedOn, onPick }: Props) {
  if (rows.length === 0) {
    return <p className="events-empty">No slots this day.</p>
  }
  return (
    <div className="events-slots">
      {rows.map((row, index) => (
        <EventCard
          key={`${row.seriesId}-${row.originalOn}`}
          row={row}
          selected={row.seriesId === selectedId && row.originalOn === selectedOn}
          delay={index}
          onPick={onPick}
        />
      ))}
    </div>
  )
}
