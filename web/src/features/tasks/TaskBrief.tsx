import { ArrowCounterClockwise, ArrowsClockwise, ChatTeardrop, Clock, Copy, Pause, Trash } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { OpenUrl } from '../../shared/UrlField'
import { createdLabel } from '../../shared/format'
import {
  KINDS,
  kindLabel,
  statusesForKind,
  statusLabel,
  type Item,
  type ItemKind,
  type ItemStatus,
  type ProjectLink,
} from '../../types'
import { TaskChecks } from './TaskChecks'
import { TaskConfirm } from './TaskConfirm'
import { TaskDueRail } from './TaskDueRail'
import { TaskFlag } from './TaskFlag'
import { TaskPacking } from './TaskPacking'
import { TaskTimeLog } from './TaskTimeLog'
import { useTaskLogPrompt } from './TaskLogPrompt'
import { TaskTimer } from './TaskTimer'
import { useTaskUndo } from './TaskUndo'
import type { TaskPatch } from './useTaskPatch'

type Props = {
  item: Item
  task: TaskPatch
  onGone: () => void
}

function toLocalInput(iso: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function isoFromInput(raw: string): string {
  return new Date(raw).toISOString()
}

function sourceHref(links: ProjectLink[]): string {
  return links.find((link) => link.label === 'Jira')?.url || links.find((link) => /^https?:/.test(link.url))?.url || ''
}

function statusTone(item: Item): string {
  if (item.archivedAt) return 'tone-stall'
  if (item.status === 'blocked') return 'tone-blocked'
  if (item.status === 'awaiting_decision') return 'tone-awaiting'
  if (item.status === 'in_progress') return 'tone-progress'
  return ''
}

export function TaskBrief({ item, task, onGone }: Props) {
  const { patch, mark, hint } = task
  const promptLog = useTaskLogPrompt()
  const offerUndo = useTaskUndo()
  const queryClient = useQueryClient()
  const sources = useQuery({ queryKey: ['sources'], queryFn: api.sources })
  const intervals = useQuery({ queryKey: ['intervals'], queryFn: api.intervals, refetchInterval: 1000 })
  const [description, setDescription] = useState(item.description)
  const [dueAt, setDueAt] = useState(toLocalInput(item.dueAt))
  const [devDueAt, setDevDueAt] = useState(toLocalInput(item.devDueAt))
  const [reviewDueAt, setReviewDueAt] = useState(toLocalInput(item.reviewDueAt))
  const [testDueAt, setTestDueAt] = useState(toLocalInput(item.testDueAt))
  const [externalKey, setExternalKey] = useState(item.externalKey)
  const [copied, setCopied] = useState(false)
  const [kindConfirm, setKindConfirm] = useState<ItemKind | null>(null)
  const [modal, setModal] = useState<'stall' | 'delete' | 'time' | null>(null)

  useEffect(() => {
    setDescription(item.description)
    setDueAt(toLocalInput(item.dueAt))
    setDevDueAt(toLocalInput(item.devDueAt))
    setReviewDueAt(toLocalInput(item.reviewDueAt))
    setTestDueAt(toLocalInput(item.testDueAt))
    setExternalKey(item.externalKey)
  }, [
    item.id,
    item.updatedAt,
    item.description,
    item.dueAt,
    item.devDueAt,
    item.reviewDueAt,
    item.testDueAt,
    item.externalKey,
  ])

  const syncRemote = useMutation({
    mutationFn: () => api.syncItem(item.id),
    onSuccess: () => {
      mark('sync', 'saved')
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      void queryClient.invalidateQueries({ queryKey: ['item-notes', item.id] })
      void queryClient.invalidateQueries({ queryKey: ['sources'] })
    },
    onError: (err) => mark('sync', 'error', err instanceof Error ? err.message : 'Could not sync.'),
  })
  const remove = useMutation({
    mutationFn: () => api.deleteItem(item.id),
    onSuccess: () => {
      setModal(null)
      offerUndo('Task deleted.', async () => {
        await api.undeleteItem(item.id)
        void queryClient.invalidateQueries({ queryKey: ['items'] })
        void queryClient.invalidateQueries({ queryKey: ['schedule'] })
        void queryClient.invalidateQueries({ queryKey: ['load'] })
        void queryClient.invalidateQueries({ queryKey: ['people'] })
      })
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      void queryClient.invalidateQueries({ queryKey: ['schedule'] })
      void queryClient.invalidateQueries({ queryKey: ['load'] })
      void queryClient.invalidateQueries({ queryKey: ['people'] })
      onGone()
    },
    onError: (err) => mark('delete', 'error', err instanceof Error ? err.message : 'Could not delete.'),
  })

  function saveDescription(raw: string) {
    if (raw.trim() === item.description) return
    patch.mutate({ body: { description: raw }, field: 'description' })
  }

  function saveDueField(raw: string, current: string | null, key: string) {
    if (!raw) {
      if (current) patch.mutate({ body: { [key]: '' }, field: key })
      return
    }
    const iso = isoFromInput(raw)
    if (iso === current) return
    patch.mutate({ body: { [key]: iso }, field: key })
  }

  function saveKey(raw: string) {
    const next = raw.trim()
    if (next === item.externalKey) return
    patch.mutate({ body: { externalKey: next }, field: 'key' })
  }

  function applyKind(next: ItemKind) {
    const body: Record<string, unknown> = { kind: next }
    if (!statusesForKind(next).includes(item.status)) body.status = 'backlog'
    patch.mutate({ body, field: 'kind' })
  }

  function saveKind(next: ItemKind) {
    if (next === item.kind) return
    if (!statusesForKind(next).includes(item.status)) {
      setKindConfirm(next)
      return
    }
    applyKind(next)
  }

  function saveStatus(status: ItemStatus) {
    if (status === item.status) return
    const previous = item.status
    patch.mutate(
      { body: { status }, field: 'status' },
      {
        onSuccess: () => {
          promptLog(item.id, 'status')
          offerUndo('Status updated.', async () => {
            await api.patchItem(item.id, { status: previous })
            void queryClient.invalidateQueries({ queryKey: ['items'] })
          })
        },
      },
    )
  }

  function applyStall() {
    const next = !item.archivedAt
    const previous = Boolean(item.archivedAt)
    patch.mutate(
      { body: { archived: next }, field: 'stall' },
      {
        onSuccess: () => {
          setModal(null)
          offerUndo(next ? 'Task stalled.' : 'Task restored.', async () => {
            await api.patchItem(item.id, { archived: previous })
            void queryClient.invalidateQueries({ queryKey: ['items'] })
          })
        },
      },
    )
  }

  const manual = item.sourceKind === 'manual'
  const remote = item.sourceKind === 'jira' || item.sourceKind === 'todoist'
  const running = (intervals.data ?? []).find((entry) => entry.itemId === item.id)
  const source = (sources.data ?? []).find((entry) => entry.id === item.sourceId)
  const lastSync = source?.lastSyncAt ? createdLabel(source.lastSyncAt) : 'never'
  const pullLabel = item.sourceKind === 'todoist' ? 'Pull Todoist' : 'Pull Jira'
  const link = sourceHref(item.links ?? [])
  const identity = Boolean(item.archivedAt || item.externalStatus || (!manual && item.externalKey) || link)

  return (
    <>
      <section className="dossier-card">
        {identity ? (
          <div className="dossier-identity">
            {item.archivedAt ? <span className="dossier-chip tone-stall">Stalled</span> : null}
            {item.externalStatus ? (
              <span className={`dossier-chip ${statusTone(item)}`.trim()}>{item.externalStatus}</span>
            ) : null}
            {!manual && item.externalKey ? (
              <button
                type="button"
                className="dossier-chip mono"
                aria-label="Copy key"
                onClick={() => {
                  void navigator.clipboard.writeText(item.externalKey)
                  setCopied(true)
                  window.setTimeout(() => setCopied(false), 1200)
                }}
              >
                <Copy size={14} weight="light" aria-hidden />
                {copied ? 'Copied' : item.externalKey}
              </button>
            ) : null}
            <OpenUrl href={link} label="Open source" />
          </div>
        ) : null}
        {manual ? (
          <label className="tasks-key-field">
            External ID
            <input
              value={externalKey}
              onChange={(e) => setExternalKey(e.target.value)}
              onBlur={(e) => saveKey(e.currentTarget.value)}
              autoComplete="off"
            />
            {hint('key')}
          </label>
        ) : null}
        <div className="dossier-controls">
          <label>
            Type
            <select value={item.kind} onChange={(e) => saveKind(e.target.value as ItemKind)}>
              {KINDS.map((value) => (
                <option key={value} value={value}>
                  {kindLabel(value)}
                </option>
              ))}
            </select>
            {hint('kind')}
          </label>
          <label>
            Status
            <select value={item.status} onChange={(e) => saveStatus(e.target.value as ItemStatus)}>
              {statusesForKind(item.kind).map((value) => (
                <option key={value} value={value}>
                  {statusLabel(value)}
                </option>
              ))}
            </select>
            {hint('status')}
          </label>
        </div>
        <div className="dossier-bar">
          <div className="tasks-actions">
            {remote ? (
              <button
                type="button"
                className="ghost"
                disabled={syncRemote.isPending}
                title={`${pullLabel} · ${lastSync}`}
                aria-label={pullLabel}
                onClick={() => syncRemote.mutate()}
              >
                <ArrowsClockwise size={18} weight="light" />
              </button>
            ) : null}
            <button type="button" className="ghost" title="Log" aria-label="Log" onClick={() => promptLog(item.id, 'status')}>
              <ChatTeardrop size={18} weight="light" />
            </button>
            <button
              type="button"
              className="ghost"
              title="Log time"
              aria-label="Log time"
              onClick={() => setModal('time')}
            >
              <Clock size={18} weight="light" />
            </button>
            <button
              type="button"
              className="ghost"
              title={item.archivedAt ? 'Restore' : 'Stall'}
              aria-label={item.archivedAt ? 'Restore' : 'Stall'}
              onClick={() => setModal('stall')}
            >
              {item.archivedAt ? <ArrowCounterClockwise size={18} weight="light" /> : <Pause size={18} weight="light" />}
            </button>
            <button
              type="button"
              className="ghost danger"
              title="Delete"
              aria-label="Delete"
              disabled={remove.isPending}
              onClick={() => setModal('delete')}
            >
              <Trash size={18} weight="light" />
            </button>
            {hint('sync')}
            {hint('delete')}
          </div>
          <TaskTimer itemId={item.id} running={running} prominent />
        </div>
      </section>

      <section className="dossier-card">
        <header>
          <h3>Priority</h3>
          <p>Drives the matrix and the schedule order.</p>
        </header>
        <div className="tasks-chips">
          <TaskFlag
            glyph="🏃"
            label="Urgent"
            showLabel
            on={item.urgent}
            onClick={() => patch.mutate({ body: { urgent: !item.urgent }, field: 'urgent' })}
          />
          <TaskFlag
            glyph="🔑"
            label="Important"
            showLabel
            on={item.important}
            onClick={() => patch.mutate({ body: { important: !item.important }, field: 'important' })}
          />
          <TaskFlag
            glyph="📌"
            label="Pin"
            showLabel
            on={item.pinned}
            onClick={() => patch.mutate({ body: { pinned: !item.pinned }, field: 'pinned' })}
          />
        </div>
        <TaskPacking item={item} task={task} />
      </section>

      <section className="dossier-card">
        <label>
          Description
          <textarea
            rows={5}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            onBlur={(e) => saveDescription(e.currentTarget.value)}
          />
          {hint('description')}
        </label>
      </section>

      <section className="dossier-card">
        <header>
          <h3>Deadlines</h3>
          <p>Task, dev, review and test dates.</p>
        </header>
        <TaskDueRail
          item={item}
          values={{ dueAt, devDueAt, reviewDueAt, testDueAt }}
          onChange={(key, next) => {
            if (key === 'dueAt') setDueAt(next)
            if (key === 'devDueAt') setDevDueAt(next)
            if (key === 'reviewDueAt') setReviewDueAt(next)
            if (key === 'testDueAt') setTestDueAt(next)
          }}
          onCommit={(key, raw) => saveDueField(raw, item[key], key)}
        />
      </section>

      <section className="dossier-card">
        <TaskChecks itemId={item.id} />
      </section>

      <TaskConfirm
        open={kindConfirm != null}
        title="Type change resets status to Backlog"
        body="This type cannot keep the current status."
        confirmLabel="Change type"
        onCancel={() => setKindConfirm(null)}
        onConfirm={() => {
          if (kindConfirm) applyKind(kindConfirm)
          setKindConfirm(null)
        }}
      />
      <TaskConfirm
        open={modal === 'stall'}
        title={item.archivedAt ? 'Return this task to the board?' : 'Stall this task?'}
        body={item.archivedAt ? 'It will show in Active again.' : 'Hidden from schedule, priorities, and default lists.'}
        confirmLabel={item.archivedAt ? 'Restore' : 'Stall'}
        busy={patch.isPending}
        onCancel={() => setModal(null)}
        onConfirm={applyStall}
      />
      <TaskConfirm
        open={modal === 'delete'}
        title={`Delete “${item.externalKey || item.title}”?`}
        body="This hides it from sync."
        confirmLabel="Delete"
        danger
        busy={remove.isPending}
        onCancel={() => setModal(null)}
        onConfirm={() => remove.mutate()}
      />
      <TaskTimeLog itemId={item.id} open={modal === 'time'} onClose={() => setModal(null)} />
    </>
  )
}
