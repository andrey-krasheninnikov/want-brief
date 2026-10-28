import type { RiskRow } from './todayModel'

type Props = {
  rows: RiskRow[]
  loading: boolean
  onOpen: (itemId: string) => void
}

export function TodayRisk({ rows, loading, onOpen }: Props) {
  const empty = !loading && rows.length === 0

  return (
    <section className="agenda">
      <h3>At risk</h3>
      {loading ? (
        <div className="agenda-skels" aria-hidden>
          <div className="agenda-skel" />
        </div>
      ) : null}
      {empty ? <p className="muted">Nothing at risk.</p> : null}
      {rows.map((row) => {
        const title = row.key ? `[${row.key}] ${row.title}` : row.title
        return (
          <article key={row.itemId} className="agenda-row" onClick={() => onOpen(row.itemId)}>
            <div className="today-agenda-core">
              <div className="agenda-body">
                <strong>{title}</strong>
                {row.overdue ? <span className="agenda-chip late">overdue</span> : null}
                {row.late ? <span className="agenda-chip late">late</span> : null}
                {!row.overdue && !row.late ? <span className="agenda-chip">due</span> : null}
              </div>
            </div>
          </article>
        )
      })}
    </section>
  )
}
