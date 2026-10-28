import { useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useLocation } from 'react-router-dom'
import { api } from '../../api'
import { taskHref } from '../../shared/taskOverlay'
import type { JournalSource } from '../../types'

const filters: { id: JournalSource | 'all'; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'loose', label: 'Loose' },
  { id: 'project', label: 'Project' },
  { id: 'item', label: 'Task' },
  { id: 'person', label: 'Person' },
]

function ownerHref(source: JournalSource, ownerId: string | null, search: string): string | { search: string } | null {
  if (!ownerId) return null
  if (source === 'project') return `/projects/${ownerId}`
  if (source === 'person') return `/people/${ownerId}`
  if (source === 'item' || source === 'loose') return taskHref(ownerId, search)
  return null
}

function stamp(iso: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
}

export function PulseJournal() {
  const queryClient = useQueryClient()
  const location = useLocation()
  const journal = useQuery({ queryKey: ['journal'], queryFn: api.journal })
  const [filter, setFilter] = useState<(typeof filters)[number]['id']>('all')
  const [body, setBody] = useState('')

  const rows = useMemo(() => {
    const list = journal.data ?? []
    if (filter === 'all') return list
    return list.filter((row) => row.source === filter)
  }, [journal.data, filter])

  const create = useMutation({
    mutationFn: () => api.createNote(body),
    onSuccess: () => {
      setBody('')
      void queryClient.invalidateQueries({ queryKey: ['journal'] })
      void queryClient.invalidateQueries({ queryKey: ['notes'] })
    },
  })
  const remove = useMutation({
    mutationFn: api.deleteNote,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['journal'] })
      void queryClient.invalidateQueries({ queryKey: ['notes'] })
    },
  })

  function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (!body.trim()) return
    create.mutate()
  }

  return (
    <div className="pulse-card">
      <div className="pulse-head">
        <p className="pulse-kicker">Journal</p>
        <div className="pulse-chips">
          {filters.map((item) => (
            <button
              key={item.id}
              type="button"
              className={filter === item.id ? 'pulse-chip on' : 'pulse-chip'}
              onClick={() => setFilter(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>
      {journal.isLoading ? <p className="muted">Loading notes…</p> : null}
      {rows.length === 0 && !journal.isLoading ? <p className="muted">No notes yet.</p> : null}
      {rows.length > 0 ? (
        <div className="pulse-table-wrap">
          <table className="pulse-table pulse-journal-table">
            <thead>
              <tr>
                <th>Source</th>
                <th>Owner</th>
                <th>When</th>
                <th>Note</th>
                <th aria-label="Actions"></th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const href = ownerHref(row.source, row.ownerId, location.search)
                return (
                  <tr key={`${row.source}-${row.id}`}>
                    <td>
                      <span className="pulse-badge">{row.source}</span>
                    </td>
                    <td>{href ? <Link to={href}>{row.ownerName}</Link> : <strong>{row.ownerName}</strong>}</td>
                    <td className="mono">{stamp(row.createdAt)}</td>
                    <td className="pulse-note-body">{row.body}</td>
                    <td>
                      {row.source === 'loose' ? (
                        <button type="button" className="ghost pulse-del" onClick={() => remove.mutate(row.id)}>
                          Delete
                        </button>
                      ) : null}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      ) : null}
      <form className="pulse-compose" onSubmit={onSubmit}>
        <textarea
          rows={3}
          placeholder="Loose thought, decision, or agreement"
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
        <button type="submit" disabled={!body.trim() || create.isPending}>
          Save
        </button>
      </form>
    </div>
  )
}
