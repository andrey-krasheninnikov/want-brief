import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import type { TaskLogKind } from '../../types'

type Props = {
  itemId: string | null
  kind: TaskLogKind
  onClose: () => void
}

export function TaskLogDialog({ itemId, kind, onClose }: Props) {
  const queryClient = useQueryClient()
  const [level, setLevel] = useState<number | null>(null)
  const [body, setBody] = useState('')

  useEffect(() => {
    if (!itemId) return
    setLevel(null)
    setBody('')
  }, [itemId])

  useEffect(() => {
    if (!itemId) return
    function onKey(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopImmediatePropagation()
      onClose()
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [itemId, onClose])

  const save = useMutation({
    mutationFn: async () => {
      if (!itemId) return
      const note = body.trim()
      await api.annotateItemEvent(itemId, {
        kinds: [kind],
        note: note || undefined,
        stress: level ?? undefined,
      })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      void queryClient.invalidateQueries({ queryKey: ['item-notes'] })
      void queryClient.invalidateQueries({ queryKey: ['item-events'] })
      void queryClient.invalidateQueries({ queryKey: ['load'] })
      void queryClient.invalidateQueries({ queryKey: ['journal'] })
      void queryClient.invalidateQueries({ queryKey: ['checkins'] })
      onClose()
    },
  })

  if (!itemId) return null

  function submit() {
    if (level == null && !body.trim()) {
      onClose()
      return
    }
    save.mutate()
  }

  return createPortal(
    <div className="tasks-log-root">
      <button type="button" className="tasks-log-back" aria-label="Skip" onClick={onClose} />
      <div className="tasks-log-dialog" role="dialog" aria-modal="true" aria-labelledby="tasks-log-title">
        <div className="tasks-log-core">
          <p className="tasks-kicker">Log</p>
          <h2 id="tasks-log-title">How did it go?</h2>
          <p className="muted">Stress and a note are optional. Skip leaves the board as-is.</p>
          <div className="tasks-log-levels" role="group" aria-label="Stress">
            {[1, 2, 3, 4, 5].map((value) => (
              <button
                key={value}
                type="button"
                className={level === value ? 'tasks-log-level on' : 'tasks-log-level'}
                onClick={() => setLevel(value)}
              >
                {value}
              </button>
            ))}
          </div>
          <label>
            Comment
            <textarea
              rows={3}
              value={body}
              onChange={(event) => setBody(event.target.value)}
              placeholder="What happened"
            />
          </label>
          <div className="tasks-log-actions">
            <button type="button" className="ghost" onClick={onClose}>
              Skip
            </button>
            <button type="button" className="tasks-log-save" disabled={save.isPending} onClick={submit}>
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
