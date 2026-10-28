import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { span } from '../../shared/format'
import { useTimezone } from '../../shared/timezone'
import { zonedInstant, zonedLocalInput, zonedWeek } from '../../shared/zoned'
import { OCCUPANCIES, occupancyLabel, type Item, type Occupancy } from '../../types'
import type { TaskPatch } from './useTaskPatch'

const WEEK_MS = 7 * 24 * 60 * 60 * 1000

function toLocalInput(iso: string | null, tz: string): string {
  return iso ? zonedLocalInput(iso, tz) : ''
}

const OCCUPANCY_NOTE: Record<Occupancy, string> = {
  solo: 'Solo takes the whole slot.',
  parallel: 'Parallel takes the active track; waiting tasks may sit beside it.',
  waiting: 'Waiting sits on a side track next to active work.',
}

function packingDue(item: Item): string | null {
  if (item.kind === 'request') return item.dueAt
  if (item.kind === 'task') return item.devDueAt
  return null
}

function packable(item: Item): boolean {
  return !item.archivedAt && item.plannedSeconds > 0 && Boolean(packingDue(item))
}

function signed(value: number, unit: (v: number) => string = String): string {
  return `${value < 0 ? '−' : '+'}${unit(Math.abs(value))}`
}

function deltaLabel(seconds: number): string {
  if (seconds === 0) return 'no change in lateness'
  return `late ${signed(seconds, span)}`
}

// WhatIfLine answers one question: what happens to the week if this task slips by seven days.
function WhatIfLine({ item }: { item: Item }) {
  const tz = useTimezone()
  const week = zonedWeek(new Date(), tz)
  const due = packingDue(item)
  const query = useQuery({
    queryKey: ['task-whatif', item.id, due, week.from, tz],
    enabled: Boolean(due) && packable(item),
    queryFn: () =>
      api.scheduleWhatIf({
        kind: 'work',
        from: week.from,
        to: week.to,
        moveDue: [{ itemId: item.id, dueAt: new Date(new Date(due!).getTime() + WEEK_MS).toISOString() }],
        dropItems: [],
        skipEvents: [],
      }),
    staleTime: 60_000,
  })
  if (!packable(item)) return null
  if (query.isLoading) return <p className="dossier-note">Checking what a week's slip would do…</p>
  if (!query.data) return null
  const { delta, variant } = query.data
  const parts = [deltaLabel(delta.lateSeconds)]
  if (delta.overflowItems !== 0) parts.push(`overflow ${signed(delta.overflowItems)}`)
  if (delta.atRisk !== 0) parts.push(`at risk ${signed(delta.atRisk)}`)
  parts.push(`score ${variant.score.toFixed(1)} (${delta.score === 0 ? '±0' : signed(delta.score, (v) => v.toFixed(1))})`)
  return <p className="dossier-note">If moved a week later: {parts.join(', ')}.</p>
}

// TaskPacking groups the knobs the packer reads from a task: occupancy, pin instant, slip preview.
export function TaskPacking({ item, task }: { item: Item; task: TaskPatch }) {
  const tz = useTimezone()
  const { patch, hint } = task
  // The draft is valid only against the server value it was typed over; a fresh
  // server value wins without an effect.
  const stored = toLocalInput(item.pinnedAt, tz)
  const [draft, setDraft] = useState({ base: stored, value: stored })
  const pinnedAt = draft.base === stored ? draft.value : stored
  const setPinnedAt = (value: string) => setDraft({ base: stored, value })

  function savePin(raw: string) {
    if (!raw) {
      if (item.pinnedAt) patch.mutate({ body: { pinnedAt: '' }, field: 'pinnedAt' })
      return
    }
    const iso = zonedInstant(raw, tz)
    if (!iso) return
    if (iso === item.pinnedAt) return
    patch.mutate({ body: { pinnedAt: iso }, field: 'pinnedAt' })
  }

  const occupancy = item.occupancy || 'solo'
  return (
    <>
      <label>
        Occupancy
        <select value={occupancy} onChange={(e) => patch.mutate({ body: { occupancy: e.target.value as Occupancy }, field: 'occupancy' })}>
          {OCCUPANCIES.map((value) => (
            <option key={value} value={value}>
              {occupancyLabel(value)}
            </option>
          ))}
        </select>
        <p className="dossier-note">{OCCUPANCY_NOTE[occupancy]}</p>
        {hint('occupancy')}
      </label>
      <label className="tasks-pin-to">
        Pin to
        <DateField mode="datetime" value={pinnedAt} onChange={setPinnedAt} onCommit={savePin} />
        <p className="dossier-note">Start no earlier than this instant; the packer keeps the slot for it.</p>
        {item.pinnedAt ? (
          <button
            type="button"
            className="ghost tasks-due-clear"
            onClick={() => {
              setPinnedAt('')
              savePin('')
            }}
          >
            Clear
          </button>
        ) : null}
        {hint('pinnedAt')}
      </label>
      <WhatIfLine item={item} />
    </>
  )
}
