import type { CSSProperties } from 'react'
import { span } from '../../shared/format'
import type { Company, EventOccurrence, Project } from '../../types'

export type EventLookups = {
  projectById: Map<string, Project>
  companyById: Map<string, Company>
  companyIdsByEvent: Map<string, string[]>
}

export function eventLookups(projects: Project[], companies: Company[]): EventLookups {
  const projectById = new Map(projects.map((row) => [row.id, row]))
  const companyById = new Map(companies.map((row) => [row.id, row]))
  const companyIdsByEvent = new Map<string, string[]>()
  for (const company of companies) {
    for (const rel of company.events ?? []) {
      const list = companyIdsByEvent.get(rel.id) ?? []
      list.push(company.id)
      companyIdsByEvent.set(rel.id, list)
    }
  }
  return { projectById, companyById, companyIdsByEvent }
}

export type EventMeta = {
  projectName: string
  companyName: string
  color: string
  duration: string
  line: string
}

export function eventMeta(row: EventOccurrence, lookups: EventLookups): EventMeta {
  const project = row.projectId ? lookups.projectById.get(row.projectId) : undefined
  const ids = new Set<string>(lookups.companyIdsByEvent.get(row.seriesId) ?? [])
  for (const rel of project?.companies ?? []) ids.add(rel.id)
  const companyName = [...ids]
    .map((id) => lookups.companyById.get(id)?.name)
    .filter((name): name is string => Boolean(name))
    .join(' · ')
  const projectName = project?.name ?? ''
  const duration = span(row.durationSeconds)
  return {
    projectName,
    companyName,
    color: project?.color ?? '',
    duration,
    line: [companyName, projectName, duration].filter(Boolean).join(' · '),
  }
}

export function eventTone(color: string): CSSProperties | undefined {
  if (!color) return undefined
  return { '--event': color } as CSSProperties
}
