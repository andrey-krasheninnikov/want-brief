import { BellSimple, CalendarBlank, CaretRight, GearSix } from '@phosphor-icons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api'
import { span } from '../../shared/format'
import { moscowWeek, shiftWeeks } from '../../shared/moscow'
import { openEvent, openTask } from '../../shared/taskOverlay'
import { Window } from '../../shared/Window'
import { DayOverridePopover } from './DayOverridePopover'
import { NeedsFields } from './NeedsFields'
import { NowDeck, type DeckProject } from './NowDeck'
import { ScheduleGantt } from './ScheduleGantt'
import { ScheduleTable } from './ScheduleTable'
import { WhatIfPanel } from './WhatIfPanel'
import { DEFAULT_TZ, clock, covering, dayHead, minutesToClock, pickChartDay, pickNow, todayBlocks, topReasons, zonedMinutes, zonedYmd } from './now'
import {
  scheduleReasonLabel,
  type AtRiskItem,
  type DayOverride,
  type DayOverrideDraft,
  type ScheduleBlock,
  type ScheduleBusy,
  type ScheduleCapacity,
  type ScheduleGrid,
  type ScheduleLane,
  type ScheduleOverflow,
  type ScheduleScore,
} from '../../types'
import './schedule.css'

const HOUR = 48
const VIEW_KEY = 'schedule.view'
const FALLBACK_GRID: ScheduleGrid = {
  timezone: DEFAULT_TZ,
  startHour: 8,
  endHour: 19,
  workdays: [1, 2, 3, 4, 5],
  workStartMin: 10 * 60,
  workEndMin: 19 * 60,
}

type View = 'calendar' | 'list' | 'gantt'

// Grid is the vertical scale of the calendar, derived from the packer's response.
type Grid = { tz: string; start: number; hours: number; height: number; workStartMin: number; workEndMin: number }

function readView(): View {
  try {
    const raw = window.localStorage.getItem(VIEW_KEY)
    if (raw === 'list' || raw === 'gantt') return raw
  } catch {
    // storage unavailable
  }
  return 'calendar'
}

function storeView(view: View) {
  try {
    window.localStorage.setItem(VIEW_KEY, view)
  } catch {
    // storage unavailable, keep in memory only
  }
}

function gridOf(raw?: ScheduleGrid): Grid {
  const grid = raw ?? FALLBACK_GRID
  const hours = Math.max(1, grid.endHour - grid.startHour)
  return {
    tz: grid.timezone || DEFAULT_TZ,
    start: grid.startHour,
    hours,
    height: hours * HOUR,
    workStartMin: grid.workStartMin,
    workEndMin: grid.workEndMin,
  }
}

function hasMeeting(busy: ScheduleBusy[], ymd: string, tz: string): boolean {
  return busy.some((row) => row.seriesId && zonedYmd(row.startsAt, tz) === ymd)
}

function endMinutes(iso: string, startMin: number, tz: string): number {
  const value = zonedMinutes(iso, tz)
  if (value === 0 || value < startMin) return 24 * 60
  return value
}

function weekKicker(days: string[], tz: string): string {
  const fmt = (ymd: string) =>
    new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', timeZone: tz }).format(new Date(`${ymd}T12:00:00Z`))
  return `${fmt(days[0])} - ${fmt(days[6])}`
}

function dayTone(ymd: string, today: string, override?: DayOverride): string {
  const head = dayHead(ymd, today)
  return ['sched-day', head.today ? 'today' : '', !head.today && head.weekend ? 'weekend' : '', override?.off ? 'off' : '']
    .filter(Boolean)
    .join(' ')
}

function weekColumns(days: string[], today: string, collapseWeekend: boolean): string {
  return days
    .map((ymd) => {
      const head = dayHead(ymd, today)
      if (head.today) return 'minmax(12rem, 2fr)'
      if (head.weekend && collapseWeekend) return 'minmax(3.2rem, 0.42fr)'
      return 'minmax(4.4rem, 1fr)'
    })
    .join(' ')
}

function weekendHasTasks(lanes: ScheduleLane[], days: string[], tz: string): boolean {
  const weekend = new Set(days.filter((ymd) => dayHead(ymd, '').weekend))
  return lanes.some((lane) => lane.blocks.some((row) => weekend.has(zonedYmd(row.startsAt, tz))))
}

function firstLateId(lanes: ScheduleLane[]): string | null {
  const late = lanes.flatMap((lane) => lane.blocks.filter((row) => row.late))
  if (late.length === 0) return null
  late.sort((a, b) => a.startsAt.localeCompare(b.startsAt))
  return late[0].itemId
}

function meetingCount(busy: ScheduleBusy[]): number {
  return busy.filter((row) => row.seriesId).length
}

function HoursRail({ grid }: { grid: Grid }) {
  return (
    <ol className="sched-hours" aria-hidden style={{ height: grid.height }}>
      {Array.from({ length: grid.hours }, (_, i) => (
        <li key={i}>{String(grid.start + i).padStart(2, '0')}</li>
      ))}
    </ol>
  )
}

function dueLabel(iso: string, tz: string): string {
  return new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', timeZone: tz }).format(new Date(iso))
}

function slackLabel(seconds: number): string {
  return `−${span(Math.abs(seconds))}`
}

function isTracked(row: ScheduleBlock): boolean {
  return row.occupancy === 'parallel' || row.occupancy === 'waiting'
}

function Block({
  row,
  color,
  running,
  focus,
  tracks,
  grid,
  onOpen,
}: {
  row: ScheduleBlock
  color: string
  running: boolean
  focus: boolean
  tracks: number
  grid: Grid
  onOpen: (id: string) => void
}) {
  const start = zonedMinutes(row.startsAt, grid.tz)
  const end = endMinutes(row.endsAt, start, grid.tz)
  const top = ((start - grid.start * 60) / 60) * HOUR
  const ping = row.kind === 'ping'
  const height = ping ? Math.max(((end - start) / 60) * HOUR, 22) : Math.max(((end - start) / 60) * HOUR, 34)
  const short = ping || height < HOUR
  const split = isTracked(row) && tracks > 1
  const reasons = (row.reasons ?? []).map(scheduleReasonLabel)
  const cls = [
    'sched-block',
    ping ? 'ping' : '',
    row.late ? 'late' : '',
    row.continued ? 'continued' : '',
    row.continues ? 'continues' : '',
    running ? 'running' : '',
    focus ? 'focus' : '',
    short ? 'short' : '',
    split ? 'parallel' : '',
    row.occupancy === 'waiting' ? 'waiting' : '',
  ]
    .filter(Boolean)
    .join(' ')
  const label = [row.externalKey, row.title, reasons.length > 0 ? `· ${reasons.join(', ')}` : ''].filter(Boolean).join(' ')
  const fill = row.late
    ? `color-mix(in srgb, var(--danger) 22%, transparent)`
    : `color-mix(in srgb, ${color || 'var(--accent)'} 18%, transparent)`
  const border = focus
    ? `color-mix(in srgb, var(--accent-2) 80%, transparent)`
    : row.late
      ? `color-mix(in srgb, var(--danger) 28%, transparent)`
      : `color-mix(in srgb, ${color || 'var(--accent)'} 45%, transparent)`
  const style = {
    top,
    height,
    background: fill,
    borderColor: border,
    ...(split ? { '--track': row.lane, '--tracks': tracks } : {}),
  } as CSSProperties
  return (
    <button type="button" className={cls} title={label} style={style} onClick={() => onOpen(row.itemId)}>
      <span className="sched-block-line">
        {ping ? <BellSimple size={11} weight="fill" aria-hidden /> : null}
        {row.externalKey ? <b className="sched-key">{row.externalKey}</b> : null}
        <strong>{row.title}</strong>
        {!ping ? <CaretRight size={12} weight="light" aria-hidden /> : null}
      </span>
      {!short ? (
        <span className="sched-block-flags">
          {row.continued ? <span className="sched-chip">continues</span> : null}
          {row.late ? <span className="sched-chip late">late</span> : null}
          {running ? <span className="sched-chip live">in time</span> : null}
          {topReasons(row.reasons).map((reason) => (
            <span key={reason} className="sched-chip why">
              {scheduleReasonLabel(reason)}
            </span>
          ))}
        </span>
      ) : null}
    </button>
  )
}

function Hatch({ row, grid, onOpen }: { row: ScheduleBusy; grid: Grid; onOpen: (id: string, originalOn?: string) => void }) {
  const start = zonedMinutes(row.startsAt, grid.tz)
  const end = endMinutes(row.endsAt, start, grid.tz)
  const top = ((start - grid.start * 60) / 60) * HOUR
  const height = Math.max(((end - start) / 60) * HOUR, 16)
  const style = { top, height }
  const body = (
    <>
      {row.seriesId ? <CalendarBlank size={12} weight="light" aria-hidden /> : null}
      <span>{row.title}</span>
    </>
  )
  if (row.seriesId) {
    return (
      <button
        type="button"
        className={row.soft ? 'sched-busy meeting soft' : 'sched-busy meeting'}
        style={style}
        title={row.soft ? `${row.title} (skippable)` : row.title}
        onClick={() => onOpen(row.seriesId!, row.originalOn)}
      >
        {body}
      </button>
    )
  }
  return (
    <i className="sched-busy" style={style}>
      {body}
    </i>
  )
}

function NowLine({ today, ymd, grid }: { today: string; ymd: string; grid: Grid }) {
  if (ymd !== today) return null
  const nowIso = new Date().toISOString()
  const raw = ((zonedMinutes(nowIso, grid.tz) - grid.start * 60) / 60) * HOUR
  const top = Math.min(grid.height, Math.max(0, raw))
  const edge = raw < 0 || raw > grid.height
  const below = raw < 14
  return (
    <i className={below ? 'sched-now below' : 'sched-now'} style={{ top }}>
      <span>
        {clock(nowIso, grid.tz)}
        {edge ? ` - grid ${String(grid.start).padStart(2, '0')}-${String(grid.start + grid.hours).padStart(2, '0')}` : ''}
      </span>
    </i>
  )
}

// layoutDay returns the day's blocks and how many tracks the day uses.
// Solo blocks span the column; parallel and waiting blocks split it by track.
function layoutDay(blocks: ScheduleBlock[], ymd: string, tz: string): { rows: ScheduleBlock[]; tracks: number } {
  const rows = blocks.filter((row) => zonedYmd(row.startsAt, tz) === ymd)
  const tracks = rows.reduce((max, row) => (isTracked(row) ? Math.max(max, row.lane + 1) : max), 0)
  return { rows, tracks }
}

function DayNote({ override }: { override?: DayOverride }) {
  if (!override) return null
  if (override.off) return <p className="sched-dayoff">{override.note || 'Day off'}</p>
  return null
}

function Lane({
  lane,
  days,
  busy,
  today,
  grid,
  overrides,
  runningIds,
  focusIds,
  onOpen,
  onBusy,
}: {
  lane: ScheduleLane
  days: string[]
  busy: ScheduleBusy[]
  today: string
  grid: Grid
  overrides: Map<string, DayOverride>
  runningIds: Set<string>
  focusIds: Set<string>
  onOpen: (id: string) => void
  onBusy: (id: string, originalOn?: string) => void
}) {
  const color = lane.projectColor || 'var(--accent)'
  return (
    <div className="sched-lane">
      <div className="sched-lane-name">
        <i className="sched-swatch" style={{ background: color }} />
        <strong>{lane.projectName}</strong>
      </div>
      {days.map((ymd) => {
        const { rows, tracks } = layoutDay(lane.blocks, ymd, grid.tz)
        const override = overrides.get(ymd)
        const empty = rows.length === 0 && !override?.off && !dayHead(ymd, today).weekend && !hasMeeting(busy, ymd, grid.tz)
        return (
          <div key={ymd} className={dayTone(ymd, today, override)} style={{ height: grid.height }}>
            {ymd === today ? <HoursRail grid={grid} /> : null}
            {busy
              .filter((row) => zonedYmd(row.startsAt, grid.tz) === ymd)
              .map((row) => (
                <Hatch key={`${row.seriesId ?? 'busy'}-${row.originalOn}-${row.startsAt}`} row={row} grid={grid} onOpen={onBusy} />
              ))}
            <NowLine today={today} ymd={ymd} grid={grid} />
            {rows.map((row) => (
              <Block
                key={`${row.itemId}-${row.startsAt}`}
                row={row}
                color={color}
                running={runningIds.has(row.itemId)}
                focus={focusIds.has(`${row.itemId}-${row.startsAt}`)}
                tracks={tracks}
                grid={grid}
                onOpen={onOpen}
              />
            ))}
            <DayNote override={override} />
            {empty ? <p className="sched-noslot">No slots</p> : null}
          </div>
        )
      })}
    </div>
  )
}

function Pressure({
  overflow,
  capacity,
  atRisk,
  score,
  meetings,
  tz,
  onOpen,
}: {
  overflow: ScheduleOverflow
  capacity: ScheduleCapacity
  atRisk: AtRiskItem[]
  score: ScheduleScore
  meetings: number
  tz: string
  onOpen: (id: string) => void
}) {
  const packed = capacity.packedSeconds
  const free = capacity.freeSeconds
  const over = overflow.seconds
  const total = packed + free + over
  const packedPct = total > 0 ? Math.min(100, (packed / total) * 100) : 0
  const overPct = total > 0 ? Math.min(100 - packedPct, (over / total) * 100) : 0
  return (
    <div className="sched-pressure">
      <div className="sched-cap" aria-hidden>
        <i className="packed" style={{ width: `${packedPct}%` }} />
        {overPct > 0 ? <i className="over" style={{ width: `${overPct}%` }} /> : null}
      </div>
      <p className="sched-cap-meta">
        {overflow.itemCount > 0 && overflow.firstDueAt
          ? `Will not fit: ${span(overflow.seconds)} past ${dueLabel(overflow.firstDueAt, tz)}`
          : overflow.itemCount > 0
            ? `Will not fit: ${span(overflow.seconds)}`
            : `This week: ${meetings} meetings, ${span(free)} free for tasks`}
        <span className="sched-score mono" title="Late seconds, fragments, switches and load variance folded into one number; lower is better">
          {' · '}score {score.total.toFixed(1)} · {score.fragments} fragments · {score.switches} switches
        </span>
      </p>
      {atRisk.length > 0 ? (
        <p className="sched-atrisk">
          <span>At risk:</span>
          {atRisk.map((row) => (
            <button key={row.itemId} type="button" className="ghost sched-atrisk-item" onClick={() => onOpen(row.itemId)}>
              <b className="sched-key">{row.key || row.title}</b>
              <span className="mono">{slackLabel(row.slackSeconds)}</span>
            </button>
          ))}
        </p>
      ) : null}
      <ul className="sched-legend">
        <li className="late">Late</li>
        <li className="live">In time</li>
        <li className="parallel">Parallel</li>
        <li className="waiting">Waiting</li>
        <li className="ping">Ping</li>
      </ul>
    </div>
  )
}

function DayCells({
  days,
  today,
  busy,
  grid,
  overrides,
  onBusy,
}: {
  days: string[]
  today: string
  busy: ScheduleBusy[]
  grid: Grid
  overrides: Map<string, DayOverride>
  onBusy: (id: string, originalOn?: string) => void
}) {
  return (
    <>
      {days.map((ymd) => {
        const override = overrides.get(ymd)
        return (
          <div key={ymd} className={dayTone(ymd, today, override)} style={{ height: grid.height }}>
            {ymd === today ? <HoursRail grid={grid} /> : null}
            {busy
              .filter((row) => zonedYmd(row.startsAt, grid.tz) === ymd)
              .map((row) => (
                <Hatch key={`${row.seriesId ?? 'busy'}-${row.originalOn}-${row.startsAt}`} row={row} grid={grid} onOpen={onBusy} />
              ))}
            <NowLine today={today} ymd={ymd} grid={grid} />
            <DayNote override={override} />
            {!override?.off && !dayHead(ymd, today).weekend && !hasMeeting(busy, ymd, grid.tz) ? <p className="sched-noslot">No slots</p> : null}
          </div>
        )
      })}
    </>
  )
}

function DayHeads({
  days,
  today,
  overrides,
  open,
  onToggle,
}: {
  days: string[]
  today: string
  overrides: Map<string, DayOverride>
  open: string | null
  onToggle: (ymd: string) => void
}) {
  return (
    <div className="sched-head">
      {days.map((ymd) => {
        const head = dayHead(ymd, today)
        const override = overrides.get(ymd)
        const custom = override && !override.off && override.workStartMin != null && override.workEndMin != null
        return (
          <button
            key={ymd}
            type="button"
            className={[
              'sched-head-day',
              head.today ? 'today' : '',
              !head.today && head.weekend ? 'weekend' : '',
              override ? 'overridden' : '',
              open === ymd ? 'open' : '',
            ]
              .filter(Boolean)
              .join(' ')}
            title={override?.note || 'Day off, custom hours or a note'}
            onClick={() => onToggle(ymd)}
          >
            <small>{head.week}</small>
            <strong>{head.num}</strong>
            {head.today ? <em>Today</em> : null}
            {override?.off ? <em className="off">Off</em> : null}
            {custom ? (
              <em className="hours">
                {minutesToClock(override.workStartMin!)}–{minutesToClock(override.workEndMin!)}
              </em>
            ) : null}
          </button>
        )
      })}
    </div>
  )
}

export function ScheduleScreen() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const gantt = useRef<HTMLDivElement>(null)
  const [anchor, setAnchor] = useState(() => new Date())
  const [kind, setKind] = useState<'work' | 'followup'>('work')
  const [view, setView] = useState<View>(readView)
  const [ganttDay, setGanttDay] = useState<string | null>(null)
  const [whatIf, setWhatIf] = useState(false)
  const [openDay, setOpenDay] = useState<string | null>(null)
  const [tick, setTick] = useState(() => Date.now())
  useEffect(() => {
    const id = window.setInterval(() => setTick(Date.now()), 60_000)
    return () => window.clearInterval(id)
  }, [])
  const week = useMemo(() => moscowWeek(anchor), [anchor])
  const intervals = useQuery({
    queryKey: ['intervals'],
    queryFn: api.intervals,
    refetchInterval: 10_000,
  })
  const running = intervals.data ?? []
  const report = useQuery({
    queryKey: ['schedule', kind, week.from, week.to],
    queryFn: () => api.schedule(week.from, week.to, kind),
    refetchInterval: running.length > 0 ? 10_000 : false,
  })
  const dayOverrides = useQuery({
    queryKey: ['schedule-days', week.days[0], week.days[6]],
    queryFn: () => api.dayOverrides(week.days[0], week.days[6]),
  })
  const upsertDay = useMutation({
    mutationFn: ({ day, draft }: { day: string; draft: DayOverrideDraft }) => api.putDayOverride(day, draft),
    onSuccess: () => {
      setOpenDay(null)
      void queryClient.invalidateQueries({ queryKey: ['schedule'] })
      void queryClient.invalidateQueries({ queryKey: ['schedule-days'] })
    },
  })
  const clearDay = useMutation({
    mutationFn: (day: string) => api.deleteDayOverride(day),
    onSuccess: () => {
      setOpenDay(null)
      void queryClient.invalidateQueries({ queryKey: ['schedule'] })
      void queryClient.invalidateQueries({ queryKey: ['schedule-days'] })
    },
  })
  const grid = gridOf(report.data?.grid)
  const today = zonedYmd(new Date(tick), grid.tz)
  const chartDay = pickChartDay(week.days, today, ganttDay, report.data?.grid.workdays ?? FALLBACK_GRID.workdays)
  const lanes = report.data?.lanes ?? []
  const unplanned = report.data?.unplanned ?? []
  const busy = report.data?.busy ?? []
  const overrides = useMemo(() => new Map((dayOverrides.data ?? []).map((row) => [row.day, row])), [dayOverrides.data])
  const runningIds = new Set(running.map((row) => row.itemId))
  const collapseWeekend = !weekendHasTasks(lanes, week.days, grid.tz)
  const cols = weekColumns(week.days, today, collapseWeekend)
  const gridStyle = { '--sched-cols': cols } as CSSProperties
  const lateId = firstLateId(lanes)
  const range = weekKicker(week.days, grid.tz)
  const todays = todayBlocks(lanes, today, grid.tz)
  const pick = pickNow(todays, tick)
  const focusIds = new Set(covering(todays, tick).map((row) => `${row.itemId}-${row.startsAt}`))
  const projectByItem = new Map<string, DeckProject>()
  const titleByItem = new Map<string, string>()
  for (const lane of lanes) {
    for (const row of lane.blocks) {
      projectByItem.set(row.itemId, { name: lane.projectName, color: lane.projectColor || 'var(--accent)' })
      titleByItem.set(row.itemId, row.externalKey ? `${row.externalKey} ${row.title}` : row.title)
    }
  }

  function switchView(next: View) {
    setView(next)
    storeView(next)
  }

  function scrollNow() {
    const target = view === 'list' ? '.sched-table-day.today' : view === 'gantt' ? '.sched-chart-now' : '.sched-now'
    gantt.current?.querySelector(target)?.scrollIntoView({ block: 'center', inline: 'nearest', behavior: 'smooth' })
  }

  const openHead = openDay ? dayHead(openDay, today) : null

  return (
    <Window
      className="sched-window"
      title={kind === 'followup' ? 'Follow-up' : 'Schedule'}
      actions={
        <div className="sched-actions">
          <div className="sched-cluster">
            <div className="sched-tabs">
              <button type="button" className={kind === 'work' ? 'ghost on' : 'ghost'} onClick={() => setKind('work')}>
                Work
              </button>
              <button
                type="button"
                className={kind === 'followup' ? 'ghost on' : 'ghost'}
                onClick={() => setKind('followup')}
              >
                Follow-up
              </button>
            </div>
            <p className="sched-pack-hint">{kind === 'work' ? 'Dev due' : 'Review and QA due'}</p>
          </div>
          <div className="sched-tabs">
            <button type="button" className={view === 'calendar' ? 'ghost on' : 'ghost'} onClick={() => switchView('calendar')}>
              Calendar
            </button>
            <button type="button" className={view === 'list' ? 'ghost on' : 'ghost'} onClick={() => switchView('list')}>
              List
            </button>
            <button type="button" className={view === 'gantt' ? 'ghost on' : 'ghost'} onClick={() => switchView('gantt')}>
              Gantt
            </button>
          </div>
          <div className="sched-tabs">
            <button type="button" className="ghost" aria-label="Previous week" onClick={() => setAnchor((d) => shiftWeeks(d, -1))}>
              ‹
            </button>
            <button type="button" className="ghost" aria-label="This week" onClick={() => setAnchor(new Date())}>
              This week
            </button>
            <button type="button" className="ghost" aria-label="Next week" onClick={() => setAnchor((d) => shiftWeeks(d, 1))}>
              ›
            </button>
          </div>
          <div className="sched-tabs">
            <button type="button" className="ghost" aria-label="Now" onClick={scrollNow}>
              Now
            </button>
            <button type="button" className={whatIf ? 'ghost on' : 'ghost'} onClick={() => setWhatIf((v) => !v)}>
              What if
            </button>
            <button type="button" className="ghost" aria-label="Schedule settings" onClick={() => navigate('/schedule/settings')}>
              <GearSix size={14} weight="regular" aria-hidden />
              Settings
            </button>
          </div>
        </div>
      }
    >
      {view === 'list' ? (
        <NowDeck
          pick={pick}
          loading={report.isLoading && !report.data}
          running={running}
          lateCount={report.data?.overflow.itemCount ?? 0}
          lateId={lateId}
          tz={grid.tz}
          projectOf={(id) => projectByItem.get(id)}
          titleOf={(id) => titleByItem.get(id)}
          onOpen={(id) => openTask(navigate, id)}
          onNextWeek={() => setAnchor((d) => shiftWeeks(d, 1))}
        />
      ) : null}
      {whatIf ? (
        // A new kind or week starts a fresh scenario: choices and results from the old one no longer apply.
        <WhatIfPanel key={`${kind}:${week.from}`} kind={kind} from={week.from} to={week.to} lanes={lanes} busy={busy} onClose={() => setWhatIf(false)} />
      ) : null}
      <div className={unplanned.length > 0 ? 'sched-split' : 'sched-split solo'}>
        <div className="sched-gantt">
          <div className={collapseWeekend ? 'sched-gantt-core compact' : 'sched-gantt-core'} ref={gantt} style={gridStyle}>
            {report.isError ? <p className="error">{report.error.message}</p> : null}
            {report.isLoading && !report.data ? (
              <div className="sched-skel" aria-hidden>
                <p className="sched-week-range">{range}</p>
                <div className="sched-head">
                  {week.days.map((ymd) => (
                    <div key={ymd} className="sched-head-day" />
                  ))}
                </div>
                <div className="sched-lane">
                  <div className="sched-lane-name" />
                  {week.days.map((ymd) => (
                    <div key={ymd} className="sched-day" style={{ height: grid.height }} />
                  ))}
                </div>
              </div>
            ) : report.data || !report.isError ? (
              <>
                {report.data ? (
                  <Pressure
                    overflow={report.data.overflow}
                    capacity={report.data.capacity}
                    atRisk={report.data.atRisk ?? []}
                    score={report.data.score}
                    meetings={meetingCount(busy)}
                    tz={grid.tz}
                    onOpen={(id) => openTask(navigate, id)}
                  />
                ) : null}
                <p className="sched-week-range">
                  {range}
                  <span className="sched-week-tz"> · {grid.tz}</span>
                </p>
                {view === 'list' ? (
                  <ScheduleTable
                    lanes={lanes}
                    busy={busy}
                    days={week.days}
                    today={today}
                    tz={grid.tz}
                    overrides={overrides}
                    runningIds={runningIds}
                    focusIds={focusIds}
                    projectOf={(id) => projectByItem.get(id)}
                    onOpen={(id) => openTask(navigate, id)}
                    onBusy={(id, on) => openEvent(navigate, id, on)}
                  />
                ) : view === 'gantt' ? (
                  <ScheduleGantt
                    lanes={lanes}
                    busy={busy}
                    days={week.days}
                    day={chartDay}
                    today={today}
                    grid={grid}
                    runningIds={runningIds}
                    focusIds={focusIds}
                    now={tick}
                    onDay={setGanttDay}
                    onOpen={(id) => openTask(navigate, id)}
                    onBusy={(id, on) => openEvent(navigate, id, on)}
                  />
                ) : (
                  <>
                    <div className="sched-head-wrap">
                      <DayHeads
                        days={week.days}
                        today={today}
                        overrides={overrides}
                        open={openDay}
                        onToggle={(ymd) => setOpenDay((current) => (current === ymd ? null : ymd))}
                      />
                      {openDay && openHead ? (
                        <DayOverridePopover
                          key={openDay}
                          day={openDay}
                          label={`${openHead.week} ${openHead.num}`}
                          override={overrides.get(openDay)}
                          defaultStartMin={grid.workStartMin}
                          defaultEndMin={grid.workEndMin}
                          busy={upsertDay.isPending || clearDay.isPending}
                          error={upsertDay.error?.message ?? clearDay.error?.message}
                          onSave={(draft) => upsertDay.mutate({ day: openDay, draft })}
                          onClear={() => clearDay.mutate(openDay)}
                          onClose={() => setOpenDay(null)}
                        />
                      ) : null}
                    </div>
                    {lanes.length === 0 ? (
                      <>
                        <div className="sched-lane sched-lane-empty">
                          <div className="sched-lane-name" />
                          <DayCells
                            days={week.days}
                            today={today}
                            busy={busy}
                            grid={grid}
                            overrides={overrides}
                            onBusy={(id, on) => openEvent(navigate, id, on)}
                          />
                        </div>
                        <p className="sched-empty muted">No scheduled work this week.</p>
                      </>
                    ) : (
                      lanes.map((lane) => (
                        <Lane
                          key={lane.projectId ?? 'none'}
                          lane={lane}
                          days={week.days}
                          busy={busy}
                          today={today}
                          grid={grid}
                          overrides={overrides}
                          runningIds={runningIds}
                          focusIds={focusIds}
                          onOpen={(id) => openTask(navigate, id)}
                          onBusy={(id, on) => openEvent(navigate, id, on)}
                        />
                      ))
                    )}
                    <p className="sched-readonly">Packed layout, not a drag calendar</p>
                  </>
                )}
              </>
            ) : null}
          </div>
        </div>
        <NeedsFields rows={unplanned} kind={kind} />
      </div>
    </Window>
  )
}
