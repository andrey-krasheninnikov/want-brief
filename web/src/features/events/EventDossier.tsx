import { useEffect, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { OpenUrl, UrlField } from '../../shared/UrlField'
import { PeoplePicker } from '../people/PeoplePicker'
import {
  EVENT_TYPES,
  eventTypeLabel,
  recurrenceLabel,
  weekdaysFromStart,
  type EventKind,
  type EventNote,
  type EventRecurrence,
  type EventSeries,
  type EventType,
  type ProjectLink,
} from '../../types'
import { WeekdayGrid } from './WeekdayGrid'

type Props = {
  seriesId?: string
  originalOn?: string
  onCreated: (id: string) => void
  onDeleted: () => void
}

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

function toLocalInput(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function emptyStart(): string {
  const d = new Date(Date.now() + 60 * 60 * 1000)
  return toLocalInput(d.toISOString())
}

function noteStamp(iso: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
}

function slotStamp(ymd: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeZone: 'Europe/Moscow' }).format(
    new Date(`${ymd}T12:00:00+03:00`),
  )
}

function draftFrom(row?: EventSeries) {
  const seconds = row?.durationSeconds ?? 1800
  return {
    title: row?.title ?? '',
    description: row?.description ?? '',
    agenda: row?.agenda ?? '',
    kind: (row?.kind ?? 'call') as EventKind,
    type: (row?.type ?? 'sync') as EventType,
    projectId: row?.projectId ?? '',
    startsAt: row ? toLocalInput(row.startsAt) : emptyStart(),
    hours: String(Math.floor(seconds / 3600)),
    minutes: String(Math.floor((seconds % 3600) / 60)),
    recurrence: (row?.recurrence ?? 'once') as EventRecurrence,
    repeatUntil: row?.repeatUntil ?? '',
    weekdays: row?.weekdays ?? [],
    meetUrl: row?.meetUrl ?? '',
    involvement: row?.involvement ?? 5,
    activeOn: row?.activeStartOffset != null && row.activeEndOffset != null,
    activeStart: String(Math.floor((row?.activeStartOffset ?? 0) / 60)),
    activeEnd: String(Math.floor((row?.activeEndOffset ?? Math.min(seconds, 900)) / 60)),
    canSkip: row?.canSkip ?? true,
    links: row?.links ?? [],
    people: row?.people ?? [],
  }
}

export function EventDossier({ seriesId, originalOn, onCreated, onDeleted }: Props) {
  const queryClient = useQueryClient()
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const series = useQuery({
    queryKey: ['event', seriesId],
    queryFn: () => api.event(seriesId!),
    enabled: Boolean(seriesId),
  })
  const occurrence = useQuery({
    queryKey: ['event-occurrence', seriesId, originalOn],
    queryFn: () => api.eventOccurrence(seriesId!, originalOn!),
    enabled: Boolean(seriesId && originalOn),
  })
  const [allNotes, setAllNotes] = useState(false)
  const notes = useQuery({
    queryKey: ['event-notes', seriesId, allNotes ? 'all' : originalOn],
    queryFn: () => api.eventNotes(seriesId!, allNotes ? undefined : originalOn),
    enabled: Boolean(seriesId && originalOn),
  })
  const [form, setForm] = useState(() => draftFrom())
  const [occStart, setOccStart] = useState('')
  const [occHours, setOccHours] = useState('0')
  const [occMinutes, setOccMinutes] = useState('30')
  const [occSkipped, setOccSkipped] = useState(false)
  const [noteBody, setNoteBody] = useState('')
  const [noteOpen, setNoteOpen] = useState(false)
  const [linkLabel, setLinkLabel] = useState('')
  const [linkUrl, setLinkUrl] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (seriesId && series.data) setForm(draftFrom(series.data))
    if (!seriesId) setForm(draftFrom())
  }, [seriesId, series.data?.id, series.data?.updatedAt])

  useEffect(() => {
    setAllNotes(false)
    setNoteOpen(false)
    setNoteBody('')
  }, [seriesId, originalOn])

  useEffect(() => {
    if (!occurrence.data) return
    const seconds = occurrence.data.durationSeconds
    setOccStart(toLocalInput(occurrence.data.startsAt))
    setOccHours(String(Math.floor(seconds / 3600)))
    setOccMinutes(String(Math.floor((seconds % 3600) / 60)))
    setOccSkipped(Boolean(occurrence.data.skipped))
  }, [occurrence.data?.startsAt, occurrence.data?.durationSeconds, occurrence.data?.skipped, occurrence.data?.originalOn])

  function body() {
    const duration = Math.round(Number(form.hours) || 0) * 3600 + Math.round(Number(form.minutes) || 0) * 60
    const startMin = Math.round(Number(form.activeStart) || 0)
    const endMin = Math.round(Number(form.activeEnd) || 0)
    return {
      title: form.title.trim(),
      description: form.description,
      agenda: form.agenda,
      kind: form.kind,
      type: form.type,
      projectId: form.projectId || null,
      startsAt: new Date(form.startsAt).toISOString(),
      durationSeconds: duration,
      recurrence: form.recurrence,
      repeatUntil: form.recurrence === 'once' ? null : form.repeatUntil || null,
      weekdays: form.recurrence === 'weekly' ? form.weekdays : [],
      meetUrl: form.meetUrl.trim(),
      involvement: form.involvement,
      activeStartOffset: form.activeOn ? startMin * 60 : null,
      activeEndOffset: form.activeOn ? endMin * 60 : null,
      canSkip: form.canSkip,
      links: form.links,
      people: form.people,
    }
  }

  const save = useMutation({
    mutationFn: () => (seriesId ? api.patchEvent(seriesId, body()) : api.createEvent(body())),
    onSuccess: (row) => {
      setError('')
      void queryClient.invalidateQueries({ queryKey: ['events'] })
      void queryClient.invalidateQueries({ queryKey: ['event'] })
      void queryClient.invalidateQueries({ queryKey: ['event-series'] })
      void queryClient.invalidateQueries({ queryKey: ['event-occurrence'] })
      void queryClient.invalidateQueries({ queryKey: ['schedule'] })
      void queryClient.invalidateQueries({ queryKey: ['people'] })
      void queryClient.invalidateQueries({ queryKey: ['person'] })
      if (!seriesId) onCreated(row.id)
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not save.'),
  })
  const saveOcc = useMutation({
    mutationFn: () => {
      const duration = Math.round(Number(occHours) || 0) * 3600 + Math.round(Number(occMinutes) || 0) * 60
      return api.putEventOccurrence(seriesId!, {
        originalOn: originalOn!,
        startsAt: new Date(occStart).toISOString(),
        durationSeconds: duration,
        skipped: occSkipped,
      })
    },
    onSuccess: () => {
      setError('')
      void queryClient.invalidateQueries({ queryKey: ['events'] })
      void queryClient.invalidateQueries({ queryKey: ['event-occurrence'] })
      void queryClient.invalidateQueries({ queryKey: ['schedule'] })
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not save occurrence.'),
  })
  const resetOcc = useMutation({
    mutationFn: () => api.resetEventOccurrence(seriesId!, originalOn!),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['events'] })
      void queryClient.invalidateQueries({ queryKey: ['event-occurrence'] })
      void queryClient.invalidateQueries({ queryKey: ['schedule'] })
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not reset.'),
  })
  const addNote = useMutation({
    mutationFn: () => api.createEventNote(seriesId!, originalOn!, noteBody),
    onSuccess: () => {
      setNoteBody('')
      setNoteOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['event-notes'] })
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not add note.'),
  })
  const remove = useMutation({
    mutationFn: () => api.deleteEvent(seriesId!),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['events'] })
      void queryClient.invalidateQueries({ queryKey: ['schedule'] })
      onDeleted()
    },
    onError: (err) => setError(err instanceof Error ? err.message : 'Could not delete.'),
  })

  function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (!form.title.trim()) {
      setError('Title is required.')
      return
    }
    if (form.recurrence === 'weekly' && form.weekdays.length === 0) {
      setError('Pick at least one weekday.')
      return
    }
    if (form.people.some((rel) => !rel.comment.trim())) {
      setError('Comment is required for each person.')
      return
    }
    save.mutate()
  }

  function setRecurrence(recurrence: EventRecurrence) {
    setForm((current) => ({
      ...current,
      recurrence,
      weekdays: recurrence === 'weekly' ? (current.weekdays.length ? current.weekdays : weekdaysFromStart(current.startsAt)) : [],
    }))
  }

  function addLink() {
    const url = linkUrl.trim()
    if (!url) return
    setForm((current) => ({ ...current, links: [...current.links, { label: linkLabel.trim(), url }] }))
    setLinkLabel('')
    setLinkUrl('')
  }

  function dropLink(index: number) {
    setForm((current) => ({ ...current, links: current.links.filter((_, i) => i !== index) }))
  }

  if (seriesId && series.isLoading) return <p className="muted">Loading…</p>
  if (seriesId && series.isError) return <p className="error">{series.error.message}</p>

  const log = notes.data ?? []

  return (
    <form className="events-form" onSubmit={onSubmit}>
      {seriesId && originalOn ? (
        <section className="events-block">
          <p className="events-kicker">This occurrence</p>
          {occurrence.isLoading ? <p className="muted">Loading…</p> : null}
          {occurrence.isError ? <p className="error">{occurrence.error.message}</p> : null}
          <label>
            Starts
            <DateField mode="datetime" value={occStart} onChange={setOccStart} />
          </label>
          <div className="events-pair">
            <label>
              Hours
              <input type="number" min={0} value={occHours} onChange={(e) => setOccHours(e.target.value)} />
            </label>
            <label>
              Minutes
              <input type="number" min={0} max={59} value={occMinutes} onChange={(e) => setOccMinutes(e.target.value)} />
            </label>
          </div>
          <label className="tasks-toggle">
            <input type="checkbox" checked={occSkipped} onChange={(e) => setOccSkipped(e.target.checked)} />
            Skip this slot
          </label>
          <div className="events-pair">
            <button type="button" onClick={() => saveOcc.mutate()} disabled={saveOcc.isPending || !occStart}>
              Save occurrence
            </button>
            <button type="button" className="ghost" onClick={() => resetOcc.mutate()} disabled={resetOcc.isPending}>
              Reset
            </button>
          </div>
          <div className="tasks-head">
            <h3>Log</h3>
            <div className="people-tabs">
              <button type="button" className={allNotes ? '' : 'on'} onClick={() => setAllNotes(false)}>
                This slot
              </button>
              <button type="button" className={allNotes ? 'on' : ''} onClick={() => setAllNotes(true)}>
                All notes
              </button>
            </div>
            <button type="button" className="tasks-plus" aria-label="Add note" onClick={() => setNoteOpen((on) => !on)}>
              +
            </button>
          </div>
          {noteOpen ? (
            <div className="stack">
              <label>
                Note
                <textarea rows={4} value={noteBody} onChange={(e) => setNoteBody(e.target.value)} />
              </label>
              <button type="button" onClick={() => addNote.mutate()} disabled={addNote.isPending || !noteBody.trim()}>
                Add
              </button>
            </div>
          ) : null}
          {log.length === 0 && !noteOpen ? <p className="muted">No notes yet.</p> : null}
          {log.map((note: EventNote) => (
            <article key={note.id} className="tasks-note">
              <p className="tasks-kicker">
                {noteStamp(note.createdAt)}
                {allNotes ? ` · ${slotStamp(note.originalOn)}` : ''}
              </p>
              <p>{note.body}</p>
            </article>
          ))}
        </section>
      ) : null}
      <p className="events-kicker">{seriesId ? recurrenceLabel(form.recurrence, form.weekdays) : 'New series'}</p>
      <label>
        Title
        <input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} autoComplete="off" />
      </label>
      <label>
        Description
        <textarea rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
      </label>
      <label>
        Agenda
        <textarea rows={4} value={form.agenda} onChange={(e) => setForm({ ...form, agenda: e.target.value })} />
      </label>
      <div className="events-pair">
        <label>
          Kind
          <select value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value as EventKind })}>
            <option value="call">Call</option>
            <option value="event">Event</option>
          </select>
        </label>
        <label>
          Type
          <select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value as EventType })}>
            {EVENT_TYPES.map((value) => (
              <option key={value} value={value}>
                {eventTypeLabel(value)}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label>
        Project
        <select value={form.projectId} onChange={(e) => setForm({ ...form, projectId: e.target.value })}>
          <option value="">No project</option>
          {(projects.data ?? []).map((project) => (
            <option key={project.id} value={project.id}>
              {project.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        Starts
        <DateField mode="datetime" value={form.startsAt} onChange={(startsAt) => setForm({ ...form, startsAt })} />
      </label>
      {form.recurrence !== 'once' ? (
        <label>
          Repeat until
          <DateField mode="date" value={form.repeatUntil} onChange={(repeatUntil) => setForm({ ...form, repeatUntil })} />
        </label>
      ) : null}
      <label>
        Repeat
        <select value={form.recurrence} onChange={(e) => setRecurrence(e.target.value as EventRecurrence)}>
          <option value="once">Once</option>
          <option value="weekly">Weekly</option>
          <option value="monthly">Monthly</option>
        </select>
      </label>
      {form.recurrence === 'weekly' ? (
        <div className="events-days-field">
          Days
          <WeekdayGrid value={form.weekdays} onChange={(weekdays) => setForm({ ...form, weekdays })} />
        </div>
      ) : null}
      <div className="events-pair">
        <label>
          Hours
          <input type="number" min={0} value={form.hours} onChange={(e) => setForm({ ...form, hours: e.target.value })} />
        </label>
        <label>
          Minutes
          <input type="number" min={0} max={59} value={form.minutes} onChange={(e) => setForm({ ...form, minutes: e.target.value })} />
        </label>
      </div>
      <UrlField label="Meet" value={form.meetUrl} onChange={(meetUrl) => setForm({ ...form, meetUrl })} />
      <label className="events-range">
        Involvement {form.involvement}
        <input
          type="range"
          min={0}
          max={10}
          value={form.involvement}
          onChange={(e) => setForm({ ...form, involvement: Number(e.target.value) })}
        />
      </label>
      <label className="tasks-toggle">
        <input
          type="checkbox"
          checked={form.activeOn}
          onChange={(e) => setForm({ ...form, activeOn: e.target.checked })}
        />
        Peak interval
      </label>
      {form.activeOn ? (
        <div className="events-pair">
          <label>
            From min
            <input value={form.activeStart} onChange={(e) => setForm({ ...form, activeStart: e.target.value })} />
          </label>
          <label>
            To min
            <input value={form.activeEnd} onChange={(e) => setForm({ ...form, activeEnd: e.target.value })} />
          </label>
        </div>
      ) : null}
      <label className="tasks-toggle">
        <input type="checkbox" checked={form.canSkip} onChange={(e) => setForm({ ...form, canSkip: e.target.checked })} />
        Can skip
      </label>
      <PeoplePicker value={form.people} onChange={(people) => setForm({ ...form, people })} requireComment />
      {form.links.length > 0 ? (
        <ul className="events-slots">
          {form.links.map((link: ProjectLink, index) => (
            <li key={`${link.url}-${index}`}>
              <span className="url-field-row">
                <a href={link.url} target="_blank" rel="noreferrer">
                  {link.label || link.url}
                </a>
                <OpenUrl href={link.url} />
              </span>
              <button type="button" className="ghost" onClick={() => dropLink(index)}>
                Remove
              </button>
            </li>
          ))}
        </ul>
      ) : null}
      <div className="events-pair">
        <label>
          Link label
          <input value={linkLabel} onChange={(e) => setLinkLabel(e.target.value)} />
        </label>
        <label>
          Link URL
          <span className="url-field-row">
            <input value={linkUrl} placeholder="https://" onChange={(e) => setLinkUrl(e.target.value)} />
            <OpenUrl href={linkUrl} />
          </span>
        </label>
      </div>
      <button type="button" className="ghost" onClick={addLink}>
        Add link
      </button>
      {error ? <p className="error">{error}</p> : null}
      <button type="submit" disabled={save.isPending}>
        {seriesId ? 'Save series' : 'Create'}
      </button>
      {seriesId ? (
        <button type="button" className="ghost" onClick={() => remove.mutate()} disabled={remove.isPending}>
          Delete
        </button>
      ) : null}
    </form>
  )
}
