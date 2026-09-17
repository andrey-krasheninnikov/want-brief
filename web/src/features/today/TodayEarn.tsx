import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { money } from '../../shared/format'
import { moscowRange, moscowYmd } from '../../shared/moscow'
import {
  DEFAULT_USD_RUB,
  earnTotals,
  parseUsdRubRate,
  todayEarn,
} from './todayModel'

const RATE_KEY = 'today-usd-rub'

function readRate(): number {
  try {
    const raw = window.localStorage.getItem(RATE_KEY)
    if (raw == null) return DEFAULT_USD_RUB
    return parseUsdRubRate(raw)
  } catch {
    return DEFAULT_USD_RUB
  }
}

function storeRate(rate: number) {
  try {
    window.localStorage.setItem(RATE_KEY, String(rate))
  } catch {
    // storage unavailable, keep in memory only
  }
}

export function TodayEarn() {
  const [rate, setRate] = useState(readRate)
  const [draft, setDraft] = useState(() => String(readRate()))
  const load = useQuery({
    queryKey: ['load', 'day'],
    queryFn: () => {
      const bounds = moscowRange('day')
      return api.load(bounds.from, bounds.to)
    },
    refetchInterval: 5000,
  })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const earned = todayEarn(projects.data ?? [], load.data?.byProject ?? [], moscowYmd())
  const totals = earnTotals(earned.usd, earned.rub, rate)

  function commitRate(raw: string) {
    const next = parseUsdRubRate(raw, rate)
    setRate(next)
    setDraft(String(next))
    storeRate(next)
  }

  return (
    <section className="clocks">
      <article>
        <div className="today-clock-core">
          <small>Earned USD</small>
          <strong className="mono">{money(earned.usd, 'en-US')}</strong>
        </div>
      </article>
      <article>
        <div className="today-clock-core">
          <small>Earned RUB</small>
          <strong className="mono">{money(earned.rub, 'ru-RU')}</strong>
        </div>
      </article>
      <article>
        <div className="today-clock-core">
          <small>Total</small>
          <strong className="mono">{money(totals.usd, 'en-US')} USD</strong>
          <span className="today-earn-sub mono">{money(totals.rub, 'ru-RU')} RUB</span>
          <label className="today-earn-rate">
            1 USD =
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
                if (Number.isFinite(next) && next > 0) {
                  setRate(next)
                  storeRate(next)
                }
              }}
              onBlur={(event) => commitRate(event.currentTarget.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') event.currentTarget.blur()
              }}
            />
            RUB
          </label>
        </div>
      </article>
    </section>
  )
}
