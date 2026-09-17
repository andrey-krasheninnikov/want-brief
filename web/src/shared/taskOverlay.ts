import type { NavigateFunction } from 'react-router-dom'

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

export function openTask(navigate: NavigateFunction, id: string) {
  navigate({ search: taskSearch(id) })
}

export function openEvent(navigate: NavigateFunction, seriesId: string, originalOn?: string) {
  navigate({ search: eventSearch(seriesId, originalOn) })
}

export function taskHref(id: string, search = window.location.search) {
  return { search: taskSearch(id, search) }
}
