import type { Item, ItemStatus, Quadrant } from '../../types'

const ORDER: Record<Quadrant, number> = {
  do: 0,
  schedule: 1,
  delegate: 2,
  drop: 3,
}

const PRIORITY_STATUSES = new Set<ItemStatus>(['backlog', 'to_do', 'in_progress'])

export function priorityItems(items: Item[]): Item[] {
  return items.filter((item) => PRIORITY_STATUSES.has(item.status))
}

export function compareWithinQuadrant(a: Item, b: Item): number {
  const aDue = a.dueAt ? new Date(a.dueAt).getTime() : Number.POSITIVE_INFINITY
  const bDue = b.dueAt ? new Date(b.dueAt).getTime() : Number.POSITIVE_INFINITY
  if (aDue !== bDue) return aDue - bDue
  const stress = (b.stress ?? -1) - (a.stress ?? -1)
  if (stress !== 0) return stress
  return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
}

export function rankItems(items: Item[]): Item[] {
  return [...items].sort((a, b) => {
    const quadrant = ORDER[a.quadrant] - ORDER[b.quadrant]
    if (quadrant !== 0) return quadrant
    return compareWithinQuadrant(a, b)
  })
}

export function itemsInQuadrant(items: Item[], quadrant: Quadrant): Item[] {
  return items.filter((item) => item.quadrant === quadrant).sort(compareWithinQuadrant)
}
