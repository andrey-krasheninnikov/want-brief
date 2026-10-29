import { Button } from '@want-foundation/desyst-react'
import { Link } from 'react-router-dom'
import { TaskTimer } from '../tasks/TaskTimer'
import { clock, phaseLabel, type NowPick } from '../schedule/now'
import type { ScheduleBlock, TimeInterval } from '../../types'

type Props = {
  pick: NowPick
  loading: boolean
  running: TimeInterval[]
  tz: string
  titleOf: (itemId: string) => string | undefined
  onOpen: (itemId: string) => void
}

function intervalOf(running: TimeInterval[], itemId: string) {
  return running.find((row) => row.itemId === itemId)
}

function WorkCard({
  block,
  running,
  tz,
  featured,
  onOpen,
}: {
  block: ScheduleBlock
  running?: TimeInterval
  tz: string
  featured?: boolean
  onOpen: (itemId: string) => void
}) {
  const title = block.externalKey ? `[${block.externalKey}] ${block.title}` : block.title
  return (
    <article className={featured ? 'agenda-next' : 'agenda-row'} onClick={() => onOpen(block.itemId)}>
      <div className="today-agenda-core">
        <time className="mono" dateTime={block.startsAt}>
          {clock(block.startsAt, tz)}
          <small>{clock(block.endsAt, tz)}</small>
        </time>
        <div className="agenda-body">
          <strong>{title}</strong>
          <span className="agenda-chip">Work</span>
          {block.late ? <span className="agenda-chip late">late</span> : null}
        </div>
        <TaskTimer itemId={block.itemId} running={running} prominent />
      </div>
    </article>
  )
}

export function TodayNow({ pick, loading, running, tz, titleOf, onOpen }: Props) {
  const featured = pick.current[0] ?? pick.upcoming[0]
  const alsoNow = pick.current.slice(1)
  const planned = new Set([...pick.current.map((row) => row.itemId), featured?.itemId].filter(Boolean))
  const offPlan = running.filter((row) => !planned.has(row.itemId))
  const empty = !loading && !featured

  return (
    <section className="agenda">
      <h3>{loading && !featured ? 'Packing' : phaseLabel(pick, tz)}</h3>
      {loading && !featured ? (
        <div className="agenda-skels" aria-hidden>
          <div className="agenda-skel" />
        </div>
      ) : null}
      {featured ? (
        <WorkCard
          block={featured}
          running={intervalOf(running, featured.itemId)}
          tz={tz}
          featured
          onOpen={onOpen}
        />
      ) : null}
      {empty ? (
        <p className="muted today-empty">
          {pick.phase === 'after' ? 'Nothing left in the plan today.' : 'Nothing packed for today.'}
          <Link to="/schedule">Open schedule</Link>
        </p>
      ) : null}
      {alsoNow.map((row) => (
        <WorkCard
          key={`${row.itemId}-${row.startsAt}`}
          block={row}
          running={intervalOf(running, row.itemId)}
          tz={tz}
          onOpen={onOpen}
        />
      ))}
      {offPlan.map((row) => (
        <p key={row.id} className="today-offplan">
          <span>
            Running <em>off plan</em>:
          </span>
          <Button type="button" className="ghost" onClick={() => onOpen(row.itemId)}>
            {titleOf(row.itemId) ?? 'a task outside this plan'}
          </Button>
          <TaskTimer itemId={row.itemId} running={row} />
        </p>
      ))}
    </section>
  )
}
