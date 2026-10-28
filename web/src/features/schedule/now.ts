import type { Quadrant, ScheduleBlock, ScheduleLane } from '../../types'

export const DEFAULT_TZ = 'Europe/Moscow'

export type NowPhase = 'now' | 'before' | 'gap' | 'after' | 'off'

export type NowPick = {
  current: ScheduleBlock[]
  upcoming: ScheduleBlock[]
  phase: NowPhase
}

// zonedYmd returns the calendar day of an instant in the schedule timezone.
export function zonedYmd(iso: string | Date, tz = DEFAULT_TZ): string {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: tz,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(typeof iso === 'string' ? new Date(iso) : iso)
}

// zonedMinutes returns minutes since local midnight in the schedule timezone.
export function zonedMinutes(iso: string | Date, tz = DEFAULT_TZ): number {
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: tz,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(typeof iso === 'string' ? new Date(iso) : iso)
  const hour = Number(parts.find((part) => part.type === 'hour')?.value ?? '0') % 24
  const minute = Number(parts.find((part) => part.type === 'minute')?.value ?? '0')
  return hour * 60 + minute
}

export function clock(iso: string, tz = DEFAULT_TZ): string {
  return new Intl.DateTimeFormat('en-GB', {
    timeZone: tz,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(iso))
}

export function dayHead(ymd: string, today: string, tz = DEFAULT_TZ) {
  // Noon UTC keeps the weekday stable in any zone within ±11h.
  const date = new Date(`${ymd}T12:00:00Z`)
  const weekday = date.getUTCDay()
  return {
    today: ymd === today,
    weekend: weekday === 0 || weekday === 6,
    week: new Intl.DateTimeFormat('en-GB', { weekday: 'short', timeZone: tz }).format(date),
    num: new Intl.DateTimeFormat('en-GB', { day: 'numeric', timeZone: tz }).format(date),
  }
}

export function blocksOn(lanes: ScheduleLane[], ymd: string, tz = DEFAULT_TZ): ScheduleBlock[] {
  return lanes
    .flatMap((lane) => lane.blocks)
    .filter((row) => zonedYmd(row.startsAt, tz) === ymd)
    .sort((a, b) => a.startsAt.localeCompare(b.startsAt) || a.lane - b.lane)
}

export function todayBlocks(lanes: ScheduleLane[], today: string, tz = DEFAULT_TZ): ScheduleBlock[] {
  return blocksOn(lanes, today, tz)
}

export function covering(blocks: ScheduleBlock[], now = Date.now()): ScheduleBlock[] {
  return blocks.filter((row) => new Date(row.startsAt).getTime() <= now && now < new Date(row.endsAt).getTime())
}

function firstPerItem(blocks: ScheduleBlock[]): ScheduleBlock[] {
  const seen = new Set<string>()
  return blocks.filter((row) => !seen.has(row.itemId) && seen.add(row.itemId))
}

export function pickNow(blocks: ScheduleBlock[], now = Date.now()): NowPick {
  if (blocks.length === 0) return { current: [], upcoming: [], phase: 'off' }
  const current = firstPerItem(covering(blocks, now))
  const currentIds = new Set(current.map((row) => row.itemId))
  const later = blocks.filter((row) => new Date(row.startsAt).getTime() > now)
  const upcoming = firstPerItem(later).filter((row) => !currentIds.has(row.itemId))
  if (current.length > 0) return { current, upcoming, phase: 'now' }
  if (later.length === blocks.length) return { current, upcoming, phase: 'before' }
  if (later.length > 0) return { current, upcoming, phase: 'gap' }
  return { current, upcoming, phase: 'after' }
}

export function phaseLabel(pick: NowPick, tz = DEFAULT_TZ): string {
  switch (pick.phase) {
    case 'now':
      return 'Now'
    case 'before':
      return `Up first at ${clock(pick.upcoming[0].startsAt, tz)}`
    case 'gap':
      return `Next at ${clock(pick.upcoming[0].startsAt, tz)}`
    case 'after':
      return 'Done for today'
    default:
      return 'No slots today'
  }
}

export function quadrantLabel(quadrant: Quadrant): string {
  switch (quadrant) {
    case 'do':
      return 'Do first'
    case 'schedule':
      return 'Schedule'
    case 'delegate':
      return 'Delegate'
    default:
      return 'Drop'
  }
}

// blockSeconds is the visible length of a block.
export function blockSeconds(block: ScheduleBlock): number {
  return Math.max(0, Math.round((new Date(block.endsAt).getTime() - new Date(block.startsAt).getTime()) / 1000))
}

// topReasons keeps the chips short: the first two reasons, the rest in the tooltip.
export function topReasons(reasons: string[] | undefined, limit = 2): string[] {
  return (reasons ?? []).slice(0, limit)
}

// minutesToClock renders minutes-from-midnight as HH:MM for time inputs.
export function minutesToClock(min: number): string {
  return `${String(Math.floor(min / 60)).padStart(2, '0')}:${String(min % 60).padStart(2, '0')}`
}

export function pickChartDay(days: string[], today: string, selected: string | null, workdays: number[]): string {
  if (selected && days.includes(selected)) return selected
  if (days.includes(today)) return today
  const work = days.find((ymd) => {
    const weekday = new Date(`${ymd}T12:00:00Z`).getUTCDay()
    const iso = weekday === 0 ? 7 : weekday
    return workdays.includes(iso)
  })
  return work ?? days[0] ?? today
}

// clockToMinutes parses HH:MM back into minutes-from-midnight.
export function clockToMinutes(raw: string): number | null {
  const [h, m] = raw.split(':').map(Number)
  if (!Number.isFinite(h) || !Number.isFinite(m)) return null
  return h * 60 + m
}
