import { isOverdue, nearestDue, sortColumn, type SortableItem } from './sortColumn'

const now = Date.parse('2026-09-17T00:00:00Z')

function item(partial: Partial<SortableItem>): SortableItem {
  return {
    urgent: false,
    important: false,
    createdAt: '2026-09-01T00:00:00Z',
    status: 'to_do',
    dueAt: null,
    devDueAt: null,
    reviewDueAt: null,
    testDueAt: null,
    ...partial,
  }
}

function equal<T>(got: T, want: T, label: string) {
  if (got !== want) throw new Error(`${label}: got ${String(got)}`)
}

const overdue = item({ dueAt: '2026-09-10T00:00:00Z', createdAt: '2026-09-16T00:00:00Z' })
const focus = item({ urgent: true, important: true, createdAt: '2026-09-15T00:00:00Z' })
const soon = item({ dueAt: '2026-09-20T00:00:00Z', createdAt: '2026-09-14T00:00:00Z' })
const later = item({ dueAt: '2026-10-01T00:00:00Z', createdAt: '2026-09-13T00:00:00Z' })
const old = item({ createdAt: '2026-08-01T00:00:00Z' })
const fresh = item({ createdAt: '2026-09-16T12:00:00Z' })

const ranked = [fresh, later, old, soon, focus, overdue].sort((a, b) => sortColumn(a, b, now))
equal(ranked[0], overdue, 'overdue first')
equal(ranked[1], focus, 'U+I second')
equal(ranked[2], soon, 'nearest due')
equal(ranked[3], later, 'later due')
equal(ranked[4], fresh, 'newer created')
equal(ranked[5], old, 'older created last')
equal(nearestDue(soon), Date.parse('2026-09-20T00:00:00Z'), 'nearestDue')
equal(isOverdue(item({ status: 'review', devDueAt: '2026-09-10T00:00:00Z' }), now), false, 'closed dev due')
equal(isOverdue(item({ status: 'review', reviewDueAt: '2026-09-10T00:00:00Z' }), now), true, 'open review due')
equal(
  nearestDue(item({ status: 'review', devDueAt: '2026-09-01T00:00:00Z', dueAt: '2026-09-20T00:00:00Z' })),
  Date.parse('2026-09-20T00:00:00Z'),
  'nearestDue skips closed slot',
)
console.log('sortColumn ok')
