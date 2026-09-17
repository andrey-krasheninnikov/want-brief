export function hours(seconds: number): string {
  const h = seconds / 3600
  if (h < 0.1) return `${Math.round(seconds / 60)}m`
  return `${h.toFixed(1)}h`
}

export function span(seconds: number): string {
  const total = Math.max(0, Math.round(seconds / 60))
  const h = Math.floor(total / 60)
  const m = total % 60
  if (h > 0 && m > 0) return `${h}h ${m}m`
  if (h > 0) return `${h}h`
  return `${m}m`
}

export function money(value: number, locale: string): string {
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }).format(value)
}

export function hourlyRate(income: number, seconds: number, locale: string): string {
  if (seconds <= 0) return '—'
  return money(income / (seconds / 3600), locale)
}

export function itemCost(income: number, itemSeconds: number, projectSeconds: number, locale: string): string {
  if (projectSeconds <= 0) return '—'
  return money((income * itemSeconds) / projectSeconds, locale)
}

export function elapsed(startedAt: string, now = Date.now()): string {
  const ms = now - new Date(startedAt).getTime()
  const total = Math.max(0, Math.floor(ms / 1000))
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}

const WEEK_MS = 7 * 24 * 60 * 60 * 1000

export type DueSlot = 'dueAt' | 'devDueAt' | 'reviewDueAt' | 'testDueAt'

const DEV_OPEN = new Set(['backlog', 'needs_grooming', 'to_do', 'in_progress', 'blocked'])
const REVIEW_OPEN = new Set([...DEV_OPEN, 'review'])
const TEST_OPEN = new Set([...REVIEW_OPEN, 'qa'])

export function dueStageOpen(status: string, slot: DueSlot): boolean {
  if (status === 'done' || status === 'cancelled') return false
  if (slot === 'dueAt') return true
  if (slot === 'devDueAt') return DEV_OPEN.has(status)
  if (slot === 'reviewDueAt') return REVIEW_OPEN.has(status)
  return TEST_OPEN.has(status)
}

export function dueHeat(iso: string | null, status: string, slot: DueSlot, now = Date.now()): number {
  if (!iso || !dueStageOpen(status, slot)) return 0
  const due = new Date(iso).getTime()
  if (Number.isNaN(due)) return 0
  if (due <= now) return 1
  const remain = due - now
  if (remain >= WEEK_MS) return 0
  return 1 - remain / WEEK_MS
}

export function dueRemain(iso: string | null, now = Date.now()): string {
  if (!iso) return ''
  const due = new Date(iso).getTime()
  if (Number.isNaN(due)) return ''
  const diff = due - now
  const abs = Math.abs(diff)
  const d = Math.floor(abs / (24 * 60 * 60 * 1000))
  const h = Math.floor((abs % (24 * 60 * 60 * 1000)) / (60 * 60 * 1000))
  const label = d > 0 && h > 0 ? `${d}d ${h}h` : d > 0 ? `${d}d` : `${Math.max(h, 0)}h`
  return diff < 0 ? `-${label}` : label
}

export function dueWithinWeek(iso: string | null, now = Date.now()): boolean {
  if (!iso) return false
  const due = new Date(iso).getTime()
  if (Number.isNaN(due)) return false
  return due <= now + WEEK_MS
}

export function createdLabel(iso: string): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return ''
  return new Intl.DateTimeFormat('en-GB', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(at)
}

export function liveTracked(base: number, startedAt: string | undefined, now = Date.now()): number {
  if (!startedAt) return base
  return base + Math.max(0, Math.floor((now - new Date(startedAt).getTime()) / 1000))
}
