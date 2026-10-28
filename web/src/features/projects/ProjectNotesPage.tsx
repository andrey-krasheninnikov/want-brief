import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, Navigate, useParams } from 'react-router-dom'
import { api } from '../../api'

function noteStamp(iso: string): string {
  return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso))
}

export function ProjectNotesPage() {
  const { id = '' } = useParams()
  const queryClient = useQueryClient()
  const project = useQuery({ queryKey: ['projects', id], queryFn: () => api.project(id), enabled: Boolean(id) })
  const notes = useQuery({
    queryKey: ['project-notes', id],
    queryFn: () => api.projectNotes(id),
    enabled: Boolean(id),
  })
  const removeNote = useMutation({
    mutationFn: (noteId: string) => api.deleteProjectNote(id, noteId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['project-notes', id] }),
  })

  if (project.isError) {
    return <Navigate to="/projects" replace />
  }
  if (!project.data) {
    return (
      <div className="plaza">
        <p className="muted">Loading…</p>
      </div>
    )
  }

  const log = notes.data ?? []

  return (
    <div className="plaza plaza-detail">
      <Link to={`/projects/${id}`} className="plaza-back">
        {project.data.name}
      </Link>
      <header className="plaza-hero">
        <div>
          <p className="plaza-kicker">Log</p>
          <h1>Notes</h1>
        </div>
      </header>
      <section className="plaza-tile">
        <div className="plaza-core">
          {notes.isLoading ? <p className="muted">Loading…</p> : null}
          {notes.isError ? <p className="error">{notes.error.message}</p> : null}
          {!notes.isLoading && !notes.isError && log.length === 0 ? <p className="muted">No notes yet.</p> : null}
          <ul className="plaza-log">
            {log.map((note) => (
              <li key={note.id}>
                <div>
                  <time className="muted">{noteStamp(note.createdAt)}</time>
                  <p>{note.body}</p>
                </div>
                <button type="button" className="ghost" onClick={() => removeNote.mutate(note.id)}>
                  Remove
                </button>
              </li>
            ))}
          </ul>
        </div>
      </section>
    </div>
  )
}
