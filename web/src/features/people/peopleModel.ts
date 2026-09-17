import type { BondKind, Person, PersonContact, PersonProfession, PersonRel, SalaryPeriod, SiteKind } from '../../types'

export const LAST_PERSON_KEY = 'want-people-last-id'
export const PERSON_DRAG = 'application/x-want-item'

export const BOND_KINDS: { id: BondKind; label: string }[] = [
  { id: 'acquaintance', label: 'Acquaintance' },
  { id: 'colleague', label: 'Colleague' },
  { id: 'comrade', label: 'Comrade' },
  { id: 'friend', label: 'Friend' },
  { id: 'relative', label: 'Relative' },
  { id: 'spouse', label: 'Spouse' },
  { id: 'adversary', label: 'Adversary' },
  { id: 'other', label: 'Other' },
]

export const SITE_KINDS: { id: SiteKind; label: string }[] = [
  { id: 'personal_site', label: 'Personal site' },
  { id: 'company_site', label: 'Company site' },
  { id: 'github', label: 'GitHub' },
  { id: 'linkedin', label: 'LinkedIn' },
  { id: 'youtube', label: 'YouTube' },
  { id: 'telegram_channel', label: 'Telegram channel' },
  { id: 'other', label: 'Other' },
]

export function bondKindLabel(kind: BondKind): string {
  return BOND_KINDS.find((row) => row.id === kind)?.label ?? kind
}

export function siteKindLabel(kind: SiteKind): string {
  return SITE_KINDS.find((row) => row.id === kind)?.label ?? kind
}

export function currentSalary(profession: PersonProfession): SalaryPeriod | undefined {
  return (profession.salaries ?? []).find((row) => !row.endedOn)
}

export function salaryText(period?: SalaryPeriod | null): string {
  if (!period) return ''
  const parts: string[] = []
  if (period.monthlySalaryUsd) parts.push(`$${period.monthlySalaryUsd}`)
  if (period.monthlySalaryRub) parts.push(`₽${period.monthlySalaryRub}`)
  return parts.join(' · ')
}

export function professionLabel(person: Person): string {
  return (person.professions ?? [])
    .filter((row) => !row.endedOn)
    .map((row) => row.title)
    .filter(Boolean)
    .join(' · ')
}

export function relLabels(rels: PersonRel[] | undefined, names: Map<string, string>): string {
  return (rels ?? []).map((rel) => names.get(rel.id)).filter((name): name is string => Boolean(name)).join(' · ')
}

export function dateOnly(iso: string | null | undefined): string {
  if (!iso) return ''
  return iso.slice(0, 10)
}

export function dateRange(startedOn: string, endedOn: string | null): string {
  const start = dateOnly(startedOn)
  const end = dateOnly(endedOn)
  return end ? `${start} – ${end}` : start
}

export function primaryContact(person: { contacts?: PersonContact[] }): PersonContact | null {
  const contacts = person.contacts ?? []
  return contacts.find((row) => row.kind === 'phone') ?? contacts.find((row) => row.kind === 'telegram') ?? contacts[0] ?? null
}

export function personMatches(person: Person, raw: string): boolean {
  const needle = raw.trim().toLowerCase()
  if (!needle) return true
  const hay = [
    person.name,
    professionLabel(person),
    ...(person.professions ?? []).map((row) => row.title),
    person.meBond?.kind ?? '',
    ...(person.contacts ?? []).flatMap((row) => [row.label, row.value]),
    ...(person.sites ?? []).map((row) => row.url),
  ]
    .join(' ')
    .toLowerCase()
  return hay.includes(needle)
}

export async function copyText(value: string): Promise<boolean> {
  if (!value.trim()) return false
  try {
    await navigator.clipboard.writeText(value)
    return true
  } catch {
    return false
  }
}

export function rememberPerson(id: string) {
  sessionStorage.setItem(LAST_PERSON_KEY, id)
}

export function rememberedPerson(ids: string[]): string | null {
  const last = sessionStorage.getItem(LAST_PERSON_KEY)
  if (last && ids.includes(last)) return last
  return ids[0] ?? null
}

export function typingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable
}
