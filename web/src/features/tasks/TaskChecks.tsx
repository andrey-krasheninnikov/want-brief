import { Input } from '@want-foundation/desyst-react'
import { Button } from '@want-foundation/desyst-react'
import { DotsSixVertical, Trash } from '@phosphor-icons/react'
import { useEffect, useState, type DragEvent, type KeyboardEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import type { ItemCheck } from '../../types'

const DRAG_TYPE = 'application/x-want-check'

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
  const [dragging, setDragging] = useState<string | null>(null)

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
  const reorder = useMutation({
    mutationFn: (ids: string[]) => api.reorderItemChecks(itemId, ids),
    onSuccess: (rows) => {
      queryClient.setQueryData(['item-checks', itemId], rows)
      void queryClient.invalidateQueries({ queryKey: ['items'] })
    },
    onError: () => refresh(),
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

  function move(fromId: string, toId: string) {
    if (fromId === toId) return
    const from = rows.findIndex((row) => row.id === fromId)
    const to = rows.findIndex((row) => row.id === toId)
    if (from < 0 || to < 0) return
    const next = [...rows]
    const [row] = next.splice(from, 1)
    next.splice(to, 0, row)
    queryClient.setQueryData(['item-checks', itemId], next)
    reorder.mutate(next.map((entry) => entry.id))
  }

  const done = rows.filter((row) => row.done).length

  return (
    <section className="tasks-checks">
      <div className="tasks-head">
        <h3>Checklist</h3>
        <Button type="button" className="tasks-plus" aria-label="Add check" onClick={() => setOpen(true)}>
          +
        </Button>
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
            dragging={dragging === row.id}
            onDragBegin={() => setDragging(row.id)}
            onDragEnd={() => setDragging(null)}
            onDropOn={(fromId) => {
              move(fromId, row.id)
              setDragging(null)
            }}
            onToggle={() => patch.mutate({ id: row.id, body: { done: !row.done } })}
            onBody={(body) => {
              if (body === row.body) return
              if (!body) {
                setError('Check is required.')
                return
              }
              setError('')
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
          <Input
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
  dragging,
  onDragBegin,
  onDragEnd,
  onDropOn,
  onToggle,
  onBody,
  onRemove,
  onEnter,
}: {
  row: ItemCheck
  dragging: boolean
  onDragBegin: () => void
  onDragEnd: () => void
  onDropOn: (fromId: string) => void
  onToggle: () => void
  onBody: (body: string) => void
  onRemove: () => void
  onEnter: () => void
}) {
  const [body, setBody] = useState(row.body)

  useEffect(() => setBody(row.body), [row.body])

  function onKey(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Enter') {
      event.preventDefault()
      onBody(body.trim())
      onEnter()
    }
    if (event.key === 'Escape') setBody(row.body)
  }

  function onDragStart(event: DragEvent) {
    event.dataTransfer.setData(DRAG_TYPE, row.id)
    event.dataTransfer.setData('text/plain', row.id)
    event.dataTransfer.effectAllowed = 'move'
    onDragBegin()
  }

  return (
    <li
      className={['tasks-check', row.done ? 'done' : '', dragging ? 'dragging' : ''].filter(Boolean).join(' ')}
      onDragOver={(event) => {
        event.preventDefault()
        event.dataTransfer.dropEffect = 'move'
      }}
      onDrop={(event) => {
        event.preventDefault()
        const fromId = event.dataTransfer.getData(DRAG_TYPE) || event.dataTransfer.getData('text/plain')
        if (fromId) onDropOn(fromId)
      }}
    >
      <Button
        type="button"
        className="tasks-check-handle"
        draggable
        aria-label="Reorder check"
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
      >
        <DotsSixVertical size={16} weight="light" />
      </Button>
      <input type="checkbox" checked={row.done} onChange={onToggle} aria-label="Done" />
      <Input
        className="tasks-check-text"
        value={body}
        onChange={(e) => setBody(e.target.value)}
        onBlur={() => onBody(body.trim())}
        onKeyDown={onKey}
        aria-label="Check body"
        autoComplete="off"
      />
      <Button type="button" className="ghost tasks-check-remove" aria-label="Remove check" onClick={onRemove}>
        <Trash size={16} weight="light" />
      </Button>
    </li>
  )
}
