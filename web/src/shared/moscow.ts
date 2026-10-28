const MOSCOW = 'Europe/Moscow'
const TASHKENT = 'Asia/Tashkent'

export type LoadPeriod = 'day' | 'week' | 'month' | 'year'

function ymdInZone(now: Date, timeZone: string): string {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(now)
}

function moscowMidnight(ymd: string): Date {
  return new Date(`${ymd}T00:00:00+03:00`)
}

function shiftYmd(ymd: string, days: number): string {
  const [year, month, day] = ymd.split('-').map(Number)
  const next = new Date(Date.UTC(year, month - 1, day + days))
  return next.toISOString().slice(0, 10)
}

export function formatClock(now: Date, timeZone: string): string {
  return new Intl.DateTimeFormat('en-GB', {
    timeZone,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(now)
}

export function moscowClocks(now = new Date()) {
  return {
    moscow: formatClock(now, MOSCOW),
    tashkent: formatClock(now, TASHKENT),
  }
}

export function moscowYmd(now = new Date()): string {
  return ymdInZone(now, MOSCOW)
}

export function moscowDayRange(now = new Date()): { from: string; to: string; ymd: string } {
  const ymd = moscowYmd(now)
  return {
    from: moscowMidnight(ymd).toISOString(),
    to: moscowMidnight(shiftYmd(ymd, 1)).toISOString(),
    ymd,
  }
}

export function moscowWeek(anchor = new Date()): { from: string; to: string; days: string[] } {
  const today = moscowYmd(anchor)
  const [year, month, day] = today.split('-').map(Number)
  const weekday = new Date(Date.UTC(year, month - 1, day, 12)).getUTCDay()
  const monday = shiftYmd(today, -((weekday + 6) % 7))
  const days = Array.from({ length: 7 }, (_, i) => shiftYmd(monday, i))
  return {
    from: moscowMidnight(days[0]).toISOString(),
    to: moscowMidnight(shiftYmd(days[6], 1)).toISOString(),
    days,
  }
}

export function moscowRange(period: LoadPeriod, now = new Date()): { from: string; to: string } {
  const today = ymdInZone(now, MOSCOW)
  const startToday = moscowMidnight(today)
  const [year, month, day] = today.split('-').map(Number)
  let from = startToday
  if (period === 'week') {
    const weekday = new Date(Date.UTC(year, month - 1, day, 12)).getUTCDay()
    const mondayOffset = (weekday + 6) % 7
    from = moscowMidnight(shiftYmd(today, -mondayOffset))
  }
  if (period === 'month') {
    from = moscowMidnight(`${year}-${String(month).padStart(2, '0')}-01`)
  }
  if (period === 'year') {
    from = moscowMidnight(`${year}-01-01`)
  }
  return { from: from.toISOString(), to: now.toISOString() }
}

export function moscowMinutes(iso: string): number {
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: MOSCOW,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(new Date(iso))
  const hour = Number(parts.find((part) => part.type === 'hour')?.value ?? '0')
  const minute = Number(parts.find((part) => part.type === 'minute')?.value ?? '0')
  return hour * 60 + minute
}

export function moscowMonth(anchor = new Date()) {
  const today = moscowYmd(anchor)
  const [year, month] = today.split('-').map(Number)
  const first = `${year}-${String(month).padStart(2, '0')}-01`
  const weekday = new Date(Date.UTC(year, month - 1, 1, 12)).getUTCDay()
  const monday = shiftYmd(first, -((weekday + 6) % 7))
  const weeks = Array.from({ length: 6 }, (_, week) =>
    Array.from({ length: 7 }, (_, day) => shiftYmd(monday, week * 7 + day)),
  )
  const last = weeks[5][6]
  return {
    year,
    month,
    weeks,
    from: moscowMidnight(monday).toISOString(),
    to: moscowMidnight(shiftYmd(last, 1)).toISOString(),
    label: new Intl.DateTimeFormat('en-GB', { month: 'long', year: 'numeric', timeZone: MOSCOW }).format(
      moscowMidnight(first),
    ),
  }
}

export function shiftWeeks(anchor: Date, weeks: number): Date {
  return new Date(anchor.getTime() + weeks * 7 * 24 * 60 * 60 * 1000)
}

export function shiftMonths(anchor: Date, months: number): Date {
  const ymd = moscowYmd(anchor)
  const [year, month, day] = ymd.split('-').map(Number)
  return new Date(Date.UTC(year, month - 1 + months, Math.min(day, 28), 12))
}
