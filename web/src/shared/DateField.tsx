import { useEffect, useRef, useState } from 'react'

type Mode = 'date' | 'datetime'

type Props = {
  mode: Mode
  value: string
  onChange: (next: string) => void
  onCommit?: (next: string) => void
}

const WEEK = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

function parseStamp(value: string): Date | null {
  if (!value) return null
  const date = value.includes('T') ? new Date(value) : new Date(`${value}T00:00:00`)
  if (Number.isNaN(date.getTime())) return null
  return date
}

function ymd(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function stamp(date: Date, hours: number, minutes: number, mode: Mode): string {
  const day = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
  if (mode === 'date') return day
  return `${day}T${pad(hours)}:${pad(minutes)}`
}

function labelOf(value: string, mode: Mode): string {
  const date = parseStamp(value)
  if (!date) return mode === 'date' ? 'Pick date' : 'Pick date and time'
  if (mode === 'date') {
    return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium' }).format(date)
  }
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function monthLabel(year: number, month: number): string {
  return new Intl.DateTimeFormat('en-GB', { month: 'long' }).format(new Date(year, month, 1))
}

function yearOptions(now: Date): number[] {
  const max = now.getFullYear() + 2
  return Array.from({ length: max - 1920 + 1 }, (_, i) => 1920 + i)
}

function cells(year: number, month: number): Date[] {
  const first = new Date(year, month, 1)
  const shift = (first.getDay() + 6) % 7
  const start = new Date(year, month, 1 - shift)
  return Array.from({ length: 42 }, (_, i) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + i))
}

export function DateField({ mode, value, onChange, onCommit }: Props) {
  const root = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(false)
  const selected = parseStamp(value)
  const now = new Date()
  const [cursor, setCursor] = useState(() => selected ?? now)
  const hours = selected?.getHours() ?? now.getHours()
  const minutes = selected?.getMinutes() ?? now.getMinutes()

  useEffect(() => {
    if (open) setCursor(selected ?? new Date())
  }, [open, value])

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
  }, [open, value])

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
    setCursor(new Date(Math.min(maxYear, Math.max(minYear, cursor.getFullYear() + delta)), cursor.getMonth(), 1))
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
              onClick={() => setCursor(new Date(cursor.getFullYear(), cursor.getMonth() - 1, 1))}
            >
              ‹
            </button>
            <strong>{monthLabel(cursor.getFullYear(), cursor.getMonth())}</strong>
            <select
              aria-label="Year"
              value={cursor.getFullYear()}
              onChange={(e) => setCursor(new Date(Number(e.target.value), cursor.getMonth(), 1))}
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
              onClick={() => setCursor(new Date(cursor.getFullYear(), cursor.getMonth() + 1, 1))}
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
            {cells(cursor.getFullYear(), cursor.getMonth()).map((day) => {
              const key = ymd(day)
              const muted = day.getMonth() !== cursor.getMonth()
              const classes = [
                'cal-day',
                muted ? 'mute' : '',
                key === today ? 'today' : '',
                key === chosen ? 'on' : '',
              ]
                .filter(Boolean)
                .join(' ')
              return (
                <button key={key + String(day.getMonth())} type="button" className={classes} onClick={() => pickDay(day)}>
                  {day.getDate()}
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
                const next = stamp(now, now.getHours(), now.getMinutes(), mode)
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
