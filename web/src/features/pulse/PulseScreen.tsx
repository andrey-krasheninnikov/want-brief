import { Heartbeat, Lightning, Smiley, Sparkle, Target, type Icon } from '@phosphor-icons/react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { hours } from '../../shared/format'
import { moscowRange, type LoadPeriod } from '../../shared/moscow'
import { taskHref } from '../../shared/taskOverlay'
import { Window } from '../../shared/Window'
import type { LoadReport } from '../../types'
import { PulseChart } from './PulseChart'
import { PulseJournal } from './PulseJournal'
import './pulse.css'

const periods: { id: LoadPeriod; label: string }[] = [
  { id: 'day', label: 'Day' },
  { id: 'week', label: 'Week' },
  { id: 'month', label: 'Month' },
  { id: 'year', label: 'Year' },
]

const kpis: { id: keyof NonNullable<LoadReport['averages']>; label: string; Icon: Icon }[] = [
  { id: 'stress', label: 'Stress', Icon: Heartbeat },
  { id: 'focus', label: 'Focus', Icon: Target },
  { id: 'energy', label: 'Energy', Icon: Lightning },
  { id: 'interest', label: 'Interest', Icon: Sparkle },
  { id: 'happiness', label: 'Happiness', Icon: Smiley },
]

function score(value: number | null | undefined): string {
  return value == null ? '-' : value.toFixed(1)
}

export function PulseScreen() {
  const [period, setPeriod] = useState<LoadPeriod>('week')
  const load = useQuery({
    queryKey: ['load', period],
    queryFn: () => {
      const bounds = moscowRange(period)
      return api.load(bounds.from, bounds.to)
    },
    placeholderData: keepPreviousData,
  })
  const report = load.data
  const allocated = report?.allocatedSeconds ?? 0
  const wall = report?.wallSeconds ?? 0
  const overlap = Math.max(0, allocated - wall)
  const avgs = report?.averages
  const dayMax = Math.max(1, ...(report?.byDay ?? []).map((row) => row.allocatedSeconds))
  const projectMax = Math.max(1, ...(report?.byProject ?? []).map((row) => row.allocatedSeconds))
  const items = [...(report?.byItem ?? [])].sort((a, b) => b.allocatedSeconds - a.allocatedSeconds)
  const days = report?.byDay ?? []
  const projects = report?.byProject ?? []

  return (
    <Window
      className="pulse-window"
      kicker="Analysis"
      title="Pulse"
      actions={
        <div className="pulse-chips" role="tablist" aria-label="Period">
          {periods.map((item) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              aria-selected={period === item.id}
              className={period === item.id ? 'pulse-chip on' : 'pulse-chip'}
              onClick={() => setPeriod(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>
      }
    >
      <div className="pulse-dash">
        <div className="pulse-kpis">
          {kpis.map((item) => (
            <article key={item.id} className="pulse-card pulse-kpi">
              <span className="pulse-kpi-icon" aria-hidden>
                <item.Icon size={18} weight="light" />
              </span>
              <div>
                <span>{item.label}</span>
                <strong>{score(avgs?.[item.id])}</strong>
              </div>
            </article>
          ))}
        </div>
        <div className="pulse-charts">
          {!report ? (
            <div className="pulse-card">
              <p className="pulse-kicker">Check-in</p>
              <p className="muted">{load.isError ? load.error.message : 'Reading the window…'}</p>
            </div>
          ) : (
            <PulseChart report={report} />
          )}
          <div className="pulse-card">
            <div className="pulse-head">
              <p className="pulse-kicker">Hours by day</p>
              <dl className="pulse-load">
                <div>
                  <dt>Allocated</dt>
                  <dd className="mono">{hours(allocated)}</dd>
                </div>
                <div>
                  <dt>Wall</dt>
                  <dd className="mono">{hours(wall)}</dd>
                </div>
                <div>
                  <dt>Overlap</dt>
                  <dd className="mono">{hours(overlap)}</dd>
                </div>
              </dl>
            </div>
            {days.length === 0 ? <p className="muted">No tracked time in this window.</p> : null}
            {days.length > 0 ? (
              <ul className="pulse-days">
                {days.map((row) => (
                  <li key={row.date}>
                    <span>{row.date.slice(5)}</span>
                    <b style={{ height: `${Math.max(6, (row.allocatedSeconds / dayMax) * 100)}%` }} />
                    <small>{hours(row.allocatedSeconds)}</small>
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        </div>
        <div className="pulse-pair">
          <div className="pulse-card">
            <p className="pulse-kicker">Projects</p>
            {projects.length === 0 ? <p className="muted">No tracked time in this window.</p> : null}
            {projects.length > 0 ? (
              <ul className="bars">
                {projects.map((row) => (
                  <li key={row.projectId ?? row.name}>
                    <span>{row.name}</span>
                    <b style={{ width: `${(row.allocatedSeconds / projectMax) * 100}%`, background: row.color }} />
                    <small>
                      {hours(row.allocatedSeconds)}
                      {row.targetHoursWeek ? ` / ${row.targetHoursWeek}h` : ''}
                    </small>
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
          <div className="pulse-card">
            <p className="pulse-kicker">Tasks</p>
            {items.length === 0 ? <p className="muted">No time on tasks.</p> : null}
            {items.length > 0 ? (
              <div className="pulse-table-wrap">
                <table className="pulse-table">
                  <thead>
                    <tr>
                      <th>Task</th>
                      <th>Kind</th>
                      <th>Project</th>
                      <th>Hours</th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((row) => (
                      <tr key={row.itemId}>
                        <td>
                          <Link to={taskHref(row.itemId)}>{row.title}</Link>
                        </td>
                        <td>{row.kind}</td>
                        <td>{row.projectName || 'unassigned'}</td>
                        <td className="mono">{hours(row.allocatedSeconds)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : null}
          </div>
        </div>
        <PulseJournal />
      </div>
    </Window>
  )
}
