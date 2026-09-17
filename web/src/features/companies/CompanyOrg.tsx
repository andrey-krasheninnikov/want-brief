import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { moscowYmd } from '../../shared/moscow'
import type { Company, ContractKind, Person } from '../../types'
import { CONTRACT_KINDS, contractKindLabel, openOf } from './companiesModel'

type Props = {
  company: Company
  people: Person[]
}

export function CompanyOrg({ company, people }: Props) {
  const queryClient = useQueryClient()
  const [reportId, setReportId] = useState('')
  const names = new Map(people.map((row) => [row.id, row.name]))
  const manager = openOf(company.managers)
  const reports = (company.reports ?? []).filter((row) => !row.endedOn)
  const meContract = (company.contracts ?? []).find((row) => row.personId == null && !row.endedOn)
  const taken = new Set(reports.map((row) => row.personId))
  if (manager) taken.add(manager.personId)
  const reportChoices = people.filter((row) => !taken.has(row.id))

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['company', company.id] })
    void queryClient.invalidateQueries({ queryKey: ['companies'] })
  }

  const setManager = useMutation({
    mutationFn: (personId: string | null) => api.setCompanyManager(company.id, { personId, startedOn: moscowYmd() }),
    onSuccess: refresh,
  })
  const addReport = useMutation({
    mutationFn: (personId: string) => api.addCompanyReport(company.id, { personId, startedOn: moscowYmd() }),
    onSuccess: () => {
      setReportId('')
      refresh()
    },
  })
  const endReport = useMutation({
    mutationFn: (reportId: string) => api.endCompanyReport(company.id, reportId),
    onSuccess: refresh,
  })
  const setContract = useMutation({
    mutationFn: (body: { personId?: string | null; kind: ContractKind }) =>
      api.setCompanyContract(company.id, { ...body, startedOn: moscowYmd() }),
    onSuccess: refresh,
  })

  return (
    <section id="company-sec-org" className="people-section">
      <p className="people-kicker">Org</p>
      <div className="companies-form">
        <label>
          Manager
          <select
            value={manager?.personId ?? ''}
            onChange={(e) => setManager.mutate(e.target.value || null)}
          >
            <option value="">None</option>
            {people.map((row) => (
              <option key={row.id} value={row.id} disabled={reports.some((item) => item.personId === row.id)}>
                {row.name}
              </option>
            ))}
          </select>
        </label>
        {setManager.isError ? <p className="error">{setManager.error.message}</p> : null}

        <p className="people-kicker">Reports</p>
        {reports.length === 0 ? <p className="people-empty">No reports.</p> : (
          <div className="people-table-wrap">
            <table className="people-table companies-log-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {reports.map((row) => (
                  <tr key={row.id}>
                    <td>{names.get(row.personId) ?? row.personId}</td>
                    <td>
                      <button
                        type="button"
                        className="ghost"
                        onClick={() => endReport.mutate(row.id)}
                        disabled={endReport.isPending}
                      >
                        End
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="companies-actions">
          <select value={reportId} onChange={(e) => setReportId(e.target.value)}>
            <option value="">Add report</option>
            {reportChoices.map((row) => (
              <option key={row.id} value={row.id}>
                {row.name}
              </option>
            ))}
          </select>
          <button type="button" disabled={!reportId || addReport.isPending} onClick={() => addReport.mutate(reportId)}>
            Add
          </button>
        </div>
        {addReport.isError ? <p className="error">{addReport.error.message}</p> : null}

        <p className="people-kicker">Papers</p>
        <label className="companies-paper-row">
          <span>Me</span>
          {meContract ? <span className="companies-chip">{contractKindLabel(meContract.kind)}</span> : null}
          <select
            value={meContract?.kind ?? ''}
            onChange={(e) => {
              const kind = e.target.value as ContractKind
              if (kind) setContract.mutate({ kind })
            }}
          >
            <option value="">None</option>
            {CONTRACT_KINDS.map((row) => (
              <option key={row.id} value={row.id}>
                {row.label}
              </option>
            ))}
          </select>
        </label>
        {(company.people ?? []).map((rel) => {
          const current = (company.contracts ?? []).find((row) => row.personId === rel.id && !row.endedOn)
          return (
            <label key={rel.id} className="companies-paper-row">
              <span>{names.get(rel.id) ?? rel.id}</span>
              {current ? <span className="companies-chip">{contractKindLabel(current.kind)}</span> : null}
              <select
                value={current?.kind ?? ''}
                onChange={(e) => {
                  const kind = e.target.value as ContractKind
                  if (kind) setContract.mutate({ personId: rel.id, kind })
                }}
              >
                <option value="">None</option>
                {CONTRACT_KINDS.map((row) => (
                  <option key={row.id} value={row.id}>
                    {row.label}
                  </option>
                ))}
              </select>
            </label>
          )
        })}
        {setContract.isError ? <p className="error">{setContract.error.message}</p> : null}
      </div>
    </section>
  )
}
