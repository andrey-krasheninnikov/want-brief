import { Link } from 'react-router-dom'
import { hours } from '../../shared/format'
import type { Project } from '../../types'

type Props = {
  project: Project
  monthSeconds: number
  closed: number
  total: number
  events: number
  people: number
  companies: string
  delay: number
}

export function ProjectCard({
  project,
  monthSeconds,
  closed,
  total,
  events,
  people,
  companies,
  delay,
}: Props) {
  const classes = ['plaza-tile']
  if (project.archivedAt) classes.push('plaza-tile-archived')
  return (
    <Link
      to={`/projects/${project.id}`}
      className={classes.join(' ')}
      style={{ animationDelay: `${delay}ms` }}
    >
      <div className="plaza-core">
        {project.archivedAt ? <p className="plaza-kicker">Archived</p> : null}
        <div className="plaza-title">
          <span className="plaza-swatch" style={{ background: project.color }} />
          <h2>{project.name}</h2>
        </div>
        <p className="plaza-hours">
          <strong className="mono">{hours(monthSeconds)}</strong>
          <span>this month</span>
        </p>
        <p className="plaza-meta">
          <span className="mono">
            {closed}/{total}
          </span>
          <span>
            {events} {events === 1 ? 'event' : 'events'}
          </span>
          <span>
            {people} {people === 1 ? 'person' : 'people'}
          </span>
          {companies ? <span>{companies}</span> : null}
        </p>
      </div>
    </Link>
  )
}
