import { Handshake, Heart, Kanban, Lightning, NotePencil, Scales } from '@phosphor-icons/react'
import { useEffect, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { kindLabel, type Item, type ItemKind } from '../../types'
import { TaskActivity } from './TaskActivity'
import { TaskBrief } from './TaskBrief'
import { TaskConnections } from './TaskConnections'
import { TaskSheet } from './TaskSheet'
import { TaskStats } from './TaskStats'
import { useTaskPatch } from './useTaskPatch'
import './dossier.css'

const TABS = ['brief', 'stats', 'connections', 'activity'] as const

type Tab = (typeof TABS)[number]

const TAB_LABELS: Record<Tab, string> = {
  brief: 'Brief',
  stats: 'Stats',
  connections: 'Connections',
  activity: 'Activity',
}

type Props = {
  id: string
  onGone: () => void
}

function kindIcon(kind: ItemKind): ReactNode {
  const size = 22
  switch (kind) {
    case 'note':
      return <NotePencil size={size} weight="light" />
    case 'agreement':
      return <Handshake size={size} weight="light" />
    case 'obligation':
      return <Scales size={size} weight="light" />
    case 'initiative':
      return <Lightning size={size} weight="light" />
    case 'life':
      return <Heart size={size} weight="light" />
    default:
      return <Kanban size={size} weight="light" />
  }
}

function TaskHero({ item }: { item: Item }) {
  const { patch, hint } = useTaskPatch(item.id)
  const [draft, setDraft] = useState(item.title)

  useEffect(() => setDraft(item.title), [item.id, item.title])

  function save(raw: string) {
    const next = raw.trim()
    if (!next || next === item.title) {
      setDraft(item.title)
      return
    }
    patch.mutate({ body: { title: next }, field: 'title' })
  }

  return (
    <div className="tasks-hero-head">
      <span className="tasks-hero-glyph" title={kindLabel(item.kind)} aria-hidden>
        {kindIcon(item.kind)}
      </span>
      <div className="tasks-sheet-title">
        {item.sourceKind === 'manual' ? (
          <input
            aria-label="Task name"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onBlur={(event) => save(event.currentTarget.value)}
          />
        ) : (
          <h2>{item.title}</h2>
        )}
        {hint('title')}
      </div>
    </div>
  )
}

export function TaskDossierSheet({ id, onClose }: { id: string; onClose: () => void }) {
  const item = useQuery({ queryKey: ['items', id], queryFn: () => api.item(id), enabled: Boolean(id) })
  const row = item.data
  const title = item.isError ? 'Task not found' : row?.title || '…'
  return (
    <TaskSheet
      open
      wide
      title={title}
      kicker="Task"
      titleSlot={row ? <TaskHero item={row} /> : undefined}
      onClose={onClose}
    >
      <TaskDossier id={id} onGone={onClose} />
    </TaskSheet>
  )
}

export function TaskDossier({ id, onGone }: Props) {
  const item = useQuery({ queryKey: ['items', id], queryFn: () => api.item(id), enabled: Boolean(id) })
  const task = useTaskPatch(id)
  const [tab, setTab] = useState<Tab>('brief')

  if (item.isError) {
    return (
      <div className="tasks-missing">
        <p>Task not found</p>
        <button type="button" onClick={onGone}>
          Close
        </button>
      </div>
    )
  }

  const row = item.data
  if (!row) {
    return <p className="muted">Loading…</p>
  }

  return (
    <div className="tasks-dossier">
      <div className="tasks-tabs sticky" role="tablist" aria-label="Task sections">
        {TABS.map((value) => (
          <button
            key={value}
            type="button"
            role="tab"
            aria-selected={tab === value}
            className={tab === value ? 'on' : undefined}
            onClick={() => setTab(value)}
          >
            {TAB_LABELS[value]}
          </button>
        ))}
      </div>
      {tab === 'brief' ? <TaskBrief item={row} task={task} onGone={onGone} /> : null}
      {tab === 'stats' ? <TaskStats item={row} task={task} /> : null}
      {tab === 'connections' ? <TaskConnections item={row} task={task} /> : null}
      {tab === 'activity' ? <TaskActivity itemId={row.id} /> : null}
    </div>
  )
}
