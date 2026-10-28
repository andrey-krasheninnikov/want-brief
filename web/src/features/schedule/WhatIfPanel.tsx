import { useMutation } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { api } from '../../api'
import { span } from '../../shared/format'
import type { ScheduleBusy, ScheduleLane, WhatIfResult, WhatIfSummary } from '../../types'

type Props = {
  kind: 'work' | 'followup'
  from: string
  to: string
  lanes: ScheduleLane[]
  busy: ScheduleBusy[]
  onClose: () => void
}

type TaskMove = 'keep' | 'week' | 'drop'

type TaskChoice = { itemId: string; key: string; title: string; dueAt: string; project: string }

const WEEK_MS = 7 * 24 * 60 * 60 * 1000

function uniqueTasks(lanes: ScheduleLane[]): TaskChoice[] {
  const seen = new Map<string, TaskChoice>()
  for (const lane of lanes) {
    for (const block of lane.blocks) {
      if (seen.has(block.itemId)) continue
      seen.set(block.itemId, {
        itemId: block.itemId,
        key: block.externalKey,
        title: block.title,
        dueAt: block.dueAt,
        project: lane.projectName,
      })
    }
  }
  return [...seen.values()]
}

function uniqueMeetings(busy: ScheduleBusy[]): { seriesId: string; title: string }[] {
  const seen = new Map<string, string>()
  for (const row of busy) {
    if (row.seriesId && !seen.has(row.seriesId)) seen.set(row.seriesId, row.title)
  }
  return [...seen.entries()].map(([seriesId, title]) => ({ seriesId, title }))
}

function signed(value: number, unit: (v: number) => string): string {
  if (value === 0) return '±0'
  return `${value > 0 ? '+' : '−'}${unit(Math.abs(value))}`
}

function DeltaRow({ label, base, variant, delta, unit }: { label: string; base: number; variant: number; delta: number; unit: (v: number) => string }) {
  const tone = delta < 0 ? 'better' : delta > 0 ? 'worse' : ''
  return (
    <li className={tone}>
      <span>{label}</span>
      <span className="mono">
        {unit(base)} → {unit(variant)}
      </span>
      <em className="mono">{signed(delta, unit)}</em>
    </li>
  )
}

function Summary({ result }: { result: WhatIfResult }) {
  const rows: { label: string; pick: (s: WhatIfSummary) => number; unit: (v: number) => string }[] = [
    { label: 'Late', pick: (s) => s.lateSeconds, unit: span },
    { label: 'Overflow tasks', pick: (s) => s.overflowItems, unit: String },
    { label: 'At risk', pick: (s) => s.atRisk, unit: String },
    { label: 'Fragments', pick: (s) => s.fragments, unit: String },
    { label: 'Switches', pick: (s) => s.switches, unit: String },
    { label: 'Score', pick: (s) => s.score, unit: (v) => (Number.isInteger(v) ? String(v) : v.toFixed(1)) },
  ]
  return (
    <ul className="sched-whatif-result">
      {rows.map((row) => (
        <DeltaRow
          key={row.label}
          label={row.label}
          base={row.pick(result.base)}
          variant={row.pick(result.variant)}
          delta={row.pick(result.delta)}
          unit={row.unit}
        />
      ))}
    </ul>
  )
}

// WhatIfPanel builds a scenario from the visible week and compares the two layouts.
export function WhatIfPanel({ kind, from, to, lanes, busy, onClose }: Props) {
  const tasks = useMemo(() => uniqueTasks(lanes), [lanes])
  const meetings = useMemo(() => uniqueMeetings(busy), [busy])
  const [moves, setMoves] = useState<Record<string, TaskMove>>({})
  const [skips, setSkips] = useState<Set<string>>(() => new Set())

  const run = useMutation({
    mutationFn: () =>
      api.scheduleWhatIf({
        kind,
        from,
        to,
        moveDue: tasks
          .filter((task) => moves[task.itemId] === 'week')
          .map((task) => ({ itemId: task.itemId, dueAt: new Date(new Date(task.dueAt).getTime() + WEEK_MS).toISOString() })),
        dropItems: tasks.filter((task) => moves[task.itemId] === 'drop').map((task) => task.itemId),
        skipEvents: [...skips],
      }),
  })

  const touched = Object.values(moves).some((move) => move !== 'keep') || skips.size > 0

  function setMove(itemId: string, move: TaskMove) {
    setMoves((prev) => ({ ...prev, [itemId]: move }))
  }

  function toggleSkip(seriesId: string) {
    setSkips((prev) => {
      const next = new Set(prev)
      if (next.has(seriesId)) next.delete(seriesId)
      else next.add(seriesId)
      return next
    })
  }

  return (
    <section className="sched-whatif">
      <div className="sched-whatif-core">
        <header className="sched-whatif-head">
          <div>
            <p className="sched-deck-kicker">What if</p>
            <p className="sched-whatif-hint">Move a due date a week, drop a task or skip a meeting, then compare.</p>
          </div>
          <button type="button" className="ghost" onClick={onClose}>
            Close
          </button>
        </header>
        <div className="sched-whatif-grid">
          <div>
            <p className="sched-whatif-label">Tasks this week</p>
            {tasks.length === 0 ? <p className="muted">Nothing packed.</p> : null}
            <ul className="sched-whatif-list">
              {tasks.map((task) => {
                const move = moves[task.itemId] ?? 'keep'
                return (
                  <li key={task.itemId}>
                    <span className="sched-whatif-task">
                      {task.key ? <b className="sched-key">{task.key}</b> : null}
                      <span>{task.title}</span>
                      <small>{task.project}</small>
                    </span>
                    <span className="sched-whatif-moves">
                      {(['keep', 'week', 'drop'] as TaskMove[]).map((value) => (
                        <button
                          key={value}
                          type="button"
                          className={move === value ? 'chip on' : 'chip'}
                          onClick={() => setMove(task.itemId, value)}
                        >
                          {value === 'keep' ? 'Keep' : value === 'week' ? '+1 week' : 'Drop'}
                        </button>
                      ))}
                    </span>
                  </li>
                )
              })}
            </ul>
          </div>
          <div>
            <p className="sched-whatif-label">Meetings</p>
            {meetings.length === 0 ? <p className="muted">No meetings this week.</p> : null}
            <ul className="sched-whatif-list">
              {meetings.map((meeting) => (
                <li key={meeting.seriesId}>
                  <span className="sched-whatif-task">
                    <span>{meeting.title}</span>
                  </span>
                  <span className="sched-whatif-moves">
                    <button
                      type="button"
                      className={skips.has(meeting.seriesId) ? 'chip on' : 'chip'}
                      onClick={() => toggleSkip(meeting.seriesId)}
                    >
                      {skips.has(meeting.seriesId) ? 'Skipped' : 'Skip'}
                    </button>
                  </span>
                </li>
              ))}
            </ul>
          </div>
        </div>
        <div className="sched-whatif-actions">
          <button type="button" disabled={!touched || run.isPending} onClick={() => run.mutate()}>
            {run.isPending ? 'Comparing…' : 'Compare'}
          </button>
          {run.isError ? <p className="error">{run.error.message}</p> : null}
        </div>
        {run.data ? <Summary result={run.data} /> : null}
      </div>
    </section>
  )
}
