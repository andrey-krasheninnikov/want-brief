import { Button } from '@want-foundation/desyst-react'
import { zonedYmd } from '../../shared/zoned'
import type { EventOccurrence } from '../../types'
import { eventMeta, eventTone, type EventLookups } from './eventMeta'

type Props = {
  timeZone: string
  weeks: string[][]
  month: number
  rows: EventOccurrence[]
  lookups: EventLookups
  today: string
  selectedId?: string
  selectedOn?: string
  onPick: (seriesId: string, originalOn: string) => void
}

function dayNum(ymd: string): string {
  return new Intl.DateTimeFormat('en-GB', { day: 'numeric', timeZone: 'UTC' }).format(
    new Date(`${ymd}T12:00:00Z`),
  )
}

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

export function MonthGrid({ timeZone, weeks, month, rows, lookups, today, selectedId, selectedOn, onPick }: Props) {
  const byDay = new Map<string, EventOccurrence[]>()
  for (const row of rows) {
    const key = zonedYmd(row.startsAt, timeZone)
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
                    <Button
                      key={`${row.seriesId}-${row.originalOn}`}
                      type="button"
                      className={row.seriesId === selectedId && row.originalOn === selectedOn ? 'cal-chip on' : 'cal-chip'}
                      style={eventTone(meta.color)}
                      onClick={() => onPick(row.seriesId, row.originalOn)}
                    >
                      <span>{row.title}</span>
                      {meta.line ? <small>{meta.line}</small> : null}
                    </Button>
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
