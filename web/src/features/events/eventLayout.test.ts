import { layoutLanes } from './eventLayout'

function equal<T>(got: T, want: T, label: string) {
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    throw new Error(`${label}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`)
  }
}

equal(layoutLanes([]), [], 'empty')

equal(
  layoutLanes([
    { start: 11 * 60, end: 11 * 60 + 30 },
    { start: 11 * 60, end: 11 * 60 + 30 },
  ]),
  [
    { col: 0, cols: 2 },
    { col: 1, cols: 2 },
  ],
  'two 30m at 11:00',
)

equal(
  layoutLanes([
    { start: 10 * 60, end: 11 * 60 },
    { start: 12 * 60, end: 13 * 60 },
  ]),
  [
    { col: 0, cols: 1 },
    { col: 0, cols: 1 },
  ],
  'non-overlapping 10:00 and 12:00',
)

equal(
  layoutLanes([
    { start: 10 * 60, end: 11 * 60 },
    { start: 11 * 60, end: 12 * 60 },
  ]),
  [
    { col: 0, cols: 1 },
    { col: 0, cols: 1 },
  ],
  'touching end==start share no cluster',
)

equal(
  layoutLanes([
    { start: 10 * 60, end: 11 * 60 + 30 },
    { start: 11 * 60, end: 12 * 60 },
    { start: 12 * 60, end: 13 * 60 },
  ]),
  [
    { col: 0, cols: 2 },
    { col: 1, cols: 2 },
    { col: 0, cols: 1 },
  ],
  'chain then separate',
)

equal(
  layoutLanes([
    { start: 10 * 60, end: 12 * 60 },
    { start: 10 * 60, end: 11 * 60 },
  ]),
  [
    { col: 0, cols: 2 },
    { col: 1, cols: 2 },
  ],
  'longer same-start takes col 0',
)

console.log('eventLayout ok')
