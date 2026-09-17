import { useEffect, useRef, useState, type FormEvent } from 'react'
import type { DayOverride, DayOverrideDraft } from '../../types'
import { clockToMinutes, minutesToClock } from './now'

type Props = {
  day: string
  label: string
  override?: DayOverride
  defaultStartMin: number
  defaultEndMin: number
  busy: boolean
  error?: string
  onSave: (draft: DayOverrideDraft) => void
  onClear: () => void
  onClose: () => void
}

type Mode = 'default' | 'off' | 'custom'

function modeOf(override?: DayOverride): Mode {
  if (!override) return 'default'
  if (override.off) return 'off'
  if (override.workStartMin != null) return 'custom'
  return 'default'
}

// DayOverridePopover edits one calendar day: off, custom hours or just a note.
export function DayOverridePopover({ day, label, override, defaultStartMin, defaultEndMin, busy, error, onSave, onClear, onClose }: Props) {
  const box = useRef<HTMLFormElement>(null)
  const [mode, setMode] = useState<Mode>(() => modeOf(override))
  const [start, setStart] = useState(() => minutesToClock(override?.workStartMin ?? defaultStartMin))
  const [end, setEnd] = useState(() => minutesToClock(override?.workEndMin ?? defaultEndMin))
  const [note, setNote] = useState(override?.note ?? '')
  const [invalid, setInvalid] = useState('')

  useEffect(() => {
    function onDown(event: MouseEvent) {
      if (box.current && !box.current.contains(event.target as Node)) onClose()
    }
    function onKey(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('mousedown', onDown)
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('mousedown', onDown)
      window.removeEventListener('keydown', onKey)
    }
  }, [onClose])

  function submit(event: FormEvent) {
    event.preventDefault()
    setInvalid('')
    if (mode === 'default' && !note.trim()) {
      if (override) onClear()
      else onClose()
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
    onSave(draft)
  }

  return (
    <form ref={box} className="sched-daypop" data-day={day} onSubmit={submit}>
      <p className="sched-daypop-title">{label}</p>
      <div className="sched-daypop-modes" role="radiogroup" aria-label="Day mode">
        {(['default', 'off', 'custom'] as Mode[]).map((value) => (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={mode === value}
            className={mode === value ? 'chip on' : 'chip'}
            onClick={() => setMode(value)}
          >
            {value === 'default' ? 'Usual hours' : value === 'off' ? 'Day off' : 'Custom hours'}
          </button>
        ))}
      </div>
      {mode === 'custom' ? (
        <div className="sched-daypop-hours">
          <label>
            From
            <input type="time" step={300} value={start} onChange={(e) => setStart(e.target.value)} />
          </label>
          <label>
            To
            <input type="time" step={300} value={end} onChange={(e) => setEnd(e.target.value)} />
          </label>
        </div>
      ) : null}
      <label>
        Note
        <input type="text" value={note} placeholder="Why this day differs" onChange={(e) => setNote(e.target.value)} />
      </label>
      {invalid || error ? <p className="error">{invalid || error}</p> : null}
      <div className="sched-daypop-actions">
        <button type="submit" disabled={busy}>
          Save
        </button>
        {override ? (
          <button type="button" className="ghost" disabled={busy} onClick={onClear}>
            Reset
          </button>
        ) : null}
        <button type="button" className="ghost" onClick={onClose}>
          Cancel
        </button>
      </div>
    </form>
  )
}
