import { DotsSixVertical, UsersThree } from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useRef, useState, type DragEvent, type KeyboardEvent, type MouseEvent } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { api } from '../../api'
import { createdLabel, liveTracked, span } from '../../shared/format'
import { kindLabel, occupancyLabel, statusLabel, statusesForKind, type Item, type ItemStatus, type TimeInterval } from '../../types'
import { TaskConfirm } from './TaskConfirm'
import { TaskDueRail } from './TaskDueRail'
import { TaskFlag } from './TaskFlag'
import { TaskTimeLog } from './TaskTimeLog'
import { TaskTimer } from './TaskTimer'
import { useTaskUndo } from './TaskUndo'

const DRAG_TYPE = 'application/x-want-item'

export function encodeTaskDrag(item: Item): string {
  return JSON.stringify({ id: item.id, kind: item.kind })
}

export function decodeTaskDrag(raw: string): { id: string; kind: Item['kind'] } | null {
  try {
    const data = JSON.parse(raw) as { id?: string; kind?: Item['kind'] }
    if (!data.id || !data.kind) return null
    return { id: data.id, kind: data.kind }
  } catch {
    return null
  }
}

export { DRAG_TYPE }

function cardTone(item: Item): string {
  if (item.archivedAt) return 'tone-stall'
  if (item.status === 'blocked') return 'tone-blocked'
  if (item.status === 'awaiting_decision') return 'tone-awaiting'
  if (item.status === 'in_progress') return 'tone-progress'
  return ''
}

function hasDue(item: Item): boolean {
  return Boolean(item.dueAt || item.devDueAt || item.reviewDueAt || item.testDueAt)
}

function foreignStatus(item: Item): string {
  const raw = item.externalStatus.trim()
  if (!raw) return ''
  if (raw === item.status || raw === statusLabel(item.status)) return ''
  return raw
}

type Props = {
  item: Item
  running?: TimeInterval
  yieldUsd?: string
  yieldRub?: string
  selected?: boolean
  visibleStatuses: ItemStatus[]
  onDragBegin: (item: Item) => void
  onMove: (id: string, status: ItemStatus) => void
}

export function TaskCard({
  item,
  running,
  yieldUsd,
  yieldRub,
  selected,
  visibleStatuses,
  onDragBegin,
  onMove,
}: Props) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const location = useLocation()
  const offerUndo = useTaskUndo()
  const dragged = useRef(false)
  const [modal, setModal] = useState<'stall' | 'time' | null>(null)
  const patch = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.patchItem(item.id, body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['items'] }),
  })
  const syncRemote = useMutation({
    mutationFn: () => api.syncItem(item.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      void queryClient.invalidateQueries({ queryKey: ['sources'] })
    },
  })
  const tracked = liveTracked(item.trackedSeconds, running?.startedAt)
  const planned = item.plannedSeconds
  const overflow = planned > 0 && tracked > planned
  const ratio = planned > 0 ? Math.min(1, tracked / planned) : 0
  const showYield = Boolean(yieldUsd || yieldRub)
  const checks = item.checkTotal > 0
  const checkRatio = checks ? Math.min(1, item.checkDone / item.checkTotal) : 0
  const foreign = foreignStatus(item)
  const canSync = item.sourceKind === 'jira' || item.sourceKind === 'todoist'
  const showCreated = !item.externalKey && !hasDue(item)
  const href = { pathname: `/tasks/${item.id}`, search: location.search }

  function applyStall() {
    const next = !item.archivedAt
    const previous = Boolean(item.archivedAt)
    patch.mutate(
      { archived: next },
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

  function open() {
    navigate(href)
  }

  function onDragStart(event: DragEvent) {
    dragged.current = true
    event.dataTransfer.setData(DRAG_TYPE, encodeTaskDrag(item))
    event.dataTransfer.setData('text/plain', encodeTaskDrag(item))
    event.dataTransfer.effectAllowed = 'move'
    onDragBegin(item)
  }

  function onCardClick(event: MouseEvent) {
    if (dragged.current) {
      dragged.current = false
      return
    }
    const target = event.target as HTMLElement
    if (target.closest('button, a, .tasks-card-handle, input, select, textarea')) return
    open()
  }

  function onKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      event.preventDefault()
      open()
      return
    }
    if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return
    const allowed = visibleStatuses.filter((status) => statusesForKind(item.kind).includes(status))
    const index = allowed.indexOf(item.status)
    const next = allowed[index + (event.key === 'ArrowRight' ? 1 : -1)]
    if (!next) return
    event.preventDefault()
    onMove(item.id, next)
  }

  const classes = [
    'tasks-card',
    item.archivedAt ? 'archived' : '',
    selected ? 'current' : '',
    cardTone(item),
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <article
      className={classes}
      aria-current={selected ? 'true' : undefined}
      tabIndex={0}
      onClick={onCardClick}
      onKeyDown={onKeyDown}
      onDragEnd={() => {
        window.setTimeout(() => {
          dragged.current = false
        }, 0)
      }}
    >
      <div
        className="tasks-card-handle"
        draggable
        aria-label="Move"
        title="Move"
        onClick={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
        onDragStart={onDragStart}
      >
        <DotsSixVertical size={16} weight="light" />
      </div>
      <div className="tasks-card-core">
        <div className="tasks-card-top">
          <span className="tasks-kind">{kindLabel(item.kind)}</span>
          {item.occupancy === 'parallel' || item.occupancy === 'waiting' ? (
            <span className="tasks-occupancy" title={occupancyLabel(item.occupancy)}>
              <UsersThree size={14} weight="light" />
              {occupancyLabel(item.occupancy)}
            </span>
          ) : null}
          {item.archivedAt ? <span className="tasks-stall">Stalled</span> : null}
        </div>
        <Link className="tasks-card-title" to={href} draggable={false} title={item.title}>
          {item.externalKey ? <span className="tasks-key mono">[{item.externalKey}]</span> : null}
          {item.externalKey ? ' ' : null}
          {item.title}
        </Link>
        {item.projectName || foreign || showCreated ? (
          <p className="tasks-card-meta">
            {item.projectName ? (
              <span className="tasks-project">
                <i className="tasks-swatch" style={{ background: item.projectColor || 'var(--accent)' }} />
                {item.projectName}
              </span>
            ) : null}
            {foreign ? <span className="tasks-ext muted">{foreign}</span> : null}
            {showCreated && item.createdAt ? <span className="muted">{createdLabel(item.createdAt)}</span> : null}
          </p>
        ) : null}
        <TaskDueRail item={item} compact />
        {planned > 0 ? (
          <div className={overflow ? 'tasks-track overflow' : 'tasks-track'}>
            <span className="tasks-track-rail">
              <span className="tasks-track-bar" style={{ width: `${Math.round(ratio * 100)}%` }} />
            </span>
            <small>
              {span(tracked)} / {span(planned)}
            </small>
          </div>
        ) : (
          <p className="tasks-track-fact muted">{span(tracked)}</p>
        )}
        {checks ? (
          <div className="tasks-track">
            <span className="tasks-track-rail">
              <span className="tasks-track-bar" style={{ width: `${Math.round(checkRatio * 100)}%` }} />
            </span>
            <small>
              {item.checkDone}/{item.checkTotal}
            </small>
          </div>
        ) : null}
        {showYield ? (
          <p className="tasks-yield mono">
            This month · ${yieldUsd} / ₽{yieldRub}
          </p>
        ) : null}
        <div className="tasks-card-flags">
          <TaskFlag glyph="🏃" label="Urgent" on={item.urgent} onClick={() => patch.mutate({ urgent: !item.urgent })} />
          <TaskFlag
            glyph="🔑"
            label="Important"
            on={item.important}
            onClick={() => patch.mutate({ important: !item.important })}
          />
          <TaskFlag glyph="📌" label="Pin" on={item.pinned} onClick={() => patch.mutate({ pinned: !item.pinned })} />
          {canSync ? (
            <TaskFlag
              glyph="🔄"
              label={item.sourceKind === 'todoist' ? 'Pull Todoist' : 'Pull Jira'}
              onClick={() => syncRemote.mutate()}
            />
          ) : null}
          <TaskFlag glyph="⏱️" label="Log time" onClick={() => setModal('time')} />
          <TaskFlag
            glyph={item.archivedAt ? '↩️' : '⏸️'}
            label={item.archivedAt ? 'Restore' : 'Stall'}
            on={Boolean(item.archivedAt)}
            onClick={() => setModal('stall')}
          />
          <TaskTimer itemId={item.id} running={running} />
        </div>
      </div>
      {modal === 'stall' ? (
        <TaskConfirm
          open
          title={item.archivedAt ? 'Return this task to the board?' : 'Stall this task?'}
          body={item.archivedAt ? 'It will show in Active again.' : 'Hidden from schedule, priorities, and default lists.'}
          confirmLabel={item.archivedAt ? 'Restore' : 'Stall'}
          busy={patch.isPending}
          onCancel={() => setModal(null)}
          onConfirm={applyStall}
        />
      ) : null}
      {modal === 'time' ? <TaskTimeLog itemId={item.id} open onClose={() => setModal(null)} /> : null}
    </article>
  )
}
