import type { CSSProperties } from 'react'
import { Link } from 'react-router-dom'
import type { Item, Quadrant } from '../../types'
import { taskHref } from '../../shared/taskOverlay'
import { rankItems } from './rank'

const labels: Record<Quadrant, string> = {
  do: 'Do',
  schedule: 'Schedule',
  delegate: 'Delegate',
  drop: 'Drop',
}

function dueLabel(iso: string | null): string {
  if (!iso) return '-'
  return new Intl.DateTimeFormat('en-GB', { day: '2-digit', month: 'short' }).format(new Date(iso))
}

export function PriorityList({ items }: { items: Item[] }) {
  const rows = rankItems(items)
  return (
    <div className="prio-shell" style={{ '--d': 4 } as CSSProperties}>
      <div className="prio-core">
        <header className="prio-list-head">
          <h3>Ranked</h3>
          <small>{rows.length} open</small>
        </header>
        {rows.length === 0 ? <p className="muted">No open tasks.</p> : null}
        {rows.length > 0 ? (
          <div className="prio-table-wrap">
            <table className="prio-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th>Quadrant</th>
                  <th>Task</th>
                  <th>Project</th>
                  <th>Due</th>
                  <th>Source</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((item, index) => (
                  <tr key={item.id}>
                    <td className="mono">{String(index + 1).padStart(2, '0')}</td>
                    <td>
                      <span className={`prio-badge ${item.quadrant}`}>{labels[item.quadrant]}</span>
                    </td>
                    <td>
                      <Link to={taskHref(item.id)}>{item.title}</Link>
                    </td>
                    <td>
                      <span className="prio-project">
                        <i style={{ background: item.projectColor || 'rgba(255,255,255,0.2)' }} />
                        {item.projectName || 'unassigned'}
                      </span>
                    </td>
                    <td className="mono">{dueLabel(item.dueAt)}</td>
                    <td>{item.sourceName}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </div>
    </div>
  )
}
