import { Input } from '@want-foundation/desyst-react'
import { Button, Textarea } from '@want-foundation/desyst-react'
import { useState } from 'react'
import { PencilSimple } from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import type { AddressKind, PersonAddress } from '../../types'
import { TaskSheet } from '../tasks/TaskSheet'
import { PersonRail, PersonRailAdd } from './PersonRail'
import { copyText } from './peopleModel'

const KINDS: { id: AddressKind; label: string }[] = [
  { id: 'home', label: 'Home' },
  { id: 'work', label: 'Work' },
  { id: 'other', label: 'Other' },
]

type Draft = { kind: AddressKind; label: string; value: string; note: string }
const empty: Draft = { kind: 'home', label: '', value: '', note: '' }

type Props = {
  personId: string
  addresses: PersonAddress[]
}

export function PersonAddresses({ personId, addresses }: Props) {
  const queryClient = useQueryClient()
  const [sheet, setSheet] = useState<'new' | string | null>(null)
  const [draft, setDraft] = useState<Draft>(empty)
  const [copiedId, setCopiedId] = useState('')
  const editing = addresses.find((row) => row.id === sheet)

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['person', personId] })
    void queryClient.invalidateQueries({ queryKey: ['people'] })
  }

  const save = useMutation({
    mutationFn: () =>
      editing ? api.patchPersonAddress(personId, editing.id, draft) : api.createPersonAddress(personId, draft),
    onSuccess: () => {
      setSheet(null)
      setDraft(empty)
      refresh()
    },
  })
  const drop = useMutation({
    mutationFn: (addressId: string) => api.deletePersonAddress(personId, addressId),
    onSuccess: () => {
      setSheet(null)
      refresh()
    },
  })

  async function onCopy(address: PersonAddress) {
    if (!(await copyText(address.value))) return
    setCopiedId(address.id)
    window.setTimeout(() => setCopiedId((id) => (id === address.id ? '' : id)), 1200)
  }

  return (
    <section id="people-sec-addresses" className="people-section">
      <p className="people-kicker">Addresses</p>
      <PersonRail>
        {addresses.map((address) => (
          <article key={address.id} className="people-rail-card">
            <p className="people-kicker">{KINDS.find((row) => row.id === address.kind)?.label ?? address.kind}</p>
            <strong>{address.label || '—'}</strong>
            <Button type="button" className="people-contact-value" disabled={!address.value} onClick={() => void onCopy(address)}>
              {copiedId === address.id ? 'Copied' : address.value || '—'}
            </Button>
            {address.note ? <p className="people-rail-meta">{address.note}</p> : null}
            <Button
              type="button"
              className="ghost rel-edit"
              aria-label="Edit address"
              onClick={() => {
                setDraft({ kind: address.kind, label: address.label, value: address.value, note: address.note })
                setSheet(address.id)
              }}
            >
              <PencilSimple size={16} weight="light" />
            </Button>
          </article>
        ))}
        <PersonRailAdd
          label="Add address"
          onClick={() => {
            setDraft(empty)
            setSheet('new')
          }}
        />
      </PersonRail>
      <TaskSheet open={sheet !== null} kicker="Address" title={editing ? editing.label : 'New address'} onClose={() => setSheet(null)}>
        <label>
          Type
          <select value={draft.kind} onChange={(e) => setDraft({ ...draft, kind: e.target.value as AddressKind })}>
            {KINDS.map((kind) => (
              <option key={kind.id} value={kind.id}>
                {kind.label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Name
          <Input value={draft.label} onChange={(e) => setDraft({ ...draft, label: e.target.value })} placeholder="Home" />
        </label>
        <label>
          Value
          <Textarea value={draft.value} onChange={(e) => setDraft({ ...draft, value: e.target.value })} rows={3} />
        </label>
        <label>
          Note
          <Input value={draft.note} onChange={(e) => setDraft({ ...draft, note: e.target.value })} />
        </label>
        <div className="people-contact-actions">
          <Button type="button" onClick={() => save.mutate()} disabled={!draft.label.trim() || !draft.value.trim() || save.isPending}>
            Save
          </Button>
          {editing ? (
            <Button type="button" className="ghost" onClick={() => drop.mutate(editing.id)} disabled={drop.isPending}>
              Remove
            </Button>
          ) : null}
        </div>
        {save.isError ? <p className="error">{save.error.message}</p> : null}
      </TaskSheet>
    </section>
  )
}
