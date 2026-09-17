import { useEffect, useState, type CSSProperties } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { moscowDayRange } from '../../shared/moscow'
import { openEvent, openTask } from '../../shared/taskOverlay'
import { Window } from '../../shared/Window'
import { DEFAULT_TZ, pickNow, todayBlocks, zonedYmd } from '../schedule/now'
import { ClockRow } from './ClockRow'
import { TodayAgenda } from './TodayAgenda'
import { TodayEarn } from './TodayEarn'
import { TodayNow } from './TodayNow'
import { TodayRisk } from './TodayRisk'
import { atRiskToday, remainingAgenda } from './todayModel'
import './today.css'

function todayKicker(now: Date): string {
  return new Intl.DateTimeFormat('en-GB', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    timeZone: 'Europe/Moscow',
  }).format(now)
}

export function TodayScreen() {
  const navigate = useNavigate()
  const [tick, setTick] = useState(() => Date.now())
  const intervals = useQuery({
    queryKey: ['intervals'],
    queryFn: api.intervals,
    refetchInterval: 10_000,
  })
  const running = intervals.data ?? []
  useEffect(() => {
    const ms = running.length > 0 ? 1_000 : 60_000
    const id = window.setInterval(() => setTick(Date.now()), ms)
    return () => window.clearInterval(id)
  }, [running.length])

  const bounds = moscowDayRange(new Date(tick))
  const schedule = useQuery({
    queryKey: ['schedule', 'work', bounds.from, bounds.to],
    queryFn: () => api.schedule(bounds.from, bounds.to),
    refetchInterval: running.length > 0 ? 10_000 : false,
  })
  const items = useQuery({
    queryKey: ['items'],
    queryFn: () => api.items({ openOnly: true }),
  })

  const tz = schedule.data?.grid.timezone ?? DEFAULT_TZ
  const today = zonedYmd(new Date(tick), tz)
  const lanes = schedule.data?.lanes ?? []
  const blocks = todayBlocks(lanes, today, tz)
  const pick = pickNow(blocks, tick)
  const agenda = remainingAgenda(pick, blocks, schedule.data?.busy ?? [], tick)
  const risk = atRiskToday(blocks, schedule.data?.atRisk ?? [], items.data ?? [], today, tz, tick)
  const packing = schedule.isLoading && !schedule.data
  const names = new Map<string, string>()
  for (const item of items.data ?? []) {
    names.set(item.id, item.externalKey ? `[${item.externalKey}] ${item.title}` : item.title)
  }
  for (const lane of lanes) {
    for (const row of lane.blocks) {
      if (!names.has(row.itemId)) {
        names.set(row.itemId, row.externalKey ? `[${row.externalKey}] ${row.title}` : row.title)
      }
    }
  }

  return (
    <Window className="today-window" kicker={todayKicker(new Date(tick))} title="Today">
      <div className="today-bento">
        <div className="today-tile today-clocks" style={{ '--d': 0 } as CSSProperties}>
          <div className="today-core">
            <ClockRow />
          </div>
        </div>
        <div className="today-tile today-earn" style={{ '--d': 1 } as CSSProperties}>
          <div className="today-core">
            <TodayEarn />
          </div>
        </div>
        <div className="today-tile today-now" style={{ '--d': 2 } as CSSProperties}>
          <div className="today-core">
            <TodayNow
              pick={pick}
              loading={packing}
              running={running}
              tz={tz}
              titleOf={(id) => names.get(id)}
              onOpen={(id) => openTask(navigate, id)}
            />
          </div>
        </div>
        <div className="today-tile today-agenda" style={{ '--d': 3 } as CSSProperties}>
          <div className="today-core">
            <TodayAgenda
              rows={agenda}
              loading={packing}
              error={schedule.isError ? schedule.error.message : undefined}
              tz={tz}
              onOpenWork={(id) => openTask(navigate, id)}
              onOpenMeeting={(seriesId, originalOn) => openEvent(navigate, seriesId, originalOn)}
            />
          </div>
        </div>
        <div className="today-tile today-risk" style={{ '--d': 4 } as CSSProperties}>
          <div className="today-core">
            <TodayRisk
              rows={risk}
              loading={(packing || items.isLoading) && risk.length === 0}
              onOpen={(id) => openTask(navigate, id)}
            />
          </div>
        </div>
      </div>
    </Window>
  )
}
