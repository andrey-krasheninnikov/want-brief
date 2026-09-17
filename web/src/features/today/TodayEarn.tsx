import { useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { money } from '../../shared/format'
import { moscowYmd } from '../../shared/moscow'
import {
  DEFAULT_USD_RUB,
  dailyEarn,
  earnChartDays,
  earnChartMonths,
  earnTotals,
  monthEarn,
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

function monthTitle(ymd: string): string {
  return new Intl.DateTimeFormat('en-GB', { month: 'long', timeZone: 'Europe/Moscow' }).format(
    new Date(`${ymd}T12:00:00+03:00`),
  )
}

function barHeight(value: number, peak: number): string {
  return `${peak > 0 ? Math.max(6, (value / peak) * 100) : 6}%`
}

export function TodayEarn() {
  const queryClient = useQueryClient()
  const [rate, setRate] = useState(readRate)
  const [draft, setDraft] = useState(() => String(readRate()))
  const ymd = moscowYmd()
  const yearStart = `${ymd.slice(0, 4)}-01-01`
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const snapshots = useQuery({
    queryKey: ['earn-days', yearStart, ymd],
    queryFn: () => api.earnDays(yearStart, ymd),
  })
  const list = projects.data ?? []
  const live = dailyEarn(list, ymd)
  const earned = todayEarn(list, ymd)
  const month = monthEarn(list)
  const totals = earnTotals(earned.usd, earned.rub, rate)
  const monthTotals = earnTotals(month.usd, month.rub, rate)
  const days = earnChartDays(ymd, snapshots.data ?? [], live, rate)
  const months = earnChartMonths(ymd, snapshots.data ?? [], live, rate)
  const dayPeak = Math.max(0, ...days.map((row) => row.daily))
  const monthPeak = Math.max(0, ...months.map((row) => row.total))
  const bookedMonth = months.at(-1)?.total ?? 0

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
      <section className="clocks">
        <article>
          <div className="today-clock-core">
            <small>Month USD</small>
            <strong className="mono">{money(month.usd, 'en-US')}</strong>
          </div>
        </article>
        <article>
          <div className="today-clock-core">
            <small>Month RUB</small>
            <strong className="mono">{money(month.rub, 'ru-RU')}</strong>
          </div>
        </article>
        <article>
          <div className="today-clock-core">
            <small>Month total</small>
            <strong className="mono">{money(monthTotals.usd, 'en-US')} USD</strong>
            <span className="today-earn-sub mono">{money(monthTotals.rub, 'ru-RU')} RUB</span>
          </div>
        </article>
      </section>
      <div className="today-earn-chart">
        <div className="today-earn-chart-head">
          <small>{monthTitle(ymd)}</small>
        </div>
        <ul className="today-earn-days">
          {days.map((row) => (
            <li key={row.day} title={`${row.day} · ${money(row.daily, 'en-US')} USD`}>
              <b style={{ height: barHeight(row.daily, dayPeak) }} />
              <small>{Number(row.day.slice(8))}</small>
            </li>
          ))}
        </ul>
      </div>
      <div className="today-earn-chart">
        <div className="today-earn-chart-head">
          <small>This year</small>
          <strong className="mono">{money(bookedMonth, 'en-US')} USD</strong>
        </div>
        <ul className="today-earn-months">
          {months.map((row) => (
            <li key={row.month} title={`${row.month} · ${money(row.total, 'en-US')} USD`}>
              <b style={{ height: barHeight(row.total, monthPeak) }} />
              <small>{row.label}</small>
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}
