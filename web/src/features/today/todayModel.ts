import { dueStageOpen, type DueSlot } from '../../shared/format'
import { isOverdue, nearestDue } from '../tasks/sortColumn'
import { zonedYmd, type NowPick } from '../schedule/now'
import type { AtRiskItem, Item, ScheduleBlock, ScheduleBusy } from '../../types'

export const DEFAULT_USD_RUB = 85
export const EARN_WEEKDAYS = 20.5

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
  usd: number
  rub: number
  rate: number
  daily: number
}

export type EarnMonthPoint = {
  month: string
  label: string
  total: number
}

export type EarnSnap = {
  day: string
  usd: number
  rub: number
  rate?: number
}

const MONTH_LABELS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

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

export function dailyEarn(projects: EarnProject[]): EarnResult {
  const income = activeIncome(projects)
  return { usd: income.usd / EARN_WEEKDAYS, rub: income.rub / EARN_WEEKDAYS }
}

export function todayEarn(projects: EarnProject[], ymd: string): EarnResult {
  if (!isMoscowWeekday(ymd)) return { usd: 0, rub: 0 }
  return dailyEarn(projects)
}

export function monthEarn(projects: EarnProject[]): EarnResult {
  return activeIncome(projects)
}

export function monthYmdsThrough(ymd: string): string[] {
  const [year, month] = ymd.split('-').map(Number)
  const last = Number(ymd.slice(8))
  return Array.from({ length: last }, (_, i) => {
    const day = i + 1
    return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`
  })
}

export function snapRate(row: EarnSnap | undefined, fallback: number): number {
  if (row?.rate != null && row.rate > 0 && Number.isFinite(row.rate)) return row.rate
  return fallback
}

export function meanRate(rates: number[], fallback = DEFAULT_USD_RUB): number {
  const ok = rates.filter((rate) => rate > 0 && Number.isFinite(rate))
  if (ok.length === 0) return fallback
  return ok.reduce((sum, rate) => sum + rate, 0) / ok.length
}

export function monthAvgRate(ymd: string, snapshots: EarnSnap[], fallback: number): number {
  const key = ymd.slice(0, 7)
  return meanRate(
    snapshots.filter((row) => row.day.startsWith(`${key}-`)).map((row) => snapRate(row, 0)),
    fallback,
  )
}

function monthSums(rows: { usd: number; rub: number }[]): EarnResult {
  let usd = 0
  let rub = 0
  for (const row of rows) {
    usd += row.usd
    rub += row.rub
  }
  return { usd, rub }
}

export function earnChartDays(
  ymd: string,
  snapshots: EarnSnap[],
  live: EarnResult,
  fallbackRate: number,
): EarnDayPoint[] {
  const byDay = new Map(snapshots.map((row) => [row.day, row]))
  return monthYmdsThrough(ymd).map((day) => {
    const snap = byDay.get(day)
    if (snap) {
      const rate = snapRate(snap, fallbackRate)
      return { day, usd: snap.usd, rub: snap.rub, rate, daily: earnTotals(snap.usd, snap.rub, rate).usd }
    }
    const raw = isMoscowWeekday(day) ? live : { usd: 0, rub: 0 }
    return { day, usd: raw.usd, rub: raw.rub, rate: fallbackRate, daily: earnTotals(raw.usd, raw.rub, fallbackRate).usd }
  })
}

export function earnChartMonths(
  ymd: string,
  snapshots: EarnSnap[],
  live: EarnResult,
  fallbackRate: number,
): EarnMonthPoint[] {
  const year = ymd.slice(0, 4)
  const current = Number(ymd.slice(5, 7))
  const byMonth = new Map<string, EarnSnap[]>()
  for (const row of snapshots) {
    const key = row.day.slice(0, 7)
    if (!key.startsWith(`${year}-`)) continue
    const month = Number(key.slice(5, 7))
    if (month < 1 || month >= current) continue
    const list = byMonth.get(key) ?? []
    list.push(row)
    byMonth.set(key, list)
  }
  const currentDays = earnChartDays(ymd, snapshots, live, fallbackRate)
  const currentRaw = monthSums(currentDays)
  const currentRate = monthAvgRate(ymd, snapshots, fallbackRate)
  return Array.from({ length: current }, (_, i) => {
    const month = i + 1
    const key = `${year}-${String(month).padStart(2, '0')}`
    if (month === current) {
      return {
        month: key,
        label: MONTH_LABELS[i],
        total: earnTotals(currentRaw.usd, currentRaw.rub, currentRate).usd,
      }
    }
    const rows = byMonth.get(key) ?? []
    const raw = monthSums(rows)
    const avg = meanRate(
      rows.map((row) => snapRate(row, 0)),
      fallbackRate,
    )
    return { month: key, label: MONTH_LABELS[i], total: earnTotals(raw.usd, raw.rub, avg).usd }
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
