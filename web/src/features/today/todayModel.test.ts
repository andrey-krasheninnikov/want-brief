import { pickNow } from '../schedule/now'
import {
  atRiskToday,
  dailyEarn,
  daysInYmdMonth,
  earnChartDays,
  earnTotals,
  featuredBlock,
  parseUsdRubRate,
  remainingAgenda,
  todayEarn,
  weekdaysInMonth,
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

const closedDev = item({
  id: 'closed-dev',
  title: 'Closed Dev',
  status: 'qa',
  devDueAt: '2026-09-01T00:00:00Z',
})
equal(atRiskToday([], [], [closedDev], ymd, tz, now).length, 0, 'qa ignores past dev due')

const qaReviewToday = item({
  id: 'qa-rev',
  title: 'QA review',
  status: 'qa',
  reviewDueAt: '2026-09-17T18:00:00Z',
})
equal(atRiskToday([], [], [qaReviewToday], ymd, tz, now).length, 0, 'qa ignores review due today')

const qaTestToday = item({
  id: 'qa-test',
  title: 'QA test',
  status: 'qa',
  testDueAt: '2026-09-17T18:00:00Z',
})
equal(atRiskToday([], [], [qaTestToday], ymd, tz, now)[0]?.itemId, 'qa-test', 'qa still sees test due today')

const many = Array.from({ length: 9 }, (_, i) =>
  item({ id: `cap-${i}`, title: `Cap ${i}`, dueAt: `2026-09-17T0${i}:00:00Z` }),
)
equal(atRiskToday([], [], many, ymd, tz, now).length, 8, 'caps at 8')

equal(daysInYmdMonth('2026-09-17'), 30, 'september days')
equal(daysInYmdMonth('2026-02-01'), 28, 'february 2026')
equal(weekdaysInMonth('2026-09-17'), 22, 'september weekdays')

const projects = [
  { id: 'p1', monthlyIncomeUsd: 1200, monthlyIncomeRub: 22000, archivedAt: null },
  { id: 'p2', monthlyIncomeUsd: 1000, monthlyIncomeRub: 0, archivedAt: null },
  { id: 'p3', monthlyIncomeUsd: 9999, monthlyIncomeRub: 9999, archivedAt: '2026-01-01T00:00:00Z' },
]
const earned = todayEarn(projects, ymd)
equal(earned.usd, 100, 'usd = active income / 22 weekdays')
equal(earned.rub, 1000, 'rub independent of usd')
equal(todayEarn(projects, '2026-09-19').usd, 0, 'saturday is zero')
equal(dailyEarn(projects, '2026-09-19').usd, 100, 'weekday rate still available on saturday')

const chart = earnChartDays('2026-09-03', [], { usd: 100, rub: 0 }, 85)
equal(
  chart.map((row) => Math.round(row.cumulative)),
  [100, 200, 300],
  'cumulative bars grow on weekdays 1-3 september',
)
equal(
  earnChartDays('2026-09-07', [], { usd: 100, rub: 0 }, 85).map((row) => Math.round(row.cumulative)),
  [100, 200, 300, 400, 400, 400, 500],
  'weekend days add no height',
)
equal(
  earnChartDays('2026-09-02', [{ day: '2026-09-01', usd: 50, rub: 0 }], { usd: 100, rub: 0 }, 85).map((row) =>
    Math.round(row.cumulative),
  ),
  [50, 150],
  'snapshot beats live estimate',
)

const totals = earnTotals(earned.usd, earned.rub, 85)
equal(Math.round(totals.usd), 112, 'total usd adds rub/rate')
equal(Math.round(totals.rub), 9500, 'total rub adds usd*rate')
equal(parseUsdRubRate('90'), 90, 'parses rate')
equal(parseUsdRubRate('nope', 85), 85, 'junk keeps fallback')
equal(parseUsdRubRate('0', 85), 85, 'non-positive ignored')

console.log('todayModel ok')
