import { useState } from 'react'
import { PencilSimple } from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import type { CompanyNote } from '../../types'
import { TaskSheet } from '../tasks/TaskSheet'
import { PersonRail, PersonRailAdd } from '../people/PersonRail'

type Props = {
  companyId: string
  notes: CompanyNote[]
}

function stamp(iso: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium' }).format(new Date(iso))
}

function preview(body: string): string {
  const text = body.trim()
  if (text.length <= 80) return text
  return `${text.slice(0, 80)}...`
}

export function CompanyNotes({ companyId, notes }: Props) {
  const queryClient = useQueryClient()
  const [sheet, setSheet] = useState<'new' | string | null>(null)
  const [body, setBody] = useState('')
  const editing = notes.find((row) => row.id === sheet)

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['company-notes', companyId] })
    void queryClient.invalidateQueries({ queryKey: ['companies'] })
    void queryClient.invalidateQueries({ queryKey: ['company', companyId] })
  }

  const save = useMutation({
    mutationFn: () => (editing ? api.patchCompanyNote(companyId, editing.id, body) : api.createCompanyNote(companyId, body)),
    onSuccess: () => {
      setSheet(null)
      setBody('')
      refresh()
    },
  })
  const drop = useMutation({
    mutationFn: (noteId: string) => api.deleteCompanyNote(companyId, noteId),
    onSuccess: () => {
      setSheet(null)
      refresh()
    },
  })

  return (
    <section id="company-sec-notes" className="people-section">
      <p className="people-kicker">Notes</p>
      <PersonRail>
        {notes.map((note) => (
          <article key={note.id} className="people-rail-card">
            <p className="people-kicker">{stamp(note.createdAt)}</p>
            <p>{preview(note.body)}</p>
            <button
              type="button"
              className="ghost rel-edit"
              aria-label="Edit note"
              onClick={() => {
                setBody(note.body)
                setSheet(note.id)
              }}
            >
              <PencilSimple size={16} weight="light" />
            </button>
          </article>
        ))}
        <PersonRailAdd
          label="Add note"
          onClick={() => {
            setBody('')
            setSheet('new')
          }}
        />
      </PersonRail>
      <TaskSheet open={sheet !== null} kicker="Note" title={editing ? stamp(editing.createdAt) : 'New note'} onClose={() => setSheet(null)}>
        <label>
          Note
          <textarea rows={8} value={body} onChange={(e) => setBody(e.target.value)} autoFocus />
        </label>
        <div className="people-contact-actions">
          <button type="button" onClick={() => save.mutate()} disabled={!body.trim() || save.isPending}>
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
