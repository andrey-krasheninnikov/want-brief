import { ArrowsClockwise } from '@phosphor-icons/react'
import { useCallback, useMemo, useState, type DragEvent, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../../api'
import { dueWithinWeek, itemCost } from '../../shared/format'
import { moscowRange } from '../../shared/moscow'
import {
  KINDS,
  kindLabel,
  statusLabel,
  statusesForKind,
  type Item,
  type ItemKind,
  type ItemStatus,
  type LoadReport,
  type Project,
} from '../../types'
import { DRAG_TYPE, TaskCard, decodeTaskDrag } from './TaskCard'
import { TaskFlag } from './TaskFlag'
import { useTaskLogPrompt } from './TaskLogPrompt'
import { TaskDossierSheet } from './TaskPage'
import { TaskSheet } from './TaskSheet'
import { useTaskUndo } from './TaskUndo'
import { sortColumn } from './sortColumn'

const OPEN_COLUMNS: ItemStatus[] = [
  'backlog',
  'needs_grooming',
  'to_do',
  'in_progress',
  'blocked',
  'review',
  'qa',
  'awaiting_decision',
  'release_candidate',
]
const DONE_COLUMNS: ItemStatus[] = ['done', 'cancelled']
const WIP_LIMIT = 3

function monthSeconds(rows: { itemId: string; allocatedSeconds: number }[] | undefined, id: string): number {
  return rows?.find((row) => row.itemId === id)?.allocatedSeconds ?? 0
}

function projectMonth(rows: { projectId: string | null; allocatedSeconds: number }[] | undefined, id: string | null): number {
  if (!id) return 0
  return rows?.find((row) => row.projectId === id)?.allocatedSeconds ?? 0
}

function cardYield(item: Item, projects: Project[], load: LoadReport | undefined) {
  const project = projects.find((entry) => entry.id === item.projectId)
  if (!project) return undefined
  const monthProject = projectMonth(load?.byProject, item.projectId)
  if (monthProject <= 0) return undefined
  const monthItem = monthSeconds(load?.byItem, item.id)
  if (monthItem <= 0) return undefined
  return {
    usd: itemCost(project.monthlyIncomeUsd, monthItem, monthProject, 'en-US'),
    rub: itemCost(project.monthlyIncomeRub, monthItem, monthProject, 'ru-RU'),
  }
}

function emptyBuckets(): Record<ItemStatus, Item[]> {
  return {
    backlog: [],
    needs_grooming: [],
    to_do: [],
    in_progress: [],
    blocked: [],
    review: [],
    qa: [],
    awaiting_decision: [],
    release_candidate: [],
    done: [],
    cancelled: [],
  }
}

export function TasksScreen() {
  return <TasksWorkspace />
}

function TasksWorkspace() {
  const promptLog = useTaskLogPrompt()
  const offerUndo = useTaskUndo()
  const { id } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const [params, setParams] = useSearchParams()
  const queryClient = useQueryClient()
  const query = params.get('q') ?? ''
  const projectId = params.get('project') ?? ''
  const sourceId = params.get('source') ?? ''
  const kind = (params.get('kind') ?? '') as ItemKind | ''
  const stall = (params.get('stall') as 'active' | 'stalled' | 'all') || 'active'
  const onlyU = params.get('u') === '1'
  const onlyI = params.get('i') === '1'
  const week = params.get('week') === '1'
  const showDone = params.get('done') === '1'
  const closeDossier = useCallback(() => {
    navigate({ pathname: '/tasks', search: location.search })
  }, [navigate, location.search])
  const [open, setOpen] = useState(false)
  const [createStatus, setCreateStatus] = useState<ItemStatus>('backlog')
  const [dropStatus, setDropStatus] = useState<ItemStatus | null>(null)
  const [dragging, setDragging] = useState<{ id: string; kind: ItemKind; status: ItemStatus } | null>(null)
  const [title, setTitle] = useState('')
  const [newKind, setNewKind] = useState<ItemKind>('task')
  const [formError, setFormError] = useState('')
  const sources = useQuery({ queryKey: ['sources'], queryFn: api.sources })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const monthLoad = useQuery({
    queryKey: ['load', 'month'],
    queryFn: () => {
      const bounds = moscowRange('month')
      return api.load(bounds.from, bounds.to)
    },
  })
  const items = useQuery({
    queryKey: ['items', sourceId, projectId, kind, showDone, stall],
    queryFn: () =>
      api.items({
        sourceId: sourceId || undefined,
        projectId: projectId || undefined,
        kind: kind || undefined,
        openOnly: !showDone,
        includeArchived: stall === 'all',
        archivedOnly: stall === 'stalled',
      }),
  })
  const intervals = useQuery({ queryKey: ['intervals'], queryFn: api.intervals, refetchInterval: 1000 })
  const create = useMutation({
    mutationFn: async () => {
      const item = await api.createItem({ title: title.trim(), kind: newKind })
      if (createStatus === 'backlog') return item
      if (!statusesForKind(newKind).includes(createStatus)) return item
      return api.patchItem(item.id, { status: createStatus })
    },
    onSuccess: (item) => {
      setTitle('')
      setFormError('')
      setOpen(false)
      setCreateStatus('backlog')
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      navigate({ pathname: `/tasks/${item.id}`, search: location.search })
    },
    onError: (err) => setFormError(err instanceof Error ? err.message : 'Could not add.'),
  })
  const syncActive = useMutation({
    mutationFn: () => api.syncActiveItems(),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['items'] })
    },
  })
  const move = useMutation({
    mutationFn: ({ id: itemId, status }: { id: string; status: ItemStatus; previous: ItemStatus }) =>
      api.patchItem(itemId, { status }),
    onSuccess: (_row, vars) => {
      offerUndo('Status updated.', async () => {
        await api.patchItem(vars.id, { status: vars.previous })
        void queryClient.invalidateQueries({ queryKey: ['items'] })
      })
      promptLog(vars.id, 'status')
      void queryClient.invalidateQueries({ queryKey: ['items'] })
    },
  })

  function setFilter(key: string, value: string) {
    const next = new URLSearchParams(params)
    if (!value) next.delete(key)
    else next.set(key, value)
    setParams(next, { replace: true })
  }

  function setFlag(key: string, on: boolean) {
    const next = new URLSearchParams(params)
    if (on) next.set(key, '1')
    else next.delete(key)
    setParams(next, { replace: true })
  }

  function clearFilters() {
    setParams(new URLSearchParams(), { replace: true })
  }

  const filterCount = [
    query,
    projectId,
    sourceId,
    kind,
    stall !== 'active' ? stall : '',
    onlyU ? '1' : '',
    onlyI ? '1' : '',
    week ? '1' : '',
    showDone ? '1' : '',
  ].filter(Boolean).length

  function openCreate(status: ItemStatus) {
    setCreateStatus(status)
    const allowed = KINDS.filter((value) => statusesForKind(value).includes(status))
    setNewKind(allowed.includes(newKind) ? newKind : (allowed[0] ?? 'task'))
    setFormError('')
    setOpen(true)
  }

  function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (!title.trim()) {
      setFormError('Title is required.')
      return
    }
    setFormError('')
    create.mutate()
  }

  function payloadFrom(event: DragEvent): { id: string; kind: Item['kind'] } | null {
    return decodeTaskDrag(event.dataTransfer.getData(DRAG_TYPE) || event.dataTransfer.getData('text/plain'))
  }

  function onDragOver(status: ItemStatus, event: DragEvent) {
    event.preventDefault()
    const kindNow = dragging?.kind
    const allowed = !kindNow || statusesForKind(kindNow).includes(status)
    event.dataTransfer.dropEffect = allowed ? 'move' : 'none'
    setDropStatus(status)
  }

  function applyMove(itemId: string, status: ItemStatus) {
    const current = (items.data ?? []).find((item) => item.id === itemId)
    if (!current || current.status === status) return
    if (!statusesForKind(current.kind).includes(status)) return
    move.mutate({ id: itemId, status, previous: current.status })
  }

  function onDrop(status: ItemStatus, event: DragEvent) {
    event.preventDefault()
    setDropStatus(null)
    setDragging(null)
    const payload = payloadFrom(event) ?? (dragging ? { id: dragging.id, kind: dragging.kind } : null)
    if (!payload || !statusesForKind(payload.kind).includes(status)) return
    applyMove(payload.id, status)
  }

  function clearDrag() {
    setDropStatus(null)
    setDragging(null)
  }

  const list = useMemo(() => {
    const needle = query.trim().toLowerCase()
    const now = Date.now()
    return (items.data ?? []).filter((item) => {
      if (needle && !`${item.externalKey} ${item.title}`.toLowerCase().includes(needle)) return false
      if (onlyU && !item.urgent) return false
      if (onlyI && !item.important) return false
      if (
        week &&
        !['dueAt', 'devDueAt', 'reviewDueAt', 'testDueAt'].some((key) =>
          dueWithinWeek(item[key as 'dueAt'], now),
        )
      )
        return false
      return true
    })
  }, [items.data, query, onlyU, onlyI, week])
  const runningByItem = new Map((intervals.data ?? []).map((row) => [row.itemId, row]))
  const visibleStatuses = showDone ? [...OPEN_COLUMNS, ...DONE_COLUMNS] : OPEN_COLUMNS

  const grouped = useMemo(() => {
    const buckets = emptyBuckets()
    for (const item of list) {
      buckets[item.status]?.push(item)
    }
    for (const status of Object.keys(buckets) as ItemStatus[]) {
      buckets[status].sort(sortColumn)
    }
    return buckets
  }, [list])

  const createKinds = KINDS.filter((value) => statusesForKind(value).includes(createStatus))
  const projectList = projects.data ?? []
  const showSkeleton = items.isLoading && !items.data
  const noMatch = Boolean(items.data && list.length === 0 && filterCount > 0)

  function renderColumn(status: ItemStatus, narrow = false) {
    const rows = grouped[status]
    const over = dropStatus === status && dragging
    const allowed = !dragging || statusesForKind(dragging.kind).includes(status)
    const blocked = Boolean(over && !allowed)
    const dropping = Boolean(over && allowed && dragging && dragging.status !== status)
    const wipWarn = status === 'in_progress' && rows.length >= WIP_LIMIT
    const cls = ['tasks-col', narrow ? 'narrow' : '', blocked ? 'blocked' : '', dropping ? 'drop' : '']
      .filter(Boolean)
      .join(' ')
    return (
      <section
        key={status}
        className={cls}
        onDragOver={(event) => onDragOver(status, event)}
        onDrop={(event) => onDrop(status, event)}
      >
        <header className="tasks-col-head">
          <h2>
            {statusLabel(status)} <span className={wipWarn ? 'warn' : undefined}>{rows.length}</span>
          </h2>
          <button
            type="button"
            className="tasks-plus"
            aria-label={`Add to ${statusLabel(status)}`}
            onClick={() => openCreate(status)}
          >
            +
          </button>
        </header>
        {rows.map((item) => {
          const yieldRow = cardYield(item, projectList, monthLoad.data)
          return (
            <TaskCard
              key={item.id}
              item={item}
              selected={item.id === id}
              running={runningByItem.get(item.id)}
              yieldUsd={yieldRow?.usd}
              yieldRub={yieldRow?.rub}
              visibleStatuses={visibleStatuses}
              onDragBegin={(row) => setDragging({ id: row.id, kind: row.kind, status: row.status })}
              onMove={applyMove}
            />
          )
        })}
        {dropping ? <div className="tasks-col-ghost" aria-hidden /> : null}
        {rows.length === 0 && !dropping ? <p className="tasks-col-empty">Drop tasks here</p> : null}
        {blocked ? <p className="tasks-col-blocked">Not for this type</p> : null}
      </section>
    )
  }

  return (
    <div className="tasks">
      <header className="tasks-bar">
        <h1>
          Tasks <span className="tasks-count">{list.length}</span>
        </h1>
        <label className="tasks-search">
          Search
          <input
            value={query}
            onChange={(e) => setFilter('q', e.target.value)}
            placeholder="Title or key"
            autoComplete="off"
          />
        </label>
        <label className="tasks-toggle">
          <input type="checkbox" checked={showDone} onChange={(e) => setFlag('done', e.target.checked)} />
          Show done
        </label>
        <label className="tasks-toggle">
          Stall
          <select
            value={stall}
            onChange={(e) => setFilter('stall', e.target.value === 'active' ? '' : e.target.value)}
            aria-label="Stall filter"
          >
            <option value="active">Active</option>
            <option value="stalled">Stalled</option>
            <option value="all">All</option>
          </select>
        </label>
        <div className="tasks-chips">
          <TaskFlag glyph="🏃" label="Urgent" on={onlyU} onClick={() => setFlag('u', !onlyU)} />
          <TaskFlag glyph="🔑" label="Important" on={onlyI} onClick={() => setFlag('i', !onlyI)} />
          <button type="button" className={week ? 'chip on' : 'chip'} onClick={() => setFlag('week', !week)}>
            Due this week
          </button>
          {filterCount > 0 ? (
            <button type="button" className="chip" onClick={clearFilters}>
              {filterCount} filters · Clear
            </button>
          ) : null}
        </div>
        <div className="tasks-filters">
          <select value={projectId} onChange={(e) => setFilter('project', e.target.value)} aria-label="Project">
            <option value="">All projects</option>
            {projectList.map((project) => (
              <option key={project.id} value={project.id}>
                {project.archivedAt ? `${project.name} (archived)` : project.name}
              </option>
            ))}
          </select>
          <select value={sourceId} onChange={(e) => setFilter('source', e.target.value)} aria-label="Source">
            <option value="">All sources</option>
            {(sources.data ?? []).map((source) => (
              <option key={source.id} value={source.id}>
                {source.name}
              </option>
            ))}
          </select>
          <select value={kind} onChange={(e) => setFilter('kind', e.target.value)} aria-label="Type">
            <option value="">All types</option>
            {KINDS.map((value) => (
              <option key={value} value={value}>
                {kindLabel(value)}
              </option>
            ))}
          </select>
        </div>
        <button
          type="button"
          className="ghost tasks-sync"
          title="Sync active"
          aria-label="Sync active"
          disabled={syncActive.isPending}
          onClick={() => syncActive.mutate()}
        >
          <ArrowsClockwise size={18} weight="light" />
        </button>
        <button type="button" className="tasks-add" onClick={() => openCreate('backlog')}>
          Add task
        </button>
      </header>
      {items.isError ? <p className="error">{items.error.message}</p> : null}
      {syncActive.isError ? (
        <p className="error">{syncActive.error instanceof Error ? syncActive.error.message : 'Could not sync.'}</p>
      ) : null}
      {noMatch ? (
        <p className="tasks-empty-match">
          No tasks match{query ? ` “${query}”` : ''}.{' '}
          <button type="button" className="ghost" onClick={clearFilters}>
            Reset filters
          </button>
        </p>
      ) : null}
      {showSkeleton ? (
        <div className="tasks-kanban" aria-hidden>
          {OPEN_COLUMNS.map((status) => (
            <div key={status} className="tasks-skel-col" />
          ))}
        </div>
      ) : items.data || !items.isError ? (
        <div className="tasks-board" onDragEnd={clearDrag}>
          <div className="tasks-kanban-wrap">
            <div className="tasks-kanban">{OPEN_COLUMNS.map((status) => renderColumn(status))}</div>
          </div>
          {showDone ? <div className="tasks-done-stack">{DONE_COLUMNS.map((status) => renderColumn(status, true))}</div> : null}
        </div>
      ) : null}
      <TaskSheet
        open={open}
        title={createStatus === 'backlog' ? 'Add task' : `Add to ${statusLabel(createStatus)}`}
        onClose={() => {
          setOpen(false)
          setCreateStatus('backlog')
          setFormError('')
        }}
      >
        <form className="tasks-form" onSubmit={onSubmit}>
          <label>
            Title
            <input value={title} onChange={(e) => setTitle(e.target.value)} autoComplete="off" autoFocus />
          </label>
          <label>
            Type
            <select value={newKind} onChange={(e) => setNewKind(e.target.value as ItemKind)}>
              {createKinds.map((value) => (
                <option key={value} value={value}>
                  {kindLabel(value)}
                </option>
              ))}
            </select>
          </label>
          {formError ? <p className="error">{formError}</p> : null}
          <button type="submit">Create</button>
        </form>
      </TaskSheet>
      {id ? <TaskDossierSheet id={id} onClose={closeDossier} /> : null}
    </div>
  )
}
