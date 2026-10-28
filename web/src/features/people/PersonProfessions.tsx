import { useState } from 'react'
import { PencilSimple } from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { moscowYmd } from '../../shared/moscow'
import { TaskSheet } from '../tasks/TaskSheet'
import type { PersonProfession, SalaryPeriod } from '../../types'
import { PersonRail, PersonRailAdd } from './PersonRail'
import { currentSalary, dateOnly, dateRange, salaryText } from './peopleModel'

type Draft = {
  title: string
  comment: string
  startedOn: string
  endedOn: string
  monthlySalaryUsd: string
  monthlySalaryRub: string
}

const empty: Draft = {
  title: '',
  comment: '',
  startedOn: moscowYmd(),
  endedOn: '',
  monthlySalaryUsd: '0',
  monthlySalaryRub: '0',
}

function fromProfession(row: PersonProfession): Draft {
  const current = currentSalary(row)
  return {
    title: row.title,
    comment: row.comment,
    startedOn: dateOnly(row.startedOn) || moscowYmd(),
    endedOn: dateOnly(row.endedOn),
    monthlySalaryUsd: String(current?.monthlySalaryUsd ?? 0),
    monthlySalaryRub: String(current?.monthlySalaryRub ?? 0),
  }
}

function payload(draft: Draft) {
  return {
    title: draft.title.trim(),
    comment: draft.comment,
    startedOn: draft.startedOn || null,
    endedOn: draft.endedOn || null,
    monthlySalaryUsd: Number(draft.monthlySalaryUsd) || 0,
    monthlySalaryRub: Number(draft.monthlySalaryRub) || 0,
  }
}

function closedPeriods(row: PersonProfession): SalaryPeriod[] {
  return (row.salaries ?? []).filter((period) => period.endedOn)
}

type Props = {
  personId: string
  professions: PersonProfession[]
}

export function PersonProfessions({ personId, professions }: Props) {
  const queryClient = useQueryClient()
  const [sheet, setSheet] = useState<'new' | string | null>(null)
  const [draft, setDraft] = useState<Draft>(empty)
  const editing = professions.find((row) => row.id === sheet)

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['person', personId] })
    void queryClient.invalidateQueries({ queryKey: ['people'] })
  }

  const save = useMutation({
    mutationFn: () =>
      editing ? api.patchPersonProfession(personId, editing.id, payload(draft)) : api.createPersonProfession(personId, payload(draft)),
    onSuccess: () => {
      setSheet(null)
      setDraft(empty)
      refresh()
    },
  })
  const drop = useMutation({
    mutationFn: (professionId: string) => api.deletePersonProfession(personId, professionId),
    onSuccess: () => {
      setSheet(null)
      refresh()
    },
  })

  function openNew() {
    setDraft({ ...empty, startedOn: moscowYmd() })
    setSheet('new')
  }

  function openEdit(row: PersonProfession) {
    setDraft(fromProfession(row))
    setSheet(row.id)
  }

  function onSave() {
    if (editing && !window.confirm('Update this profession?')) return
    save.mutate()
  }

  return (
    <section id="people-sec-professions" className="people-section">
      <p className="people-kicker">Professions</p>
      <PersonRail>
        {professions.map((row) => {
          const pay = salaryText(currentSalary(row))
          return (
            <article key={row.id} className="people-rail-card">
              <p className="people-kicker">{row.endedOn ? 'Inactive' : 'Active'}</p>
              <strong>{row.title}</strong>
              {pay ? <p>{pay}</p> : null}
              <p className="people-rail-meta">{dateRange(row.startedOn, row.endedOn)}</p>
              <button type="button" className="ghost rel-edit" aria-label="Edit profession" onClick={() => openEdit(row)}>
                <PencilSimple size={16} weight="light" />
              </button>
            </article>
          )
        })}
        <PersonRailAdd label="Add profession" onClick={openNew} />
      </PersonRail>
      <TaskSheet
        open={sheet !== null}
        kicker="Profession"
        title={editing ? editing.title : 'New profession'}
        onClose={() => setSheet(null)}
      >
        <label>
          Title
          <input value={draft.title} onChange={(e) => setDraft({ ...draft, title: e.target.value })} autoFocus />
        </label>
        <label>
          Comment
          <input value={draft.comment} onChange={(e) => setDraft({ ...draft, comment: e.target.value })} />
        </label>
        <div className="people-pair">
          <label>
            Started
            <DateField mode="date" value={draft.startedOn} onChange={(startedOn) => setDraft({ ...draft, startedOn })} />
          </label>
          <label>
            Ended
            <DateField mode="date" value={draft.endedOn} onChange={(endedOn) => setDraft({ ...draft, endedOn })} />
          </label>
        </div>
        <div className="people-pair">
          <label>
            Salary USD
            <input
              type="number"
              min={0}
              step={0.01}
              value={draft.monthlySalaryUsd}
              onChange={(e) => setDraft({ ...draft, monthlySalaryUsd: e.target.value })}
            />
          </label>
          <label>
            Salary RUB
            <input
              type="number"
              min={0}
              step={0.01}
              value={draft.monthlySalaryRub}
              onChange={(e) => setDraft({ ...draft, monthlySalaryRub: e.target.value })}
            />
          </label>
        </div>
        {editing && closedPeriods(editing).length > 0 ? (
          <ul className="people-salary-log">
            {closedPeriods(editing).map((period) => (
              <li key={period.id}>
                {dateRange(period.startedOn, period.endedOn)} · {salaryText(period) || '0'}
              </li>
            ))}
          </ul>
        ) : null}
        <div className="people-contact-actions">
          <button type="button" onClick={onSave} disabled={!draft.title.trim() || save.isPending}>
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
