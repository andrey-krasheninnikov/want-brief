import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { api } from '../../../api'
import { DateField } from '../../../shared/DateField'
import type { DayOverride, DayOverrideDraft } from '../../../types'
import { clockToMinutes, minutesToClock } from '../now'
import { zonedYmd } from '../now'

type Mode = 'off' | 'custom' | 'note'

function shiftYmd(ymd: string, days: number): string {
  const [year, month, day] = ymd.split('-').map(Number)
  return new Date(Date.UTC(year, month - 1, day + days)).toISOString().slice(0, 10)
}

function describe(row: DayOverride): string {
  if (row.off) return 'Day off'
  if (row.workStartMin != null && row.workEndMin != null) return `${minutesToClock(row.workStartMin)}–${minutesToClock(row.workEndMin)}`
  return 'Note'
}

function dayLabel(ymd: string, tz: string): string {
  return new Intl.DateTimeFormat('en-GB', { weekday: 'short', day: 'numeric', month: 'short', timeZone: tz }).format(
    new Date(`${ymd}T12:00:00Z`),
  )
}

// DayOverridesEditor lists exceptions for the coming weeks and adds new ones.
export function DayOverridesEditor({ tz, workStartMin, workEndMin }: { tz: string; workStartMin: number; workEndMin: number }) {
  const queryClient = useQueryClient()
  const today = zonedYmd(new Date(), tz)
  const from = shiftYmd(today, -7)
  const to = shiftYmd(today, 90)
  const list = useQuery({ queryKey: ['schedule-days', from, to], queryFn: () => api.dayOverrides(from, to) })
  const [day, setDay] = useState('')
  const [mode, setMode] = useState<Mode>('off')
  const [start, setStart] = useState(minutesToClock(workStartMin))
  const [end, setEnd] = useState(minutesToClock(workEndMin))
  const [note, setNote] = useState('')
  const [invalid, setInvalid] = useState('')

  function invalidate() {
    void queryClient.invalidateQueries({ queryKey: ['schedule-days'] })
    void queryClient.invalidateQueries({ queryKey: ['schedule'] })
  }
  const upsert = useMutation({
    mutationFn: ({ day, draft }: { day: string; draft: DayOverrideDraft }) => api.putDayOverride(day, draft),
    onSuccess: () => {
      setDay('')
      setNote('')
      invalidate()
    },
  })
  const remove = useMutation({ mutationFn: (day: string) => api.deleteDayOverride(day), onSuccess: invalidate })

  function submit(event: FormEvent) {
    event.preventDefault()
    setInvalid('')
    if (!day) {
      setInvalid('Pick a day')
      return
    }
    const draft: DayOverrideDraft = { off: mode === 'off', workStartMin: null, workEndMin: null, note: note.trim() }
    if (mode === 'custom') {
      const s = clockToMinutes(start)
      const e = clockToMinutes(end)
      if (s == null || e == null || s >= e) {
        setInvalid('Start must be before end')
        return
      }
      draft.workStartMin = s
      draft.workEndMin = e
    }
    if (mode === 'note' && !draft.note) {
      setInvalid('Write the note')
      return
    }
    upsert.mutate({ day, draft })
  }

  const rows = [...(list.data ?? [])].sort((a, b) => a.day.localeCompare(b.day))

  return (
    <section className="sset-section">
      <header>
        <h3>Day exceptions</h3>
        <p>Days off, shorter days and notes. The packer skips or shrinks these days.</p>
      </header>
      <form className="sset-days-form" onSubmit={submit}>
        <label className="sset-field">
          <span>Day</span>
          <DateField mode="date" value={day} onChange={setDay} />
        </label>
        <div className="sset-days-modes" role="radiogroup" aria-label="Exception kind">
          {(['off', 'custom', 'note'] as Mode[]).map((value) => (
            <button
              key={value}
              type="button"
              role="radio"
              aria-checked={mode === value}
              className={mode === value ? 'chip on' : 'chip'}
              onClick={() => setMode(value)}
            >
              {value === 'off' ? 'Day off' : value === 'custom' ? 'Custom hours' : 'Note only'}
            </button>
          ))}
        </div>
        {mode === 'custom' ? (
          <div className="sset-days-hours">
            <label className="sset-field">
              <span>From</span>
              <input type="time" step={300} value={start} onChange={(e) => setStart(e.target.value)} />
            </label>
            <label className="sset-field">
              <span>To</span>
              <input type="time" step={300} value={end} onChange={(e) => setEnd(e.target.value)} />
            </label>
          </div>
        ) : null}
        <label className="sset-field">
          <span>Note</span>
          <input type="text" value={note} placeholder="Optional" onChange={(e) => setNote(e.target.value)} />
        </label>
        {invalid || upsert.isError ? <p className="error">{invalid || upsert.error?.message}</p> : null}
        <button type="submit" disabled={upsert.isPending}>
          Add exception
        </button>
      </form>
      {list.isError ? <p className="error">{list.error.message}</p> : null}
      {rows.length === 0 ? <p className="muted">No exceptions in the next three months.</p> : null}
      <ul className="sset-days">
        {rows.map((row) => (
          <li key={row.day} className={row.day < today ? 'past' : ''}>
            <span className="mono">{dayLabel(row.day, tz)}</span>
            <b>{describe(row)}</b>
            <small>{row.note}</small>
            <button type="button" className="ghost" disabled={remove.isPending} onClick={() => remove.mutate(row.day)}>
              Remove
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}
