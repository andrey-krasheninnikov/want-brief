import { money } from '../../shared/format'

export type EarnPlotRow = {
  key: string
  value: number
  label: string
}

function chartMax(values: number[]): number {
  const peak = Math.max(0, ...values)
  if (peak <= 0) return 1
  const exp = 10 ** Math.floor(Math.log10(peak))
  const n = peak / exp
  const nice = n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10
  return nice * exp
}

export function TodayEarnPlot({
  title,
  total,
  rows,
}: {
  title: string
  total: number
  rows: EarnPlotRow[]
}) {
  const max = chartMax(rows.map((row) => row.value))
  const ticks = [max, max / 2, 0]

  return (
    <div className="today-earn-plot">
      <div className="today-earn-plot-well">
        <ol className="today-earn-plot-axis" aria-hidden="true">
          {ticks.map((tick, index) => (
            <li key={index}>{money(tick, 'en-US')}</li>
          ))}
        </ol>
        <div className="today-earn-plot-track">
          <ul className="today-earn-plot-cols">
            {rows.map((row) => (
              <li key={row.key}>
                <div className="today-earn-plot-bar">
                  <span style={{ height: `${(row.value / max) * 100}%` }} />
                </div>
                <b>{money(row.value, 'en-US')}</b>
                <small>{row.label}</small>
              </li>
            ))}
          </ul>
        </div>
      </div>
      <div className="today-earn-plot-foot">
        <small>{title}</small>
        <strong className="mono">{money(total, 'en-US')} USD</strong>
      </div>
    </div>
  )
}
