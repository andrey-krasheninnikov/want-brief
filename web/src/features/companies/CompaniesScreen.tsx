import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { Window } from '../../shared/Window'
import { typingTarget } from '../people/peopleModel'
import { CompanyCard } from './CompanyCard'
import { CompanyDossier } from './CompanyDossier'
import { companyMatches, rememberCompany, rememberedCompany } from './companiesModel'
import '../people/people.css'
import './companies.css'

function companiesHref(opts: { id?: string; creating?: boolean }): string {
  const params = new URLSearchParams()
  if (opts.creating) params.set('new', '1')
  const query = params.toString()
  const base = opts.id ? `/companies/${opts.id}` : '/companies'
  return query ? `${base}?${query}` : base
}

export function CompaniesScreen() {
  const { id } = useParams()
  const [params] = useSearchParams()
  const creating = !id && params.get('new') === '1'
  const open = Boolean(id) || creating
  const navigate = useNavigate()
  const companies = useQuery({ queryKey: ['companies'], queryFn: api.companies })
  const [query, setQuery] = useState('')
  const searchRef = useRef<HTMLInputElement>(null)
  const list = useMemo(
    () => (companies.data ?? []).filter((row) => companyMatches(row, query)),
    [companies.data, query],
  )

  function pick(next: string) {
    rememberCompany(next)
    navigate(companiesHref({ id: next }))
  }

  useEffect(() => {
    if (creating || id || companies.isLoading) return
    const rows = companies.data ?? []
    if (rows.length === 0) return
    if (!window.matchMedia('(min-width: 769px)').matches) return
    const next = rememberedCompany(rows.map((row) => row.id))
    if (next) navigate(companiesHref({ id: next }), { replace: true })
  }, [creating, id, companies.isLoading, companies.data, navigate])

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (typingTarget(event.target)) return
      if (event.key === '/') {
        event.preventDefault()
        searchRef.current?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const empty = !companies.isLoading && !companies.isError && list.length === 0
  const none = !companies.isLoading && !companies.isError && (companies.data ?? []).length === 0

  return (
    <Window
      className="people-window"
      kicker="Studio"
      title="Companies"
      actions={
        <button
          type="button"
          className="tasks-plus"
          aria-label="Add company"
          onClick={() => navigate(companiesHref({ creating: true }))}
        >
          +
        </button>
      }
    >
      <div className={open ? 'people-split open' : 'people-split'}>
        <div className="people-list">
          <input
            ref={searchRef}
            className="people-search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search"
            aria-label="Search"
            autoComplete="off"
          />
          {companies.isLoading ? <p className="muted">Loading…</p> : null}
          {companies.isError ? <p className="error">{companies.error.message}</p> : null}
          {empty ? <p className="people-empty">{query.trim() ? 'No matches.' : none ? 'No companies yet.' : 'No matches.'}</p> : null}
          {none && !query.trim() ? (
            <button type="button" onClick={() => navigate(companiesHref({ creating: true }))}>
              Create company
            </button>
          ) : null}
          {list.map((row) => (
            <CompanyCard key={row.id} company={row} selected={row.id === id} onPick={pick} />
          ))}
        </div>
        <div className="people-detail">
          {open ? (
            <CompanyDossier
              companyId={id}
              onCreated={(next) => pick(next)}
              onDeleted={() => navigate('/companies')}
              onClose={() => navigate('/companies')}
            />
          ) : (
            <p className="people-detail-empty muted">Pick a company.</p>
          )}
        </div>
      </div>
    </Window>
  )
}
