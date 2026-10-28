export type LoadPeriod = 'day' | 'week' | 'month' | 'year'

const dateFormat = (timeZone: string) => new Intl.DateTimeFormat('en-CA', {
  timeZone, year: 'numeric', month: '2-digit', day: '2-digit',
})

function parts(instant: Date, timeZone: string) {
  const values = new Intl.DateTimeFormat('en-GB', {
    timeZone, year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
  }).formatToParts(instant)
  const value = (key: Intl.DateTimeFormatPartTypes) => Number(values.find((part) => part.type === key)?.value)
  return { year: value('year'), month: value('month'), day: value('day'), hour: value('hour'), minute: value('minute'), second: value('second') }
}

export function zonedYmd(instant: Date | string, timeZone: string): string {
  return dateFormat(timeZone).format(typeof instant === 'string' ? new Date(instant) : instant)
}

export function shiftYmd(ymd: string, days: number): string {
  const [year, month, day] = ymd.split('-').map(Number)
  return new Date(Date.UTC(year, month - 1, day + days)).toISOString().slice(0, 10)
}

export function zonedStartOfDay(ymd: string, timeZone: string): Date {
  const pivot = Date.parse(`${ymd}T00:00:00Z`)
  let low = pivot - 24 * 60 * 60 * 1000
  let high = pivot + 24 * 60 * 60 * 1000
  while (high - low > 1) {
    const middle = Math.floor((low + high) / 2)
    if (zonedYmd(new Date(middle), timeZone) >= ymd) high = middle
    else low = middle
  }
  return new Date(high)
}

export function zonedDayRange(now: Date, timeZone: string) {
  const ymd = zonedYmd(now, timeZone)
  return {
    from: zonedStartOfDay(ymd, timeZone).toISOString(),
    to: zonedStartOfDay(shiftYmd(ymd, 1), timeZone).toISOString(),
    ymd,
  }
}

function weekday(ymd: string): number {
  const [year, month, day] = ymd.split('-').map(Number)
  return new Date(Date.UTC(year, month - 1, day)).getUTCDay()
}

export function zonedWeek(anchor: Date, timeZone: string) {
  const today = zonedYmd(anchor, timeZone)
  const monday = shiftYmd(today, -((weekday(today) + 6) % 7))
  const days = Array.from({ length: 7 }, (_, index) => shiftYmd(monday, index))
  return {
    from: zonedStartOfDay(monday, timeZone).toISOString(),
    to: zonedStartOfDay(shiftYmd(monday, 7), timeZone).toISOString(),
    days,
  }
}

export function zonedMonth(anchor: Date, timeZone: string) {
  const today = zonedYmd(anchor, timeZone)
  const [year, month] = today.split('-').map(Number)
  const first = `${year}-${String(month).padStart(2, '0')}-01`
  const monday = shiftYmd(first, -((weekday(first) + 6) % 7))
  const weeks = Array.from({ length: 6 }, (_, week) =>
    Array.from({ length: 7 }, (_, day) => shiftYmd(monday, week * 7 + day)),
  )
  return {
    year, month, weeks,
    from: zonedStartOfDay(monday, timeZone).toISOString(),
    to: zonedStartOfDay(shiftYmd(monday, 42), timeZone).toISOString(),
    label: new Intl.DateTimeFormat('en-GB', { month: 'long', year: 'numeric', timeZone: 'UTC' }).format(new Date(`${first}T12:00:00Z`)),
  }
}

export function zonedRange(period: LoadPeriod, now: Date, timeZone: string) {
  const today = zonedYmd(now, timeZone)
  const start = period === 'week' ? zonedWeek(now, timeZone).days[0]
    : period === 'month' ? `${today.slice(0, 7)}-01`
    : period === 'year' ? `${today.slice(0, 4)}-01-01`
    : today
  return { from: zonedStartOfDay(start, timeZone).toISOString(), to: now.toISOString() }
}

export function zonedLocalInput(instant: Date | string, timeZone: string): string {
  const p = parts(typeof instant === 'string' ? new Date(instant) : instant, timeZone)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${p.year}-${pad(p.month)}-${pad(p.day)}T${pad(p.hour)}:${pad(p.minute)}`
}

function offsetMinutes(instant: number, timeZone: string): number {
  const p = parts(new Date(instant), timeZone)
  return (Date.UTC(p.year, p.month - 1, p.day, p.hour, p.minute, p.second) - instant) / 60000
}

export function zonedInstant(local: string, timeZone: string): string | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(local)
  if (!match) return null
  const [, y, m, d, h, min] = match
  const year = Number(y), month = Number(m), day = Number(d), hour = Number(h), minute = Number(min)
  const naive = Date.UTC(year, month - 1, day, hour, minute)
  if (new Date(Date.UTC(year, month - 1, day)).toISOString().slice(0, 10) !== `${y}-${m}-${d}` || hour > 23 || minute > 59) return null
  const offsets = new Set([-2, -1, 0, 1, 2].map((days) => offsetMinutes(naive + days * 86400000, timeZone)))
  const candidates = [...offsets].map((offset) => naive - offset * 60000).sort((a, b) => a - b)
  const exact = candidates.find((candidate) => zonedLocalInput(new Date(candidate), timeZone) === local)
  if (exact !== undefined) return new Date(exact).toISOString()
  let low = candidates[0]
  let high = candidates[candidates.length - 1]
  if (zonedLocalInput(new Date(low), timeZone) >= local || zonedLocalInput(new Date(high), timeZone) < local) return null
  while (high - low > 1) {
    const middle = Math.floor((low + high) / 2)
    if (zonedLocalInput(new Date(middle), timeZone) >= local) high = middle
    else low = middle
  }
  return new Date(high).toISOString()
}

export function zonedMinutes(instant: Date | string, timeZone: string): number {
  const p = parts(typeof instant === 'string' ? new Date(instant) : instant, timeZone)
  return p.hour * 60 + p.minute
}

export function shiftWeeks(anchor: Date, weeks: number, timeZone: string): Date {
  const day = shiftYmd(zonedYmd(anchor, timeZone), weeks * 7)
  return new Date(zonedInstant(`${day}T12:00`, timeZone)!)
}

export function shiftMonths(anchor: Date, months: number, timeZone: string): Date {
  const [year, month, day] = zonedYmd(anchor, timeZone).split('-').map(Number)
  const target = new Date(Date.UTC(year, month - 1 + months, Math.min(day, 28)))
  return new Date(zonedInstant(`${target.toISOString().slice(0, 10)}T12:00`, timeZone)!)
}
