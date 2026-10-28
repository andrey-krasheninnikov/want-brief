import { useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { money } from '../../shared/format'
import { moscowYmd } from '../../shared/moscow'
import { TodayEarnPlot } from './TodayEarnPlot'
import { TodayFxRate } from './TodayFxRate'
import {
  DEFAULT_USD_RUB,
  dailyEarn,
  earnChartDays,
  earnChartMonths,
  earnTotals,
  monthAvgRate,
  monthEarn,
  snapRate,
  todayEarn,
} from './todayModel'

function monthTitle(ymd: string): string {
  return new Intl.DateTimeFormat('en-GB', { month: 'long', timeZone: 'Europe/Moscow' }).format(
    new Date(`${ymd}T12:00:00+03:00`),
  )
}

export function TodayEarn() {
  const queryClient = useQueryClient()
  const ymd = moscowYmd()
  const yearStart = `${ymd.slice(0, 4)}-01-01`
  const [rate, setRate] = useState<number | null>(null)
  const [rateDay, setRateDay] = useState(ymd)
  if (rateDay !== ymd) {
    setRateDay(ymd)
    setRate(null)
  }
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const snapshots = useQuery({
    queryKey: ['earn-days', yearStart, ymd],
    queryFn: () => api.earnDays(yearStart, ymd),
  })
  const list = projects.data ?? []
  const snaps = snapshots.data ?? []
  const todaySnap = snaps.find((row) => row.day === ymd)
  const fx = rate ?? snapRate(todaySnap, DEFAULT_USD_RUB)
  const live = dailyEarn(list)
  const earned = todayEarn(list, ymd)
  const month = monthEarn(list)
  const avg = monthAvgRate(ymd, snaps, fx)
  const totals = earnTotals(earned.usd, earned.rub, fx)
  const monthTotals = earnTotals(month.usd, month.rub, avg)
  const days = earnChartDays(ymd, snaps, live, fx)
  const months = earnChartMonths(ymd, snaps, live, fx)
  const dayTotal = days.reduce((sum, row) => sum + row.daily, 0)
  const monthPlotTotal = months.at(-1)?.total ?? 0

  useEffect(() => {
    if (!projects.isSuccess || !snapshots.isSuccess) return
    let cancelled = false
    void api.putEarnDay(ymd, { usd: earned.usd, rub: earned.rub, rate: fx }).then(() => {
      if (!cancelled) void queryClient.invalidateQueries({ queryKey: ['earn-days'] })
    })
    return () => {
      cancelled = true
    }
  }, [projects.isSuccess, snapshots.isSuccess, ymd, earned.usd, earned.rub, fx, queryClient])

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
          </div>
        </article>
      </section>
      <TodayFxRate rate={fx} onChange={setRate} />
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
      <TodayEarnPlot
        title={monthTitle(ymd)}
        total={dayTotal}
        rows={days.map((row) => ({
          key: row.day,
          value: row.daily,
          label: String(Number(row.day.slice(8))),
        }))}
      />
      <TodayEarnPlot
        title="This year"
        total={monthPlotTotal}
        rows={months.map((row) => ({
          key: row.month,
          value: row.total,
          label: row.label,
        }))}
      />
    </section>
  )
}
