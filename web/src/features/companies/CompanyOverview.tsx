import type { Company, CompanySalary, CompanyTitle, ContractKind, Person } from '../../types'
import { moscowYmd } from '../../shared/moscow'
import { CareerLogTable } from './CompanyLog'
import {
  careerLog,
  companySalaryText,
  dateOnly,
  dayShort,
  openOf,
  papersMix,
  periodShare,
  salaryDelta,
  tenureText,
} from './companiesModel'

type Props = {
  company: Company
  people: Person[]
}

type Point = { x: number; y: number }

const PAD = 90 * 86400000
const SLICE: Record<ContractKind, string> = {
  informal: 'companies-slice-0',
  gph: 'companies-slice-1',
  ip: 'companies-slice-2',
  labor: 'companies-slice-3',
  contract: 'companies-slice-4',
}

function dayMs(iso: string): number {
  return new Date(`${dateOnly(iso)}T00:00:00Z`).getTime()
}

function axisOf(salaries: CompanySalary[], startedOn?: string | null): { from: number; to: number } {
  const stamps = [
    ...salaries.map((row) => row.startedOn),
    ...salaries.map((row) => row.endedOn ?? moscowYmd()),
    startedOn,
  ].filter(Boolean) as string[]
  const today = dayMs(moscowYmd())
  if (stamps.length === 0) return { from: today - PAD, to: today }
  const times = stamps.map(dayMs)
  const to = Math.max(...times, today)
  return { from: Math.min(to - PAD, ...times), to }
}

function xAt(at: number, from: number, to: number): number {
  const span = Math.max(1, to - from)
  return 56 + ((at - from) / span) * 600
}

function salarySteps(rows: CompanySalary[], currency: CompanySalary['currency'], from: number, to: number, max: number): Point[] {
  const series = rows.filter((row) => row.currency === currency).sort((a, b) => a.startedOn.localeCompare(b.startedOn))
  const points: Point[] = []
  let cursor = -Infinity
  for (const row of series) {
    const y = 18 + ((max - row.amount) / Math.max(max, 1)) * 148
    let start = xAt(dayMs(row.startedOn), from, to)
    let end = xAt(dayMs(row.endedOn ?? moscowYmd()), from, to)
    start = Math.max(start, cursor)
    end = Math.max(end, start + 36)
    cursor = end
    points.push({ x: start, y }, { x: end, y })
  }
  return points
}

function linePath(points: Point[]): string {
  if (points.length === 0) return ''
  return points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')
}

function areaPath(points: Point[]): string {
  if (points.length === 0) return ''
  const first = points[0]
  const last = points[points.length - 1]
  return `${linePath(points)} L ${last.x.toFixed(1)} 166 L ${first.x.toFixed(1)} 166 Z`
}

function SalaryChart({ salaries }: { salaries: CompanySalary[] }) {
  if (salaries.length === 0) {
    return (
      <div className="companies-empty">
        <p className="muted">No salary history yet.</p>
        <p className="muted">Save a salary to draw the step.</p>
      </div>
    )
  }
  const { from, to } = axisOf(salaries)
  const usdMax = Math.max(1, ...salaries.filter((row) => row.currency === 'usd').map((row) => row.amount))
  const rubMax = Math.max(1, ...salaries.filter((row) => row.currency === 'rub').map((row) => row.amount))
  const usd = salarySteps(salaries, 'usd', from, to, usdMax)
  const rub = salarySteps(salaries, 'rub', from, to, rubMax)
  const ticks = [0, 1, 2, 3, 4]
  const scale = usd.length ? usdMax : rubMax
  return (
    <svg className="companies-svg" viewBox="0 0 712 196" role="img" aria-label="Salary over time">
      {ticks.map((step) => {
        const y = 18 + (step / 4) * 148
        const value = Math.round(scale * (1 - step / 4))
        return (
          <g key={step}>
            <line x1="56" x2="656" y1={y} y2={y} className="companies-grid" />
            <text x="8" y={y + 4} className="companies-axis">
              {value}
            </text>
          </g>
        )
      })}
      {usd.length ? <path d={areaPath(usd)} className="companies-usd-fill" /> : null}
      {rub.length ? <path d={areaPath(rub)} className="companies-rub-fill" /> : null}
      {usd.length ? <path d={linePath(usd)} className="companies-usd-line" /> : null}
      {rub.length ? <path d={linePath(rub)} className="companies-rub-line" /> : null}
      {usd.map((p, i) => (
        <circle key={`usd-${i}`} cx={p.x} cy={p.y} r="3.2" className="companies-usd-dot" />
      ))}
      {rub.map((p, i) => (
        <circle key={`rub-${i}`} cx={p.x} cy={p.y} r="3.2" className="companies-rub-dot" />
      ))}
    </svg>
  )
}

function PapersDonut({ company }: { company: Company }) {
  const slices = papersMix(company)
  const total = slices.reduce((sum, row) => sum + row.count, 0)
  if (total === 0) {
    return (
      <div className="companies-empty">
        <p className="muted">No papers yet.</p>
        <p className="muted">Set a contract for Me or a linked person.</p>
      </div>
    )
  }
  const r = 52
  const c = 2 * Math.PI * r
  let offset = 0
  return (
    <div className="companies-donut-row">
      <svg className="companies-donut-svg" viewBox="0 0 160 160" role="img" aria-label="Papers mix">
        <circle cx="80" cy="80" r={r} className="companies-donut-track" />
        {slices.map((row) => {
          const len = (row.count / total) * c
          const dash = offset
          offset += len
          return (
            <circle
              key={row.kind}
              cx="80"
              cy="80"
              r={r}
              className={SLICE[row.kind]}
              strokeDasharray={`${len} ${c - len}`}
              strokeDashoffset={-dash}
              transform="rotate(-90 80 80)"
            />
          )
        })}
        <text x="80" y="76" textAnchor="middle" className="companies-donut-total">
          {total}
        </text>
        <text x="80" y="94" textAnchor="middle" className="companies-axis">
          open
        </text>
      </svg>
      <ul className="companies-legend">
        {slices.map((row) => (
          <li key={row.kind}>
            <i className={SLICE[row.kind]} />
            {row.label}
            <span>{Math.round((row.count / total) * 100)}%</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

function TitleRails({ titles, from, to }: { titles: CompanyTitle[]; from: string; to: string }) {
  if (titles.length === 0) {
    return (
      <div className="companies-empty">
        <p className="muted">No titles yet.</p>
        <p className="muted">Add a title to fill this rail.</p>
      </div>
    )
  }
  const rows = titles.slice().sort((a, b) => a.startedOn.localeCompare(b.startedOn))
  return (
    <div className="companies-rails">
      {rows.map((row) => (
        <div key={row.id} className="companies-rail">
          <div className="companies-rail-copy">
            <strong>{row.title}</strong>
            <span>{row.endedOn ? `${dayShort(row.startedOn)} to ${dayShort(row.endedOn)}` : `since ${dayShort(row.startedOn)}`}</span>
          </div>
          <div className="companies-rail-track">
            <i style={{ width: `${periodShare(row.startedOn, row.endedOn, from, to)}%` }} />
          </div>
        </div>
      ))}
    </div>
  )
}

export function CompanyOverview({ company, people }: Props) {
  const title = openOf(company.titles)
  const salary = openOf(company.salaries)
  const delta = salaryDelta(company.salaries)
  const tenure = tenureText(company.tenure) || 'No tenure'
  const managerId = openOf(company.managers)?.personId
  const manager = people.find((row) => row.id === managerId)?.name || 'No manager'
  const names = new Map(people.map((row) => [row.id, row.name]))
  const log = careerLog(company, names).slice(0, 5)
  const railFrom = dateOnly(company.startedOn) || dateOnly(company.titles[0]?.startedOn) || moscowYmd()
  const railTo = dateOnly(company.endedOn) || moscowYmd()

  return (
    <section id="company-sec-overview" className="people-section">
      <p className="people-kicker">Overview</p>
      <div className="companies-bento">
        <article className="companies-tile companies-kpi">
          <div className="companies-tile-core">
            <p className="people-kicker">Title</p>
            <strong className="companies-kpi-value">{title?.title || 'No title'}</strong>
            <p className="companies-kpi-meta">{title ? `since ${dayShort(title.startedOn)}` : 'Add a title'}</p>
          </div>
        </article>
        <article className="companies-tile companies-kpi">
          <div className="companies-tile-core">
            <p className="people-kicker">Salary</p>
            <strong className="companies-kpi-value">{salary ? companySalaryText(salary) : 'No salary'}</strong>
            {delta ? (
              <p className={delta.up ? 'companies-kpi-delta up' : 'companies-kpi-delta'}>{delta.text}</p>
            ) : (
              <p className="companies-kpi-meta">Save a salary</p>
            )}
          </div>
        </article>
        <article className="companies-tile companies-kpi">
          <div className="companies-tile-core">
            <p className="people-kicker">Tenure</p>
            <strong className="companies-kpi-value">{tenure}</strong>
            <p className="companies-kpi-meta">{company.startedOn ? `from ${dayShort(company.startedOn)}` : 'Set a start date'}</p>
          </div>
        </article>
        <article className="companies-tile companies-kpi">
          <div className="companies-tile-core">
            <p className="people-kicker">Manager</p>
            <strong className="companies-kpi-value">{manager}</strong>
            <p className="companies-kpi-meta">{managerId ? 'Current' : 'None'}</p>
          </div>
        </article>
        <article className="companies-tile companies-salary-chart">
          <div className="companies-tile-core">
            <div className="companies-tile-head">
              <p className="people-kicker">Salary</p>
              <ul className="companies-legend inline">
                <li>
                  <i className="companies-slice-3" />
                  USD
                </li>
                <li>
                  <i className="companies-slice-ok" />
                  RUB
                </li>
              </ul>
            </div>
            <SalaryChart salaries={company.salaries ?? []} />
          </div>
        </article>
        <article className="companies-tile companies-donut">
          <div className="companies-tile-core">
            <p className="people-kicker">Papers</p>
            <PapersDonut company={company} />
          </div>
        </article>
        <article className="companies-tile companies-span-all">
          <div className="companies-tile-core">
            <p className="people-kicker">Titles</p>
            <TitleRails titles={company.titles ?? []} from={railFrom} to={railTo} />
          </div>
        </article>
        <article className="companies-tile companies-span-all">
          <div className="companies-tile-core">
            <p className="people-kicker">Recent</p>
            <CareerLogTable entries={log} empty="No career events yet." />
          </div>
        </article>
      </div>
    </section>
  )
}
