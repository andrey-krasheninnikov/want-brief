import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useRef, type DragEvent } from 'react'
import { api } from '../../api'
import type { BondKind, Person } from '../../types'
import { personInitials } from './PersonCard'
import { BOND_KINDS, PERSON_DRAG, bondKindLabel, primaryContact, professionLabel, relLabels } from './peopleModel'

type Col = BondKind | 'unlinked'

type Props = {
  people: Person[]
  companyNames: Map<string, string>
  onPick: (id: string) => void
}

const COLUMNS: { id: Col; label: string }[] = [
  ...BOND_KINDS.map((row) => ({ id: row.id as Col, label: row.label })),
  { id: 'unlinked', label: 'Unlinked' },
]

function columnOf(person: Person): Col {
  return person.meBond?.kind ?? 'unlinked'
}

export function PeopleBoard({ people, companyNames, onPick }: Props) {
  const queryClient = useQueryClient()
  const dragged = useRef('')
  const move = useMutation({
    mutationFn: async ({ person, col }: { person: Person; col: Col }) => {
      const current = columnOf(person)
      if (current === col) return
      if (col === 'unlinked') {
        if (!person.meBond) return
        await api.endPersonBond(person.id, person.meBond.id, { comment: '' })
        return
      }
      if (!person.meBond) {
        await api.createPersonBond(person.id, { otherId: null, kind: col })
        return
      }
      await api.patchPersonBond(person.id, person.meBond.id, { kind: col })
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['people'] })
      void queryClient.invalidateQueries({ queryKey: ['person'] })
    },
  })

  function payload(event: DragEvent): string {
    return event.dataTransfer.getData(PERSON_DRAG) || event.dataTransfer.getData('text/plain')
  }

  function onDrop(col: Col, event: DragEvent) {
    event.preventDefault()
    const id = payload(event)
    const person = people.find((row) => row.id === id)
    if (!person) return
    move.mutate({ person, col })
  }

  return (
    <div className="people-board">
      {COLUMNS.map((col) => {
        const rows = people.filter((person) => columnOf(person) === col.id)
        return (
          <section
            key={col.id}
            className="people-col"
            onDragOver={(event) => {
              event.preventDefault()
              event.dataTransfer.dropEffect = 'move'
            }}
            onDrop={(event) => onDrop(col.id, event)}
          >
            <header>
              <h3>{col.label}</h3>
              <span>{rows.length}</span>
            </header>
            {rows.map((person) => (
              <article
                key={person.id}
                className="people-tile"
                data-person-id={person.id}
                draggable
                aria-label={person.name}
                onDragStart={(event) => {
                  dragged.current = person.id
                  event.dataTransfer.setData(PERSON_DRAG, person.id)
                  event.dataTransfer.setData('text/plain', person.id)
                  event.dataTransfer.effectAllowed = 'move'
                }}
                onClick={() => {
                  if (dragged.current === person.id) {
                    dragged.current = ''
                    return
                  }
                  onPick(person.id)
                }}
              >
                <span className="people-avatar" aria-hidden>
                  {personInitials(person.name)}
                </span>
                <span className="people-row-copy">
                  <strong>{person.name}</strong>
                  <span>{professionLabel(person) || 'No profession'}</span>
                  <span>
                    {[person.age ?? '', person.meBond ? bondKindLabel(person.meBond.kind) : '', primaryContact(person)?.value, relLabels(person.companies, companyNames)]
                      .filter(Boolean)
                      .join(' · ')}
                  </span>
                </span>
              </article>
            ))}
          </section>
        )
      })}
    </div>
  )
}
