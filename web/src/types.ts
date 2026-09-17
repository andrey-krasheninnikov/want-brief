export type ItemKind =
  | 'task'
  | 'note'
  | 'agreement'
  | 'obligation'
  | 'initiative'
  | 'life'

export type Occupancy = 'solo' | 'parallel' | 'waiting'

export type ItemStatus =
  | 'backlog'
  | 'needs_grooming'
  | 'to_do'
  | 'in_progress'
  | 'blocked'
  | 'review'
  | 'qa'
  | 'awaiting_decision'
  | 'release_candidate'
  | 'done'
  | 'cancelled'
export type Quadrant = 'do' | 'schedule' | 'delegate' | 'drop'
export type SourceKind = 'jira' | 'todoist' | 'manual'

export type ProjectLink = {
  label: string
  url: string
}

export type PersonRel = {
  id: string
  comment: string
}

export type ContactKind = 'phone' | 'telegram' | 'url'
export type SiteKind = 'personal_site' | 'company_site' | 'github' | 'linkedin' | 'youtube' | 'telegram_channel' | 'other'
export type BondKind = 'acquaintance' | 'colleague' | 'comrade' | 'friend' | 'relative' | 'spouse' | 'adversary' | 'other'
export type BondAction = 'open' | 'change' | 'end'

export type PersonContact = {
  id: string
  personId: string
  kind: ContactKind
  label: string
  value: string
  note: string
  createdAt: string
  updatedAt: string
}

export type PersonSite = {
  id: string
  personId: string
  kind: SiteKind
  url: string
  comment: string
  createdAt: string
  updatedAt: string
}

export type PersonBondEvent = {
  id: string
  bondId: string
  action: BondAction
  kind: BondKind
  comment: string
  startedOn: string
  changedOn: string
  endedOn: string | null
  at: string
}

export type PersonBond = {
  id: string
  personAId: string | null
  personBId: string
  otherId: string | null
  otherName: string
  kind: BondKind
  comment: string
  startedOn: string
  changedOn: string
  endedOn: string | null
  events: PersonBondEvent[]
  createdAt: string
  updatedAt: string
}

export type MeBond = {
  id: string
  kind: BondKind
}

export type SalaryPeriod = {
  id: string
  professionId: string
  startedOn: string
  endedOn: string | null
  monthlySalaryUsd: number
  monthlySalaryRub: number
}

export type PersonProfession = {
  id: string
  personId: string
  title: string
  comment: string
  startedOn: string
  endedOn: string | null
  salaries: SalaryPeriod[]
  createdAt: string
  updatedAt: string
}

export type Project = {
  id: string
  name: string
  color: string
  description: string
  monthlyIncomeUsd: number
  monthlyIncomeRub: number
  targetHoursDay: number
  targetHoursWeek: number
  links: ProjectLink[]
  people: PersonRel[]
  companies: PersonRel[]
  archivedAt: string | null
  createdAt: string
  updatedAt: string
}

export type Person = {
  id: string
  name: string
  bornOn: string | null
  age: number | null
  projects: PersonRel[]
  events: PersonRel[]
  companies: PersonRel[]
  itemIds: string[]
  contacts: PersonContact[]
  sites: PersonSite[]
  bonds: PersonBond[]
  professions: PersonProfession[]
  absences: PersonAbsence[]
  meBond: MeBond | null
  lastNoteAt: string | null
  createdAt: string
  updatedAt: string
}

export type PersonNote = {
  id: string
  personId: string
  body: string
  createdAt: string
}

export type SalaryCurrency = 'usd' | 'rub'
export type ContractKind = 'informal' | 'gph' | 'ip' | 'labor' | 'contract'

export type Tenure = {
  years: number
  months: number
}

export type CompanyTitle = {
  id: string
  companyId: string
  title: string
  startedOn: string
  endedOn: string | null
}

export type CompanySalary = {
  id: string
  companyId: string
  currency: SalaryCurrency
  amount: number
  comment: string
  startedOn: string
  endedOn: string | null
}

export type CompanyManager = {
  id: string
  companyId: string
  personId: string
  startedOn: string
  endedOn: string | null
}

export type CompanyReport = {
  id: string
  companyId: string
  personId: string
  startedOn: string
  endedOn: string | null
}

export type CompanyContract = {
  id: string
  companyId: string
  personId: string | null
  kind: ContractKind
  startedOn: string
  endedOn: string | null
}

export type Company = {
  id: string
  name: string
  description: string
  links: ProjectLink[]
  projects: PersonRel[]
  events: PersonRel[]
  people: PersonRel[]
  startedOn: string | null
  endedOn: string | null
  tenure: Tenure | null
  titles: CompanyTitle[]
  salaries: CompanySalary[]
  managers: CompanyManager[]
  reports: CompanyReport[]
  contracts: CompanyContract[]
  createdAt: string
  updatedAt: string
}

export type CompanyNote = {
  id: string
  companyId: string
  body: string
  createdAt: string
}

export type PersonAbsence = {
  id: string
  personId: string
  startsOn: string
  endsOn: string
  note: string
  createdAt: string
  updatedAt: string
}

export type ProjectNote = {
  id: string
  projectId: string
  body: string
  createdAt: string
}

export type Source = {
  id: string
  kind: SourceKind
  name: string
  baseUrl: string
  hasToken: boolean
  email?: string
  query: string
  projectId: string | null
	connected: boolean
	insecureTls?: boolean
	lastError?: string
  lastSyncAt: string | null
}

export type Item = {
  id: string
  sourceId: string
  externalKey: string
  title: string
  status: ItemStatus
  occupancy: Occupancy
  externalStatus: string
  kind: ItemKind
  projectId: string | null
  urgent: boolean
  important: boolean
  pinned: boolean
  pinnedAt: string | null
  stress: number | null
  dueAt: string | null
  devDueAt: string | null
  reviewDueAt: string | null
  testDueAt: string | null
  description: string
  plannedSeconds: number
  links: ProjectLink[]
  trackedSeconds: number
  archivedAt: string | null
  deletedAt: string | null
  createdAt: string
  updatedAt: string
  sourceName: string
  sourceKind: SourceKind
  projectName: string
  projectColor: string
  quadrant: Quadrant
  personIds: string[]
  checkTotal: number
  checkDone: number
}

export type CheckinKind = 'stress' | 'focus' | 'energy' | 'interest' | 'happiness'

export type LatestCheckins = {
  stress: number
  focus: number
  energy: number
  interest: number
  happiness: number
}

export type EventKind = 'event' | 'call'
export type EventType =
  | 'sync'
  | 'grooming'
  | 'lesson'
  | 'mentorship'
  | 'planning'
  | 'daily'
  | 'retro'
  | 'one_on_one'
  | 'global'
  | 'team_building'
  | 'external'
export type EventRecurrence = 'once' | 'weekly' | 'monthly'

export type CalendarEvent = EventOccurrence

export type EventOccurrence = {
  id: string
  seriesId: string
  title: string
  description: string
  agenda: string
  kind: EventKind
  type: EventType
  projectId: string | null
  startsAt: string
  endsAt: string
  durationSeconds: number
  recurrence: EventRecurrence
  originalOn: string
  overridden: boolean
  skipped?: boolean
  links: ProjectLink[]
  meetUrl: string
  involvement: number
  activeStartOffset: number | null
  activeEndOffset: number | null
  canSkip: boolean
  people: PersonRel[]
  createdAt: string
}

export type EventSeries = {
  id: string
  title: string
  description: string
  agenda: string
  kind: EventKind
  type: EventType
  projectId: string | null
  startsAt: string
  durationSeconds: number
  recurrence: EventRecurrence
  repeatUntil: string | null
  weekdays: number[]
  links: ProjectLink[]
  meetUrl: string
  involvement: number
  activeStartOffset: number | null
  activeEndOffset: number | null
  canSkip: boolean
  people: PersonRel[]
  createdAt: string
  updatedAt: string
}

export const EVENT_TYPES: EventType[] = [
  'sync',
  'grooming',
  'lesson',
  'mentorship',
  'planning',
  'daily',
  'retro',
  'one_on_one',
  'global',
  'team_building',
  'external',
]

export function eventTypeLabel(type: EventType): string {
  switch (type) {
    case 'one_on_one':
      return '1-1'
    case 'team_building':
      return 'Team-building'
    case 'external':
      return 'External'
    default:
      return type.replaceAll('_', ' ')
  }
}

const WEEKDAY_NAMES = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

export function weekdaysFromStart(iso: string): number[] {
  const short = new Intl.DateTimeFormat('en-US', { weekday: 'short', timeZone: 'Europe/Moscow' }).format(new Date(iso))
  const index = WEEKDAY_NAMES.indexOf(short)
  if (index < 0) return [0, 7]
  return [index, index + 7]
}

export function recurrenceLabel(recurrence: EventRecurrence, weekdays: number[] = []): string {
  if (recurrence === 'once') return 'Once'
  if (recurrence === 'monthly') return 'Monthly'
  const week1 = weekdays.filter((slot) => slot < 7).sort((a, b) => a - b)
  const week2 = weekdays.filter((slot) => slot >= 7).map((slot) => slot - 7).sort((a, b) => a - b)
  const fmt = (slots: number[]) => slots.map((slot) => WEEKDAY_NAMES[slot]).join(', ')
  const same = week1.length === week2.length && week1.every((slot, i) => slot === week2[i])
  if (same && week1.length) return `Weekly ${fmt(week1)}`
  const parts: string[] = []
  if (week1.length) parts.push(`W1 ${fmt(week1)}`)
  if (week2.length) parts.push(`W2 ${fmt(week2)}`)
  return parts.length ? parts.join(' ') : 'Weekly'
}

export type EventNote = {
  id: string
  seriesId: string
  originalOn: string
  body: string
  createdAt: string
}

export type ItemCheck = {
  id: string
  itemId: string
  body: string
  done: boolean
  position: number
  createdAt: string
  updatedAt: string
}

export type ItemNote = {
  id: string
  itemId: string
  body: string
  sourceKind: SourceKind
  externalId: string
  authorName: string
  url: string
  createdAt: string
}

export type ItemEventKind = 'status' | 'field' | 'timer_start' | 'timer_stop' | 'timer_log' | 'check'

export type TaskLogKind = 'status' | 'timer_stop'

export type ItemEvent = {
  id: string
  itemId: string
  kind: ItemEventKind
  field: string
  from: string
  to: string
  note: string
  stress: number | null
  createdAt: string
}

export type Note = {
  id: string
  itemId: string | null
  body: string
  updatedAt: string
}

export type TimeInterval = {
  id: string
  itemId: string
  startedAt: string
  endedAt: string | null
}

export type LoadReport = {
  from: string
  to: string
  allocatedSeconds: number
  wallSeconds: number
  averageStress: number | null
  averages: {
    stress: number | null
    focus: number | null
    energy: number | null
    interest: number | null
    happiness: number | null
  }
  byProject: {
    projectId: string | null
    name: string
    color: string
    targetHoursWeek: number
    allocatedSeconds: number
  }[]
  byItem: {
    itemId: string
    title: string
    kind: ItemKind
    projectName: string
    allocatedSeconds: number
  }[]
  byDay: { date: string; allocatedSeconds: number; wallSeconds: number }[]
  stress: { id: string; kind: CheckinKind; level: number; loggedAt: string; itemId: string | null }[]
}

export type JournalSource = 'loose' | 'project' | 'item' | 'person'

export type JournalEntry = {
  id: string
  source: JournalSource
  ownerId: string | null
  ownerName: string
  body: string
  createdAt: string
}

export type ScheduleBlockKind = 'work' | 'ping'

export type ScheduleBlock = {
  itemId: string
  title: string
  externalKey: string
  kind: ScheduleBlockKind
  startsAt: string
  endsAt: string
  late: boolean
  continued: boolean
  continues: boolean
  lane: number
  occupancy: Occupancy
  pinned: boolean
  quadrant: Quadrant
  dueAt: string
  stress: number | null
  remainingSeconds: number
  estimateFactor: number
  reasons: string[]
  people?: SchedulePerson[]
}

export type SchedulePerson = {
  id: string
  name: string
}

export type ScheduleLane = {
  projectId: string | null
  projectName: string
  projectColor: string
  blocks: ScheduleBlock[]
}

export type ScheduleBusy = {
  startsAt: string
  endsAt: string
  title: string
  soft: boolean
  seriesId?: string
  originalOn?: string
  activeStartsAt?: string
  activeEndsAt?: string
  people?: SchedulePerson[]
  canSkip?: boolean
}

export type UnplannedItem = {
  item: Item
  missingDevDue: boolean
  missingPlan: boolean
}

export type ScheduleOverflow = {
  itemCount: number
  seconds: number
  firstDueAt: string | null
}

export type ScheduleCapacity = {
  freeSeconds: number
  packedSeconds: number
  busySeconds: number
}

export type ScheduleGrid = {
  timezone: string
  startHour: number
  endHour: number
  workdays: number[]
  workStartMin: number
  workEndMin: number
}

export type AtRiskItem = {
  itemId: string
  key: string
  title: string
  slackSeconds: number
}

export type ScheduleScore = {
  lateSeconds: number
  fragments: number
  switches: number
  loadVariance: number
  total: number
}

export type Schedule = {
  lanes: ScheduleLane[]
  unplanned: UnplannedItem[]
  busy: ScheduleBusy[]
  overflow: ScheduleOverflow
  capacity: ScheduleCapacity
  grid: ScheduleGrid
  atRisk: AtRiskItem[]
  score: ScheduleScore
}

export type BreakWindow = {
  startMin: number
  endMin: number
}

export type ScheduleSettings = {
  timezone: string
  workStartMin: number
  workEndMin: number
  workdays: number[]
  break: BreakWindow | null
  meetingBufferMin: number
  dailyFocusMin: number
  maxTasksPerDay: number
  minSliceMin: number
  maxSliceMin: number
  sliceBreakMin: number
  gridMin: number
  estimateBuffer: boolean
  estimateMaxK: number
  oldestFirst: boolean
  stressShiftHours: number
  targetLeadWorkdays: number
  softBusy: boolean
  waitingTracks: number
  followupPingMin: number
  checkWindows: string[]
  energyAware: boolean
  goldenHours: boolean
  stabilityThresholdMin: number
}

export type DayOverride = {
  day: string
  off: boolean
  workStartMin: number | null
  workEndMin: number | null
  note: string
  updatedAt: string
}

export type DayOverrideDraft = {
  off: boolean
  workStartMin: number | null
  workEndMin: number | null
  note: string
}

export type WhatIfScenario = {
  moveDue: { itemId: string; dueAt: string }[]
  dropItems: string[]
  skipEvents: string[]
}

export type WhatIfSummary = {
  lateSeconds: number
  overflowItems: number
  overflowSeconds: number
  atRisk: number
  fragments: number
  switches: number
  score: number
}

export type WhatIfResult = {
  base: WhatIfSummary
  variant: WhatIfSummary
  delta: WhatIfSummary
}

export const SCHEDULE_REASON_LABELS: Record<string, string> = {
  pinned: 'Pinned',
  pinned_at: 'Pinned to time',
  active: 'Tracking now',
  in_progress: 'In progress',
  no_slack: 'No slack',
  due_soon: 'Due soon',
  same_project: 'Same project',
  sticky: 'Kept in place',
  golden_hour: 'Golden hour',
  light_task: 'Light task',
  soft_busy: 'Over skippable meeting',
  person_away: 'Waits for',
  person_busy: 'In meeting with',
  estimate: 'Estimate',
  pushed_by: 'Pushed by',
}

export function scheduleReasonLabel(reason: string): string {
  const [head, ...rest] = reason.split(':')
  const tail = rest.join(':')
  if (head.startsWith('estimate_x')) return `Estimate ×${head.slice('estimate_x'.length)}`
  const label = SCHEDULE_REASON_LABELS[head] ?? head.replaceAll('_', ' ')
  return tail ? `${label} ${tail}` : label
}

export const KINDS: ItemKind[] = [
  'task',
  'note',
  'agreement',
  'obligation',
  'initiative',
  'life',
]

export const ITEM_STATUSES: ItemStatus[] = [
  'backlog',
  'needs_grooming',
  'to_do',
  'in_progress',
  'blocked',
  'review',
  'qa',
  'awaiting_decision',
  'release_candidate',
  'done',
  'cancelled',
]

export function statusesForKind(kind: ItemKind): ItemStatus[] {
  if (kind === 'task') return ITEM_STATUSES
  return ['backlog', 'needs_grooming', 'to_do', 'in_progress', 'blocked', 'done', 'cancelled']
}

export function kindLabel(kind: ItemKind): string {
  if (kind === 'task') return 'IT-ticket'
  return kind
}

export const OCCUPANCIES: Occupancy[] = ['solo', 'parallel', 'waiting']

export function occupancyLabel(occupancy: Occupancy): string {
  if (occupancy === 'parallel') return 'Parallel'
  if (occupancy === 'waiting') return 'Waiting'
  return 'Solo'
}

export function sourceKindLabel(kind: SourceKind): string {
  if (kind === 'jira') return 'Jira'
  if (kind === 'todoist') return 'Todoist'
  return 'Manual'
}

const STATUS_LABELS: Record<ItemStatus, string> = {
  backlog: 'Backlog',
  needs_grooming: 'Needs grooming',
  to_do: 'To do',
  in_progress: 'In progress',
  blocked: 'Blocked',
  review: 'Review',
  qa: 'QA',
  awaiting_decision: 'Awaiting decision',
  release_candidate: 'Release candidate',
  done: 'Done',
  cancelled: 'Cancelled',
}

export function statusLabel(status: ItemStatus): string {
  return STATUS_LABELS[status] ?? status.replaceAll('_', ' ')
}
