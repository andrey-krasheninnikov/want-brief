import { useState } from 'react'
import { PencilSimple } from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import type { ContactKind, PersonContact } from '../../types'
import { TaskSheet } from '../tasks/TaskSheet'
import { PersonRail, PersonRailAdd } from './PersonRail'
import { copyText, primaryContact } from './peopleModel'

const KINDS: { id: ContactKind; label: string }[] = [
  { id: 'phone', label: 'Phone' },
  { id: 'telegram', label: 'Telegram' },
  { id: 'url', label: 'URL' },
]

type Draft = { kind: ContactKind; label: string; value: string; note: string }
const empty: Draft = { kind: 'phone', label: '', value: '', note: '' }

type Props = {
  personId: string
  contacts: PersonContact[]
}

export function PersonContacts({ personId, contacts }: Props) {
  const queryClient = useQueryClient()
  const [sheet, setSheet] = useState<'new' | string | null>(null)
  const [draft, setDraft] = useState<Draft>(empty)
  const [copiedId, setCopiedId] = useState('')
  const editing = contacts.find((row) => row.id === sheet)
  const primary = primaryContact({ contacts })

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['person', personId] })
    void queryClient.invalidateQueries({ queryKey: ['people'] })
  }

  const save = useMutation({
    mutationFn: () =>
      editing ? api.patchPersonContact(personId, editing.id, draft) : api.createPersonContact(personId, draft),
    onSuccess: () => {
      setSheet(null)
      setDraft(empty)
      refresh()
    },
  })
  const drop = useMutation({
    mutationFn: (contactId: string) => api.deletePersonContact(personId, contactId),
    onSuccess: () => {
      setSheet(null)
      refresh()
    },
  })

  async function onCopy(contact: PersonContact) {
    if (!(await copyText(contact.value))) return
    setCopiedId(contact.id)
    window.setTimeout(() => setCopiedId((id) => (id === contact.id ? '' : id)), 1200)
  }

  return (
    <section id="people-sec-contacts" className="people-section">
      <p className="people-kicker">Contacts</p>
      <PersonRail>
        {contacts.map((contact) => (
          <article key={contact.id} className={primary?.id === contact.id ? 'people-rail-card primary' : 'people-rail-card'}>
            <p className="people-kicker">{KINDS.find((row) => row.id === contact.kind)?.label ?? contact.kind}</p>
            <strong>{contact.label || '—'}</strong>
            <button type="button" className="people-contact-value" disabled={!contact.value} onClick={() => void onCopy(contact)}>
              {copiedId === contact.id ? 'Copied' : contact.value || '—'}
            </button>
            {contact.note ? <p className="people-rail-meta">{contact.note}</p> : null}
            <button
              type="button"
              className="ghost rel-edit"
              aria-label="Edit contact"
              onClick={() => {
                setDraft({ kind: contact.kind, label: contact.label, value: contact.value, note: contact.note })
                setSheet(contact.id)
              }}
            >
              <PencilSimple size={16} weight="light" />
            </button>
          </article>
        ))}
        <PersonRailAdd
          label="Add contact"
          onClick={() => {
            setDraft(empty)
            setSheet('new')
          }}
        />
      </PersonRail>
      <TaskSheet open={sheet !== null} kicker="Contact" title={editing ? editing.label : 'New contact'} onClose={() => setSheet(null)}>
        <label>
          Type
          <select value={draft.kind} onChange={(e) => setDraft({ ...draft, kind: e.target.value as ContactKind })}>
            {KINDS.map((kind) => (
              <option key={kind.id} value={kind.id}>
                {kind.label}
              </option>
            ))}
          </select>
        </label>
        <label>
          Name
          <input value={draft.label} onChange={(e) => setDraft({ ...draft, label: e.target.value })} placeholder="Work" />
        </label>
        <label>
          Value
          <input value={draft.value} onChange={(e) => setDraft({ ...draft, value: e.target.value })} />
        </label>
        <label>
          Note
          <input value={draft.note} onChange={(e) => setDraft({ ...draft, note: e.target.value })} />
        </label>
        <div className="people-contact-actions">
          <button type="button" onClick={() => save.mutate()} disabled={!draft.label.trim() || !draft.value.trim() || save.isPending}>
            Save
          </button>
          {editing ? (
            <button type="button" className="ghost" onClick={() => drop.mutate(editing.id)} disabled={drop.isPending}>
              Remove
            </button>
          ) : null}
        </div>
        {save.isError ? <p className="error">{save.error.message}</p> : null}
      </TaskSheet>
    </section>
  )
}
