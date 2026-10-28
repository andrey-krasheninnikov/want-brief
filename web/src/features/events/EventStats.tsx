import { hours } from '../../shared/format'
import { zonedYmd } from '../../shared/zoned'
import { eventTypeLabel, type EventOccurrence } from '../../types'
import { eventTone, type EventLookups } from './eventMeta'

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
const SHORT_MAX = 30 * 60

type Props = {
  rows: EventOccurrence[]
  lookups: EventLookups
  timeZone: string
}

type Bucket = {
  id: string
  name: string
  color?: string
  seconds: number
}

function weekdayMon0(iso: string, timeZone: string): number {
  const ymd = zonedYmd(iso, timeZone)
  const [year, month, day] = ymd.split('-').map(Number)
  return (new Date(Date.UTC(year, month - 1, day, 12)).getUTCDay() + 6) % 7
}

function activeSeconds(row: EventOccurrence): number {
  if (row.activeStartOffset == null || row.activeEndOffset == null) return row.durationSeconds
  const span = row.activeEndOffset - row.activeStartOffset
  return span > 0 ? span : row.durationSeconds
}

function hasPeak(row: EventOccurrence): boolean {
  return row.activeStartOffset != null && row.activeEndOffset != null
}

function involvementBucket(value: number): 'low' | 'mid' | 'high' {
  if (value <= 3) return 'low'
  if (value <= 7) return 'mid'
  return 'high'
}

function add(map: Map<string, Bucket>, id: string, name: string, seconds: number, color = '') {
  const current = map.get(id) ?? { id, name, color, seconds: 0 }
  current.seconds += seconds
  map.set(id, current)
}

function sorted(map: Map<string, Bucket>): Bucket[] {
  return [...map.values()].sort((a, b) => b.seconds - a.seconds || a.name.localeCompare(b.name))
}

function Bar({ name, seconds, max, color }: { name: string; seconds: number; max: number; color?: string }) {
  const pct = Math.round((seconds / max) * 100)
  return (
    <li className="event-stats-row">
      <span>{name}</span>
      <strong>{hours(seconds)}</strong>
      <span className="event-stats-track">
        <i style={{ width: `${pct}%`, ...eventTone(color ?? '') }} />
      </span>
    </li>
  )
}

function Bars({ rows }: { rows: Bucket[] }) {
  const max = Math.max(1, ...rows.map((row) => row.seconds))
  return (
    <ul>
      {rows.map((row) => (
        <Bar key={row.id} name={row.name} seconds={row.seconds} max={max} color={row.color} />
      ))}
    </ul>
  )
}

export function EventStats({ rows, lookups, timeZone }: Props) {
  const totalSeconds = rows.reduce((sum, row) => sum + row.durationSeconds, 0)
  const shorts = rows.filter((row) => row.durationSeconds <= SHORT_MAX)
  const shortSeconds = shorts.reduce((sum, row) => sum + row.durationSeconds, 0)
  const loadSeconds = rows.reduce((sum, row) => sum + (activeSeconds(row) * row.involvement) / 10, 0)
  const invWeight = rows.reduce((sum, row) => sum + row.involvement * row.durationSeconds, 0)
  const avgInv = totalSeconds > 0 ? invWeight / totalSeconds : 0

  const byProject = new Map<string, Bucket>()
  const byType = new Map<string, Bucket>()
  const skip = new Map<string, Bucket>([
    ['required', { id: 'required', name: 'Required', seconds: 0 }],
    ['skip', { id: 'skip', name: 'Can skip', seconds: 0 }],
  ])
  const involvement = new Map<string, Bucket>([
    ['low', { id: 'low', name: 'Low 0–3', seconds: 0 }],
    ['mid', { id: 'mid', name: 'Mid 4–7', seconds: 0 }],
    ['high', { id: 'high', name: 'High 8–10', seconds: 0 }],
  ])
  const peak = new Map<string, Bucket>([
    ['on', { id: 'on', name: 'Peak on', seconds: 0 }],
    ['off', { id: 'off', name: 'Full slot', seconds: 0 }],
  ])
  const days = WEEKDAYS.map((name) => ({ id: name, name, seconds: 0 }))

  for (const row of rows) {
    const project = row.projectId ? lookups.projectById.get(row.projectId) : undefined
    add(byProject, project?.id ?? 'none', project?.name ?? 'No project', row.durationSeconds, project?.color ?? '')
    add(byType, row.type, eventTypeLabel(row.type), row.durationSeconds)
    add(skip, row.canSkip ? 'skip' : 'required', row.canSkip ? 'Can skip' : 'Required', row.durationSeconds)
    const inv = involvementBucket(row.involvement)
    add(involvement, inv, involvement.get(inv)!.name, row.durationSeconds)
    add(peak, hasPeak(row) ? 'on' : 'off', hasPeak(row) ? 'Peak on' : 'Full slot', row.durationSeconds)
    days[weekdayMon0(row.startsAt, timeZone)].seconds += row.durationSeconds
  }

  const projects = sorted(byProject)
  const types = sorted(byType)

  return (
    <div className="event-stats">
      <div className="event-stats-kpis">
        <p>
          <span>Hours</span>
          <strong>{hours(totalSeconds)}</strong>
        </p>
        <p>
          <span>Slots</span>
          <strong>{rows.length}</strong>
        </p>
        <p>
          <span>Load</span>
          <strong>{hours(loadSeconds)}</strong>
        </p>
        <p>
          <span>Avg inv</span>
          <strong>{avgInv.toFixed(1)}</strong>
        </p>
      </div>
      <section>
        <h3>By project</h3>
        {projects.length === 0 ? <p className="muted">No events in this range.</p> : <Bars rows={projects} />}
      </section>
      <section>
        <h3>By type</h3>
        {types.length === 0 ? <p className="muted">No events in this range.</p> : <Bars rows={types} />}
      </section>
      <section>
        <h3>Skip</h3>
        <Bars rows={[skip.get('required')!, skip.get('skip')!]} />
      </section>
      <section>
        <h3>Involvement</h3>
        <Bars rows={[involvement.get('low')!, involvement.get('mid')!, involvement.get('high')!]} />
      </section>
      <section>
        <h3>Peak</h3>
        <Bars rows={[peak.get('on')!, peak.get('off')!]} />
      </section>
      <section>
        <h3>By weekday</h3>
        <Bars rows={days} />
      </section>
      <section>
        <h3>Short ≤30m</h3>
        <p className="event-stats-short">
          <strong>{shorts.length}</strong>
          <span>{hours(shortSeconds)}</span>
        </p>
      </section>
    </div>
  )
}
