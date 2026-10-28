import { zonedDayRange, zonedInstant, zonedWeek, zonedYmd } from './zoned'

function equal(actual: unknown, expected: unknown, name: string) {
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(`${name}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`)
  }
}

const beforeLondonMidnight = new Date('2026-03-29T22:30:00Z')
equal(zonedYmd(beforeLondonMidnight, 'Europe/London'), '2026-03-29', 'London day')
equal(zonedYmd(beforeLondonMidnight, 'Europe/Moscow'), '2026-03-30', 'Moscow day')

equal(
  zonedDayRange(new Date('2026-03-29T12:00:00Z'), 'Europe/London'),
  { from: '2026-03-29T00:00:00.000Z', to: '2026-03-29T23:00:00.000Z', ymd: '2026-03-29' },
  'spring day has 23 hours',
)
equal(
  zonedDayRange(new Date('2026-10-25T12:00:00Z'), 'Europe/London'),
  { from: '2026-10-24T23:00:00.000Z', to: '2026-10-26T00:00:00.000Z', ymd: '2026-10-25' },
  'autumn day has 25 hours',
)
equal(zonedInstant('2026-03-29T01:30', 'Europe/London'), '2026-03-29T01:00:00.000Z', 'missing hour moves forward')
equal(zonedInstant('2026-10-25T01:30', 'Europe/London'), '2026-10-25T00:30:00.000Z', 'repeated hour uses first')
equal(zonedInstant('2026-10-25T01:30', 'UTC'), '2026-10-25T01:30:00.000Z', 'UTC input')
equal(zonedInstant('2026-02-30T01:30', 'Europe/London'), null, 'invalid date')

const week = zonedWeek(new Date('2026-03-29T22:30:00Z'), 'Europe/London')
equal(week.days[0], '2026-03-23', 'week uses local Sunday')
equal(week.to, '2026-03-29T23:00:00.000Z', 'week ends at local Monday')
