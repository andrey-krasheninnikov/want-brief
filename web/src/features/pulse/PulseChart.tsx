import type { CheckinKind, LoadReport } from '../../types'

const series: { kind: CheckinKind; label: string; color: string }[] = [
  { kind: 'stress', label: 'Stress', color: '#e16b80' },
  { kind: 'focus', label: 'Focus', color: '#6152ed' },
  { kind: 'energy', label: 'Energy', color: '#74c991' },
  { kind: 'interest', label: 'Interest', color: '#a79fff' },
  { kind: 'happiness', label: 'Happiness', color: '#f2c14e' },
]

type Point = { x: number; y: number }

function line(logs: LoadReport['stress'], kind: CheckinKind, from: number, to: number): Point[] {
  const span = Math.max(1, to - from)
  return logs
    .filter((row) => (row.kind || 'stress') === kind)
    .slice()
    .sort((a, b) => a.loggedAt.localeCompare(b.loggedAt))
    .map((row) => {
      const at = new Date(row.loggedAt).getTime()
      const x = 36 + ((at - from) / span) * 640
      const y = 18 + ((5 - row.level) / 4) * 148
      return { x, y }
    })
}

function path(points: Point[]): string {
  if (points.length === 0) return ''
  if (points.length === 1) {
    const p = points[0]
    return `M ${p.x - 10} ${p.y} L ${p.x + 10} ${p.y}`
  }
  return points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')
}

export function PulseChart({ report }: { report: LoadReport }) {
  const from = new Date(report.from).getTime()
  const to = new Date(report.to).getTime()
  const logs = report.stress ?? []
  const empty = logs.length === 0

  return (
    <div className="pulse-card">
      <div className="pulse-head">
        <p className="pulse-kicker">Check-in</p>
        <ul className="pulse-legend">
          {series.map((row) => (
            <li key={row.kind}>
              <i style={{ background: row.color }} />
              {row.label}
            </li>
          ))}
        </ul>
      </div>
      {empty ? <p className="muted">No check-ins in this window.</p> : null}
      {!empty ? (
        <svg className="pulse-svg" viewBox="0 0 712 196" role="img" aria-label="Check-in levels over time">
          {[1, 2, 3, 4, 5].map((level) => {
            const y = 18 + ((5 - level) / 4) * 148
            return (
              <g key={level}>
                <line x1="36" x2="676" y1={y} y2={y} className="pulse-grid" />
                <text x="8" y={y + 4} className="pulse-axis">
                  {level}
                </text>
              </g>
            )
          })}
          {series.map((row) => {
            const points = line(logs, row.kind, from, to)
            if (points.length === 0) return null
            return (
              <g key={row.kind}>
                <path d={path(points)} fill="none" stroke={row.color} strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
                {points.map((p, i) => (
                  <circle key={`${row.kind}-${i}`} cx={p.x} cy={p.y} r="3.2" fill={row.color} />
                ))}
              </g>
            )
          })}
        </svg>
      ) : null}
    </div>
  )
}
