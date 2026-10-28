import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type CSSProperties } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { zonedInstant, zonedYmd } from '../../shared/zoned'
import { openTask } from '../../shared/taskOverlay'
import type { UnplannedItem } from '../../types'

function dueField(item: UnplannedItem['item'], kind: 'work' | 'followup'): 'devDueAt' | 'reviewDueAt' | 'testDueAt' | 'dueAt' {
  if (item.kind === 'request') return 'dueAt'
  if (kind !== 'followup') return 'devDueAt'
  if (item.status === 'review') return 'reviewDueAt'
  if (item.status === 'qa') return 'testDueAt'
  return 'dueAt'
}

function dueTitle(field: ReturnType<typeof dueField>, itemKind: string): string {
  if (field === 'reviewDueAt') return 'Review due'
  if (field === 'testDueAt') return 'Tests due'
  if (field === 'dueAt') return itemKind === 'request' ? 'Due' : 'Task due'
  return 'Dev due'
}

function shiftYmd(ymd: string, days: number): string {
  const [year, month, day] = ymd.split('-').map(Number)
  const next = new Date(Date.UTC(year, month - 1, day + days))
  return next.toISOString().slice(0, 10)
}

function nextFriday1800(tz: string, now = new Date()): string {
  const ymd = zonedYmd(now, tz)
  const [year, month, day] = ymd.split('-').map(Number)
  const weekday = new Date(Date.UTC(year, month - 1, day, 12)).getUTCDay()
  const friday = shiftYmd(ymd, (5 - weekday + 7) % 7)
  const at = zonedInstant(`${friday}T18:00`, tz)!
  return new Date(at).getTime() <= now.getTime() ? zonedInstant(`${shiftYmd(friday, 7)}T18:00`, tz)! : at
}

function NeedCard({ row, delay, kind, tz }: { row: UnplannedItem; delay: number; kind: 'work' | 'followup'; tz: string }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [due, setDue] = useState('')
  const [hours, setHours] = useState('0')
  const [minutes, setMinutes] = useState('0')
  const field = dueField(row.item, kind)
  const patch = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.patchItem(row.item.id, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['schedule'] })
      void queryClient.invalidateQueries({ queryKey: ['items'] })
    },
  })
  const gaps: string[] = []
  if (row.missingDevDue) gaps.push(field === 'devDueAt' ? 'Missing dev due' : 'Missing due')
  if (row.missingPlan) gaps.push('Missing estimate')

  function saveDue(raw: string) {
    if (!raw) return
    const iso = zonedInstant(raw, tz)
    if (iso) patch.mutate({ [field]: iso })
  }

  function savePlan(hoursRaw: string, minutesRaw: string) {
    const h = Number(hoursRaw)
    const m = Number(minutesRaw)
    if (!Number.isFinite(h) || !Number.isFinite(m) || h < 0 || m < 0 || m > 59) return
    const seconds = Math.round(h) * 3600 + Math.round(m) * 60
    if (seconds <= 0) return
    patch.mutate({ plannedSeconds: seconds })
  }

  return (
    <article className="sched-need" style={{ '--d': delay } as CSSProperties}>
      <div className="sched-need-core">
        <p className="sched-need-eye">{gaps.join(' · ')}</p>
        <strong>
          <button type="button" className="ghost" onClick={() => openTask(navigate, row.item.id)}>
            {row.item.externalKey ? <span className="mono">[{row.item.externalKey}] </span> : null}
            {row.item.title}
          </button>
        </strong>
        <small>{row.item.projectName || 'No project'}</small>
        {row.missingDevDue ? (
          <>
            <label>
              {dueTitle(field, row.item.kind)}
              <DateField mode="datetime" value={due} onChange={setDue} onCommit={saveDue} />
            </label>
            <button type="button" className="ghost sched-need-chip" onClick={() => patch.mutate({ [field]: nextFriday1800(tz) })}>
              Friday 18:00
            </button>
          </>
        ) : null}
        {row.missingPlan ? (
          <>
            <form
              className="sched-need-plan"
              onSubmit={(e) => {
                e.preventDefault()
                savePlan(hours, minutes)
              }}
            >
              <label>
                Hours
                <input
                  type="number"
                  min={0}
                  step={1}
                  value={hours}
                  onChange={(e) => setHours(e.target.value)}
                  onBlur={(e) => savePlan(e.currentTarget.value, minutes)}
                />
              </label>
              <label>
                Minutes
                <input
                  type="number"
                  min={0}
                  max={59}
                  step={1}
                  value={minutes}
                  onChange={(e) => setMinutes(e.target.value)}
                  onBlur={(e) => savePlan(hours, e.currentTarget.value)}
                />
              </label>
            </form>
            <div className="sched-need-chips">
              {[1, 2, 4].map((h) => (
                <button
                  key={h}
                  type="button"
                  className="ghost sched-need-chip"
                  onClick={() => patch.mutate({ plannedSeconds: h * 3600 })}
                >
                  {h}h
                </button>
              ))}
            </div>
          </>
        ) : null}
        {patch.isError ? <p className="error">{patch.error.message}</p> : null}
      </div>
    </article>
  )
}

// rows arrive in packer order from the server.
export function NeedsFields({ rows, kind, tz }: { rows: UnplannedItem[]; kind: 'work' | 'followup'; tz: string }) {
  if (rows.length === 0) return null
  return (
    <aside className="sched-needs">
      <p className="sched-need-kicker">Needs fields</p>
      {rows.map((row, i) => (
        <NeedCard key={row.item.id} row={row} delay={i} kind={kind} tz={tz} />
      ))}
    </aside>
  )
}
