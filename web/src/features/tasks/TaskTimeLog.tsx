import { Input } from '@want-foundation/desyst-react'
import { Button } from '@want-foundation/desyst-react'
import { useTimezone } from '../../shared/timezone'
import { zonedInstant, zonedYmd } from '../../shared/zoned'
import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { TaskOverlay } from './TaskConfirm'

function logRange(date: string, seconds: number, tz: string): { startedAt: string; endedAt: string } {
  const ended = date === zonedYmd(new Date(), tz) ? new Date() : new Date(zonedInstant(`${date}T12:00`, tz)!)
  return { startedAt: new Date(ended.getTime() - seconds * 1000).toISOString(), endedAt: ended.toISOString() }
}

type Props = {
  itemId: string
  open: boolean
  onClose: () => void
}

export function TaskTimeLog({ itemId, open, onClose }: Props) {
  const tz = useTimezone()
  const queryClient = useQueryClient()
  const [date, setDate] = useState(() => zonedYmd(new Date(), tz))
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
    save.mutate(logRange(date, seconds, tz))
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
            <Input type="number" min={0} step={1} value={hours} onChange={(e) => setHours(e.target.value)} />
          </label>
          <label>
            Minutes
            <Input
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
          <Button type="button" className="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={save.isPending}>
            Log time
          </Button>
        </div>
      </form>
    </TaskOverlay>
  )
}
