import type {
  CalendarEvent,
  EventOccurrence,
  EventNote,
  EventSeries,
  CheckinKind,
  DayOverride,
  DayOverrideDraft,
  Person,
  PersonAbsence,
  PersonNote,
  PersonContact,
  PersonSite,
  PersonBond,
  PersonProfession,
  Company,
  CompanyNote,
  Item,
  ItemCheck,
  ItemEvent,
  ItemKind,
  ItemNote,
  ItemStatus,
  JournalEntry,
  LatestCheckins,
  LoadReport,
  Note,
  Project,
  ProjectNote,
  Schedule,
  ScheduleSettings,
  TaskLogKind,
  Source,
  TimeInterval,
  WhatIfResult,
  WhatIfScenario,
} from './types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })
  if (response.status === 204) {
    return undefined as T
  }
  const data = (await response.json().catch(() => ({}))) as T & { error?: string }
  if (!response.ok) {
    throw new Error(data.error || `HTTP ${response.status}`)
  }
  return data
}

export const api = {
  login: (password: string) =>
    request<{ ok: boolean }>('/api/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),
  logout: () => request('/api/logout', { method: 'POST' }),
  me: () => request<{ ok: boolean }>('/api/me'),
  projects: () => request<Project[]>('/api/projects'),
  project: (id: string) => request<Project>(`/api/projects/${id}`),
  createProject: (body: { name: string; color?: string }) =>
    request<Project>('/api/projects', { method: 'POST', body: JSON.stringify(body) }),
  patchProject: (id: string, body: Partial<Project> & { archived?: boolean }) =>
    request<Project>(`/api/projects/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteProject: (id: string) => request(`/api/projects/${id}`, { method: 'DELETE' }),
  projectNotes: (id: string) => request<ProjectNote[]>(`/api/projects/${id}/notes`),
  createProjectNote: (id: string, body: string) =>
    request<ProjectNote>(`/api/projects/${id}/notes`, { method: 'POST', body: JSON.stringify({ body }) }),
  deleteProjectNote: (id: string, noteId: string) =>
    request(`/api/projects/${id}/notes/${noteId}`, { method: 'DELETE' }),
  sources: () => request<Source[]>('/api/sources'),
  createSource: (body: {
    kind: string
    name: string
    baseUrl?: string
    token?: string
    email?: string
    projectId: string
    insecureTls?: boolean
  }) => request<Source>('/api/sources', { method: 'POST', body: JSON.stringify(body) }),
  patchSource: (id: string, body: Record<string, string | boolean>) =>
    request<Source>(`/api/sources/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteSource: (id: string) => request(`/api/sources/${id}`, { method: 'DELETE' }),
  syncSource: (id: string) =>
    request<{ upserted: number }>(`/api/sources/${id}/sync`, { method: 'POST' }),
  items: (query?: {
    sourceId?: string
    projectId?: string
    kind?: ItemKind
    status?: ItemStatus
    openOnly?: boolean
    includeArchived?: boolean
    archivedOnly?: boolean
  }) => {
    const params = new URLSearchParams()
    if (query?.sourceId) params.set('sourceId', query.sourceId)
    if (query?.projectId) params.set('projectId', query.projectId)
    if (query?.kind) params.set('kind', query.kind)
    if (query?.status) params.set('status', query.status)
    if (query?.openOnly) params.set('openOnly', 'true')
    if (query?.includeArchived) params.set('includeArchived', 'true')
    if (query?.archivedOnly) params.set('archivedOnly', 'true')
    const suffix = params.toString() ? `?${params}` : ''
    return request<Item[]>(`/api/items${suffix}`)
  },
  item: (id: string) => request<Item>(`/api/items/${id}`),
  createItem: (body: {
    title: string
    kind: ItemKind
    projectId?: string
    urgent?: boolean
    important?: boolean
  }) => request<Item>('/api/items', { method: 'POST', body: JSON.stringify(body) }),
  patchItem: (id: string, body: Record<string, unknown>) =>
    request<Item>(`/api/items/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteItem: (id: string) => request(`/api/items/${id}`, { method: 'DELETE' }),
  undeleteItem: (id: string) => request<Item>(`/api/items/${id}/undelete`, { method: 'POST' }),
  itemNotes: (id: string) => request<ItemNote[]>(`/api/items/${id}/notes`),
  createItemNote: (id: string, body: string) =>
    request<ItemNote>(`/api/items/${id}/notes`, { method: 'POST', body: JSON.stringify({ body }) }),
  deleteItemNote: (id: string, noteId: string) =>
    request(`/api/items/${id}/notes/${noteId}`, { method: 'DELETE' }),
  itemEvents: (id: string) => request<ItemEvent[]>(`/api/items/${id}/events`),
  annotateItemEvent: (id: string, body: { kinds: TaskLogKind[]; note?: string; stress?: number }) =>
    request<ItemEvent>(`/api/items/${id}/events/annotate`, { method: 'POST', body: JSON.stringify(body) }),
  itemChecks: (id: string) => request<ItemCheck[]>(`/api/items/${id}/checks`),
  createItemCheck: (id: string, body: string) =>
    request<ItemCheck>(`/api/items/${id}/checks`, { method: 'POST', body: JSON.stringify({ body }) }),
  patchItemCheck: (id: string, checkId: string, body: Record<string, unknown>) =>
    request<ItemCheck>(`/api/items/${id}/checks/${checkId}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteItemCheck: (id: string, checkId: string) =>
    request(`/api/items/${id}/checks/${checkId}`, { method: 'DELETE' }),
  syncItem: (id: string) => request<Item>(`/api/items/${id}/sync`, { method: 'POST' }),
  syncActiveItems: () =>
    request<{ synced: number; failed: number }>('/api/items/sync-active', { method: 'POST' }),
  notes: (itemId?: string) => {
    const suffix = itemId ? `?itemId=${itemId}` : ''
    return request<Note[]>(`/api/notes${suffix}`)
  },
  createNote: (body: string, itemId?: string) =>
    request<Note>('/api/notes', { method: 'POST', body: JSON.stringify({ body, itemId }) }),
  patchNote: (id: string, body: string) =>
    request<Note>(`/api/notes/${id}`, { method: 'PATCH', body: JSON.stringify({ body }) }),
  deleteNote: (id: string) => request(`/api/notes/${id}`, { method: 'DELETE' }),
  intervals: () => request<TimeInterval[]>('/api/intervals'),
  startInterval: (itemId: string) =>
    request<TimeInterval>('/api/intervals', {
      method: 'POST',
      body: JSON.stringify({ itemId }),
    }),
  logInterval: (itemId: string, startedAt: string, endedAt: string) =>
    request<TimeInterval>('/api/intervals', {
      method: 'POST',
      body: JSON.stringify({ itemId, startedAt, endedAt }),
    }),
  stopInterval: (id: string) =>
    request<TimeInterval>(`/api/intervals/${id}/stop`, { method: 'POST' }),
  createStress: (level: number, itemId?: string) =>
    request('/api/stress', { method: 'POST', body: JSON.stringify({ level, itemId }) }),
  latestCheckins: () => request<LatestCheckins>('/api/checkins/latest'),
  createCheckin: (kind: CheckinKind, level: number) =>
    request('/api/checkins', { method: 'POST', body: JSON.stringify({ kind, level }) }),
  events: (from?: string, to?: string) => {
    const params = new URLSearchParams()
    if (from) params.set('from', from)
    if (to) params.set('to', to)
    const suffix = params.toString() ? `?${params}` : ''
    return request<CalendarEvent[]>(`/api/events${suffix}`)
  },
  event: (id: string) => request<EventSeries>(`/api/events/${id}`),
  createEvent: (body: Record<string, unknown>) =>
    request<EventSeries>('/api/events', { method: 'POST', body: JSON.stringify(body) }),
  patchEvent: (id: string, body: Record<string, unknown>) =>
    request<EventSeries>(`/api/events/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteEvent: (id: string) => request(`/api/events/${id}`, { method: 'DELETE' }),
  eventOccurrence: (id: string, originalOn: string) =>
    request<EventOccurrence>(`/api/events/${id}/occurrence?on=${encodeURIComponent(originalOn)}`),
  putEventOccurrence: (
    id: string,
    body: { originalOn: string; startsAt?: string; durationSeconds?: number; skipped?: boolean },
  ) => request<EventOccurrence>(`/api/events/${id}/occurrence`, { method: 'PUT', body: JSON.stringify(body) }),
  resetEventOccurrence: (id: string, originalOn: string) =>
    request(`/api/events/${id}/occurrence?on=${encodeURIComponent(originalOn)}`, { method: 'DELETE' }),
  eventNotes: (id: string, originalOn?: string) => {
    const params = new URLSearchParams()
    if (originalOn) params.set('on', originalOn)
    const suffix = params.toString() ? `?${params}` : ''
    return request<EventNote[]>(`/api/events/${id}/notes${suffix}`)
  },
  createEventNote: (id: string, originalOn: string, body: string) =>
    request<EventNote>(`/api/events/${id}/notes`, { method: 'POST', body: JSON.stringify({ originalOn, body }) }),
  deleteEventNote: (id: string, noteId: string) => request(`/api/events/${id}/notes/${noteId}`, { method: 'DELETE' }),
  eventSeries: () => request<EventSeries[]>('/api/event-series'),
  people: () => request<Person[]>('/api/people'),
  person: (id: string) => request<Person>(`/api/people/${id}`),
  createPerson: (body: Record<string, unknown>) =>
    request<Person>('/api/people', { method: 'POST', body: JSON.stringify(body) }),
  patchPerson: (id: string, body: Record<string, unknown>) =>
    request<Person>(`/api/people/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deletePerson: (id: string) => request(`/api/people/${id}`, { method: 'DELETE' }),
  personNotes: (id: string) => request<PersonNote[]>(`/api/people/${id}/notes`),
  createPersonNote: (id: string, body: string) =>
    request<PersonNote>(`/api/people/${id}/notes`, { method: 'POST', body: JSON.stringify({ body }) }),
  deletePersonNote: (id: string, noteId: string) =>
    request(`/api/people/${id}/notes/${noteId}`, { method: 'DELETE' }),
  patchPersonNote: (id: string, noteId: string, body: string) =>
    request<PersonNote>(`/api/people/${id}/notes/${noteId}`, { method: 'PATCH', body: JSON.stringify({ body }) }),
  createPersonProfession: (id: string, body: Record<string, unknown>) =>
    request<PersonProfession>(`/api/people/${id}/professions`, { method: 'POST', body: JSON.stringify(body) }),
  patchPersonProfession: (id: string, professionId: string, body: Record<string, unknown>) =>
    request<PersonProfession>(`/api/people/${id}/professions/${professionId}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deletePersonProfession: (id: string, professionId: string) =>
    request(`/api/people/${id}/professions/${professionId}`, { method: 'DELETE' }),
  createPersonContact: (id: string, body: Record<string, unknown>) =>
    request<PersonContact>(`/api/people/${id}/contacts`, { method: 'POST', body: JSON.stringify(body) }),
  patchPersonContact: (id: string, contactId: string, body: Record<string, unknown>) =>
    request<PersonContact>(`/api/people/${id}/contacts/${contactId}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deletePersonContact: (id: string, contactId: string) =>
    request(`/api/people/${id}/contacts/${contactId}`, { method: 'DELETE' }),
  createPersonSite: (id: string, body: Record<string, unknown>) =>
    request<PersonSite>(`/api/people/${id}/sites`, { method: 'POST', body: JSON.stringify(body) }),
  patchPersonSite: (id: string, siteId: string, body: Record<string, unknown>) =>
    request<PersonSite>(`/api/people/${id}/sites/${siteId}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deletePersonSite: (id: string, siteId: string) =>
    request(`/api/people/${id}/sites/${siteId}`, { method: 'DELETE' }),
  createPersonBond: (id: string, body: Record<string, unknown>) =>
    request<PersonBond>(`/api/people/${id}/bonds`, { method: 'POST', body: JSON.stringify(body) }),
  patchPersonBond: (id: string, bondId: string, body: Record<string, unknown>) =>
    request<PersonBond>(`/api/people/${id}/bonds/${bondId}`, { method: 'PATCH', body: JSON.stringify(body) }),
  endPersonBond: (id: string, bondId: string, body: Record<string, unknown>) =>
    request<PersonBond>(`/api/people/${id}/bonds/${bondId}/end`, { method: 'POST', body: JSON.stringify(body) }),
  load: (from?: string, to?: string) => {
    const params = new URLSearchParams()
    if (from) params.set('from', from)
    if (to) params.set('to', to)
    const suffix = params.toString() ? `?${params}` : ''
    return request<LoadReport>(`/api/load${suffix}`)
  },
  schedule: (from: string, to: string, kind?: 'work' | 'followup') => {
    const params = new URLSearchParams({ from, to })
    if (kind) params.set('kind', kind)
    return request<Schedule>(`/api/schedule?${params}`)
  },
  scheduleSettings: () => request<ScheduleSettings>('/api/schedule/settings'),
  saveScheduleSettings: (body: ScheduleSettings) =>
    request<ScheduleSettings>('/api/schedule/settings', { method: 'PUT', body: JSON.stringify(body) }),
  dayOverrides: (from: string, to: string) =>
    request<DayOverride[]>(`/api/schedule/days?${new URLSearchParams({ from, to })}`),
  putDayOverride: (day: string, body: DayOverrideDraft) =>
    request<DayOverride>(`/api/schedule/days/${day}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteDayOverride: (day: string) => request(`/api/schedule/days/${day}`, { method: 'DELETE' }),
  scheduleWhatIf: (body: WhatIfScenario & { kind?: 'work' | 'followup'; from?: string; to?: string }) =>
    request<WhatIfResult>('/api/schedule/whatif', { method: 'POST', body: JSON.stringify(body) }),
  createPersonAbsence: (id: string, body: { startsOn: string; endsOn: string; note: string }) =>
    request<PersonAbsence>(`/api/people/${id}/absences`, { method: 'POST', body: JSON.stringify(body) }),
  updatePersonAbsence: (id: string, absenceId: string, body: { startsOn: string; endsOn: string; note: string }) =>
    request<PersonAbsence>(`/api/people/${id}/absences/${absenceId}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deletePersonAbsence: (id: string, absenceId: string) =>
    request(`/api/people/${id}/absences/${absenceId}`, { method: 'DELETE' }),
  journal: () => request<JournalEntry[]>('/api/journal'),
  companies: () => request<Company[]>('/api/companies'),
  company: (id: string) => request<Company>(`/api/companies/${id}`),
  createCompany: (body: Record<string, unknown>) =>
    request<Company>('/api/companies', { method: 'POST', body: JSON.stringify(body) }),
  patchCompany: (id: string, body: Record<string, unknown>) =>
    request<Company>(`/api/companies/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteCompany: (id: string) => request(`/api/companies/${id}`, { method: 'DELETE' }),
  addCompanyTitle: (id: string, body: { title: string; startedOn?: string }) =>
    request<Company>(`/api/companies/${id}/titles`, { method: 'POST', body: JSON.stringify(body) }),
  addCompanySalary: (id: string, body: { currency: string; amount: number; comment?: string; startedOn?: string }) =>
    request<Company>(`/api/companies/${id}/salaries`, { method: 'POST', body: JSON.stringify(body) }),
  setCompanyManager: (id: string, body: { personId: string | null; startedOn?: string }) =>
    request<Company>(`/api/companies/${id}/manager`, { method: 'POST', body: JSON.stringify(body) }),
  addCompanyReport: (id: string, body: { personId: string; startedOn?: string }) =>
    request<Company>(`/api/companies/${id}/reports`, { method: 'POST', body: JSON.stringify(body) }),
  endCompanyReport: (id: string, reportId: string) =>
    request<Company>(`/api/companies/${id}/reports/${reportId}`, { method: 'DELETE' }),
  setCompanyContract: (id: string, body: { personId?: string | null; kind: string; startedOn?: string }) =>
    request<Company>(`/api/companies/${id}/contracts`, { method: 'POST', body: JSON.stringify(body) }),
  companyNotes: (id: string) => request<CompanyNote[]>(`/api/companies/${id}/notes`),
  createCompanyNote: (id: string, body: string) =>
    request<CompanyNote>(`/api/companies/${id}/notes`, { method: 'POST', body: JSON.stringify({ body }) }),
  patchCompanyNote: (id: string, noteId: string, body: string) =>
    request<CompanyNote>(`/api/companies/${id}/notes/${noteId}`, { method: 'PATCH', body: JSON.stringify({ body }) }),
  deleteCompanyNote: (id: string, noteId: string) =>
    request(`/api/companies/${id}/notes/${noteId}`, { method: 'DELETE' }),
}
