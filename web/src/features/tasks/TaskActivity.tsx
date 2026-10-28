import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { OpenUrl } from '../../shared/UrlField'
import {
  occupancyLabel,
  sourceKindLabel,
  statusLabel,
  type ItemEvent,
  type ItemNote,
  type ItemStatus,
  type Occupancy,
  type SourceKind,
} from '../../types'

type Filter = 'all' | 'activity' | SourceKind

type Row =
  | { id: string; at: string; kind: 'note'; note: ItemNote }
  | { id: string; at: string; kind: 'event'; event: ItemEvent }

type Props = {
  itemId: string
}

function noteStamp(iso: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
}

function filterLabel(filter: Filter): string {
  if (filter === 'all') return 'All'
  if (filter === 'activity') return 'Activity'
  return sourceKindLabel(filter)
}

function formatSeconds(raw: string): string {
  const total = Number(raw)
  if (!Number.isFinite(total) || total < 0) return raw
  const s = Math.floor(total)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m`
  return `${s}s`
}

function shortValue(value: string): string {
  if (!value) return ''
  if (/^\d{4}-\d{2}-\d{2}T/.test(value)) {
    const at = new Date(value)
    if (!Number.isNaN(at.getTime())) return noteStamp(value)
  }
  if (value.length > 42) return `${value.slice(0, 39)}…`
  return value.replaceAll('_', ' ')
}

const FIELD_NAMES: Record<string, string> = {
  title: 'Title',
  kind: 'Kind',
  projectId: 'Project',
  urgent: 'Urgent',
  important: 'Important',
  pinned: 'Pin',
  pinnedAt: 'Pin until',
  stress: 'Stress',
  dueAt: 'Due',
  devDueAt: 'Dev due',
  reviewDueAt: 'Review due',
  testDueAt: 'Test due',
  description: 'Description',
  plannedSeconds: 'Estimate',
  links: 'Links',
  personIds: 'People',
  occupancy: 'Occupancy',
  archived: 'Stall',
  externalKey: 'Key',
  externalStatus: 'External status',
}

function fieldLabel(event: ItemEvent): string {
  const name = FIELD_NAMES[event.field] ?? event.field
  if (event.field === 'urgent' || event.field === 'important' || event.field === 'pinned') {
    return `${name} ${event.to === 'true' ? 'on' : 'off'}`
  }
  if (event.field === 'occupancy' && (event.to === 'solo' || event.to === 'parallel' || event.to === 'waiting')) {
    return `${name} ${occupancyLabel(event.to as Occupancy)}`
  }
  if (event.field === 'archived') {
    return event.to ? 'Stalled' : 'Restored'
  }
  if (event.from && event.to) return `${name} ${shortValue(event.from)} → ${shortValue(event.to)}`
  if (event.to) return `${name} ${shortValue(event.to)}`
  if (event.from) return `${name} cleared`
  return name
}

function eventLabel(event: ItemEvent): string {
  switch (event.kind) {
    case 'status':
      return `Status ${statusLabel(event.from as ItemStatus)} → ${statusLabel(event.to as ItemStatus)}`
    case 'timer_start':
      return 'Timer started'
    case 'timer_stop':
      return `Timer ${formatSeconds(event.to)}`
    case 'timer_log':
      return `Time logged ${formatSeconds(event.to)}`
    case 'check':
      if (event.field === 'create') return event.to ? `Check added · ${event.to}` : 'Check added'
      if (event.field === 'delete') return event.from ? `Check removed · ${event.from}` : 'Check removed'
      if (event.field === 'done') return event.to === 'true' ? 'Check done' : 'Check reopened'
      if (event.field === 'body') return 'Check renamed'
      return 'Check'
    default:
      return fieldLabel(event)
  }
}

export function TaskActivity({ itemId }: Props) {
  const queryClient = useQueryClient()
  const notes = useQuery({ queryKey: ['item-notes', itemId], queryFn: () => api.itemNotes(itemId) })
  const events = useQuery({ queryKey: ['item-events', itemId], queryFn: () => api.itemEvents(itemId) })
  const [body, setBody] = useState('')
  const [open, setOpen] = useState(false)
  const [error, setError] = useState('')
  const [filter, setFilter] = useState<Filter>('all')

  const add = useMutation({
    mutationFn: () => api.createItemNote(itemId, body),
    onSuccess: () => {
      setBody('')
      setOpen(false)
      setError('')
      void queryClient.invalidateQueries({ queryKey: ['item-notes', itemId] })
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not add note.'),
  })

  function onAdd(event: FormEvent) {
    event.preventDefault()
    if (!body.trim()) {
      setError('Note is required.')
      return
    }
    add.mutate()
  }

  const noteRows = notes.data ?? []
  const eventRows = events.data ?? []
  const log: Row[] = [
    ...noteRows.map((note) => ({ id: `note-${note.id}`, at: note.createdAt, kind: 'note' as const, note })),
    ...eventRows.map((event) => ({ id: `event-${event.id}`, at: event.createdAt, kind: 'event' as const, event })),
  ].sort((a, b) => new Date(b.at).getTime() - new Date(a.at).getTime())
  const remoteKinds = [...new Set(noteRows.map((note) => note.sourceKind))].filter((kind) => kind !== 'manual')
  const filters: Filter[] = ['all']
  if (eventRows.length > 0) filters.push('activity')
  if (noteRows.some((note) => note.sourceKind === 'manual') || eventRows.length === 0) filters.push('manual')
  filters.push(...remoteKinds)
  const shown =
    filter === 'all'
      ? log
      : filter === 'activity'
        ? log.filter((row) => row.kind === 'event')
        : log.filter((row) => row.kind === 'note' && row.note.sourceKind === filter)

  return (
    <section className="dossier-card">
      <div className="tasks-head">
        <h3>Log</h3>
        <button type="button" className="tasks-plus" aria-label="Add note" onClick={() => setOpen((on) => !on)}>
          +
        </button>
      </div>
      {filters.length > 2 || eventRows.length > 0 ? (
        <div className="tasks-tabs small" role="tablist" aria-label="Log origin">
          {filters.map((value) => (
            <button
              key={value}
              type="button"
              role="tab"
              aria-selected={filter === value}
              className={filter === value ? 'on' : undefined}
              onClick={() => setFilter(value)}
            >
              {filterLabel(value)}
            </button>
          ))}
        </div>
      ) : null}
      {open ? (
        <form className="tasks-form" onSubmit={onAdd}>
          <label>
            Note
            <textarea rows={4} value={body} onChange={(e) => setBody(e.target.value)} />
          </label>
          {error ? <p className="error">{error}</p> : null}
          <button type="submit" disabled={add.isPending}>
            Add
          </button>
        </form>
      ) : null}
      {log.length === 0 && !open ? <p className="dossier-empty">No notes yet. Write the first one.</p> : null}
      {log.length > 0 && shown.length === 0 ? (
        <p className="dossier-empty">Nothing from {filterLabel(filter)} yet.</p>
      ) : null}
      {shown.length > 0 ? (
        <ol className="dossier-timeline">
          {shown.map((row) =>
            row.kind === 'note' ? (
              <li
                key={row.id}
                className={row.note.sourceKind === 'manual' ? 'dossier-entry manual' : 'dossier-entry'}
              >
                <p className="dossier-entry-top">
                  <span className={row.note.sourceKind === 'manual' ? 'dossier-badge' : 'dossier-badge source'}>
                    {sourceKindLabel(row.note.sourceKind)}
                  </span>
                  {noteStamp(row.note.createdAt)}
                  {row.note.authorName ? `· ${row.note.authorName}` : ''}
                  <OpenUrl href={row.note.url || ''} label="Open comment" />
                </p>
                <p>{row.note.body}</p>
              </li>
            ) : (
              <li key={row.id} className="dossier-entry activity">
                <p className="dossier-entry-top">
                  <span className="dossier-badge source">Activity</span>
                  {noteStamp(row.event.createdAt)}
                  {row.event.stress != null ? ` · stress ${row.event.stress}` : ''}
                </p>
                <p>{eventLabel(row.event)}</p>
                {row.event.note ? <p className="muted">{row.event.note}</p> : null}
              </li>
            ),
          )}
        </ol>
      ) : null}
    </section>
  )
}
