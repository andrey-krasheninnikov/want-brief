import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import type { CheckinKind, LatestCheckins } from '../../types'

type Props = {
  open: boolean
  onClose: () => void
}

const bars: { kind: CheckinKind; label: string }[] = [
  { kind: 'stress', label: 'Stress' },
  { kind: 'focus', label: 'Focus' },
  { kind: 'energy', label: 'Energy' },
  { kind: 'interest', label: 'Interest' },
  { kind: 'happiness', label: 'Happiness' },
]

const fallback: LatestCheckins = { stress: 3, focus: 3, energy: 3, interest: 3, happiness: 3 }

function dirty(latest: LatestCheckins | undefined, draft: Partial<LatestCheckins>) {
  const base = { ...fallback, ...latest }
  return bars
    .filter((bar) => draft[bar.kind] != null && draft[bar.kind] !== base[bar.kind])
    .map((bar) => ({ kind: bar.kind, level: draft[bar.kind]! }))
}

export function CheckinDialog({ open, onClose }: Props) {
  const queryClient = useQueryClient()
  const latest = useQuery({ queryKey: ['checkins'], queryFn: api.latestCheckins, enabled: open })
  const [draft, setDraft] = useState<Partial<LatestCheckins>>({})
  const [note, setNote] = useState('')

  useEffect(() => {
    if (!open) return
    setDraft({})
    setNote('')
  }, [open])

  useEffect(() => {
    if (!open) return
    function onKey(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopImmediatePropagation()
      onClose()
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [open, onClose])

  const changes = dirty(latest.data, draft)
  const comment = note.trim()
  const save = useMutation({
    mutationFn: async () => {
      await Promise.all(changes.map((row) => api.createCheckin(row.kind, row.level)))
      if (comment) await api.createNote(comment)
    },
    onSuccess: () => {
      queryClient.setQueryData<LatestCheckins>(['checkins'], (prev) => {
        const next = { ...fallback, ...prev }
        for (const row of changes) next[row.kind] = row.level
        return next
      })
      void queryClient.invalidateQueries({ queryKey: ['checkins'] })
      void queryClient.invalidateQueries({ queryKey: ['load'] })
      void queryClient.invalidateQueries({ queryKey: ['journal'] })
      onClose()
    },
  })

  if (!open) return null

  const values = { ...fallback, ...latest.data, ...draft }
  const canSave = (changes.length > 0 || Boolean(comment)) && !save.isPending

  return createPortal(
    <div className="tasks-log-root">
      <button type="button" className="tasks-log-back" aria-label="Close" onClick={onClose} />
      <div className="tasks-log-dialog checkin-dialog" role="dialog" aria-modal="true" aria-labelledby="checkin-title">
        <div className="tasks-log-core">
          <p className="tasks-kicker">Pulse</p>
          <h2 id="checkin-title">Check-in</h2>
          <ul className="checkins">
            {bars.map((bar) => (
              <li key={bar.kind}>
                <label>
                  {bar.label}
                  <span>{values[bar.kind]}</span>
                </label>
                <input
                  type="range"
                  min={1}
                  max={5}
                  value={values[bar.kind]}
                  onChange={(event) => {
                    const level = Number(event.target.value)
                    setDraft((prev) => ({ ...prev, [bar.kind]: level }))
                  }}
                />
              </li>
            ))}
          </ul>
          <label>
            Comment
            <textarea
              rows={3}
              value={note}
              onChange={(event) => setNote(event.target.value)}
              placeholder="Optional note"
            />
          </label>
          <div className="tasks-log-actions">
            <button type="button" className="ghost" onClick={onClose}>
              Cancel
            </button>
            <button type="button" className="tasks-log-save" disabled={!canSave} onClick={() => save.mutate()}>
              Save
              <span aria-hidden>↗</span>
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  )
}
