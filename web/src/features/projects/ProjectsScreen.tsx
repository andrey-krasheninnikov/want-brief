import { useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api'
import { moscowRange } from '../../shared/moscow'
import type { EventSeries, Item } from '../../types'
import { PlazaSheet } from './PlazaSheet'
import { ProjectCard } from './ProjectCard'

function secondsFor(rows: { projectId: string | null; allocatedSeconds: number }[] | undefined, id: string): number {
  return rows?.find((row) => row.projectId === id)?.allocatedSeconds ?? 0
}

function itemCounts(items: Item[] | undefined, projectId: string): { closed: number; total: number } {
  const rows = (items ?? []).filter((item) => item.projectId === projectId)
  return {
    closed: rows.filter((item) => item.status === 'done' || item.status === 'cancelled').length,
    total: rows.length,
  }
}

function upcomingEvents(series: EventSeries[] | undefined, projectId: string, now = Date.now()): number {
  return (series ?? []).filter((row) => {
    if (row.projectId !== projectId) return false
    return row.recurrence !== 'once' || new Date(row.startsAt).getTime() >= now
  }).length
}

export function ProjectsScreen() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const monthLoad = useQuery({
    queryKey: ['load', 'month'],
    queryFn: () => {
      const bounds = moscowRange('month')
      return api.load(bounds.from, bounds.to)
    },
  })
  const items = useQuery({ queryKey: ['items'], queryFn: () => api.items() })
  const series = useQuery({ queryKey: ['event-series'], queryFn: api.eventSeries })
  const companies = useQuery({ queryKey: ['companies'], queryFn: api.companies })
  const [open, setOpen] = useState(false)
  const [showArchived, setShowArchived] = useState(false)
  const [name, setName] = useState('')
  const [formError, setFormError] = useState('')
  const create = useMutation({
    mutationFn: () => api.createProject({ name: name.trim(), color: '#6152ED' }),
    onSuccess: (project) => {
      setName('')
      setFormError('')
      setOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
      navigate(`/projects/${project.id}`)
    },
    onError: (err) => setFormError(err instanceof Error ? err.message : 'Could not add.'),
  })

  function onSubmit(event: FormEvent) {
    event.preventDefault()
    if (!name.trim()) {
      setFormError('Title is required.')
      return
    }
    setFormError('')
    create.mutate()
  }

  const all = projects.data ?? []
  const list = showArchived ? all : all.filter((project) => !project.archivedAt)
  const companyNames = useMemo(() => {
    const names = new Map<string, string>()
    for (const row of companies.data ?? []) names.set(row.id, row.name)
    return names
  }, [companies.data])

  return (
    <div className="plaza">
      <header className="plaza-hero">
        <div>
          <p className="plaza-kicker">Studio</p>
          <h1>Projects</h1>
        </div>
        <div className="plaza-hero-tools">
          <label className="plaza-toggle">
            <input
              type="checkbox"
              checked={showArchived}
              onChange={(e) => setShowArchived(e.target.checked)}
            />
            Show archived
          </label>
          <button type="button" className="plaza-plus" aria-label="Add project" onClick={() => setOpen(true)}>
            +
          </button>
        </div>
      </header>
      {projects.isLoading ? (
        <div className="plaza-bento" aria-hidden>
          <div className="plaza-tile">
            <div className="plaza-core plaza-skel" />
          </div>
          <div className="plaza-tile">
            <div className="plaza-core plaza-skel" />
          </div>
        </div>
      ) : null}
      {projects.isError ? <p className="error">{projects.error.message}</p> : null}
      {!projects.isLoading && !projects.isError && list.length === 0 ? (
        <p className="muted">{all.length === 0 ? 'No projects.' : 'No active projects.'}</p>
      ) : null}
      {!projects.isLoading && !projects.isError && list.length > 0 ? (
        <div className="plaza-bento">
          {list.map((project, index) => {
            const counts = itemCounts(items.data, project.id)
            return (
              <ProjectCard
                key={project.id}
                project={project}
                monthSeconds={secondsFor(monthLoad.data?.byProject, project.id)}
                closed={counts.closed}
                total={counts.total}
                events={upcomingEvents(series.data, project.id)}
                people={project.people?.length ?? 0}
                companies={(project.companies ?? []).map((rel) => companyNames.get(rel.id)).filter(Boolean).join(' · ')}
                delay={index * 80}
              />
            )
          })}
        </div>
      ) : null}
      <PlazaSheet open={open} title="Add project" onClose={() => setOpen(false)}>
        <form className="plaza-form" onSubmit={onSubmit}>
          <label>
            Title
            <input value={name} onChange={(e) => setName(e.target.value)} autoComplete="off" autoFocus />
          </label>
          {formError ? <p className="error">{formError}</p> : null}
          <button type="submit">Create</button>
        </form>
      </PlazaSheet>
    </div>
  )
}
