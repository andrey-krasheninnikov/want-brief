import type { NavigateFunction } from 'react-router-dom'
import type { Schedule } from '../types'
import { workspaceFromPath, workspaceUrl } from './workspacePath'

export function taskSearch(id: string, search = window.location.search): string {
  const params = new URLSearchParams(search)
  params.set('task', id)
  params.delete('event')
  params.delete('on')
  return params.toString()
}

export function eventSearch(seriesId: string, originalOn?: string, search = window.location.search): string {
  const params = new URLSearchParams(search)
  params.set('event', seriesId)
  if (originalOn) params.set('on', originalOn)
  else params.delete('on')
  params.delete('new')
  params.delete('task')
  return params.toString()
}

export function openTask(navigate: NavigateFunction, id: string, workspaceId?: string) {
  if (workspaceId && workspaceId !== workspaceFromPath(window.location.pathname)) {
    window.location.assign(workspaceUrl(workspaceId, '/tasks?' + taskSearch(id, '')))
  } else {
    navigate({ search: taskSearch(id) })
  }
}

export function openEvent(navigate: NavigateFunction, seriesId: string, originalOn?: string, workspaceId?: string) {
  if (workspaceId && workspaceId !== workspaceFromPath(window.location.pathname)) {
    window.location.assign(workspaceUrl(workspaceId, '/events?' + eventSearch(seriesId, originalOn, '')))
  } else {
    navigate({ search: eventSearch(seriesId, originalOn) })
  }
}

export function scheduleOrigins(schedule?: Schedule) {
  const tasks = new Map<string, string>()
  const events = new Map<string, string>()
  for (const lane of schedule?.lanes ?? []) {
    for (const block of lane.blocks) tasks.set(block.itemId, block.workspaceId)
  }
  for (const row of schedule?.unplanned ?? []) tasks.set(row.item.id, row.item.workspaceId)
  for (const row of schedule?.atRisk ?? []) {
    if (row.workspaceId) tasks.set(row.itemId, row.workspaceId)
  }
  for (const row of schedule?.busy ?? []) {
    if (row.seriesId && row.workspaceId) events.set(row.seriesId, row.workspaceId)
  }
  return { tasks, events }
}

export function taskHref(id: string, search = window.location.search) {
  return { search: taskSearch(id, search) }
}
