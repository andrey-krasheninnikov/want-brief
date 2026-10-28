import { CaretRight } from '@phosphor-icons/react'
import { span } from '../../shared/format'
import { TaskTimer } from '../tasks/TaskTimer'
import { scheduleReasonLabel, type ScheduleBlock, type TimeInterval } from '../../types'
import { blockSeconds, clock, phaseLabel, quadrantLabel, topReasons, type NowPick } from './now'

export type DeckProject = { name: string; color: string }

type Props = {
  pick: NowPick
  loading: boolean
  running: TimeInterval[]
  lateCount: number
  lateId: string | null
  tz: string
  projectOf: (itemId: string) => DeckProject | undefined
  titleOf: (itemId: string) => string | undefined
  onOpen: (id: string) => void
  onNextWeek: () => void
}

function dueLabel(iso: string, tz: string): string {
  return new Intl.DateTimeFormat('en-GB', { weekday: 'short', day: 'numeric', timeZone: tz }).format(new Date(iso))
}

function WhyChips({ block }: { block: ScheduleBlock }) {
  return (
    <span className="sched-block-flags" title={(block.reasons ?? []).map(scheduleReasonLabel).join(' · ')}>
      {block.pinned ? <span className="sched-chip live">Pinned</span> : null}
      <span className="sched-chip">{quadrantLabel(block.quadrant)}</span>
      {topReasons(block.reasons, 3).map((reason) => (
        <span key={reason} className="sched-chip why">
          {scheduleReasonLabel(reason)}
        </span>
      ))}
      {block.stress != null ? <span className="sched-chip">Stress {block.stress}</span> : null}
      {block.late ? <span className="sched-chip late">late</span> : null}
    </span>
  )
}

function Featured({
  block,
  project,
  interval,
  tz,
  onOpen,
}: {
  block: ScheduleBlock
  project?: DeckProject
  interval?: TimeInterval
  tz: string
  onOpen: (id: string) => void
}) {
  return (
    <article className="sched-deck-card" onClick={() => onOpen(block.itemId)}>
      <div className="sched-deck-card-main">
        <p className="sched-deck-meta">
          <i className="sched-swatch" style={{ background: project?.color || 'var(--accent)' }} />
          <span>{project?.name || 'No project'}</span>
          {block.externalKey ? <span className="mono">{block.externalKey}</span> : null}
        </p>
        <strong className="sched-deck-title">
          {block.title}
          <CaretRight size={12} weight="light" aria-hidden />
        </strong>
        <p className="sched-deck-facts mono">
          <span>
            {clock(block.startsAt, tz)}-{clock(block.endsAt, tz)}
          </span>
          {block.kind === 'ping' ? <span>ping</span> : <span>{span(block.remainingSeconds)} left</span>}
          <span>due {dueLabel(block.dueAt, tz)}</span>
        </p>
        <WhyChips block={block} />
      </div>
      <TaskTimer itemId={block.itemId} running={interval} prominent />
    </article>
  )
}

function Row({
  block,
  project,
  interval,
  tz,
  onOpen,
}: {
  block: ScheduleBlock
  project?: DeckProject
  interval?: TimeInterval
  tz: string
  onOpen: (id: string) => void
}) {
  return (
    <li className="sched-deck-row" onClick={() => onOpen(block.itemId)}>
      <time className="mono" dateTime={block.startsAt}>
        {clock(block.startsAt, tz)}
      </time>
      <i className="sched-swatch" style={{ background: project?.color || 'var(--accent)' }} />
      <span className="sched-deck-row-title">
        {block.externalKey ? <span className="mono">{block.externalKey} </span> : null}
        {block.title}
      </span>
      <span className="sched-deck-row-project">{project?.name}</span>
      <span className="sched-deck-row-span mono">{span(blockSeconds(block))}</span>
      <span className="sched-deck-row-flag">{block.late ? <span className="sched-chip late">late</span> : null}</span>
      <span className="sched-deck-row-timer">
        <TaskTimer itemId={block.itemId} running={interval} prominent />
      </span>
    </li>
  )
}

export function NowDeck({ pick, loading, running, lateCount, lateId, tz, projectOf, titleOf, onOpen, onNextWeek }: Props) {
  const featured = pick.current[0] ?? pick.upcoming[0]
  const alsoNow = pick.current.slice(1)
  const planned = new Set([...pick.current.map((row) => row.itemId), featured?.itemId].filter(Boolean))
  const offPlan = running.filter((row) => !planned.has(row.itemId))
  const intervalOf = (itemId: string) => running.find((row) => row.itemId === itemId)

  return (
    <section className="sched-deck">
      <div className="sched-deck-core">
        <header className="sched-deck-head">
          <p className="sched-deck-phase">
            {loading && !featured ? 'Packing' : phaseLabel(pick, tz)}
            {pick.phase === 'now' ? <span className="mono"> {clock(new Date().toISOString(), tz)}</span> : null}
          </p>
          {lateCount > 0 ? (
            <p className="sched-overflow">
              {lateCount} late
              <button type="button" className="ghost" disabled={!lateId} onClick={() => lateId && onOpen(lateId)}>
                Open
              </button>
            </p>
          ) : null}
        </header>
        {loading && !featured ? <div className="sched-deck-skel" aria-hidden /> : null}
        {featured ? (
          <Featured block={featured} project={projectOf(featured.itemId)} interval={intervalOf(featured.itemId)} tz={tz} onOpen={onOpen} />
        ) : null}
        {!loading && !featured ? (
          <p className="sched-deck-empty">
            {pick.phase === 'after' ? 'Nothing left in the plan today.' : 'Nothing packed for today.'}
            <button type="button" className="ghost" onClick={onNextWeek}>
              Next week ›
            </button>
          </p>
        ) : null}
        {alsoNow.length > 0 ? (
          <>
            <p className="sched-deck-kicker">Also now</p>
            <ul className="sched-deck-list">
              {alsoNow.map((row) => (
                <Row
                  key={`${row.itemId}-${row.startsAt}`}
                  block={row}
                  project={projectOf(row.itemId)}
                  interval={intervalOf(row.itemId)}
                  tz={tz}
                  onOpen={onOpen}
                />
              ))}
            </ul>
          </>
        ) : null}
        {offPlan.map((row) => (
          <p key={row.id} className="sched-deck-offplan">
            <span>
              Running <em>off plan</em>:
            </span>
            <button type="button" className="ghost sched-deck-link" onClick={() => onOpen(row.itemId)}>
              {titleOf(row.itemId) ?? 'a task outside this plan'}
            </button>
            <TaskTimer itemId={row.itemId} running={row} />
          </p>
        ))}
      </div>
    </section>
  )
}
