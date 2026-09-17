import type { Company } from '../../types'
import { companyInitials, companySalaryText, openOf, tenureText } from './companiesModel'

type Props = {
  company: Company
  selected: boolean
  onPick: (id: string) => void
}

export function CompanyCard({ company, selected, onPick }: Props) {
  const title = openOf(company.titles)?.title
  const pay = companySalaryText(openOf(company.salaries))
  const tenure = tenureText(company.tenure)
  const meta = [title || 'No title', tenure].filter(Boolean).join(' · ')
  return (
    <button
      type="button"
      className={selected ? 'people-row on' : 'people-row'}
      onClick={() => onPick(company.id)}
    >
      <span className="people-avatar" aria-hidden>
        {companyInitials(company.name)}
      </span>
      <span className="people-row-copy">
        <strong>{company.name}</strong>
        <span>{meta}</span>
      </span>
      {pay ? <span className="people-row-age companies-card-pay">{pay}</span> : null}
    </button>
  )
}
