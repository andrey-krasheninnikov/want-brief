import { BellSimple, CalendarBlank } from '@phosphor-icons/react'
import { span } from '../../shared/format'
import { scheduleReasonLabel, type DayOverride, type ScheduleBlock, type ScheduleBusy, type ScheduleLane } from '../../types'
import type { DeckProject } from './NowDeck'
import { blocksOn, clock, dayHead, minutesToClock, topReasons, zonedYmd } from './now'

type Props = {
  lanes: ScheduleLane[]
  busy: ScheduleBusy[]
  days: string[]
  today: string
  tz: string
  overrides: Map<string, DayOverride>
  runningIds: Set<string>
  focusIds: Set<string>
  projectOf: (itemId: string) => DeckProject | undefined
  onOpen: (id: string) => void
  onBusy: (id: string, originalOn?: string) => void
}

type Row = { kind: 'block'; block: ScheduleBlock } | { kind: 'meeting'; busy: ScheduleBusy }

const COLUMNS = 7

function seconds(startsAt: string, endsAt: string): number {
  return Math.max(0, Math.round((new Date(endsAt).getTime() - new Date(startsAt).getTime()) / 1000))
}

function startOf(row: Row): string {
  return row.kind === 'block' ? row.block.startsAt : row.busy.startsAt
}

function dayRows(lanes: ScheduleLane[], busy: ScheduleBusy[], ymd: string, tz: string): Row[] {
  const blocks: Row[] = blocksOn(lanes, ymd, tz).map((block) => ({ kind: 'block', block }))
  const meetings: Row[] = busy
    .filter((row) => row.seriesId && zonedYmd(row.startsAt, tz) === ymd)
    .map((row) => ({ kind: 'meeting', busy: row }))
  return [...blocks, ...meetings].sort((a, b) => startOf(a).localeCompare(startOf(b)))
}

function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`
}

function summary(rows: Row[]): string {
  const blocks = rows.filter((row): row is Extract<Row, { kind: 'block' }> => row.kind === 'block')
  const pings = blocks.filter((row) => row.block.kind === 'ping').length
  const work = blocks.length - pings
  const meetings = rows.length - blocks.length
  const total = blocks.reduce((sum, row) => (row.block.kind === 'ping' ? sum : sum + seconds(row.block.startsAt, row.block.endsAt)), 0)
  const parts: string[] = []
  if (work > 0) parts.push(plural(work, 'block', 'blocks'), span(total))
  if (pings > 0) parts.push(plural(pings, 'ping', 'pings'))
  if (meetings > 0) parts.push(plural(meetings, 'meeting', 'meetings'))
  return parts.join(', ')
}

function overrideLabel(override?: DayOverride): string {
  if (!override) return ''
  if (override.off) return override.note ? `Day off · ${override.note}` : 'Day off'
  if (override.workStartMin != null && override.workEndMin != null) {
    const hours = `${minutesToClock(override.workStartMin)}–${minutesToClock(override.workEndMin)}`
    return override.note ? `${hours} · ${override.note}` : hours
  }
  return override.note
}

function Why({ block }: { block: ScheduleBlock }) {
  const all = (block.reasons ?? []).map(scheduleReasonLabel)
  if (all.length === 0) return <td className="sched-table-why" />
  return (
    <td className="sched-table-why" title={all.join(' · ')}>
      <span className="sched-block-flags">
        {topReasons(block.reasons).map((reason) => (
          <span key={reason} className="sched-chip why">
            {scheduleReasonLabel(reason)}
          </span>
        ))}
        {all.length > 2 ? <span className="sched-chip">+{all.length - 2}</span> : null}
      </span>
    </td>
  )
}

function BlockRow({
  block,
  project,
  running,
  focus,
  tz,
  onOpen,
}: {
  block: ScheduleBlock
  project?: DeckProject
  running: boolean
  focus: boolean
  tz: string
  onOpen: (id: string) => void
}) {
  const ping = block.kind === 'ping'
  const cls = ['sched-table-row', ping ? 'ping' : '', block.late ? 'late' : '', running ? 'running' : '', focus ? 'focus' : '']
    .filter(Boolean)
    .join(' ')
  return (
    <tr className={cls} onClick={() => onOpen(block.itemId)}>
      <td className="mono">
        {clock(block.startsAt, tz)}-{clock(block.endsAt, tz)}
      </td>
      <td className="sched-table-key">
        <b className="sched-key">{block.externalKey}</b>
      </td>
      <td className="sched-table-title">
        {ping ? <BellSimple size={12} weight="fill" aria-hidden /> : null}
        {block.title}
      </td>
      <td className="sched-table-project">
        {project ? (
          <>
            <i className="sched-swatch" style={{ background: project.color }} />
            {project.name}
          </>
        ) : null}
      </td>
      <td className="mono sched-table-span">{span(seconds(block.startsAt, block.endsAt))}</td>
      <td className="sched-table-flags">
        <span className="sched-block-flags">
          {ping ? <span className="sched-chip">ping</span> : null}
          {block.late ? <span className="sched-chip late">late</span> : null}
          {block.continued ? <span className="sched-chip">continues</span> : null}
          {running ? <span className="sched-chip live">in time</span> : null}
          {block.occupancy === 'parallel' ? <span className="sched-chip">parallel</span> : null}
          {block.occupancy === 'waiting' ? <span className="sched-chip">waiting</span> : null}
        </span>
      </td>
      <Why block={block} />
    </tr>
  )
}

function MeetingRow({ busy, tz, onBusy }: { busy: ScheduleBusy; tz: string; onBusy: (id: string, originalOn?: string) => void }) {
  return (
    <tr className="sched-table-row meeting" onClick={() => busy.seriesId && onBusy(busy.seriesId, busy.originalOn)}>
      <td className="mono">
        {clock(busy.startsAt, tz)}-{clock(busy.endsAt, tz)}
      </td>
      <td />
      <td className="sched-table-title">
        <CalendarBlank size={12} weight="light" aria-hidden />
        {busy.title}
      </td>
      <td className="sched-table-project">{busy.soft ? 'Meeting · skippable' : 'Meeting'}</td>
      <td className="mono sched-table-span">{span(seconds(busy.startsAt, busy.endsAt))}</td>
      <td />
      <td />
    </tr>
  )
}

export function ScheduleTable({ lanes, busy, days, today, tz, overrides, runningIds, focusIds, projectOf, onOpen, onBusy }: Props) {
  return (
    <div className="sched-table-wrap">
      <table className="sched-table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Key</th>
            <th>Task</th>
            <th className="sched-table-project">Project</th>
            <th className="sched-table-span">Span</th>
            <th>Flags</th>
            <th className="sched-table-why">Why</th>
          </tr>
        </thead>
        {days.map((ymd) => {
          const head = dayHead(ymd, today)
          const rows = dayRows(lanes, busy, ymd, tz)
          const override = overrides.get(ymd)
          if (head.weekend && rows.length === 0 && !override) return null
          return (
            <tbody key={ymd} className={head.today ? 'today' : undefined}>
              <tr className={head.today ? 'sched-table-day today' : 'sched-table-day'}>
                <th colSpan={COLUMNS}>
                  <span className="sched-table-date">
                    {head.week} {head.num}
                  </span>
                  {head.today ? <em>Today</em> : null}
                  {override ? <em className="off">{overrideLabel(override)}</em> : null}
                  <small>{summary(rows)}</small>
                </th>
              </tr>
              {rows.length === 0 ? (
                <tr className="sched-table-row empty">
                  <td colSpan={COLUMNS}>{override?.off ? 'Day off' : 'No slots'}</td>
                </tr>
              ) : (
                rows.map((row) =>
                  row.kind === 'block' ? (
                    <BlockRow
                      key={`${row.block.itemId}-${row.block.startsAt}`}
                      block={row.block}
                      project={projectOf(row.block.itemId)}
                      running={runningIds.has(row.block.itemId)}
                      focus={focusIds.has(`${row.block.itemId}-${row.block.startsAt}`)}
                      tz={tz}
                      onOpen={onOpen}
                    />
                  ) : (
                    <MeetingRow
                      key={`${row.busy.seriesId}-${row.busy.originalOn}-${row.busy.startsAt}`}
                      busy={row.busy}
                      tz={tz}
                      onBusy={onBusy}
                    />
                  ),
                )
              )}
            </tbody>
          )
        })}
      </table>
    </div>
  )
}
