import type { Person, PersonBond } from '../../types'
import { PersonBondGraph } from './PersonBondGraph'
import { bondKindLabel, copyText, primaryContact } from './peopleModel'

type Props = {
  person: Person
}

export function PersonOverview({ person }: Props) {
  const primary = primaryContact(person)
  const open = (person.bonds ?? []).filter((bond) => !bond.endedOn).slice(0, 3)

  return (
    <section id="people-sec-overview" className="people-section">
      <p className="people-kicker">Overview</p>
      {person.age != null ? <p className="people-rail-meta">Age {person.age}</p> : null}
      {primary ? (
        <p className="people-primary">
          <span className="people-kicker">Primary</span>
          <button type="button" className="people-contact-value" onClick={() => void copyText(primary.value)}>
            {primary.value}
          </button>
        </p>
      ) : (
        <p className="muted">No primary contact.</p>
      )}
      <ul className="people-open-bonds">
        {open.map((bond: PersonBond) => (
          <li key={bond.id}>
            {bondKindLabel(bond.kind)} · {bond.otherName || 'Me'}
          </li>
        ))}
      </ul>
      <PersonBondGraph person={person} bonds={person.bonds ?? []} />
    </section>
  )
}
