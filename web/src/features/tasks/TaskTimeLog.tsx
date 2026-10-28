import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { TaskOverlay } from './TaskConfirm'

function todayDate(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function logRange(date: string, seconds: number): { startedAt: string; endedAt: string } {
  if (date === todayDate()) {
    const ended = new Date()
    return { startedAt: new Date(ended.getTime() - seconds * 1000).toISOString(), endedAt: ended.toISOString() }
  }
  const noon = new Date(`${date}T12:00:00`)
  return { startedAt: new Date(noon.getTime() - seconds * 1000).toISOString(), endedAt: noon.toISOString() }
}

type Props = {
  itemId: string
  open: boolean
  onClose: () => void
}

export function TaskTimeLog({ itemId, open, onClose }: Props) {
  const queryClient = useQueryClient()
  const [date, setDate] = useState(todayDate)
  const [hours, setHours] = useState('0')
  const [minutes, setMinutes] = useState('0')
  const [error, setError] = useState('')
  const save = useMutation({
    mutationFn: ({ startedAt, endedAt }: { startedAt: string; endedAt: string }) =>
      api.logInterval(itemId, startedAt, endedAt),
    onSuccess: () => {
      setHours('0')
      setMinutes('0')
      setError('')
      onClose()
      void queryClient.invalidateQueries({ queryKey: ['intervals'] })
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      void queryClient.invalidateQueries({ queryKey: ['load'] })
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not add time.'),
  })

  function onSubmit(event: FormEvent) {
    event.preventDefault()
    const h = Number(hours)
    const m = Number(minutes)
    if (!Number.isFinite(h) || !Number.isFinite(m) || h < 0 || m < 0 || m > 59) {
      setError('Time is invalid.')
      return
    }
    const seconds = Math.round(h) * 3600 + Math.round(m) * 60
    if (seconds <= 0) {
      setError('Time is required.')
      return
    }
    setError('')
    save.mutate(logRange(date, seconds))
  }

  return (
    <TaskOverlay open={open} onCancel={onClose}>
      <form className="tasks-confirm" onSubmit={onSubmit}>
        <p className="tasks-kicker">Log time</p>
        <h3>How long?</h3>
        <label>
          Date
          <DateField mode="date" value={date} onChange={setDate} />
        </label>
        <div className="tasks-plan">
          <label>
            Hours
            <input type="number" min={0} step={1} value={hours} onChange={(e) => setHours(e.target.value)} />
          </label>
          <label>
            Minutes
            <input
              type="number"
              min={0}
              max={59}
              step={1}
              value={minutes}
              onChange={(e) => setMinutes(e.target.value)}
            />
          </label>
        </div>
        {error ? <p className="error">{error}</p> : null}
        <div className="tasks-confirm-actions">
          <button type="button" className="ghost" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" disabled={save.isPending}>
            Log time
          </button>
        </div>
      </form>
    </TaskOverlay>
  )
}
