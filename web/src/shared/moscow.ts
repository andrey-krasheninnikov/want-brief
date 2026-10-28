const MOSCOW = 'Europe/Moscow'
const TASHKENT = 'Asia/Tashkent'

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
