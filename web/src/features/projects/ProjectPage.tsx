import { useEffect, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Navigate, useParams } from 'react-router-dom'
import { api } from '../../api'
import { OpenUrl } from '../../shared/UrlField'
import { hourlyRate, hours, money } from '../../shared/format'
import { moscowRange } from '../../shared/moscow'
import type { Project, ProjectLink } from '../../types'
import { PeoplePicker } from '../people/PeoplePicker'
import { RelationField } from '../people/RelationField'
import { PlazaSheet } from './PlazaSheet'

function linkCaption(link: ProjectLink): string {
  if (link.label) return link.label
  try {
    return new URL(link.url).hostname
  } catch {
    return link.url
  }
}

function noteStamp(iso: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
}

const DEFAULT_COLOR = '#6152ed'

function normalizeColor(raw: string): string {
  const value = raw.trim()
  if (/^#[0-9A-Fa-f]{6}$/.test(value)) return value.toLowerCase()
  return DEFAULT_COLOR
}

function Field({
  label,
  value,
  onChange,
  onBlur,
  type = 'text',
}: {
  label: string
  value: string
  onChange: (value: string) => void
  onBlur: (value: string) => void
  type?: string
}) {
  return (
    <label>
      {label}
      <input
        type={type}
        value={value}
        min={type === 'number' ? 0 : undefined}
        step={type === 'number' ? 0.01 : undefined}
        onChange={(e) => onChange(e.target.value)}
        onBlur={(e) => onBlur(e.currentTarget.value)}
      />
    </label>
  )
}

export function ProjectPage() {
  const { id = '' } = useParams()
  const queryClient = useQueryClient()
  const project = useQuery({ queryKey: ['projects', id], queryFn: () => api.project(id), enabled: Boolean(id) })
  const companies = useQuery({ queryKey: ['companies'], queryFn: api.companies })
  const notes = useQuery({
    queryKey: ['project-notes', id],
    queryFn: () => api.projectNotes(id),
    enabled: Boolean(id),
  })
  const monthLoad = useQuery({
    queryKey: ['load', 'month'],
    queryFn: () => {
      const bounds = moscowRange('month')
      return api.load(bounds.from, bounds.to)
    },
  })
  const [name, setName] = useState('')
  const [dayHours, setDayHours] = useState('')
  const [usd, setUsd] = useState('')
  const [rub, setRub] = useState('')
  const [color, setColor] = useState(DEFAULT_COLOR)
  const [noteBody, setNoteBody] = useState('')
  const [linkLabel, setLinkLabel] = useState('')
  const [linkUrl, setLinkUrl] = useState('')
  const [linkOpen, setLinkOpen] = useState(false)
  const [noteOpen, setNoteOpen] = useState(false)
  const [formError, setFormError] = useState('')

  const row = project.data
  useEffect(() => {
    if (!row) return
    setName(row.name)
    setDayHours(String(row.targetHoursDay))
    setUsd(String(row.monthlyIncomeUsd))
    setRub(String(row.monthlyIncomeRub))
    setColor(normalizeColor(row.color))
  }, [row?.id, row?.updatedAt, row?.name, row?.targetHoursDay, row?.monthlyIncomeUsd, row?.monthlyIncomeRub, row?.color])

  const patch = useMutation({
    mutationFn: (body: Partial<Project> & { archived?: boolean }) => api.patchProject(id, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
      void queryClient.invalidateQueries({ queryKey: ['load'] })
      void queryClient.invalidateQueries({ queryKey: ['people'] })
      void queryClient.invalidateQueries({ queryKey: ['person'] })
      void queryClient.invalidateQueries({ queryKey: ['companies'] })
      void queryClient.invalidateQueries({ queryKey: ['company'] })
    },
    onError: (err) => setFormError(err instanceof Error ? err.message : 'Could not save.'),
  })
  const addNote = useMutation({
    mutationFn: () => api.createProjectNote(id, noteBody),
    onSuccess: () => {
      setNoteBody('')
      setFormError('')
      setNoteOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['project-notes', id] })
    },
    onError: (err) => setFormError(err instanceof Error ? err.message : 'Could not add note.'),
  })

  function saveName(raw: string) {
    const current = row?.name ?? ''
    const next = raw.trim()
    if (!next || next === current) {
      setName(current)
      return
    }
    patch.mutate({ name: next })
  }

  function saveNumber(field: 'targetHoursDay' | 'monthlyIncomeUsd' | 'monthlyIncomeRub', raw: string, current: number) {
    const next = Number(raw)
    if (!Number.isFinite(next) || next === current) return
    patch.mutate({ [field]: next })
  }

  function saveColor(raw: string) {
    const next = normalizeColor(raw)
    setColor(next)
    if (next === normalizeColor(row?.color ?? '')) return
    patch.mutate(
      { color: next },
      { onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['items'] }) },
    )
  }

  function onAddNote(event: FormEvent) {
    event.preventDefault()
    if (!noteBody.trim()) {
      setFormError('Note is required.')
      return
    }
    addNote.mutate()
  }

  function onAddLink(event: FormEvent) {
    event.preventDefault()
    const url = linkUrl.trim()
    if (!url) {
      setFormError('URL is required.')
      return
    }
    setFormError('')
    patch.mutate(
      { links: [...(row?.links ?? []), { label: linkLabel.trim(), url }] },
      {
        onSuccess: () => {
          setLinkLabel('')
          setLinkUrl('')
          setLinkOpen(false)
        },
      },
    )
  }

  if (project.isError) {
    return <Navigate to="/projects" replace />
  }
  if (!row) {
    return (
      <div className="plaza">
        <p className="muted">Loading…</p>
      </div>
    )
  }

  const monthSeconds = monthLoad.data?.byProject?.find((item) => item.projectId === row.id)?.allocatedSeconds ?? 0
  const links = row.links ?? []
  const log = notes.data ?? []
  const preview = log.slice(0, 10)

  return (
    <div className="plaza plaza-detail">
      <Link to="/projects" className="plaza-back">
        Projects
      </Link>
      <header className="plaza-hero">
        <div>
          <p className="plaza-kicker">{row.archivedAt ? 'Archived' : 'Dossier'}</p>
          <div className="plaza-title">
            <span className="plaza-swatch" style={{ background: color }} />
            <input
              aria-label="Project name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              onBlur={(event) => saveName(event.currentTarget.value)}
            />
          </div>
        </div>
        <button
          type="button"
          className="ghost plaza-archive"
          disabled={patch.isPending}
          onClick={() => patch.mutate({ archived: !row.archivedAt })}
        >
          {row.archivedAt ? 'Restore' : 'Archive'}
        </button>
      </header>
      {formError && !linkOpen && !noteOpen ? <p className="error">{formError}</p> : null}
      <section className="plaza-metrics">
        <article className="plaza-tile">
          <div className="plaza-core">
            <p className="plaza-kicker">This month</p>
            <p className="plaza-stat">
              <strong className="mono">{hours(monthSeconds)}</strong>
              <span>tracked</span>
            </p>
            <p className="plaza-stat">
              <strong className="mono">{hourlyRate(row.monthlyIncomeUsd, monthSeconds, 'en-US')}</strong>
              <span>USD / hour</span>
            </p>
            <p className="plaza-stat">
              <strong className="mono">{hourlyRate(row.monthlyIncomeRub, monthSeconds, 'ru-RU')}</strong>
              <span>RUB / hour</span>
            </p>
          </div>
        </article>
        <article className="plaza-tile">
          <div className="plaza-core plaza-form">
            <Field
              label="Hours / day"
              type="number"
              value={dayHours}
              onChange={setDayHours}
              onBlur={(value) => saveNumber('targetHoursDay', value, row.targetHoursDay)}
            />
            <Field
              label="Income USD"
              type="number"
              value={usd}
              onChange={setUsd}
              onBlur={(value) => saveNumber('monthlyIncomeUsd', value, row.monthlyIncomeUsd)}
            />
            <Field
              label="Income RUB"
              type="number"
              value={rub}
              onChange={setRub}
              onBlur={(value) => saveNumber('monthlyIncomeRub', value, row.monthlyIncomeRub)}
            />
            <label>
              Color
              <input
                className="plaza-color"
                type="color"
                value={color}
                onChange={(e) => {
                  setColor(e.target.value)
                  saveColor(e.target.value)
                }}
                onBlur={(e) => saveColor(e.currentTarget.value)}
              />
            </label>
            <p className="muted">
              {money(row.monthlyIncomeUsd, 'en-US')} USD · {money(row.monthlyIncomeRub, 'ru-RU')} RUB
            </p>
            <PeoplePicker
              value={row.people ?? []}
              onChange={(people) => patch.mutate({ people })}
              requireComment
            />
            <RelationField
              label="Companies"
              options={(companies.data ?? []).map((item) => ({ id: item.id, name: item.name }))}
              value={row.companies ?? []}
              onChange={(next) => patch.mutate({ companies: next })}
              requireComment
            />
          </div>
        </article>
      </section>
      <section className="plaza-tile">
        <div className="plaza-core">
          <div className="plaza-head">
            <h3>Links</h3>
            <button type="button" className="plaza-plus" aria-label="Add link" onClick={() => setLinkOpen(true)}>
              +
            </button>
          </div>
          {links.length === 0 ? <p className="muted">No links.</p> : null}
          {links.length > 0 ? (
            <table className="plaza-table">
              <thead>
                <tr>
                  <th>Label</th>
                  <th>URL</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {links.map((link, index) => (
                  <tr key={`${link.url}-${index}`}>
                    <td>{linkCaption(link)}</td>
                    <td>
                      <span className="url-field-row">
                        <a href={link.url} target="_blank" rel="noreferrer">
                          {link.url}
                        </a>
                        <OpenUrl href={link.url} />
                      </span>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="ghost"
                        onClick={() => patch.mutate({ links: links.filter((_, i) => i !== index) })}
                      >
                        Remove
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : null}
        </div>
      </section>
      <section className="plaza-tile">
        <div className="plaza-core">
          <div className="plaza-head">
            <h3>Log</h3>
            <button type="button" className="plaza-plus" aria-label="Add note" onClick={() => setNoteOpen(true)}>
              +
            </button>
          </div>
          {log.length === 0 ? <p className="muted">No notes yet.</p> : null}
          {log.length > 0 ? (
            <div className="plaza-slider">
              {preview.map((note) => (
                <article key={note.id} className="plaza-tile plaza-slide">
                  <div className="plaza-core">
                    <p className="plaza-kicker">{noteStamp(note.createdAt)}</p>
                    <p>{note.body}</p>
                  </div>
                </article>
              ))}
              <Link to={`/projects/${id}/notes`} className="plaza-tile plaza-slide plaza-slide-more">
                <div className="plaza-core">
                  <p className="plaza-kicker">Archive</p>
                  <h2>Show more</h2>
                </div>
              </Link>
            </div>
          ) : null}
        </div>
      </section>
      <PlazaSheet
        open={linkOpen}
        title="Add link"
        onClose={() => {
          setLinkOpen(false)
          setFormError('')
        }}
      >
        <form className="plaza-form" onSubmit={onAddLink}>
          <label>
            Label
            <input value={linkLabel} onChange={(e) => setLinkLabel(e.target.value)} />
          </label>
          <label>
            URL
            <input value={linkUrl} onChange={(e) => setLinkUrl(e.target.value)} placeholder="https://" />
          </label>
          {formError && linkOpen ? <p className="error">{formError}</p> : null}
          <button type="submit">Add</button>
        </form>
      </PlazaSheet>
      <PlazaSheet
        open={noteOpen}
        title="Add note"
        onClose={() => {
          setNoteOpen(false)
          setFormError('')
        }}
      >
        <form className="plaza-form" onSubmit={onAddNote}>
          <label>
            Note
            <textarea rows={6} value={noteBody} onChange={(e) => setNoteBody(e.target.value)} />
          </label>
          {formError && noteOpen ? <p className="error">{formError}</p> : null}
          <button type="submit">Add</button>
        </form>
      </PlazaSheet>
    </div>
  )
}
