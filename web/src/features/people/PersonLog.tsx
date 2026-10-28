import type { PersonBond } from '../../types'
import { bondKindLabel } from './peopleModel'

type Props = {
  bonds: PersonBond[]
}

function stamp(iso: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
}

export function PersonLog({ bonds }: Props) {
  const entries = bonds
    .flatMap((bond) =>
      bond.events.map((event) => ({
        key: event.id,
        at: event.at,
        action: event.action,
        label: `${bondKindLabel(event.kind)} · ${bond.otherName || 'Me'}`,
        comment: event.comment,
      })),
    )
    .sort((a, b) => b.at.localeCompare(a.at))

  return (
    <section id="people-sec-log" className="people-section">
      <p className="people-kicker">Log</p>
      {entries.length === 0 ? <p className="people-empty">No bond events yet.</p> : null}
      {entries.map((entry) => (
        <article key={entry.key} className="people-note">
          <p className="people-kicker">
            {entry.action} · {entry.label} · {stamp(entry.at)}
          </p>
          {entry.comment ? <p>{entry.comment}</p> : null}
        </article>
      ))}
    </section>
  )
}
