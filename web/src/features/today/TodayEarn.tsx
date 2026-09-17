import { useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { money } from '../../shared/format'
import { moscowYmd } from '../../shared/moscow'
import {
  DEFAULT_USD_RUB,
  dailyEarn,
  earnChartDays,
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
  const queryClient = useQueryClient()
  const [rate, setRate] = useState(readRate)
  const [draft, setDraft] = useState(() => String(readRate()))
  const ymd = moscowYmd()
  const monthStart = `${ymd.slice(0, 7)}-01`
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const snapshots = useQuery({
    queryKey: ['earn-days', monthStart, ymd],
    queryFn: () => api.earnDays(monthStart, ymd),
  })
  const live = dailyEarn(projects.data ?? [], ymd)
  const earned = todayEarn(projects.data ?? [], ymd)
  const totals = earnTotals(earned.usd, earned.rub, rate)
  const points = earnChartDays(ymd, snapshots.data ?? [], live, rate)
  const mtd = points.at(-1)?.cumulative ?? 0
  const peak = mtd

  useEffect(() => {
    if (!projects.isSuccess) return
    let cancelled = false
    void api.putEarnDay(ymd, { usd: earned.usd, rub: earned.rub }).then(() => {
      if (!cancelled) void queryClient.invalidateQueries({ queryKey: ['earn-days'] })
    })
    return () => {
      cancelled = true
    }
  }, [projects.isSuccess, ymd, earned.usd, earned.rub, queryClient])

  function commitRate(raw: string) {
    const next = parseUsdRubRate(raw, rate)
    setRate(next)
    setDraft(String(next))
    storeRate(next)
  }

  return (
    <section className="today-earn-board">
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
      <div className="today-earn-chart">
        <div className="today-earn-chart-head">
          <small>Month to date</small>
          <strong className="mono">{money(mtd, 'en-US')} USD</strong>
        </div>
        <ul className="today-earn-days">
          {points.map((row) => (
            <li key={row.day} title={`${row.day} · ${money(row.cumulative, 'en-US')} USD`}>
              <b
                style={{
                  height: `${peak > 0 ? Math.max(6, (row.cumulative / peak) * 100) : 6}%`,
                }}
              />
              <small>{Number(row.day.slice(8))}</small>
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}
