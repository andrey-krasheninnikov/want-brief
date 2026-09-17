import { DotsThree } from '@phosphor-icons/react'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { moscowYmd } from '../../shared/moscow'
import type { Person } from '../../types'
import { personInitials } from './PersonCard'
import { PersonBonds } from './PersonBonds'
import { PersonAbsences } from './PersonAbsences'
import { PersonContacts } from './PersonContacts'
import { PersonLog } from './PersonLog'
import { PersonNotes } from './PersonNotes'
import { PersonOverview } from './PersonOverview'
import { PersonProfessions } from './PersonProfessions'
import { PersonSites } from './PersonSites'
import { idsToRels, relIds, RelationField } from './RelationField'

type Props = {
  personId?: string
  onCreated: (id: string) => void
  onDeleted: () => void
  onClose?: () => void
}

type Draft = {
  name: string
  bornOn: string
  ageYears: string
  projects: Person['projects']
  events: Person['events']
  companies: Person['companies']
  items: Person['projects']
}

const SECTIONS = [
  { id: 'people-sec-overview', label: 'Overview' },
  { id: 'people-sec-professions', label: 'Professions' },
  { id: 'people-sec-contacts', label: 'Contacts' },
  { id: 'people-sec-absences', label: 'Away' },
  { id: 'people-sec-notes', label: 'Notes' },
  { id: 'people-sec-relations', label: 'Relations' },
  { id: 'people-sec-work', label: 'Work' },
  { id: 'people-sec-log', label: 'Log' },
] as const

function dateOnly(iso: string | null): string {
  if (!iso) return ''
  return iso.slice(0, 10)
}

function presumeBornOn(age: number): string {
  const [year, month, day] = moscowYmd().split('-').map(Number)
  return new Date(Date.UTC(year - age, month - 1, day)).toISOString().slice(0, 10)
}

function ageFromBornOn(bornOn: string): number {
  const [ty, tm, td] = moscowYmd().split('-').map(Number)
  const [by, bm, bd] = bornOn.split('-').map(Number)
  let years = ty - by
  if (tm < bm || (tm === bm && td < bd)) years -= 1
  return Math.max(0, years)
}

function draftFrom(row?: Person): Draft {
  return {
    name: row?.name ?? '',
    bornOn: dateOnly(row?.bornOn ?? null),
    ageYears: row?.age != null ? String(row.age) : '',
    projects: row?.projects ?? [],
    events: row?.events ?? [],
    companies: row?.companies ?? [],
    items: idsToRels(row?.itemIds ?? []),
  }
}

function payloadFrom(form: Draft) {
  const bornOn = form.bornOn.trim()
  const years = form.ageYears.trim()
  return {
    name: form.name.trim(),
    bornOn: bornOn || null,
    ageYears: !bornOn && years !== '' ? Number(years) : null,
    projects: form.projects,
    events: form.events,
    companies: form.companies,
    itemIds: relIds(form.items),
  }
}

export function PersonDossier({ personId, onCreated, onDeleted, onClose }: Props) {
  const queryClient = useQueryClient()
  const person = useQuery({
    queryKey: ['person', personId],
    queryFn: () => api.person(personId!),
    enabled: Boolean(personId),
  })
  const notes = useQuery({
    queryKey: ['person-notes', personId],
    queryFn: () => api.personNotes(personId!),
    enabled: Boolean(personId),
  })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const companies = useQuery({ queryKey: ['companies'], queryFn: api.companies })
  const series = useQuery({ queryKey: ['event-series'], queryFn: api.eventSeries })
  const items = useQuery({ queryKey: ['items'], queryFn: () => api.items({ includeArchived: true }) })
  const [form, setForm] = useState(() => draftFrom())
  const [error, setError] = useState('')
  const [menu, setMenu] = useState(false)
  const [waiting, setWaiting] = useState(false)
  const lastSaved = useRef('')

  useEffect(() => {
    if (!personId) {
      setForm(draftFrom())
      lastSaved.current = ''
      return
    }
    if (person.data?.id === personId) {
      const next = draftFrom(person.data)
      setForm(next)
      lastSaved.current = JSON.stringify(payloadFrom(next))
    }
  }, [personId, person.data?.id])

  useEffect(() => {
    if (!onClose) return
    const close = onClose
    function onKey(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      event.preventDefault()
      close()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const save = useMutation({
    mutationFn: (body: ReturnType<typeof payloadFrom>) =>
      personId ? api.patchPerson(personId, body) : api.createPerson(body),
    onSuccess: (row) => {
      setError('')
      setWaiting(false)
      lastSaved.current = JSON.stringify(payloadFrom(draftFrom(row)))
      void queryClient.invalidateQueries({ queryKey: ['people'] })
      void queryClient.invalidateQueries({ queryKey: ['person'] })
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
      void queryClient.invalidateQueries({ queryKey: ['events'] })
      void queryClient.invalidateQueries({ queryKey: ['event'] })
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      void queryClient.invalidateQueries({ queryKey: ['companies'] })
      void queryClient.invalidateQueries({ queryKey: ['company'] })
      if (!personId) onCreated(row.id)
    },
    onError: (err) => {
      setWaiting(false)
      setError(err instanceof Error ? err.message : 'Could not save.')
    },
  })
  const remove = useMutation({
    mutationFn: () => api.deletePerson(personId!),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['people'] })
      void queryClient.invalidateQueries({ queryKey: ['person'] })
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
      void queryClient.invalidateQueries({ queryKey: ['events'] })
      void queryClient.invalidateQueries({ queryKey: ['event'] })
      void queryClient.invalidateQueries({ queryKey: ['event-series'] })
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      void queryClient.invalidateQueries({ queryKey: ['companies'] })
      void queryClient.invalidateQueries({ queryKey: ['company'] })
      onDeleted()
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not delete.'),
  })

  useEffect(() => {
    if (!personId || person.data?.id !== personId) return
    const body = payloadFrom(form)
    if (!body.name) return
    if (form.projects.some((rel) => !rel.comment.trim()) || form.events.some((rel) => !rel.comment.trim())) {
      setWaiting(false)
      setError('Comment is required for each project and event.')
      return
    }
    if (JSON.stringify(body) === lastSaved.current) {
      setWaiting(false)
      return
    }
    setError('')
    setWaiting(true)
    const timer = window.setTimeout(() => save.mutate(body), 400)
    return () => window.clearTimeout(timer)
  }, [form, personId, person.data?.id])

  function onCreate(event: FormEvent) {
    event.preventDefault()
    if (!form.name.trim()) {
      setError('Name is required.')
      return
    }
    save.mutate(payloadFrom(form))
  }

  function jump(id: string) {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  if (personId && person.isLoading) return <p className="muted">Loading…</p>
  if (personId && person.isError) return <p className="error">{person.error.message}</p>

  const row = person.data
  const title = form.name || row?.name || (personId ? 'Person' : 'New person')
  const saving = waiting || save.isPending

  if (!personId) {
    return (
      <form className="people-form" onSubmit={onCreate}>
        <header className="people-detail-head">
          {onClose ? (
            <button type="button" className="people-close" onClick={onClose}>
              Close
            </button>
          ) : null}
          <span className="people-avatar" aria-hidden>
            {personInitials(title)}
          </span>
          <div>
            <h2>New person</h2>
            <p>Create a record</p>
          </div>
        </header>
        <label>
          Name
          <input
            id="person-name"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            autoComplete="off"
            autoFocus
          />
        </label>
        {error ? <p className="error">{error}</p> : null}
        <button type="submit" disabled={save.isPending}>
          Create
        </button>
      </form>
    )
  }

  return (
    <div className="people-form">
      <div className="people-chrome">
      <header className="people-detail-head">
        {onClose ? (
          <button type="button" className="people-close" onClick={onClose}>
            Close
          </button>
        ) : null}
        <span className="people-avatar" aria-hidden>
          {personInitials(title)}
        </span>
        <div className="people-detail-title">
          <label className="people-name-field">
            Name
            <input
              id="person-name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              autoComplete="off"
            />
          </label>
          <p className="people-save-state">{saving ? 'Saving' : 'Saved'}</p>
        </div>
        <div className="people-overflow">
          <button type="button" className="ghost rel-edit" aria-label="More" onClick={() => setMenu((open) => !open)}>
            <DotsThree size={18} weight="bold" />
          </button>
          {menu ? (
            <div className="people-menu">
              <button
                type="button"
                className="ghost"
                disabled={remove.isPending}
                onClick={() => {
                  setMenu(false)
                  if (window.confirm('Delete this person?')) remove.mutate()
                }}
              >
                Delete
              </button>
            </div>
          ) : null}
        </div>
      </header>
      <nav className="people-sec-nav" aria-label="Sections">
        {SECTIONS.map((section) => (
          <button key={section.id} type="button" className="ghost" onClick={() => jump(section.id)}>
            {section.label}
          </button>
        ))}
      </nav>
      </div>
      {row ? <PersonOverview person={{ ...row, name: form.name }} /> : null}
      {row ? <PersonProfessions personId={personId} professions={row.professions ?? []} /> : null}
      {row ? <PersonContacts personId={personId} contacts={row.contacts ?? []} /> : null}
      {row ? <PersonSites personId={personId} sites={row.sites ?? []} /> : null}
      {row ? <PersonAbsences personId={personId} absences={row.absences ?? []} /> : null}
      {row ? <PersonNotes personId={personId} notes={notes.data ?? []} /> : null}
      {row ? <PersonBonds person={row} /> : null}
      <section id="people-sec-work" className="people-section">
        <p className="people-kicker">Work</p>
        <RelationField
          label="Companies"
          options={(companies.data ?? []).map((item) => ({ id: item.id, name: item.name }))}
          value={form.companies}
          onChange={(next) => setForm({ ...form, companies: next })}
        />
        <RelationField
          label="Projects"
          options={(projects.data ?? []).map((item) => ({ id: item.id, name: item.name }))}
          value={form.projects}
          onChange={(next) => setForm({ ...form, projects: next })}
          requireComment
        />
        <RelationField
          label="Events"
          options={(series.data ?? []).map((item) => ({ id: item.id, name: item.title }))}
          value={form.events}
          onChange={(next) => setForm({ ...form, events: next })}
          requireComment
        />
        <RelationField
          label="Tasks"
          options={(items.data ?? []).map((item) => ({ id: item.id, name: item.title }))}
          value={form.items}
          onChange={(next) => setForm({ ...form, items: next })}
        />
      </section>
      <PersonLog bonds={row?.bonds ?? []} />
      <section className="people-section">
        <p className="people-kicker">Born</p>
        <div className="people-pair">
          <label>
            Born
            <DateField
              mode="date"
              value={form.bornOn}
              onChange={(next) => setForm({ ...form, bornOn: next, ageYears: next ? String(ageFromBornOn(next)) : '' })}
            />
          </label>
          <label>
            Age
            <input
              type="number"
              min={0}
              max={150}
              value={form.ageYears}
              onChange={(e) => {
                const raw = e.target.value
                const years = Number(raw)
                const bornOn = raw !== '' && Number.isFinite(years) && years >= 0 && years <= 150 ? presumeBornOn(years) : ''
                setForm({ ...form, ageYears: raw, bornOn })
              }}
            />
          </label>
        </div>
      </section>
      {error ? <p className="error">{error}</p> : null}
    </div>
  )
}
