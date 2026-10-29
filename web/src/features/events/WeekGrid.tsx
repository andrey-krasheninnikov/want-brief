import { Button } from '@want-foundation/desyst-react'
import { zonedYmd } from '../../shared/zoned'
import { useEffect, useRef, type CSSProperties } from 'react'
import type { EventOccurrence } from '../../types'
import { zonedMinutes } from '../../shared/zoned'
import { layoutLanes } from './eventLayout'
import { eventMeta, eventTone, type EventLookups } from './eventMeta'

const HOUR = 96
const GAP = 3

type Props = {
  timeZone: string
  days: string[]
  rows: EventOccurrence[]
  lookups: EventLookups
  today: string
  selectedId?: string
  selectedOn?: string
  onPick: (seriesId: string, originalOn: string) => void
}

function dayLabel(ymd: string) {
  const date = new Date(`${ymd}T12:00:00Z`)
  return {
    week: new Intl.DateTimeFormat('en-GB', { weekday: 'short', timeZone: 'UTC' }).format(date),
    num: new Intl.DateTimeFormat('en-GB', { day: 'numeric', timeZone: 'UTC' }).format(date),
  }
}

function blockStyle(start: number, dur: number, col: number, cols: number, color: string): CSSProperties {
  return {
    top: (start / 60) * HOUR,
    height: (dur / 60) * HOUR,
    left: `calc(${(col / cols) * 100}% + ${GAP}px)`,
    width: `calc(${100 / cols}% - ${GAP * 2}px)`,
    ...eventTone(color),
  }
}

export function WeekGrid({ timeZone, days, rows, lookups, today, selectedId, selectedOn, onPick }: Props) {
  const scroller = useRef<HTMLDivElement>(null)
  const now = zonedYmd(new Date(), timeZone)
  const nowMin = zonedMinutes(new Date(), timeZone)

  useEffect(() => {
    scroller.current?.scrollTo({ top: 8 * HOUR })
  }, [days[0]])

  return (
    <div className="cal-week" style={{ '--hour': `${HOUR}px` } as CSSProperties}>
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
          {days.map((ymd) => {
            const dayRows = rows.filter((row) => zonedYmd(row.startsAt, timeZone) === ymd)
            const lanes = layoutLanes(
              dayRows.map((row) => {
                const start = zonedMinutes(row.startsAt, timeZone)
                return { start, end: start + row.durationSeconds / 60 }
              }),
            )
            return (
              <div key={ymd} className="cal-col">
                {Array.from({ length: 24 }, (_, hour) => (
                  <i key={hour} />
                ))}
                {ymd === now ? <b className="cal-now" style={{ top: (nowMin / 60) * HOUR }} /> : null}
                {dayRows.map((row, i) => {
                  const start = zonedMinutes(row.startsAt, timeZone)
                  const dur = row.durationSeconds / 60
                  const meta = eventMeta(row, lookups)
                  const lane = lanes[i]
                  return (
                    <Button
                      key={`${row.seriesId}-${row.originalOn}`}
                      type="button"
                      className={row.seriesId === selectedId && row.originalOn === selectedOn ? 'cal-block on' : 'cal-block'}
                      style={blockStyle(start, dur, lane.col, lane.cols, meta.color)}
                      onClick={() => onPick(row.seriesId, row.originalOn)}
                    >
                      <span>{row.title}</span>
                      {meta.line ? <small>{meta.line}</small> : null}
                    </Button>
                  )
                })}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
