import { useMemo, useState } from 'react'
import type { Person } from '../../types'
import { personInitials } from './PersonCard'
import { bondKindLabel, primaryContact, professionLabel, relLabels } from './peopleModel'

type Col = 'name' | 'profession' | 'company' | 'age' | 'primary' | 'me' | 'note'

type Props = {
  people: Person[]
  companyNames: Map<string, string>
  onPick: (id: string) => void
}

const COLS: { id: Col; label: string }[] = [
  { id: 'name', label: 'Name' },
  { id: 'profession', label: 'Profession' },
  { id: 'company', label: 'Company' },
  { id: 'age', label: 'Age' },
  { id: 'primary', label: 'Primary' },
  { id: 'me', label: 'Me' },
  { id: 'note', label: 'Last note' },
]

function noteLabel(iso: string | null | undefined): string {
  if (!iso) return ''
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium' }).format(new Date(iso))
}

function sortValue(person: Person, col: Col, companyNames: Map<string, string>): string | number {
  if (col === 'name') return person.name.toLowerCase()
  if (col === 'profession') return professionLabel(person).toLowerCase()
  if (col === 'company') return relLabels(person.companies, companyNames).toLowerCase()
  if (col === 'age') return person.age ?? -1
  if (col === 'primary') return primaryContact(person)?.value.toLowerCase() ?? ''
  if (col === 'me') return person.meBond?.kind ?? ''
  return person.lastNoteAt ?? ''
}

export function PeopleTable({ people, companyNames, onPick }: Props) {
  const [sort, setSort] = useState<{ col: Col; dir: 'asc' | 'desc' }>({ col: 'name', dir: 'asc' })
  const rows = useMemo(() => {
    const copy = [...people]
    copy.sort((a, b) => {
      const av = sortValue(a, sort.col, companyNames)
      const bv = sortValue(b, sort.col, companyNames)
      const cmp = av < bv ? -1 : av > bv ? 1 : 0
      return sort.dir === 'asc' ? cmp : -cmp
    })
    return copy
  }, [people, sort, companyNames])

  function toggle(col: Col) {
    setSort((current) => (current.col === col ? { col, dir: current.dir === 'asc' ? 'desc' : 'asc' } : { col, dir: 'asc' }))
  }

  return (
    <div className="people-table-wrap">
      <table className="people-table">
        <thead>
          <tr>
            {COLS.map((col) => (
              <th key={col.id}>
                <button type="button" className="people-th" onClick={() => toggle(col.id)}>
                  {col.label}
                  {sort.col === col.id ? (sort.dir === 'asc' ? ' ↑' : ' ↓') : ''}
                </button>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((person) => (
            <tr key={person.id} onClick={() => onPick(person.id)}>
              <td>
                <button type="button" className="people-table-name" onClick={() => onPick(person.id)}>
                  <span className="people-avatar" aria-hidden>
                    {personInitials(person.name)}
                  </span>
                  {person.name}
                </button>
              </td>
              <td>{professionLabel(person) || 'No profession'}</td>
              <td>{relLabels(person.companies, companyNames)}</td>
              <td className="mono">{person.age ?? ''}</td>
              <td>{primaryContact(person)?.value ?? ''}</td>
              <td>{person.meBond ? bondKindLabel(person.meBond.kind) : ''}</td>
              <td className="mono">{noteLabel(person.lastNoteAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
