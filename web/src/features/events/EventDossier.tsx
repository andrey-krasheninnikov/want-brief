import { Input } from '@want-foundation/desyst-react'
import { Button, Textarea } from '@want-foundation/desyst-react'
import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { useTimezone } from '../../shared/timezone'
import { zonedInstant, zonedLocalInput } from '../../shared/zoned'
import { openEvent } from '../../shared/taskOverlay'
import { OpenUrl, UrlField } from '../../shared/UrlField'
import { PeoplePicker } from '../people/PeoplePicker'
import { TaskSheet } from '../tasks/TaskSheet'
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
import './events.css'

type Props = {
  seriesId?: string
  originalOn?: string
  onCreated: (id: string) => void
  onDeleted: () => void
}

function emptyStart(tz: string): string {
  return zonedLocalInput(new Date(Date.now() + 60 * 60 * 1000), tz)
}

function noteStamp(iso: string, tz: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: tz }).format(new Date(iso))
}

function slotStamp(ymd: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(`${ymd}T12:00:00Z`))
}

function draftFrom(tz: string, row?: EventSeries) {
  const seconds = row?.durationSeconds ?? 1800
  return {
    title: row?.title ?? '',
    description: row?.description ?? '',
    agenda: row?.agenda ?? '',
    kind: (row?.kind ?? 'call') as EventKind,
    type: (row?.type ?? 'sync') as EventType,
    projectId: row?.projectId ?? '',
    timezone: row?.timezone ?? tz,
    startsAt: row ? zonedLocalInput(row.startsAt, row.timezone ?? tz) : emptyStart(tz),
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

export function EventDossierSheet({
  seriesId,
  originalOn,
  onClose,
}: {
  seriesId: string
  originalOn?: string
  onClose: () => void
}) {
  const navigate = useNavigate()
  return (
    <TaskSheet open kicker={originalOn ? 'Occurrence' : 'Series'} title="Dossier" onClose={onClose}>
      <EventDossier
        seriesId={seriesId}
        originalOn={originalOn}
        onCreated={(id) => openEvent(navigate, id)}
        onDeleted={onClose}
      />
    </TaskSheet>
  )
}

export function EventDossier({ seriesId, originalOn, onCreated, onDeleted }: Props) {
  const tz = useTimezone()
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
  const [form, setForm] = useState(() => draftFrom(tz))
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
    if (seriesId && series.data) setForm(draftFrom(tz, series.data))
    if (!seriesId) setForm(draftFrom(tz))
  }, [seriesId, series.data?.id, series.data?.updatedAt, series.data?.timezone, tz])

  useEffect(() => {
    setAllNotes(false)
    setNoteOpen(false)
    setNoteBody('')
  }, [seriesId, originalOn])

  useEffect(() => {
    if (!occurrence.data) return
    const seconds = occurrence.data.durationSeconds
    setOccStart(zonedLocalInput(occurrence.data.startsAt, series.data?.timezone ?? tz))
    setOccHours(String(Math.floor(seconds / 3600)))
    setOccMinutes(String(Math.floor((seconds % 3600) / 60)))
    setOccSkipped(Boolean(occurrence.data.skipped))
  }, [occurrence.data?.startsAt, occurrence.data?.durationSeconds, occurrence.data?.skipped, occurrence.data?.originalOn, series.data?.timezone, tz])

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
      startsAt: zonedInstant(form.startsAt, form.timezone)!,
      timezone: form.timezone,
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
        startsAt: zonedInstant(occStart, series.data?.timezone ?? tz)!,
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
      weekdays: recurrence === 'weekly' ? (current.weekdays.length ? current.weekdays : weekdaysFromStart(zonedInstant(current.startsAt, current.timezone)!, current.timezone)) : [],
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
            <DateField mode="datetime" timeZone={series.data?.timezone ?? tz} value={occStart} onChange={setOccStart} />
          </label>
          <div className="events-pair">
            <label>
              Hours
              <Input type="number" min={0} value={occHours} onChange={(e) => setOccHours(e.target.value)} />
            </label>
            <label>
              Minutes
              <Input type="number" min={0} max={59} value={occMinutes} onChange={(e) => setOccMinutes(e.target.value)} />
            </label>
          </div>
          <label className="tasks-toggle">
            <input type="checkbox" checked={occSkipped} onChange={(e) => setOccSkipped(e.target.checked)} />
            Skip this slot
          </label>
          <div className="events-pair">
            <Button type="button" onClick={() => saveOcc.mutate()} disabled={saveOcc.isPending || !occStart}>
              Save occurrence
            </Button>
            <Button type="button" className="ghost" onClick={() => resetOcc.mutate()} disabled={resetOcc.isPending}>
              Reset
            </Button>
          </div>
          <div className="tasks-head">
            <h3>Log</h3>
            <div className="people-tabs">
              <Button type="button" className={allNotes ? '' : 'on'} onClick={() => setAllNotes(false)}>
                This slot
              </Button>
              <Button type="button" className={allNotes ? 'on' : ''} onClick={() => setAllNotes(true)}>
                All notes
              </Button>
            </div>
            <Button type="button" className="tasks-plus" aria-label="Add note" onClick={() => setNoteOpen((on) => !on)}>
              +
            </Button>
          </div>
          {noteOpen ? (
            <div className="stack">
              <label>
                Note
                <Textarea rows={4} value={noteBody} onChange={(e) => setNoteBody(e.target.value)} />
              </label>
              <Button type="button" onClick={() => addNote.mutate()} disabled={addNote.isPending || !noteBody.trim()}>
                Add
              </Button>
            </div>
          ) : null}
          {log.length === 0 && !noteOpen ? <p className="muted">No notes yet.</p> : null}
          {log.map((note: EventNote) => (
            <article key={note.id} className="tasks-note">
              <p className="tasks-kicker">
                {noteStamp(note.createdAt, tz)}
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
        <Input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} autoComplete="off" />
      </label>
      <label>
        Description
        <Textarea rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
      </label>
      <label>
        Agenda
        <Textarea rows={4} value={form.agenda} onChange={(e) => setForm({ ...form, agenda: e.target.value })} />
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
      <p className="muted">Time zone: {form.timezone}</p>
      <label>
        Starts
        <DateField mode="datetime" timeZone={form.timezone} value={form.startsAt} onChange={(startsAt) => setForm({ ...form, startsAt })} />
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
          <Input type="number" min={0} value={form.hours} onChange={(e) => setForm({ ...form, hours: e.target.value })} />
        </label>
        <label>
          Minutes
          <Input type="number" min={0} max={59} value={form.minutes} onChange={(e) => setForm({ ...form, minutes: e.target.value })} />
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
            <Input value={form.activeStart} onChange={(e) => setForm({ ...form, activeStart: e.target.value })} />
          </label>
          <label>
            To min
            <Input value={form.activeEnd} onChange={(e) => setForm({ ...form, activeEnd: e.target.value })} />
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
              <Button type="button" className="ghost" onClick={() => dropLink(index)}>
                Remove
              </Button>
            </li>
          ))}
        </ul>
      ) : null}
      <div className="events-pair">
        <label>
          Link label
          <Input value={linkLabel} onChange={(e) => setLinkLabel(e.target.value)} />
        </label>
        <label>
          Link URL
          <span className="url-field-row">
            <Input value={linkUrl} placeholder="https://" onChange={(e) => setLinkUrl(e.target.value)} />
            <OpenUrl href={linkUrl} />
          </span>
        </label>
      </div>
      <Button type="button" className="ghost" onClick={addLink}>
        Add link
      </Button>
      {error ? <p className="error">{error}</p> : null}
      <Button type="submit" disabled={save.isPending}>
        {seriesId ? 'Save series' : 'Create'}
      </Button>
      {seriesId ? (
        <Button type="button" className="ghost" onClick={() => remove.mutate()} disabled={remove.isPending}>
          Delete
        </Button>
      ) : null}
    </form>
  )
}
