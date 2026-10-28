import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { Window } from '../../shared/Window'
import { PeopleBoard } from './PeopleBoard'
import { PeopleTable } from './PeopleTable'
import { PersonCard } from './PersonCard'
import { PersonDossier } from './PersonDossier'
import { copyText, personMatches, primaryContact, rememberPerson, rememberedPerson, typingTarget } from './peopleModel'
import './people.css'

const VIEWS = ['split', 'list', 'kanban'] as const
type PeopleView = (typeof VIEWS)[number]

function parseView(raw: string | null): PeopleView {
  if (raw === 'list' || raw === 'kanban') return raw
  return 'split'
}

function peopleHref(opts: { id?: string; view: PeopleView; creating?: boolean }): string {
  const params = new URLSearchParams()
  if (opts.view !== 'split' || opts.creating) params.set('view', opts.view)
  if (opts.creating) params.set('new', '1')
  const query = params.toString()
  const base = opts.id ? `/people/${opts.id}` : '/people'
  return query ? `${base}?${query}` : base
}

export function PeopleScreen() {
  const { id } = useParams()
  const [params] = useSearchParams()
  const view = parseView(params.get('view'))
  const creating = !id && params.get('new') === '1'
  const open = view === 'split' && (Boolean(id) || creating)
  const navigate = useNavigate()
  const people = useQuery({ queryKey: ['people'], queryFn: api.people })
  const companies = useQuery({ queryKey: ['companies'], queryFn: api.companies })
  const [query, setQuery] = useState('')
  const searchRef = useRef<HTMLInputElement>(null)

  const list = useMemo(() => (people.data ?? []).filter((person) => personMatches(person, query)), [people.data, query])
  const companyNames = useMemo(() => {
    const names = new Map<string, string>()
    for (const row of companies.data ?? []) names.set(row.id, row.name)
    return names
  }, [companies.data])

  function goView(next: PeopleView) {
    navigate(peopleHref({ id, view: next, creating: creating && next === 'split' }))
  }

  function pick(next: string) {
    rememberPerson(next)
    navigate(peopleHref({ id: next, view: 'split' }))
  }

  useEffect(() => {
    if (view !== 'split' || creating || id || people.isLoading) return
    const rows = people.data ?? []
    if (rows.length === 0) return
    if (!window.matchMedia('(min-width: 769px)').matches) return
    const next = rememberedPerson(rows.map((row) => row.id))
    if (next) navigate(peopleHref({ id: next, view: 'split' }), { replace: true })
  }, [view, creating, id, people.isLoading, people.data, navigate])

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (typingTarget(event.target)) return
      if (event.key === '/') {
        event.preventDefault()
        searchRef.current?.focus()
        return
      }
      if (event.key === 'j' || event.key === 'k') {
        if (list.length === 0) return
        event.preventDefault()
        const index = list.findIndex((row) => row.id === id)
        const current = index < 0 ? (event.key === 'j' ? -1 : list.length) : index
        const next = event.key === 'j' ? Math.min(list.length - 1, current + 1) : Math.max(0, current - 1)
        pick(list[next].id)
        return
      }
      if (event.key === 'c') {
        const selected = list.find((row) => row.id === id) ?? people.data?.find((row) => row.id === id)
        const primary = selected ? primaryContact(selected) : null
        if (primary) void copyText(primary.value)
        return
      }
      if (event.key === 'e') {
        event.preventDefault()
        document.getElementById('person-name')?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [id, list, people.data])

  const empty = !people.isLoading && !people.isError && list.length === 0
  const none = !people.isLoading && !people.isError && (people.data ?? []).length === 0

  return (
    <Window
      className="people-window"
      kicker="Circle"
      title="People"
      actions={
        <>
          <div className="people-tabs" role="tablist" aria-label="View">
            {VIEWS.map((value) => (
              <button
                key={value}
                type="button"
                role="tab"
                aria-selected={view === value}
                className={view === value ? 'on' : ''}
                onClick={() => goView(value)}
              >
                {value === 'list' ? 'Table' : value[0].toUpperCase() + value.slice(1)}
              </button>
            ))}
          </div>
          <button
            type="button"
            className="tasks-plus"
            aria-label="Add person"
            onClick={() => navigate(peopleHref({ view: 'split', creating: true }))}
          >
            +
          </button>
        </>
      }
    >
      {view === 'split' ? (
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
            {people.isLoading ? <p className="muted">Loading…</p> : null}
            {people.isError ? <p className="error">{people.error.message}</p> : null}
            {empty ? (
              <p className="people-empty">
                {query.trim() ? 'No matches.' : none ? 'No people yet.' : 'No matches.'}
              </p>
            ) : null}
            {none && !query.trim() ? (
              <button type="button" onClick={() => navigate(peopleHref({ view: 'split', creating: true }))}>
                Create person
              </button>
            ) : null}
            {list.map((person) => (
              <PersonCard key={person.id} person={person} selected={person.id === id} onPick={pick} companyNames={companyNames} />
            ))}
          </div>
          <div className="people-detail">
            {open ? (
              <PersonDossier
                personId={id}
                onCreated={(next) => {
                  rememberPerson(next)
                  navigate(peopleHref({ id: next, view: 'split' }))
                }}
                onDeleted={() => navigate(peopleHref({ view: 'split' }))}
                onClose={() => navigate(peopleHref({ view: 'split' }))}
              />
            ) : none ? (
              <div className="people-detail-empty">
                <p className="people-empty">No people yet.</p>
                <button type="button" onClick={() => navigate(peopleHref({ view: 'split', creating: true }))}>
                  Create person
                </button>
              </div>
            ) : (
              <p className="people-empty people-detail-empty">{query.trim() ? 'No matches.' : ''}</p>
            )}
          </div>
        </div>
      ) : (
        <div className="people-browse">
          <input
            ref={searchRef}
            className="people-search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search"
            aria-label="Search"
            autoComplete="off"
          />
          {people.isLoading ? <p className="muted">Loading…</p> : null}
          {people.isError ? <p className="error">{people.error.message}</p> : null}
          {empty ? <p className="people-empty">{query.trim() ? 'No matches.' : 'No people yet.'}</p> : null}
          {!empty && view === 'list' ? <PeopleTable people={list} companyNames={companyNames} onPick={pick} /> : null}
          {!empty && view === 'kanban' ? <PeopleBoard people={list} companyNames={companyNames} onPick={pick} /> : null}
        </div>
      )}
    </Window>
  )
}
