import { dueHeat } from '../../shared/format'
import type { Item } from '../../types'

const DUE_KEYS = ['dueAt', 'devDueAt', 'reviewDueAt', 'testDueAt'] as const

export type DueFields = Pick<Item, 'status' | 'dueAt' | 'devDueAt' | 'reviewDueAt' | 'testDueAt'>

export type SortableItem = DueFields & Pick<Item, 'urgent' | 'important' | 'createdAt'>

function dueTimes(item: DueFields): number[] {
  return DUE_KEYS.map((key) => (item[key] ? new Date(item[key] as string).getTime() : NaN)).filter(
    (time) => !Number.isNaN(time),
  )
}

export function isOverdue(item: DueFields, now = Date.now()): boolean {
  return DUE_KEYS.some((key) => dueHeat(item[key], item.status, now) >= 1)
}

export function nearestDue(item: DueFields): number | null {
  const times = dueTimes(item)
  if (times.length === 0) return null
  return Math.min(...times)
}

export function sortColumn(a: SortableItem, b: SortableItem, now = Date.now()): number {
  const overdue = Number(isOverdue(b, now)) - Number(isOverdue(a, now))
  if (overdue) return overdue
  const focus = Number(b.urgent && b.important) - Number(a.urgent && a.important)
  if (focus) return focus
  const aDue = nearestDue(a)
  const bDue = nearestDue(b)
  if (aDue != null && bDue != null && aDue !== bDue) return aDue - bDue
  if (aDue != null && bDue == null) return -1
  if (aDue == null && bDue != null) return 1
  return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
}
