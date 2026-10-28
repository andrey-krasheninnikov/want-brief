import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import type { PersonRel } from '../../types'
import { RelationField } from './RelationField'
import './people.css'

type Props = {
  value: PersonRel[]
  onChange: (next: PersonRel[]) => void
  requireComment?: boolean
}

export function PeoplePicker({ value, onChange, requireComment = false }: Props) {
  const people = useQuery({ queryKey: ['people'], queryFn: api.people })
  return (
    <RelationField
      label="People"
      options={(people.data ?? []).map((person) => ({ id: person.id, name: person.name }))}
      value={value}
      onChange={onChange}
      requireComment={requireComment}
    />
  )
}
