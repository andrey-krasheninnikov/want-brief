import type { EventOccurrence } from '../../types'
import { eventMeta, eventTone, type EventLookups } from './eventMeta'

type Props = {
  weeks: string[][]
  month: number
  rows: EventOccurrence[]
  lookups: EventLookups
  today: string
  selectedId?: string
  selectedOn?: string
  onPick: (seriesId: string, originalOn: string) => void
}

function ymdOf(iso: string): string {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Europe/Moscow',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date(iso))
}

function dayNum(ymd: string): string {
  return new Intl.DateTimeFormat('en-GB', { day: 'numeric', timeZone: 'Europe/Moscow' }).format(
    new Date(`${ymd}T12:00:00+03:00`),
  )
}

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

export function MonthGrid({ weeks, month, rows, lookups, today, selectedId, selectedOn, onPick }: Props) {
  const byDay = new Map<string, EventOccurrence[]>()
  for (const row of rows) {
    const key = ymdOf(row.startsAt)
    const list = byDay.get(key) ?? []
    list.push(row)
    byDay.set(key, list)
  }

  return (
    <div className="cal-month">
      <div className="cal-month-weekdays">
        {WEEKDAYS.map((label) => (
          <span key={label}>{label}</span>
        ))}
      </div>
      {weeks.map((week) => (
        <div key={week[0]} className="cal-month-week">
          {week.map((ymd) => {
            const inMonth = Number(ymd.slice(5, 7)) === month
            const listed = byDay.get(ymd) ?? []
            const shown = listed.slice(0, 3)
            const extra = listed.length - shown.length
            return (
              <div key={ymd} className={['cal-month-day', inMonth ? '' : 'out', ymd === today ? 'today' : ''].filter(Boolean).join(' ')}>
                <strong>{dayNum(ymd)}</strong>
                {shown.map((row) => {
                  const meta = eventMeta(row, lookups)
                  return (
                    <button
                      key={`${row.seriesId}-${row.originalOn}`}
                      type="button"
                      className={row.seriesId === selectedId && row.originalOn === selectedOn ? 'cal-chip on' : 'cal-chip'}
                      style={eventTone(meta.color)}
                      onClick={() => onPick(row.seriesId, row.originalOn)}
                    >
                      <span>{row.title}</span>
                      {meta.line ? <small>{meta.line}</small> : null}
                    </button>
                  )
                })}
                {extra > 0 ? <small>+{extra}</small> : null}
              </div>
            )
          })}
        </div>
      ))}
    </div>
  )
}
