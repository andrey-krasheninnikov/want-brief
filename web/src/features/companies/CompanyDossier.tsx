import { DotsThree } from '@phosphor-icons/react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api'
import { OpenUrl } from '../../shared/UrlField'
import { openTask } from '../../shared/taskOverlay'
import type { Company, Person, ProjectLink } from '../../types'
import { PeoplePicker } from '../people/PeoplePicker'
import { RelationField } from '../people/RelationField'
import { companyInitials } from './companiesModel'
import { CompanyCareer } from './CompanyCareer'
import { CompanyLog } from './CompanyLog'
import { CompanyNotes } from './CompanyNotes'
import { CompanyOrg } from './CompanyOrg'
import { CompanyOverview } from './CompanyOverview'

type Props = {
  companyId?: string
  onCreated: (id: string) => void
  onDeleted: () => void
  onClose?: () => void
}

type Draft = {
  name: string
  description: string
  links: ProjectLink[]
  projects: Company['projects']
  events: Company['events']
  people: Company['people']
  startedOn: string
  endedOn: string
}

const SECTIONS = [
  { id: 'company-sec-overview', label: 'Overview' },
  { id: 'company-sec-career', label: 'Career' },
  { id: 'company-sec-org', label: 'Org' },
  { id: 'company-sec-work', label: 'Work' },
  { id: 'company-sec-notes', label: 'Notes' },
  { id: 'company-sec-log', label: 'Log' },
] as const

function dateOnly(iso: string | null | undefined): string {
  if (!iso) return ''
  return iso.slice(0, 10)
}

function draftFrom(row?: Company): Draft {
  return {
    name: row?.name ?? '',
    description: row?.description ?? '',
    links: row?.links ?? [],
    projects: row?.projects ?? [],
    events: row?.events ?? [],
    people: row?.people ?? [],
    startedOn: dateOnly(row?.startedOn),
    endedOn: dateOnly(row?.endedOn),
  }
}

function payloadFrom(form: Draft) {
  return {
    name: form.name.trim(),
    description: form.description.trim(),
    links: form.links,
    projects: form.projects,
    events: form.events,
    people: form.people,
    startedOn: form.startedOn || null,
    endedOn: form.endedOn || null,
  }
}

export function CompanyDossier({ companyId, onCreated, onDeleted, onClose }: Props) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const company = useQuery({
    queryKey: ['company', companyId],
    queryFn: () => api.company(companyId!),
    enabled: Boolean(companyId),
  })
  const notes = useQuery({
    queryKey: ['company-notes', companyId],
    queryFn: () => api.companyNotes(companyId!),
    enabled: Boolean(companyId),
  })
  const people = useQuery({ queryKey: ['people'], queryFn: api.people })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const series = useQuery({ queryKey: ['event-series'], queryFn: api.eventSeries })
  const items = useQuery({ queryKey: ['items'], queryFn: () => api.items() })
  const [form, setForm] = useState<Draft>(draftFrom())
  const [error, setError] = useState('')
  const [waiting, setWaiting] = useState(false)
  const [menu, setMenu] = useState(false)
  const [linkLabel, setLinkLabel] = useState('')
  const [linkUrl, setLinkUrl] = useState('')
  const lastSaved = useRef('')
  const row = company.data

  useEffect(() => {
    if (!companyId) {
      setForm(draftFrom())
      lastSaved.current = ''
      return
    }
    if (!row || row.id !== companyId) return
    const next = draftFrom(row)
    setForm(next)
    lastSaved.current = JSON.stringify(payloadFrom(next))
  }, [companyId, row?.id, row?.updatedAt])

  const save = useMutation({
    mutationFn: (body: ReturnType<typeof payloadFrom>) =>
      companyId ? api.patchCompany(companyId, body) : api.createCompany(body),
    onSuccess: (next) => {
      setError('')
      setWaiting(false)
      lastSaved.current = JSON.stringify(payloadFrom(draftFrom(next)))
      void queryClient.invalidateQueries({ queryKey: ['companies'] })
      void queryClient.invalidateQueries({ queryKey: ['company'] })
      void queryClient.invalidateQueries({ queryKey: ['people'] })
      void queryClient.invalidateQueries({ queryKey: ['person'] })
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
      if (!companyId) onCreated(next.id)
    },
    onError: (err) => {
      setWaiting(false)
      setError(err instanceof Error ? err.message : 'Could not save.')
    },
  })
  const remove = useMutation({
    mutationFn: () => api.deleteCompany(companyId!),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['companies'] })
      void queryClient.invalidateQueries({ queryKey: ['company'] })
      void queryClient.invalidateQueries({ queryKey: ['people'] })
      void queryClient.invalidateQueries({ queryKey: ['person'] })
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
      onDeleted()
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not delete.'),
  })

  useEffect(() => {
    if (!companyId || row?.id !== companyId) return
    const body = payloadFrom(form)
    if (!body.name) return
    if (form.projects.some((rel) => !rel.comment.trim()) || form.events.some((rel) => !rel.comment.trim())) {
      setWaiting(false)
      setError('Comment is required for each project and event.')
      return
    }
    if (form.endedOn && !form.startedOn) {
      setWaiting(false)
      setError('Start date is required when an end date is set.')
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
  }, [form, companyId, row?.id])

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

  function addLink() {
    const url = linkUrl.trim()
    if (!url) {
      setError('URL is required.')
      return
    }
    setForm({ ...form, links: [...form.links, { label: linkLabel.trim(), url }] })
    setLinkLabel('')
    setLinkUrl('')
    setError('')
  }

  const title = form.name.trim() || 'New company'
  const peopleRows: Person[] = people.data ?? []
  const projectIds = useMemo(() => new Set(form.projects.map((rel) => rel.id)), [form.projects])
  const linkedTasks = useMemo(
    () => (items.data ?? []).filter((item) => item.projectId && projectIds.has(item.projectId)),
    [items.data, projectIds],
  )

  if (companyId && company.isError) {
    return <p className="error">{company.error.message}</p>
  }

  return (
    <form className="people-form" onSubmit={onCreate}>
      <div className="people-chrome">
        {onClose ? (
          <button type="button" className="ghost people-close" onClick={onClose}>
            Back
          </button>
        ) : null}
        <header className="people-detail-head">
          <span className="people-avatar" aria-hidden>
            {companyInitials(title)}
          </span>
          <div className="people-detail-title">
            <label className="people-name-field">
              Name
              <input id="company-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} autoComplete="off" />
            </label>
            <p className="people-save-state">{companyId ? (waiting ? 'Saving' : 'Saved') : 'New'}</p>
          </div>
          <div className="people-overflow">
            {companyId ? (
              <button type="button" className="ghost rel-edit" aria-label="More" onClick={() => setMenu((open) => !open)}>
                <DotsThree size={18} weight="bold" />
              </button>
            ) : null}
            {menu ? (
              <div className="people-menu">
                <button
                  type="button"
                  className="ghost"
                  disabled={remove.isPending}
                  onClick={() => {
                    setMenu(false)
                    if (window.confirm('Delete this company?')) remove.mutate()
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
      {error ? <p className="error">{error}</p> : null}
      {!companyId ? (
        <button type="submit" disabled={save.isPending || !form.name.trim()}>
          Create
        </button>
      ) : null}
      <label>
        Description
        <textarea rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
      </label>
      {row ? <CompanyOverview company={{ ...row, name: form.name }} people={peopleRows} /> : null}
      {row ? (
        <CompanyCareer
          company={row}
          startedOn={form.startedOn}
          endedOn={form.endedOn}
          onTenure={(startedOn, endedOn) => setForm({ ...form, startedOn, endedOn })}
        />
      ) : null}
      {row ? <CompanyOrg company={row} people={peopleRows} /> : null}
      <section id="company-sec-work" className="people-section">
        <p className="people-kicker">Work</p>
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
        <div className="people-block">
          <p className="people-block-label">Tasks</p>
          {linkedTasks.length === 0 ? <p className="muted">Tasks follow linked projects.</p> : null}
          {linkedTasks.length > 0 ? (
            <div className="companies-tasks">
              {linkedTasks.map((item) => (
                <button key={item.id} type="button" className="ghost" onClick={() => openTask(navigate, item.id)}>
                  {item.title}
                </button>
              ))}
            </div>
          ) : null}
        </div>
        <PeoplePicker value={form.people} onChange={(next) => setForm({ ...form, people: next })} />
        <p className="people-kicker">Links</p>
        <div className="companies-links">
          {form.links.map((link, index) => (
            <div key={`${link.url}-${index}`} className="companies-link">
              <a href={link.url} target="_blank" rel="noreferrer">
                {link.label || link.url}
              </a>
              <span className="companies-actions">
                <OpenUrl href={link.url} />
                <button
                  type="button"
                  className="ghost"
                  onClick={() => setForm({ ...form, links: form.links.filter((_, i) => i !== index) })}
                >
                  Remove
                </button>
              </span>
            </div>
          ))}
        </div>
        <div className="people-pair">
          <label>
            Label
            <input value={linkLabel} onChange={(e) => setLinkLabel(e.target.value)} autoComplete="off" />
          </label>
          <label>
            URL
            <input value={linkUrl} onChange={(e) => setLinkUrl(e.target.value)} autoComplete="off" />
          </label>
        </div>
        <button type="button" className="ghost" onClick={addLink}>
          Add link
        </button>
      </section>
      {row ? <CompanyNotes companyId={row.id} notes={notes.data ?? []} /> : null}
      {row ? <CompanyLog company={row} people={peopleRows} /> : null}
    </form>
  )
}
