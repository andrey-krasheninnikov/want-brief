import { useState } from 'react'
import { PencilSimple } from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { OpenUrl, UrlField } from '../../shared/UrlField'
import type { PersonSite, SiteKind } from '../../types'
import { TaskSheet } from '../tasks/TaskSheet'
import { PersonRail, PersonRailAdd } from './PersonRail'
import { SITE_KINDS, siteKindLabel } from './peopleModel'

type Draft = { kind: SiteKind; url: string; comment: string }
const empty: Draft = { kind: 'personal_site', url: '', comment: '' }

type Props = {
  personId: string
  sites: PersonSite[]
}

export function PersonSites({ personId, sites }: Props) {
  const queryClient = useQueryClient()
  const [sheet, setSheet] = useState<'new' | string | null>(null)
  const [draft, setDraft] = useState<Draft>(empty)
  const editing = sites.find((row) => row.id === sheet)

  function refresh() {
    void queryClient.invalidateQueries({ queryKey: ['person', personId] })
    void queryClient.invalidateQueries({ queryKey: ['people'] })
  }

  const save = useMutation({
    mutationFn: () => (editing ? api.patchPersonSite(personId, editing.id, draft) : api.createPersonSite(personId, draft)),
    onSuccess: () => {
      setSheet(null)
      setDraft(empty)
      refresh()
    },
  })
  const drop = useMutation({
    mutationFn: (siteId: string) => api.deletePersonSite(personId, siteId),
    onSuccess: () => {
      setSheet(null)
      refresh()
    },
  })

  return (
    <section>
      <p className="people-kicker">Links</p>
      <PersonRail>
        {sites.map((site) => (
          <article key={site.id} className="people-rail-card">
            <p className="people-kicker">{siteKindLabel(site.kind)}</p>
            <p className="people-site-url">
              <span>{site.url}</span>
              <OpenUrl href={site.url} />
            </p>
            {site.comment ? <p className="people-rail-meta">{site.comment}</p> : null}
            <button
              type="button"
              className="ghost rel-edit"
              aria-label="Edit link"
              onClick={() => {
                setDraft({ kind: site.kind, url: site.url, comment: site.comment })
                setSheet(site.id)
              }}
            >
              <PencilSimple size={16} weight="light" />
            </button>
          </article>
        ))}
        <PersonRailAdd
          label="Add link"
          onClick={() => {
            setDraft(empty)
            setSheet('new')
          }}
        />
      </PersonRail>
      <TaskSheet open={sheet !== null} kicker="Link" title={editing ? siteKindLabel(editing.kind) : 'New link'} onClose={() => setSheet(null)}>
        <label>
          Type
          <select value={draft.kind} onChange={(e) => setDraft({ ...draft, kind: e.target.value as SiteKind })}>
            {SITE_KINDS.map((kind) => (
              <option key={kind.id} value={kind.id}>
                {kind.label}
              </option>
            ))}
          </select>
        </label>
        <UrlField label="URL" value={draft.url} onChange={(url) => setDraft({ ...draft, url })} />
        <label>
          Comment
          <input value={draft.comment} onChange={(e) => setDraft({ ...draft, comment: e.target.value })} />
        </label>
        <div className="people-contact-actions">
          <button type="button" onClick={() => save.mutate()} disabled={!draft.url.trim() || save.isPending}>
            Save
          </button>
          {editing ? (
            <button type="button" className="ghost" onClick={() => drop.mutate(editing.id)} disabled={drop.isPending}>
              Remove
            </button>
          ) : null}
        </div>
        {save.isError ? <p className="error">{save.error.message}</p> : null}
      </TaskSheet>
    </section>
  )
}
