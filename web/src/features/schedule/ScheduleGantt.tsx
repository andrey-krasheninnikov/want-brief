import { BellSimple, CalendarBlank } from '@phosphor-icons/react'
import { span } from '../../shared/format'
import type { ScheduleBlock, ScheduleBusy, ScheduleLane, SchedulePerson } from '../../types'
import { personInitials } from '../people/PersonCard'
import { blockSeconds, clock, dayHead, zonedMinutes, zonedYmd } from './now'

const COL = 48
const LABEL = '11.5rem'

type ChartScale = { tz: string; start: number; hours: number }

type Props = {
  lanes: ScheduleLane[]
  busy: ScheduleBusy[]
  days: string[]
  day: string
  today: string
  grid: ChartScale
  runningIds: Set<string>
  focusIds: Set<string>
  now: number
  onDay: (ymd: string) => void
  onOpen: (id: string) => void
  onBusy: (id: string, originalOn?: string) => void
}

type ChartRow = {
  itemId: string
  key: string
  title: string
  color: string
  blocks: ScheduleBlock[]
}

function endMinutes(iso: string, startMin: number, tz: string): number {
  const end = zonedMinutes(iso, tz)
  return end < startMin ? end + 24 * 60 : end
}

function offsetPx(iso: string, grid: ChartScale): number {
  return ((zonedMinutes(iso, grid.tz) - grid.start * 60) / 60) * COL
}

function widthPx(startIso: string, endIso: string, grid: ChartScale): number {
  const start = zonedMinutes(startIso, grid.tz)
  const end = endMinutes(endIso, start, grid.tz)
  return Math.max(((end - start) / 60) * COL, 16)
}

function chartRows(lanes: ScheduleLane[], ymd: string, tz: string): ChartRow[] {
  const order: ChartRow[] = []
  const byId = new Map<string, ChartRow>()
  for (const lane of lanes) {
    const color = lane.projectColor || 'var(--accent)'
    const dayBlocks = lane.blocks
      .filter((row) => zonedYmd(row.startsAt, tz) === ymd)
      .sort((a, b) => a.startsAt.localeCompare(b.startsAt))
    for (const block of dayBlocks) {
      const existing = byId.get(block.itemId)
      if (existing) {
        existing.blocks.push(block)
        continue
      }
      const row: ChartRow = {
        itemId: block.itemId,
        key: block.externalKey,
        title: block.title,
        color,
        blocks: [block],
      }
      byId.set(block.itemId, row)
      order.push(row)
    }
  }
  return order
}

function Who({ people }: { people?: SchedulePerson[] }) {
  if (!people || people.length === 0) return null
  return (
    <span className="sched-chart-who">
      {people.slice(0, 3).map((person) => (
        <i key={person.id} title={person.name}>
          {personInitials(person.name)}
        </i>
      ))}
    </span>
  )
}

export function ScheduleGantt({
  lanes,
  busy,
  days,
  day,
  today,
  grid,
  runningIds,
  focusIds,
  now,
  onDay,
  onOpen,
  onBusy,
}: Props) {
  const rows = chartRows(lanes, day, grid.tz)
  const meetings = busy.filter((row) => zonedYmd(row.startsAt, grid.tz) === day)
  const hours = Array.from({ length: grid.hours }, (_, i) => grid.start + i)
  const trackW = `${grid.hours * COL}px`
  const nowIso = new Date(now).toISOString()
  const nowLeft = day === today ? ((zonedMinutes(nowIso, grid.tz) - grid.start * 60) / 60) * COL : null

  return (
    <div className="sched-chart">
      <div className="sched-chart-days" role="tablist" aria-label="Day">
        {days.map((ymd) => {
          const head = dayHead(ymd, today)
          return (
            <button
              key={ymd}
              type="button"
              role="tab"
              aria-selected={ymd === day}
              className={['ghost', ymd === day ? 'on' : '', head.today ? 'today' : '', head.weekend ? 'weekend' : '']
                .filter(Boolean)
                .join(' ')}
              onClick={() => onDay(ymd)}
            >
              <small>{head.week}</small>
              <strong>{head.num}</strong>
            </button>
          )
        })}
      </div>
      <div className="sched-chart-scroll">
        <div className="sched-chart-board" style={{ ['--chart-label' as string]: LABEL, ['--chart-track' as string]: trackW }}>
          <div className="sched-chart-axis">
            <div className="sched-chart-label" />
            <div className="sched-chart-hours" aria-hidden>
              {hours.map((hour) => (
                <span key={hour}>{String(hour).padStart(2, '0')}:00</span>
              ))}
            </div>
          </div>
          <div className="sched-chart-stack">
            <div className="sched-chart-overlay" aria-hidden={meetings.length === 0}>
              {hours.map((hour) => (
                <i key={hour} className="sched-chart-gridline" style={{ left: (hour - grid.start) * COL }} />
              ))}
              {meetings.map((row, i) => (
                <MeetBand key={`${row.seriesId ?? row.title}-${row.startsAt}-${i}`} row={row} grid={grid} onBusy={onBusy} />
              ))}
              {nowLeft != null ? (
                <i className="sched-chart-now">
                  <span style={{ left: nowLeft }}>{clock(nowIso, grid.tz)}</span>
                  <b style={{ left: nowLeft }} />
                </i>
              ) : null}
            </div>
            {rows.length === 0 ? (
              <p className="sched-chart-empty muted">No blocks this day</p>
            ) : (
              rows.map((row) => (
                <ChartLane
                  key={row.itemId}
                  row={row}
                  grid={grid}
                  runningIds={runningIds}
                  focusIds={focusIds}
                  onOpen={onOpen}
                />
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function MeetBand({
  row,
  grid,
  onBusy,
}: {
  row: ScheduleBusy
  grid: ChartScale
  onBusy: (id: string, originalOn?: string) => void
}) {
  const left = offsetPx(row.startsAt, grid)
  const width = widthPx(row.startsAt, row.endsAt, grid)
  const active =
    row.activeStartsAt &&
    row.activeEndsAt &&
    (zonedMinutes(row.activeStartsAt, grid.tz) !== zonedMinutes(row.startsAt, grid.tz) ||
      zonedMinutes(row.activeEndsAt, grid.tz) !== zonedMinutes(row.endsAt, grid.tz))
  const innerLeft = active ? offsetPx(row.activeStartsAt!, grid) - left : 0
  const innerWidth = active ? widthPx(row.activeStartsAt!, row.activeEndsAt!, grid) : width
  const cls = ['sched-chart-meet', row.soft ? 'soft' : '', row.seriesId ? 'click' : ''].filter(Boolean).join(' ')
  const body = (
    <>
      {active ? <i className="sched-chart-active" style={{ left: innerLeft, width: innerWidth }} /> : null}
      <span>
        {row.seriesId ? <CalendarBlank size={11} weight="light" aria-hidden /> : null}
        {row.title}
      </span>
      <Who people={row.people} />
    </>
  )
  if (row.seriesId) {
    return (
      <button
        type="button"
        className={cls}
        style={{ left, width }}
        title={row.soft ? `${row.title} (skippable)` : row.title}
        onClick={() => onBusy(row.seriesId!, row.originalOn)}
      >
        {body}
      </button>
    )
  }
  return (
    <i className={cls} style={{ left, width }}>
      {body}
    </i>
  )
}

function ChartLane({
  row,
  grid,
  runningIds,
  focusIds,
  onOpen,
}: {
  row: ChartRow
  grid: ChartScale
  runningIds: Set<string>
  focusIds: Set<string>
  onOpen: (id: string) => void
}) {
  return (
    <>
      <div className="sched-chart-label">
        {row.key ? <b className="sched-key">{row.key}</b> : null}
        <strong>{row.title}</strong>
      </div>
      <div className="sched-chart-track">
        {row.blocks.map((block, i) => {
          const next = row.blocks[i + 1]
          const left = offsetPx(block.startsAt, grid)
          const width = widthPx(block.startsAt, block.endsAt, grid)
          const ping = block.kind === 'ping'
          const running = runningIds.has(block.itemId)
          const focus = focusIds.has(`${block.itemId}-${block.startsAt}`)
          const fill = block.late
            ? 'color-mix(in srgb, var(--danger) 28%, transparent)'
            : `color-mix(in srgb, ${row.color} 32%, transparent)`
          const border = focus
            ? 'color-mix(in srgb, var(--accent-2) 80%, transparent)'
            : block.late
              ? 'color-mix(in srgb, var(--danger) 40%, transparent)'
              : `color-mix(in srgb, ${row.color} 70%, transparent)`
          const cls = [
            'sched-chart-bar',
            ping ? 'ping' : '',
            block.late ? 'late' : '',
            running ? 'running' : '',
            focus ? 'focus' : '',
            block.occupancy === 'waiting' ? 'waiting' : '',
          ]
            .filter(Boolean)
            .join(' ')
          return (
            <span key={`${block.startsAt}-${i}`}>
              {next && block.continues ? (
                <i
                  className="sched-chart-link"
                  style={{ left: left + width, width: Math.max(offsetPx(next.startsAt, grid) - left - width, 8) }}
                />
              ) : null}
              <button
                type="button"
                className={cls}
                style={{ left, width, background: fill, borderColor: border }}
                title={`${block.externalKey} ${block.title}`}
                onClick={() => onOpen(block.itemId)}
              >
                {ping ? <BellSimple size={11} weight="fill" aria-hidden /> : null}
                <em>{span(blockSeconds(block))}</em>
                <Who people={block.people} />
              </button>
            </span>
          )
        })}
      </div>
    </>
  )
}
