import { dueStageOpen, type DueSlot } from '../../shared/format'
import { isOverdue, nearestDue } from '../tasks/sortColumn'
import { zonedYmd, type NowPick } from '../schedule/now'
import type { AtRiskItem, Item, ScheduleBlock, ScheduleBusy } from '../../types'

export const DEFAULT_USD_RUB = 85

const DUE_KEYS = ['dueAt', 'devDueAt', 'reviewDueAt', 'testDueAt'] as const satisfies readonly DueSlot[]

export const RISK_CAP = 8

export type AgendaEntry = {
  startsAt: string
  endsAt: string
  kind: 'work' | 'meeting'
  title: string
  late?: boolean
  externalKey?: string
  itemId?: string
  seriesId?: string
  originalOn?: string
}

export type DueItem = Pick<
  Item,
  'id' | 'title' | 'externalKey' | 'status' | 'dueAt' | 'devDueAt' | 'reviewDueAt' | 'testDueAt'
>

export type RiskRow = {
  itemId: string
  title: string
  key: string
  overdue: boolean
  late: boolean
  dueMs: number | null
}

export function featuredBlock(pick: NowPick): ScheduleBlock | undefined {
  return pick.current[0] ?? pick.upcoming[0]
}

function blockKey(block: ScheduleBlock): string {
  return `${block.itemId}|${block.startsAt}`
}

export function remainingAgenda(
  pick: NowPick,
  blocks: ScheduleBlock[],
  busy: ScheduleBusy[],
  now: number,
): AgendaEntry[] {
  const featured = featuredBlock(pick)
  const skip = featured ? blockKey(featured) : ''
  const work: AgendaEntry[] = blocks
    .filter((row) => new Date(row.endsAt).getTime() > now && blockKey(row) !== skip)
    .map((row) => ({
      startsAt: row.startsAt,
      endsAt: row.endsAt,
      kind: 'work' as const,
      title: row.title,
      late: row.late,
      externalKey: row.externalKey || undefined,
      itemId: row.itemId,
    }))
  const meetings: AgendaEntry[] = busy
    .filter((row) => new Date(row.endsAt).getTime() > now)
    .map((row) => ({
      startsAt: row.startsAt,
      endsAt: row.endsAt,
      kind: 'meeting' as const,
      title: row.title,
      seriesId: row.seriesId,
      originalOn: row.originalOn,
    }))
  return [...work, ...meetings].sort((a, b) => a.startsAt.localeCompare(b.startsAt) || a.endsAt.localeCompare(b.endsAt))
}

function dueOnDay(item: DueItem, ymd: string, tz: string): boolean {
  return DUE_KEYS.some((key) => {
    const iso = item[key]
    return iso != null && dueStageOpen(item.status, key) && zonedYmd(iso, tz) === ymd
  })
}

function upsertRisk(byId: Map<string, RiskRow>, next: Partial<RiskRow> & { itemId: string }) {
  const prev = byId.get(next.itemId)
  byId.set(next.itemId, {
    itemId: next.itemId,
    title: next.title || prev?.title || '',
    key: next.key || prev?.key || '',
    overdue: Boolean(next.overdue || prev?.overdue),
    late: Boolean(next.late || prev?.late),
    dueMs: next.dueMs ?? prev?.dueMs ?? null,
  })
}

export function atRiskToday(
  blocks: ScheduleBlock[],
  atRisk: AtRiskItem[],
  items: DueItem[],
  ymd: string,
  tz: string,
  now: number,
  cap = RISK_CAP,
): RiskRow[] {
  const byId = new Map<string, RiskRow>()
  for (const block of blocks) {
    if (!block.late) continue
    upsertRisk(byId, {
      itemId: block.itemId,
      title: block.title,
      key: block.externalKey,
      late: true,
      dueMs: Date.parse(block.dueAt),
    })
  }
  for (const row of atRisk) {
    upsertRisk(byId, { itemId: row.itemId, title: row.title, key: row.key })
  }
  for (const item of items) {
    const overdue = isOverdue(item, now)
    if (!overdue && !dueOnDay(item, ymd, tz)) continue
    upsertRisk(byId, {
      itemId: item.id,
      title: item.title,
      key: item.externalKey,
      overdue,
      dueMs: nearestDue(item),
    })
  }
  return [...byId.values()]
    .sort((a, b) => {
      const overdue = Number(b.overdue) - Number(a.overdue)
      if (overdue) return overdue
      const late = Number(b.late) - Number(a.late)
      if (late) return late
      const aDue = a.dueMs ?? Number.POSITIVE_INFINITY
      const bDue = b.dueMs ?? Number.POSITIVE_INFINITY
      if (aDue !== bDue) return aDue - bDue
      return a.title.localeCompare(b.title)
    })
    .slice(0, cap)
}

export type EarnProject = {
  id: string
  monthlyIncomeUsd: number
  monthlyIncomeRub: number
  archivedAt: string | null
}

export type EarnResult = {
  usd: number
  rub: number
}

export type EarnDayPoint = {
  day: string
  daily: number
  cumulative: number
}

export function daysInYmdMonth(ymd: string): number {
  const [year, month] = ymd.split('-').map(Number)
  return new Date(Date.UTC(year, month, 0)).getUTCDate()
}

export function isMoscowWeekday(ymd: string): boolean {
  const weekday = new Date(`${ymd}T12:00:00+03:00`).getUTCDay()
  return weekday !== 0 && weekday !== 6
}

export function weekdaysInMonth(ymd: string): number {
  const [year, month] = ymd.split('-').map(Number)
  const days = daysInYmdMonth(ymd)
  let count = 0
  for (let day = 1; day <= days; day++) {
    const stamp = `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`
    if (isMoscowWeekday(stamp)) count += 1
  }
  return count
}

function activeIncome(projects: EarnProject[]): EarnResult {
  let usd = 0
  let rub = 0
  for (const project of projects) {
    if (project.archivedAt) continue
    usd += project.monthlyIncomeUsd
    rub += project.monthlyIncomeRub
  }
  return { usd, rub }
}

export function dailyEarn(projects: EarnProject[], ymd: string): EarnResult {
  const weekdays = weekdaysInMonth(ymd)
  if (weekdays <= 0) return { usd: 0, rub: 0 }
  const income = activeIncome(projects)
  return { usd: income.usd / weekdays, rub: income.rub / weekdays }
}

export function todayEarn(projects: EarnProject[], ymd: string): EarnResult {
  if (!isMoscowWeekday(ymd)) return { usd: 0, rub: 0 }
  return dailyEarn(projects, ymd)
}

export function monthYmdsThrough(ymd: string): string[] {
  const [year, month] = ymd.split('-').map(Number)
  const last = Number(ymd.slice(8))
  return Array.from({ length: last }, (_, i) => {
    const day = i + 1
    return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`
  })
}

export function earnChartDays(
  ymd: string,
  snapshots: { day: string; usd: number; rub: number }[],
  live: EarnResult,
  rate: number,
): EarnDayPoint[] {
  const byDay = new Map(snapshots.map((row) => [row.day, row]))
  let cumulative = 0
  return monthYmdsThrough(ymd).map((day) => {
    const snap = byDay.get(day)
    const raw = snap ?? (isMoscowWeekday(day) ? live : { usd: 0, rub: 0 })
    const total = earnTotals(raw.usd, raw.rub, rate)
    cumulative += total.usd
    return { day, daily: total.usd, cumulative }
  })
}

export function parseUsdRubRate(raw: string, fallback = DEFAULT_USD_RUB): number {
  const value = Number(raw)
  if (!Number.isFinite(value) || value <= 0) return fallback
  return value
}

export function earnTotals(usd: number, rub: number, rate: number): EarnResult {
  if (!(rate > 0)) return { usd, rub }
  return { usd: usd + rub / rate, rub: rub + usd * rate }
}
