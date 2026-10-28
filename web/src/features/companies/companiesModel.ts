import type { Company, CompanySalary, ContractKind, Tenure } from '../../types'
import { moscowYmd } from '../../shared/moscow'

export type CareerLogKind = 'title' | 'salary' | 'manager' | 'report' | 'papers'

export type CareerLogEntry = {
  key: string
  at: string
  kind: CareerLogKind
  who: string
  note: string
}

const KIND_LABEL: Record<CareerLogKind, string> = {
  title: 'Title',
  salary: 'Salary',
  manager: 'Manager',
  report: 'Report',
  papers: 'Papers',
}

export const LAST_COMPANY_KEY = 'want-company-last-id'

export const CONTRACT_KINDS: { id: ContractKind; label: string }[] = [
  { id: 'informal', label: 'Informal' },
  { id: 'gph', label: 'GPH' },
  { id: 'ip', label: 'Sole prop' },
  { id: 'labor', label: 'Labor code' },
  { id: 'contract', label: 'Contract' },
]

export function contractKindLabel(kind: ContractKind): string {
  return CONTRACT_KINDS.find((row) => row.id === kind)?.label ?? kind
}

export function dateOnly(iso: string | null | undefined): string {
  if (!iso) return ''
  return iso.slice(0, 10)
}

export function spanYearsMonths(start: string, end: string): Tenure {
  const [sy, sm, sd] = start.split('-').map(Number)
  const [ey, em, ed] = end.split('-').map(Number)
  if (!sy || !sm || !sd || !ey || !em || !ed) return { years: 0, months: 0 }
  let years = ey - sy
  let months = em - sm
  if (ed < sd) months -= 1
  if (months < 0) {
    years -= 1
    months += 12
  }
  if (years < 0) return { years: 0, months: 0 }
  return { years, months }
}

export function liveTenure(startedOn: string, endedOn: string, today = moscowYmd()): Tenure | null {
  if (!startedOn) return null
  return spanYearsMonths(startedOn, endedOn || today)
}

export function tenureText(row?: Tenure | null): string {
  if (!row) return ''
  return `${row.years}y ${row.months}m`
}

export function careerKindLabel(kind: CareerLogKind): string {
  return KIND_LABEL[kind]
}

export function dayLabel(iso: string): string {
  const day = dateOnly(iso)
  if (!day) return ''
  return new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', year: 'numeric' }).format(
    new Date(`${day}T00:00:00`),
  )
}

export function dayShort(iso: string): string {
  const day = dateOnly(iso)
  if (!day) return ''
  return new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short' }).format(new Date(`${day}T00:00:00`))
}

export function salaryDelta(salaries: CompanySalary[] | undefined): { text: string; up: boolean } | null {
  const current = openOf(salaries)
  if (!current) return null
  const previous = (salaries ?? [])
    .filter((row) => row.id !== current.id && row.currency === current.currency && row.endedOn)
    .sort(
      (a, b) =>
        dateOnly(b.endedOn).localeCompare(dateOnly(a.endedOn)) || dateOnly(b.startedOn).localeCompare(dateOnly(a.startedOn)),
    )[0]
  if (!previous || previous.amount <= 0) return { text: 'First', up: false }
  const pct = Math.round(((current.amount - previous.amount) / previous.amount) * 100)
  if (pct === 0) return { text: '0%', up: false }
  return { text: `${pct > 0 ? '+' : ''}${pct}%`, up: pct > 0 }
}

export function papersMix(company: Company): { kind: ContractKind; label: string; count: number }[] {
  const counts = new Map<ContractKind, number>()
  for (const row of company.contracts ?? []) {
    if (row.endedOn) continue
    counts.set(row.kind, (counts.get(row.kind) ?? 0) + 1)
  }
  return CONTRACT_KINDS.map((row) => ({
    kind: row.id,
    label: row.label,
    count: counts.get(row.id) ?? 0,
  })).filter((row) => row.count > 0)
}

export function periodShare(startedOn: string, endedOn: string | null, from: string, to: string): number {
  const start = Date.parse(`${dateOnly(startedOn)}T00:00:00Z`)
  const stop = Date.parse(`${dateOnly(endedOn || to)}T00:00:00Z`)
  const lo = Date.parse(`${dateOnly(from)}T00:00:00Z`)
  const hi = Date.parse(`${dateOnly(to)}T00:00:00Z`)
  if (!Number.isFinite(start) || !Number.isFinite(stop) || !Number.isFinite(lo) || !Number.isFinite(hi)) return 8
  const span = Math.max(1, hi - lo)
  return Math.min(100, Math.max(8, ((Math.max(stop, start) - start) / span) * 100))
}

export function careerLog(company: Company, names: Map<string, string>): CareerLogEntry[] {
  const who = (id: string | null | undefined) => (id ? names.get(id) ?? id : 'Me')
  const rows: CareerLogEntry[] = [
    ...(company.titles ?? []).map((row) => ({
      key: row.id,
      at: row.startedOn,
      kind: 'title' as const,
      who: 'Me',
      note: row.title,
    })),
    ...(company.salaries ?? []).map((row) => ({
      key: row.id,
      at: row.startedOn,
      kind: 'salary' as const,
      who: 'Me',
      note: [companySalaryText(row), row.comment.trim()].filter(Boolean).join(' · '),
    })),
    ...(company.managers ?? []).map((row) => ({
      key: row.id,
      at: row.startedOn,
      kind: 'manager' as const,
      who: who(row.personId),
      note: '',
    })),
    ...(company.reports ?? []).map((row) => ({
      key: row.id,
      at: row.startedOn,
      kind: 'report' as const,
      who: who(row.personId),
      note: '',
    })),
    ...(company.contracts ?? []).map((row) => ({
      key: row.id,
      at: row.startedOn,
      kind: 'papers' as const,
      who: who(row.personId),
      note: contractKindLabel(row.kind),
    })),
  ]
  return rows.sort((a, b) => dateOnly(b.at).localeCompare(dateOnly(a.at)) || b.at.localeCompare(a.at) || b.key.localeCompare(a.key))
}

export function openOf<T extends { endedOn: string | null }>(rows: T[] | undefined): T | undefined {
  return (rows ?? []).find((row) => !row.endedOn)
}

export function companySalaryText(row?: CompanySalary | null): string {
  if (!row) return ''
  const locale = row.currency === 'rub' ? 'ru-RU' : 'en-US'
  const code = row.currency === 'rub' ? 'RUB' : 'USD'
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(row.amount)} ${code}`
}

export function companyMatches(company: Company, query: string): boolean {
  const needle = query.trim().toLowerCase()
  if (!needle) return true
  const titles = (company.titles ?? []).map((row) => row.title).join(' ')
  return `${company.name} ${company.description} ${titles}`.toLowerCase().includes(needle)
}

export function rememberCompany(id: string) {
  sessionStorage.setItem(LAST_COMPANY_KEY, id)
}

export function rememberedCompany(ids: string[]): string | null {
  const last = sessionStorage.getItem(LAST_COMPANY_KEY)
  if (last && ids.includes(last)) return last
  return ids[0] ?? null
}

export function companyInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase()
}
