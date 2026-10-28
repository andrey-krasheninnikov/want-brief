import { useTimezone } from './timezone'
import { zonedLocalInput } from './zoned'
import { useEffect, useRef, useState } from 'react'

type Mode = 'date' | 'datetime'

type Props = {
  mode: Mode
  value: string
  onChange: (next: string) => void
  onCommit?: (next: string) => void
  timeZone?: string
}

const WEEK = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

function parseStamp(value: string): Date | null {
  if (!value) return null
  const date = value.includes('T') ? new Date(`${value}:00Z`) : new Date(`${value}T00:00:00Z`)
  if (Number.isNaN(date.getTime())) return null
  return date
}

function ymd(date: Date): string {
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`
}

function stamp(date: Date, hours: number, minutes: number, mode: Mode): string {
  const day = `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`
  if (mode === 'date') return day
  return `${day}T${pad(hours)}:${pad(minutes)}`
}

function labelOf(value: string, mode: Mode): string {
  const date = parseStamp(value)
  if (!date) return mode === 'date' ? 'Pick date' : 'Pick date and time'
  if (mode === 'date') {
    return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeZone: 'UTC' }).format(date)
  }
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' }).format(date)
}

function monthLabel(year: number, month: number): string {
  return new Intl.DateTimeFormat('en-GB', { month: 'long', timeZone: 'UTC' }).format(new Date(Date.UTC(year, month, 1)))
}

function yearOptions(now: Date): number[] {
  const max = now.getUTCFullYear() + 2
  return Array.from({ length: max - 1920 + 1 }, (_, i) => 1920 + i)
}

function cells(year: number, month: number): Date[] {
  const first = new Date(Date.UTC(year, month, 1))
  const shift = (first.getUTCDay() + 6) % 7
  const start = new Date(Date.UTC(year, month, 1 - shift))
  return Array.from({ length: 42 }, (_, i) => new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth(), start.getUTCDate() + i)))
}

export function DateField({ mode, value, onChange, onCommit, timeZone }: Props) {
  const contextTimeZone = useTimezone()
  const tz = timeZone ?? contextTimeZone
  const root = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(false)
  const selected = parseStamp(value)
  const now = parseStamp(zonedLocalInput(new Date(), tz))!
  const [cursor, setCursor] = useState(() => selected ?? now)
  const hours = selected?.getUTCHours() ?? now.getUTCHours()
  const minutes = selected?.getUTCMinutes() ?? now.getUTCMinutes()

  useEffect(() => {
    if (open) setCursor(selected ?? parseStamp(zonedLocalInput(new Date(), tz))!)
  }, [open, value, tz])

  function commit(next: string) {
    onChange(next)
    onCommit?.(next)
  }

  function close(save: boolean) {
    if (save && value) onCommit?.(value)
    setOpen(false)
  }

  useEffect(() => {
    if (!open) return
    function onDoc(event: MouseEvent) {
      if (root.current?.contains(event.target as Node)) return
      close(true)
    }
    function onKey(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopImmediatePropagation()
      close(true)
    }
    document.addEventListener('mousedown', onDoc)
    window.addEventListener('keydown', onKey, true)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      window.removeEventListener('keydown', onKey, true)
    }
  }, [open, value, tz])

  function pickDay(day: Date) {
    const next = stamp(day, hours, minutes, mode)
    if (mode === 'date') {
      commit(next)
      setOpen(false)
      return
    }
    onChange(next)
  }

  function setTime(nextHours: number, nextMinutes: number) {
    const base = selected ?? cursor
    onChange(stamp(base, nextHours, nextMinutes, 'datetime'))
  }

  const years = yearOptions(now)
  const minYear = years[0]
  const maxYear = years[years.length - 1]

  function shiftYear(delta: number) {
    setCursor(new Date(Date.UTC(Math.min(maxYear, Math.max(minYear, cursor.getUTCFullYear() + delta)), cursor.getUTCMonth(), 1)))
  }

  const today = ymd(now)
  const chosen = selected ? ymd(selected) : ''

  return (
    <div className={open ? 'cal open' : 'cal'} ref={root}>
      <button type="button" className="cal-trigger" onClick={() => setOpen((on) => !on)}>
        {labelOf(value, mode)}
      </button>
      {open ? (
        <div className="cal-pop" role="dialog" aria-label="Calendar">
          <div className="cal-nav">
            <button
              type="button"
              className="ghost"
              aria-label="Previous year"
              onClick={() => shiftYear(-1)}
            >
              ‹‹
            </button>
            <button
              type="button"
              className="ghost"
              aria-label="Previous month"
              onClick={() => setCursor(new Date(Date.UTC(cursor.getUTCFullYear(), cursor.getUTCMonth() - 1, 1)))}
            >
              ‹
            </button>
            <strong>{monthLabel(cursor.getUTCFullYear(), cursor.getUTCMonth())}</strong>
            <select
              aria-label="Year"
              value={cursor.getUTCFullYear()}
              onChange={(e) => setCursor(new Date(Date.UTC(Number(e.target.value), cursor.getUTCMonth(), 1)))}
            >
              {years.map((year) => (
                <option key={year} value={year}>
                  {year}
                </option>
              ))}
            </select>
            <button
              type="button"
              className="ghost"
              aria-label="Next month"
              onClick={() => setCursor(new Date(Date.UTC(cursor.getUTCFullYear(), cursor.getUTCMonth() + 1, 1)))}
            >
              ›
            </button>
            <button
              type="button"
              className="ghost"
              aria-label="Next year"
              onClick={() => shiftYear(1)}
            >
              ››
            </button>
          </div>
          <div className="cal-grid">
            {WEEK.map((day) => (
              <span key={day}>{day}</span>
            ))}
            {cells(cursor.getUTCFullYear(), cursor.getUTCMonth()).map((day) => {
              const key = ymd(day)
              const muted = day.getUTCMonth() !== cursor.getUTCMonth()
              const classes = [
                'cal-day',
                muted ? 'mute' : '',
                key === today ? 'today' : '',
                key === chosen ? 'on' : '',
              ]
                .filter(Boolean)
                .join(' ')
              return (
                <button key={key + String(day.getUTCMonth())} type="button" className={classes} onClick={() => pickDay(day)}>
                  {day.getUTCDate()}
                </button>
              )
            })}
          </div>
          {mode === 'datetime' ? (
            <div className="cal-time">
              <label>
                Hours
                <input
                  type="number"
                  min={0}
                  max={23}
                  step={1}
                  value={hours}
                  onChange={(e) => setTime(Math.min(23, Math.max(0, Number(e.target.value) || 0)), minutes)}
                  onBlur={() => onCommit?.(value)}
                />
              </label>
              <label>
                Minutes
                <input
                  type="number"
                  min={0}
                  max={59}
                  step={1}
                  value={minutes}
                  onChange={(e) => setTime(hours, Math.min(59, Math.max(0, Number(e.target.value) || 0)))}
                  onBlur={() => onCommit?.(value)}
                />
              </label>
            </div>
          ) : null}
          <div className="cal-foot">
            <button
              type="button"
              className="ghost"
              onClick={() => {
                commit('')
                setOpen(false)
              }}
            >
              Clear
            </button>
            <button
              type="button"
              className="ghost"
              onClick={() => {
                const next = stamp(now, now.getUTCHours(), now.getUTCMinutes(), mode)
                if (mode === 'date') {
                  commit(next)
                  setOpen(false)
                  return
                }
                onChange(next)
              }}
            >
              Today
            </button>
          </div>
        </div>
      ) : null}
    </div>
  )
}
