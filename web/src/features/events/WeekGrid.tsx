import { useEffect, useRef } from 'react'
import type { EventOccurrence } from '../../types'
import { moscowMinutes, moscowYmd } from '../../shared/moscow'
import { eventMeta, eventTone, type EventLookups } from './eventMeta'

const HOUR = 48

type Props = {
  days: string[]
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

function dayLabel(ymd: string) {
  const date = new Date(`${ymd}T12:00:00+03:00`)
  return {
    week: new Intl.DateTimeFormat('en-GB', { weekday: 'short', timeZone: 'Europe/Moscow' }).format(date),
    num: new Intl.DateTimeFormat('en-GB', { day: 'numeric', timeZone: 'Europe/Moscow' }).format(date),
  }
}

export function WeekGrid({ days, rows, lookups, today, selectedId, selectedOn, onPick }: Props) {
  const scroller = useRef<HTMLDivElement>(null)
  const now = moscowYmd()
  const nowMin = moscowMinutes(new Date().toISOString())

  useEffect(() => {
    scroller.current?.scrollTo({ top: 8 * HOUR })
  }, [days[0]])

  return (
    <div className="cal-week">
      <div className="cal-week-head">
        <span />
        {days.map((ymd) => {
          const parts = dayLabel(ymd)
          return (
            <div key={ymd} className={ymd === today ? 'cal-week-day today' : 'cal-week-day'}>
              <small>{parts.week}</small>
              <strong>{parts.num}</strong>
            </div>
          )
        })}
      </div>
      <div className="cal-week-scroll" ref={scroller}>
        <div className="cal-week-body" style={{ height: 24 * HOUR }}>
          <div className="cal-hours">
            {Array.from({ length: 24 }, (_, hour) => (
              <span key={hour}>{String(hour).padStart(2, '0')}:00</span>
            ))}
          </div>
          {days.map((ymd) => (
            <div key={ymd} className="cal-col">
              {Array.from({ length: 24 }, (_, hour) => (
                <i key={hour} />
              ))}
              {ymd === now ? <b className="cal-now" style={{ top: (nowMin / 60) * HOUR }} /> : null}
              {rows
                .filter((row) => ymdOf(row.startsAt) === ymd)
                .map((row) => {
                  const start = moscowMinutes(row.startsAt)
                  const dur = Math.max(row.durationSeconds / 60, 20)
                  const meta = eventMeta(row, lookups)
                  return (
                    <button
                      key={`${row.seriesId}-${row.originalOn}`}
                      type="button"
                      className={row.seriesId === selectedId && row.originalOn === selectedOn ? 'cal-block on' : 'cal-block'}
                      style={{ top: (start / 60) * HOUR, height: (dur / 60) * HOUR, ...eventTone(meta.color) }}
                      onClick={() => onPick(row.seriesId, row.originalOn)}
                    >
                      <span>{row.title}</span>
                      {meta.line ? <small>{meta.line}</small> : null}
                    </button>
                  )
                })}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
