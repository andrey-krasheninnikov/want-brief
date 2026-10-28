import { CalendarBlank, Clock, Coins, CurrencyDollar, CurrencyRub, Target } from '@phosphor-icons/react'
import { useEffect, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { createdLabel, itemCost, liveTracked, span } from '../../shared/format'
import { moscowRange } from '../../shared/moscow'
import type { Item } from '../../types'
import { TaskTimer } from './TaskTimer'
import type { TaskPatch } from './useTaskPatch'

type Props = {
  item: Item
  task: TaskPatch
}

function StatTile({ label, value, icon }: { label: string; value: string; icon: ReactNode }) {
  return (
    <div className="dossier-tile">
      <span className="dossier-tile-text">
        <p>{label}</p>
        <strong>{value}</strong>
      </span>
      <span className="dossier-tile-icon" aria-hidden>
        {icon}
      </span>
    </div>
  )
}

function itemSeconds(rows: { itemId: string; allocatedSeconds: number }[] | undefined, id: string): number {
  return rows?.find((row) => row.itemId === id)?.allocatedSeconds ?? 0
}

function projectSeconds(rows: { projectId: string | null; allocatedSeconds: number }[] | undefined, id: string | null): number {
  if (!id) return 0
  return rows?.find((row) => row.projectId === id)?.allocatedSeconds ?? 0
}

export function TaskStats({ item, task }: Props) {
  const { patch, hint } = task
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const intervals = useQuery({ queryKey: ['intervals'], queryFn: api.intervals, refetchInterval: 1000 })
  const monthLoad = useQuery({
    queryKey: ['load', 'month'],
    queryFn: () => {
      const bounds = moscowRange('month')
      return api.load(bounds.from, bounds.to)
    },
  })
  const [planHours, setPlanHours] = useState(String(Math.floor(item.plannedSeconds / 3600)))
  const [planMinutes, setPlanMinutes] = useState(String(Math.floor((item.plannedSeconds % 3600) / 60)))

  useEffect(() => {
    setPlanHours(String(Math.floor(item.plannedSeconds / 3600)))
    setPlanMinutes(String(Math.floor((item.plannedSeconds % 3600) / 60)))
  }, [item.id, item.plannedSeconds])

  function savePlan(hoursRaw: string, minutesRaw: string) {
    const h = Number(hoursRaw)
    const m = Number(minutesRaw)
    if (!Number.isFinite(h) || !Number.isFinite(m) || h < 0 || m < 0 || m > 59) return
    const seconds = Math.round(h) * 3600 + Math.round(m) * 60
    if (seconds === item.plannedSeconds) return
    patch.mutate({ body: { plannedSeconds: seconds }, field: 'estimate' })
  }

  const running = (intervals.data ?? []).find((entry) => entry.itemId === item.id)
  const trackedLive = liveTracked(item.trackedSeconds, running?.startedAt)
  const project = (projects.data ?? []).find((entry) => entry.id === item.projectId)
  const monthItem = itemSeconds(monthLoad.data?.byItem, item.id)
  const monthProject = projectSeconds(monthLoad.data?.byProject, item.projectId)
  const usd = project?.monthlyIncomeUsd ?? 0
  const rub = project?.monthlyIncomeRub ?? 0
  const share = item.plannedSeconds > 0 ? (trackedLive / item.plannedSeconds) * 100 : 0
  const percent = Math.round(share)
  const over = share > 100

  return (
    <>
      <section className="dossier-card">
        <header>
          <h3>Time</h3>
          <p>Tracked against the estimate.</p>
        </header>
        <div className="dossier-gauge">
          <div
            className={over ? 'dossier-ring over' : 'dossier-ring'}
            style={{ ['--ring-p' as string]: Math.min(percent, 100) }}
          >
            <span className="dossier-ring-value">
              <strong>{item.plannedSeconds > 0 ? `${percent}%` : '—'}</strong>
              <span>used</span>
            </span>
          </div>
          <div className="dossier-gauge-facts">
            {item.plannedSeconds > 0 ? (
              <p className="mono">
                {span(trackedLive)} of {span(item.plannedSeconds)}
              </p>
            ) : (
              <p className="dossier-note">No estimate yet. Set hours below to track the burn.</p>
            )}
            {over ? <p className="error">Over the estimate by {span(trackedLive - item.plannedSeconds)}.</p> : null}
            <TaskTimer itemId={item.id} running={running} prominent />
          </div>
        </div>
        <div className="dossier-controls">
          <label>
            Estimate · hours
            <input
              type="number"
              min={0}
              step={1}
              value={planHours}
              onChange={(e) => setPlanHours(e.target.value)}
              onBlur={(e) => savePlan(e.currentTarget.value, planMinutes)}
            />
          </label>
          <label>
            Estimate · minutes
            <input
              type="number"
              min={0}
              max={59}
              step={1}
              value={planMinutes}
              onChange={(e) => setPlanMinutes(e.target.value)}
              onBlur={(e) => savePlan(planHours, e.currentTarget.value)}
            />
          </label>
        </div>
        {hint('estimate')}
      </section>

      <section className="dossier-card">
        <header>
          <h3>Load</h3>
          <p>Time and money this task pulls.</p>
        </header>
        <div className="dossier-tiles">
          <StatTile label="Tracked lifetime" value={span(trackedLive)} icon={<Clock size={18} weight="light" />} />
          <StatTile label="This month" value={span(monthItem)} icon={<CalendarBlank size={18} weight="light" />} />
          {project ? (
            <>
              <StatTile
                label="This month · $"
                value={itemCost(usd, monthItem, monthProject, 'en-US')}
                icon={<CurrencyDollar size={18} weight="light" />}
              />
              <StatTile
                label="This month · ₽"
                value={itemCost(rub, monthItem, monthProject, 'ru-RU')}
                icon={<CurrencyRub size={18} weight="light" />}
              />
              <StatTile
                label="If estimate · $"
                value={itemCost(usd, item.plannedSeconds, monthProject, 'en-US')}
                icon={<Target size={18} weight="light" />}
              />
              <StatTile
                label="If estimate · ₽"
                value={itemCost(rub, item.plannedSeconds, monthProject, 'ru-RU')}
                icon={<Coins size={18} weight="light" />}
              />
            </>
          ) : null}
        </div>
        {project ? null : <p className="dossier-empty">Link a project in Connections to see the money split.</p>}
        <p className="dossier-note">Created · {createdLabel(item.createdAt)}</p>
      </section>
    </>
  )
}
