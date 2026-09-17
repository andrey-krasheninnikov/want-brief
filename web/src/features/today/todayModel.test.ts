import { pickNow } from '../schedule/now'
import {
  atRiskToday,
  daysInYmdMonth,
  earnTotals,
  featuredBlock,
  parseUsdRubRate,
  remainingAgenda,
  todayEarn,
  type DueItem,
} from './todayModel'
import type { AtRiskItem, ScheduleBlock, ScheduleBusy } from '../../types'

const now = Date.parse('2026-09-17T12:00:00Z')
const ymd = '2026-09-17'
const tz = 'Europe/Moscow'

function equal<T>(got: T, want: T, label: string) {
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    throw new Error(`${label}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`)
  }
}

function block(partial: Partial<ScheduleBlock> & Pick<ScheduleBlock, 'itemId' | 'startsAt' | 'endsAt'>): ScheduleBlock {
  return {
    title: 'Task',
    externalKey: '',
    kind: 'work',
    late: false,
    continued: false,
    continues: false,
    lane: 0,
    occupancy: 'solo',
    pinned: false,
    quadrant: 'do',
    dueAt: '2026-09-17T18:00:00Z',
    stress: null,
    remainingSeconds: 3600,
    estimateFactor: 1,
    reasons: [],
    ...partial,
  }
}

function item(partial: Partial<DueItem> & Pick<DueItem, 'id'>): DueItem {
  return {
    title: 'Item',
    externalKey: '',
    status: 'to_do',
    dueAt: null,
    devDueAt: null,
    reviewDueAt: null,
    testDueAt: null,
    ...partial,
  }
}

function busy(row: Pick<ScheduleBusy, 'startsAt' | 'endsAt' | 'title'> & Partial<ScheduleBusy>): ScheduleBusy {
  return { soft: false, ...row }
}

const past = block({ itemId: 'past', title: 'Past', startsAt: '2026-09-17T10:00:00Z', endsAt: '2026-09-17T11:00:00Z' })
const current = block({ itemId: 'now-item', title: 'Now', startsAt: '2026-09-17T11:00:00Z', endsAt: '2026-09-17T13:00:00Z' })
const laterSame = block({
  itemId: 'now-item',
  title: 'Now later',
  startsAt: '2026-09-17T15:00:00Z',
  endsAt: '2026-09-17T16:00:00Z',
})
const other = block({ itemId: 'other', title: 'Other', startsAt: '2026-09-17T14:00:00Z', endsAt: '2026-09-17T14:30:00Z' })
const pastMeet = busy({
  startsAt: '2026-09-17T08:00:00Z',
  endsAt: '2026-09-17T09:00:00Z',
  title: 'Standup',
  seriesId: 's1',
})
const nextMeet = busy({
  startsAt: '2026-09-17T16:00:00Z',
  endsAt: '2026-09-17T17:00:00Z',
  title: 'Review',
  seriesId: 's2',
})

const blocks = [past, current, other, laterSame]
const pick = pickNow(blocks, now)
equal(featuredBlock(pick)?.itemId, 'now-item', 'featured is covering block')

const agenda = remainingAgenda(pick, blocks, [pastMeet, nextMeet], now)
equal(
  agenda.map((row) => `${row.kind}:${row.title}:${row.startsAt}`),
  ['work:Other:2026-09-17T14:00:00Z', 'work:Now later:2026-09-17T15:00:00Z', 'meeting:Review:2026-09-17T16:00:00Z'],
  'hides past and featured, keeps later slice and remaining meeting',
)

const lateA = block({
  itemId: 'late-a',
  title: 'Late A',
  externalKey: 'WB-1',
  startsAt: '2026-09-17T14:00:00Z',
  endsAt: '2026-09-17T15:00:00Z',
  late: true,
})
const lateDup = block({
  itemId: 'late-a',
  title: 'Late A again',
  externalKey: 'WB-1',
  startsAt: '2026-09-17T16:00:00Z',
  endsAt: '2026-09-17T17:00:00Z',
  late: true,
})
const scheduleRisk: AtRiskItem[] = [{ itemId: 'slack-b', key: 'WB-2', title: 'Slack B', slackSeconds: 60 }]
const dueDev = item({ id: 'due-c', title: 'Due C', externalKey: 'WB-3', devDueAt: '2026-09-17T18:00:00Z' })
const overdue = item({ id: 'over-d', title: 'Over D', dueAt: '2026-09-10T00:00:00Z' })
const later = item({ id: 'later-e', title: 'Later E', dueAt: '2026-09-24T00:00:00Z' })

const risk = atRiskToday([lateA, lateDup], scheduleRisk, [dueDev, overdue, later], ymd, tz, now)
equal(
  risk.map((row) => row.itemId),
  ['over-d', 'late-a', 'due-c', 'slack-b'],
  'overdue first, unique late, due today, schedule atRisk; future due dropped',
)
equal(risk.find((row) => row.itemId === 'late-a')?.late, true, 'late flag')
equal(risk.find((row) => row.itemId === 'over-d')?.overdue, true, 'overdue flag')
equal(risk.find((row) => row.itemId === 'due-c')?.overdue, false, 'due today is not overdue at noon UTC')

const many = Array.from({ length: 9 }, (_, i) =>
  item({ id: `cap-${i}`, title: `Cap ${i}`, dueAt: `2026-09-17T0${i}:00:00Z` }),
)
equal(atRiskToday([], [], many, ymd, tz, now).length, 8, 'caps at 8')

equal(daysInYmdMonth('2026-09-17'), 30, 'september days')
equal(daysInYmdMonth('2026-02-01'), 28, 'february 2026')

const earned = todayEarn(
  [
    { id: 'p1', monthlyIncomeUsd: 3000, monthlyIncomeRub: 255000, targetHoursDay: 2 },
    { id: 'p2', monthlyIncomeUsd: 1000, monthlyIncomeRub: 1000, targetHoursDay: 0 },
  ],
  [
    { projectId: 'p1', allocatedSeconds: 2 * 3600 },
    { projectId: 'p2', allocatedSeconds: 2 * 3600 },
    { projectId: null, allocatedSeconds: 3600 },
  ],
  ymd,
)
equal(Math.round(earned.usd), 100, 'usd = monthly * 2h / (2h * 30d)')
equal(Math.round(earned.rub), 8500, 'rub independent of usd')

const totals = earnTotals(earned.usd, earned.rub, 85)
equal(Math.round(totals.usd), 200, 'total usd adds rub/rate')
equal(Math.round(totals.rub), 17000, 'total rub adds usd*rate')
equal(parseUsdRubRate('90'), 90, 'parses rate')
equal(parseUsdRubRate('nope', 85), 85, 'junk keeps fallback')
equal(parseUsdRubRate('0', 85), 85, 'non-positive ignored')

console.log('todayModel ok')
