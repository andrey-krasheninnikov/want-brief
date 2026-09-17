import { useState } from 'react'
import { parseUsdRubRate } from './todayModel'

export function TodayFxRate({ rate, onChange }: { rate: number; onChange: (rate: number) => void }) {
  const [draft, setDraft] = useState(String(rate))
  const [seen, setSeen] = useState(rate)
  if (rate !== seen) {
    setSeen(rate)
    setDraft(String(rate))
  }

  function commit(raw: string) {
    const next = parseUsdRubRate(raw, rate)
    setDraft(String(next))
    onChange(next)
  }

  return (
    <section className="clocks today-fx-board">
      <article>
        <div className="today-clock-core">
          <small>Rate</small>
          <label className="today-fx">
            <span>1 USD</span>
            <input
              type="number"
              min="0.01"
              step="0.01"
              value={draft}
              aria-label="RUB per USD"
              onChange={(event) => {
                const raw = event.target.value
                setDraft(raw)
                const next = Number(raw)
                if (Number.isFinite(next) && next > 0) onChange(next)
              }}
              onBlur={(event) => commit(event.currentTarget.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') event.currentTarget.blur()
              }}
            />
            <span>RUB</span>
          </label>
        </div>
      </article>
    </section>
  )
}
