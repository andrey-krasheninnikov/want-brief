import type { Company, Person } from '../../types'
import { careerKindLabel, careerLog, dayLabel, type CareerLogEntry } from './companiesModel'

type Props = {
  company: Company
  people: Person[]
}

type TableProps = {
  entries: CareerLogEntry[]
  empty?: string
}

export function CareerLogTable({ entries, empty = 'No career events yet.' }: TableProps) {
  if (entries.length === 0) return <p className="people-empty">{empty}</p>
  return (
    <div className="people-table-wrap">
      <table className="people-table companies-log-table">
        <thead>
          <tr>
            <th>Kind</th>
            <th>Who</th>
            <th>Date</th>
            <th>Note</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry) => (
            <tr key={entry.key}>
              <td>{careerKindLabel(entry.kind)}</td>
              <td>{entry.who}</td>
              <td className="companies-num">{dayLabel(entry.at)}</td>
              <td>{entry.note || ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function CompanyLog({ company, people }: Props) {
  const names = new Map(people.map((row) => [row.id, row.name]))
  return (
    <section id="company-sec-log" className="people-section">
      <p className="people-kicker">Log</p>
      <CareerLogTable entries={careerLog(company, names)} />
    </section>
  )
}
