import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { DateField } from '../../shared/DateField'
import { moscowYmd } from '../../shared/moscow'
import type { Company } from '../../types'
import { companySalaryText, dateOnly, dayShort, liveTenure, openOf, periodShare, tenureText } from './companiesModel'

type Props = {
  company: Company
  startedOn: string
  endedOn: string
  onTenure: (startedOn: string, endedOn: string) => void
}

export function CompanyCareer({ company, startedOn, endedOn, onTenure }: Props) {
  const queryClient = useQueryClient()
  const [title, setTitle] = useState('')
  const [titleOn, setTitleOn] = useState(moscowYmd())
  const [currency, setCurrency] = useState<'usd' | 'rub'>('usd')
  const [amount, setAmount] = useState('')
  const [comment, setComment] = useState('')
  const [salaryOn, setSalaryOn] = useState(moscowYmd())
  const openSalary = openOf(company.salaries)
  const nextAmount = Number(amount)
  const samePay = Boolean(openSalary && openSalary.currency === currency && openSalary.amount === nextAmount)
  const blocked = Boolean(openSalary) && amount.trim() !== '' && !samePay && !comment.trim()
  const tenure = liveTenure(startedOn, endedOn)
  const railFrom = startedOn || dateOnly(company.titles[0]?.startedOn) || dateOnly(company.salaries[0]?.startedOn) || moscowYmd()
  const railTo = endedOn || moscowYmd()
  const titles = (company.titles ?? []).slice().sort((a, b) => a.startedOn.localeCompare(b.startedOn))
  const salaries = (company.salaries ?? []).slice().sort((a, b) => a.startedOn.localeCompare(b.startedOn))

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['company', company.id] })
    void queryClient.invalidateQueries({ queryKey: ['companies'] })
  }

  const addTitle = useMutation({
    mutationFn: () => api.addCompanyTitle(company.id, { title: title.trim(), startedOn: titleOn }),
    onSuccess: () => {
      setTitle('')
      refresh()
    },
  })
  const addSalary = useMutation({
    mutationFn: () =>
      api.addCompanySalary(company.id, {
        currency,
        amount: nextAmount,
        comment: comment.trim(),
        startedOn: salaryOn,
      }),
    onSuccess: () => {
      setAmount('')
      setComment('')
      refresh()
    },
  })

  return (
    <section id="company-sec-career" className="people-section">
      <p className="people-kicker">Career</p>
      <div className="people-pair">
        <label>
          From
          <DateField mode="date" value={startedOn} onChange={(next) => onTenure(next, endedOn)} />
        </label>
        <label>
          To
          <DateField mode="date" value={endedOn} onChange={(next) => onTenure(startedOn, next)} />
        </label>
      </div>
      <p className="people-rail-meta">{tenure ? tenureText(tenure) : 'Set a start date to see tenure.'}</p>
      {openSalary ? <p className="companies-hero">Now {companySalaryText(openSalary)}</p> : null}

      {titles.length > 0 ? (
        <div className="companies-rails">
          {titles.map((row) => (
            <div key={row.id} className="companies-rail">
              <div className="companies-rail-copy">
                <strong>{row.title}</strong>
                <span>{row.endedOn ? `${dayShort(row.startedOn)} to ${dayShort(row.endedOn)}` : `since ${dayShort(row.startedOn)}`}</span>
              </div>
              <div className="companies-rail-track">
                <i style={{ width: `${periodShare(row.startedOn, row.endedOn, railFrom, railTo)}%` }} />
              </div>
            </div>
          ))}
        </div>
      ) : null}

      {salaries.length > 0 ? (
        <div className="companies-rails">
          {salaries.map((row) => (
            <div key={row.id} className="companies-rail">
              <div className="companies-rail-copy">
                <strong>{companySalaryText(row)}</strong>
                <span>
                  {row.endedOn ? `${dayShort(row.startedOn)} to ${dayShort(row.endedOn)}` : `since ${dayShort(row.startedOn)}`}
                  {row.comment.trim() ? ` · ${row.comment.trim()}` : ''}
                </span>
              </div>
              <div className="companies-rail-track">
                <i style={{ width: `${periodShare(row.startedOn, row.endedOn, railFrom, railTo)}%` }} />
              </div>
            </div>
          ))}
        </div>
      ) : null}

      <div className="companies-form">
        <p className="people-kicker">Title</p>
        <label>
          New title
          <input value={title} onChange={(e) => setTitle(e.target.value)} autoComplete="off" />
        </label>
        <label>
          From
          <DateField mode="date" value={titleOn} onChange={setTitleOn} />
        </label>
        <div className="companies-actions">
          <button type="button" disabled={!title.trim() || addTitle.isPending} onClick={() => addTitle.mutate()}>
            Add title
          </button>
        </div>
        {addTitle.isError ? <p className="error">{addTitle.error.message}</p> : null}

        <p className="people-kicker">Salary</p>
        <div className="people-pair">
          <label>
            Currency
            <select value={currency} onChange={(e) => setCurrency(e.target.value as 'usd' | 'rub')}>
              <option value="usd">USD</option>
              <option value="rub">RUB</option>
            </select>
          </label>
          <label>
            Amount
            <input type="number" min={0} step="0.01" value={amount} onChange={(e) => setAmount(e.target.value)} />
          </label>
        </div>
        <label>
          Comment
          <input
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder={openSalary ? 'Required when pay changes' : 'Optional'}
            autoComplete="off"
          />
        </label>
        <label>
          From
          <DateField mode="date" value={salaryOn} onChange={setSalaryOn} />
        </label>
        <div className="companies-actions">
          <button
            type="button"
            disabled={!amount.trim() || Number(amount) < 0 || addSalary.isPending || blocked}
            onClick={() => addSalary.mutate()}
          >
            Save salary
          </button>
        </div>
        {blocked ? <p className="error">Comment is required when pay changes.</p> : null}
        {addSalary.isError ? <p className="error">{addSalary.error.message}</p> : null}
      </div>
    </section>
  )
}
