import { Trash } from '@phosphor-icons/react'
import { useState, type KeyboardEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import type { ItemCheck } from '../../types'

type Props = {
  itemId: string
}

export function TaskChecks({ itemId }: Props) {
  const queryClient = useQueryClient()
  const checks = useQuery({
    queryKey: ['item-checks', itemId],
    queryFn: () => api.itemChecks(itemId),
    enabled: Boolean(itemId),
  })
  const [draft, setDraft] = useState('')
  const [open, setOpen] = useState(false)
  const [error, setError] = useState('')

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['item-checks', itemId] })
    void queryClient.invalidateQueries({ queryKey: ['items'] })
  }

  const create = useMutation({
    mutationFn: (body: string) => api.createItemCheck(itemId, body),
    onSuccess: () => {
      setDraft('')
      setOpen(true)
      setError('')
      refresh()
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not add.'),
  })
  const patch = useMutation({
    mutationFn: ({ id, body }: { id: string; body: Record<string, unknown> }) => api.patchItemCheck(itemId, id, body),
    onSuccess: refresh,
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteItemCheck(itemId, id),
    onSuccess: refresh,
  })

  const rows = checks.data ?? []

  function addLine(body: string) {
    const next = body.trim()
    if (!next) {
      setError('Check is required.')
      return
    }
    create.mutate(next)
  }

  const done = rows.filter((row) => row.done).length

  return (
    <section className="tasks-checks">
      <div className="tasks-head">
        <h3>Checklist</h3>
        <button type="button" className="tasks-plus" aria-label="Add check" onClick={() => setOpen(true)}>
          +
        </button>
      </div>
      {rows.length > 0 ? (
        <div className="dossier-progress-row">
          <div className="dossier-progress">
            <span style={{ width: `${Math.round((done / rows.length) * 100)}%` }} />
          </div>
          <small>
            {done} of {rows.length} done
          </small>
        </div>
      ) : null}
      {rows.length === 0 && !open ? <p className="dossier-empty">No checks yet. Add the first step.</p> : null}
      <ul className="tasks-check-list">
        {rows.map((row) => (
          <CheckRow
            key={row.id}
            row={row}
            onToggle={() => patch.mutate({ id: row.id, body: { done: !row.done } })}
            onBody={(body) => {
              if (body === row.body) return
              patch.mutate({ id: row.id, body: { body } })
            }}
            onRemove={() => remove.mutate(row.id)}
            onEnter={() => {
              setOpen(true)
              setDraft('')
            }}
          />
        ))}
      </ul>
      {open ? (
        <label className="tasks-check-draft">
          New check
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(event) => {
              if (event.key !== 'Enter') return
              event.preventDefault()
              addLine(draft)
            }}
            autoFocus
            autoComplete="off"
          />
        </label>
      ) : null}
      {error ? <p className="error">{error}</p> : null}
    </section>
  )
}

function CheckRow({
  row,
  onToggle,
  onBody,
  onRemove,
  onEnter,
}: {
  row: ItemCheck
  onToggle: () => void
  onBody: (body: string) => void
  onRemove: () => void
  onEnter: () => void
}) {
  const [editing, setEditing] = useState(false)
  const [body, setBody] = useState(row.body)

  function onKey(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Enter') {
      event.preventDefault()
      onBody(body.trim())
      setEditing(false)
      onEnter()
    }
    if (event.key === 'Escape') {
      setBody(row.body)
      setEditing(false)
    }
  }

  return (
    <li className={row.done ? 'tasks-check done' : 'tasks-check'}>
      <input type="checkbox" checked={row.done} onChange={onToggle} aria-label="Done" />
      {editing ? (
        <input
          value={body}
          onChange={(e) => setBody(e.target.value)}
          onBlur={() => {
            onBody(body.trim())
            setEditing(false)
          }}
          onKeyDown={onKey}
          aria-label="Check body"
          autoFocus
        />
      ) : (
        <button type="button" className="tasks-check-text" onClick={onToggle} onDoubleClick={() => setEditing(true)}>
          {row.body}
        </button>
      )}
      <button type="button" className="ghost tasks-check-remove" aria-label="Remove check" onClick={onRemove}>
        <Trash size={16} weight="light" />
      </button>
    </li>
  )
}
