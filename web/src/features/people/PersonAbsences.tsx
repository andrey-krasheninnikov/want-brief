import { useState } from 'react'
import { PencilSimple } from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import type { PersonAbsence } from '../../types'
import { TaskSheet } from '../tasks/TaskSheet'
import { PersonRail, PersonRailAdd } from './PersonRail'

type Draft = { startsOn: string; endsOn: string; note: string }
const empty: Draft = { startsOn: '', endsOn: '', note: '' }

type Props = {
  personId: string
  absences: PersonAbsence[]
}

function dateOnly(iso: string): string {
  return iso.slice(0, 10)
}

function rangeLabel(row: PersonAbsence): string {
  const fmt = (iso: string) =>
    new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(iso))
  const start = dateOnly(row.startsOn)
  const end = dateOnly(row.endsOn)
  return start === end ? fmt(row.startsOn) : `${fmt(row.startsOn)} – ${fmt(row.endsOn)}`
}

function dayCount(row: PersonAbsence): number {
  const start = new Date(dateOnly(row.startsOn)).getTime()
  const end = new Date(dateOnly(row.endsOn)).getTime()
  return Math.round((end - start) / 86_400_000) + 1
}

function stateOf(row: PersonAbsence, today: string): 'past' | 'now' | 'ahead' {
  const start = dateOnly(row.startsOn)
  const end = dateOnly(row.endsOn)
  if (end < today) return 'past'
  if (start <= today) return 'now'
  return 'ahead'
}

// PersonAbsences lists when a person is away so follow-up pings avoid those days.
export function PersonAbsences({ personId, absences }: Props) {
  const queryClient = useQueryClient()
  const [sheet, setSheet] = useState<'new' | string | null>(null)
  const [draft, setDraft] = useState<Draft>(empty)
  const editing = absences.find((row) => row.id === sheet)
  const today = new Date().toISOString().slice(0, 10)
  const rows = [...absences].sort((a, b) => b.startsOn.localeCompare(a.startsOn))

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['person', personId] })
    void queryClient.invalidateQueries({ queryKey: ['people'] })
    void queryClient.invalidateQueries({ queryKey: ['schedule'] })
  }

  const save = useMutation({
    mutationFn: () => {
      const body = { startsOn: draft.startsOn, endsOn: draft.endsOn || draft.startsOn, note: draft.note }
      return editing ? api.updatePersonAbsence(personId, editing.id, body) : api.createPersonAbsence(personId, body)
    },
    onSuccess: () => {
      setSheet(null)
      setDraft(empty)
      refresh()
    },
  })
  const drop = useMutation({
    mutationFn: (absenceId: string) => api.deletePersonAbsence(personId, absenceId),
    onSuccess: () => {
      setSheet(null)
      refresh()
    },
  })

  const valid = Boolean(draft.startsOn) && (!draft.endsOn || draft.endsOn >= draft.startsOn)

  return (
    <section id="people-sec-absences" className="people-section">
      <p className="people-kicker">Away</p>
      <PersonRail>
        {rows.map((row) => {
          const state = stateOf(row, today)
          return (
            <article key={row.id} className={`people-rail-card absence ${state}`}>
              <p className="people-kicker">{state === 'now' ? 'Away now' : state === 'ahead' ? 'Upcoming' : 'Past'}</p>
              <strong>{rangeLabel(row)}</strong>
              <p className="people-rail-meta">
                {dayCount(row)} {dayCount(row) === 1 ? 'day' : 'days'}
                {row.note ? ` · ${row.note}` : ''}
              </p>
              <button
                type="button"
                className="ghost rel-edit"
                aria-label="Edit absence"
                onClick={() => {
                  setDraft({ startsOn: dateOnly(row.startsOn), endsOn: dateOnly(row.endsOn), note: row.note })
                  setSheet(row.id)
                }}
              >
                <PencilSimple size={16} weight="light" />
              </button>
            </article>
          )
        })}
        <PersonRailAdd
          label="Add absence"
          onClick={() => {
            setDraft(empty)
            setSheet('new')
          }}
        />
      </PersonRail>
      <TaskSheet open={sheet !== null} kicker="Away" title={editing ? rangeLabel(editing) : 'New absence'} onClose={() => setSheet(null)}>
        <label>
          From
          <DateField mode="date" value={draft.startsOn} onChange={(next) => setDraft({ ...draft, startsOn: next })} />
        </label>
        <label>
          To
          <DateField mode="date" value={draft.endsOn} onChange={(next) => setDraft({ ...draft, endsOn: next })} />
        </label>
        <label>
          Note
          <input value={draft.note} onChange={(e) => setDraft({ ...draft, note: e.target.value })} placeholder="Vacation, conference…" />
        </label>
        <div className="people-contact-actions">
          <button type="button" onClick={() => save.mutate()} disabled={!valid || save.isPending}>
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
