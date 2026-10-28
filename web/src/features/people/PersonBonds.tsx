import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { moscowYmd } from '../../shared/moscow'
import type { BondKind, Person, PersonBond } from '../../types'
import { PersonBondGraph } from './PersonBondGraph'
import { BOND_KINDS, bondKindLabel } from './peopleModel'

function dateOnly(iso: string | null): string {
  if (!iso) return ''
  return iso.slice(0, 10)
}

function kindLabel(kind: BondKind): string {
  return bondKindLabel(kind)
}

type Props = {
  person: Person
}

export function PersonBonds({ person }: Props) {
  const personId = person.id
  const bonds = person.bonds ?? []
  const queryClient = useQueryClient()
  const people = useQuery({ queryKey: ['people'], queryFn: api.people })
  const [adding, setAdding] = useState(false)
  const [showEnded, setShowEnded] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [endingId, setEndingId] = useState<string | null>(null)
  const [draft, setDraft] = useState({
    otherId: '',
    kind: 'acquaintance' as BondKind,
    comment: '',
    startedOn: moscowYmd(),
    changedOn: moscowYmd(),
    actionComment: '',
    endedOn: moscowYmd(),
  })

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['person', personId] })
    void queryClient.invalidateQueries({ queryKey: ['people'] })
  }

  const open = useMutation({
    mutationFn: () =>
      api.createPersonBond(personId, {
        otherId: draft.otherId || null,
        kind: draft.kind,
        comment: draft.comment,
        startedOn: draft.startedOn || null,
      }),
    onSuccess: () => {
      setAdding(false)
      refresh()
    },
  })
  const change = useMutation({
    mutationFn: () =>
      api.patchPersonBond(personId, editingId!, {
        kind: draft.kind,
        comment: draft.comment,
        startedOn: draft.startedOn || null,
        changedOn: draft.changedOn || null,
        actionComment: draft.actionComment,
      }),
    onSuccess: () => {
      setEditingId(null)
      refresh()
    },
  })
  const end = useMutation({
    mutationFn: () =>
      api.endPersonBond(personId, endingId!, {
        endedOn: draft.endedOn || null,
        comment: draft.actionComment,
      }),
    onSuccess: () => {
      setEndingId(null)
      refresh()
    },
  })

  const openBonds = bonds.filter((bond) => !bond.endedOn)
  const endedBonds = bonds.filter((bond) => bond.endedOn)
  const others = (people.data ?? []).filter((row) => row.id !== personId)

  return (
    <section id="people-sec-relations" className="people-section">
      <p className="people-kicker">Relations</p>
      <PersonBondGraph person={person} bonds={bonds} />
      {openBonds.length === 0 ? <p className="people-empty">No open relations.</p> : null}
      {openBonds.map((bond) => (
        <BondCard
          key={bond.id}
          bond={bond}
          editing={editingId === bond.id}
          ending={endingId === bond.id}
          draft={draft}
          setDraft={setDraft}
          onEdit={() => {
            setEndingId(null)
            setAdding(false)
            setEditingId(bond.id)
            setDraft({
              ...draft,
              kind: bond.kind,
              comment: bond.comment,
              startedOn: dateOnly(bond.startedOn),
              changedOn: moscowYmd(),
              actionComment: '',
              otherId: bond.otherId ?? '',
            })
          }}
          onEnd={() => {
            setEditingId(null)
            setAdding(false)
            setEndingId(bond.id)
            setDraft({ ...draft, endedOn: moscowYmd(), actionComment: '' })
          }}
          onCancel={() => {
            setEditingId(null)
            setEndingId(null)
          }}
          onSaveChange={() => change.mutate()}
          onSaveEnd={() => end.mutate()}
          pending={change.isPending || end.isPending}
        />
      ))}
      {endedBonds.length > 0 ? (
        <button type="button" className="ghost" onClick={() => setShowEnded((open) => !open)}>
          {showEnded ? 'Hide ended' : `Ended (${endedBonds.length})`}
        </button>
      ) : null}
      {showEnded
        ? endedBonds.map((bond) => <BondCard key={bond.id} bond={bond} draft={draft} setDraft={setDraft} />)
        : null}
      {adding ? (
        <div className="people-bond">
          <label>
            With
            <select value={draft.otherId} onChange={(e) => setDraft({ ...draft, otherId: e.target.value })}>
              <option value="">Me</option>
              {others.map((row) => (
                <option key={row.id} value={row.id}>
                  {row.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Kind
            <select value={draft.kind} onChange={(e) => setDraft({ ...draft, kind: e.target.value as BondKind })}>
              {BOND_KINDS.map((kind) => (
                <option key={kind.id} value={kind.id}>
                  {kind.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            Started
            <DateField mode="date" value={draft.startedOn} onChange={(startedOn) => setDraft({ ...draft, startedOn })} />
          </label>
          <label>
            Comment
            <input value={draft.comment} onChange={(e) => setDraft({ ...draft, comment: e.target.value })} />
          </label>
          <div className="people-contact-actions">
            <button type="button" className="ghost" onClick={() => open.mutate()} disabled={open.isPending}>
              Add
            </button>
            <button type="button" className="ghost" onClick={() => setAdding(false)}>
              Cancel
            </button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          className="ghost"
          onClick={() => {
            setEditingId(null)
            setEndingId(null)
            setDraft({
              otherId: '',
              kind: 'acquaintance',
              comment: '',
              startedOn: moscowYmd(),
              changedOn: moscowYmd(),
              actionComment: '',
              endedOn: moscowYmd(),
            })
            setAdding(true)
          }}
        >
          Add relation
        </button>
      )}
      {open.isError ? <p className="error">{open.error.message}</p> : null}
      {change.isError ? <p className="error">{change.error.message}</p> : null}
      {end.isError ? <p className="error">{end.error.message}</p> : null}
    </section>
  )
}

type CardProps = {
  bond: PersonBond
  editing?: boolean
  ending?: boolean
  draft: {
    kind: BondKind
    comment: string
    startedOn: string
    changedOn: string
    actionComment: string
    endedOn: string
  }
  setDraft: (next: CardProps['draft'] & { otherId: string }) => void
  onEdit?: () => void
  onEnd?: () => void
  onCancel?: () => void
  onSaveChange?: () => void
  onSaveEnd?: () => void
  pending?: boolean
}

function BondCard({
  bond,
  editing,
  ending,
  draft,
  setDraft,
  onEdit,
  onEnd,
  onCancel,
  onSaveChange,
  onSaveEnd,
  pending,
}: CardProps) {
  return (
    <article className="people-bond">
      <p className="people-kicker">
        {kindLabel(bond.kind)} · {bond.otherName || 'Me'}
        {bond.endedOn ? ' · ended' : ''}
      </p>
      {bond.comment ? <p>{bond.comment}</p> : null}
      <p className="people-contact-note">
        {dateOnly(bond.startedOn)}
        {bond.changedOn ? ` · changed ${dateOnly(bond.changedOn)}` : ''}
        {bond.endedOn ? ` · ended ${dateOnly(bond.endedOn)}` : ''}
      </p>
      {editing ? (
        <>
          <label>
            Kind
            <select
              value={draft.kind}
              onChange={(e) => setDraft({ ...draft, otherId: '', kind: e.target.value as BondKind })}
            >
              {BOND_KINDS.map((kind) => (
                <option key={kind.id} value={kind.id}>
                  {kind.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            Comment
            <input value={draft.comment} onChange={(e) => setDraft({ ...draft, otherId: '', comment: e.target.value })} />
          </label>
          <label>
            Changed
            <DateField
              mode="date"
              value={draft.changedOn}
              onChange={(changedOn) => setDraft({ ...draft, otherId: '', changedOn })}
            />
          </label>
          <label>
            Note
            <input
              value={draft.actionComment}
              onChange={(e) => setDraft({ ...draft, otherId: '', actionComment: e.target.value })}
            />
          </label>
          <div className="people-contact-actions">
            <button type="button" className="ghost" onClick={onSaveChange} disabled={pending}>
              Save
            </button>
            <button type="button" className="ghost" onClick={onCancel}>
              Cancel
            </button>
          </div>
        </>
      ) : null}
      {ending ? (
        <>
          <label>
            Ended
            <DateField mode="date" value={draft.endedOn} onChange={(endedOn) => setDraft({ ...draft, otherId: '', endedOn })} />
          </label>
          <label>
            Note
            <input
              value={draft.actionComment}
              onChange={(e) => setDraft({ ...draft, otherId: '', actionComment: e.target.value })}
            />
          </label>
          <div className="people-contact-actions">
            <button type="button" className="ghost" onClick={onSaveEnd} disabled={pending}>
              End
            </button>
            <button type="button" className="ghost" onClick={onCancel}>
              Cancel
            </button>
          </div>
        </>
      ) : null}
      {!bond.endedOn && !editing && !ending ? (
        <div className="people-contact-actions">
          <button type="button" className="ghost" onClick={onEdit}>
            Change
          </button>
          <button type="button" className="ghost" onClick={onEnd}>
            End
          </button>
        </div>
      ) : null}
    </article>
  )
}
