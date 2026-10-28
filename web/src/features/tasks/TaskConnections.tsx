import { CloudArrowDown } from '@phosphor-icons/react'
import { useEffect, useState, type FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { OpenUrl, UrlField } from '../../shared/UrlField'
import { createdLabel } from '../../shared/format'
import { sourceKindLabel, type Item, type ProjectLink } from '../../types'
import { PeoplePicker } from '../people/PeoplePicker'
import { idsToRels, relIds } from '../people/RelationField'
import type { TaskPatch } from './useTaskPatch'

const LINK_SLOTS = ['GitLab', 'Jira', 'Confluence'] as const

type SlotLabel = (typeof LINK_SLOTS)[number]

type Props = {
  item: Item
  task: TaskPatch
}

function isSlot(label: string): label is SlotLabel {
  return (LINK_SLOTS as readonly string[]).includes(label)
}

function slotUrl(links: ProjectLink[], label: SlotLabel): string {
  return links.find((link) => link.label === label)?.url ?? ''
}

function withSlot(links: ProjectLink[], label: SlotLabel, url: string): ProjectLink[] {
  const rest = links.filter((link) => link.label !== label)
  const next = url.trim()
  if (!next) return rest
  return [...rest, { label, url: next }]
}

function linkCaption(link: ProjectLink): string {
  if (link.label) return link.label
  try {
    return new URL(link.url).hostname
  } catch {
    return link.url
  }
}

function slotsOf(links: ProjectLink[]): Record<SlotLabel, string> {
  return { GitLab: slotUrl(links, 'GitLab'), Jira: slotUrl(links, 'Jira'), Confluence: slotUrl(links, 'Confluence') }
}

export function TaskConnections({ item, task }: Props) {
  const { patch, mark, hint } = task
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const sources = useQuery({ queryKey: ['sources'], queryFn: api.sources })
  const links = item.links ?? []
  const [slots, setSlots] = useState(() => slotsOf(links))
  const [linkLabel, setLinkLabel] = useState('')
  const [linkUrl, setLinkUrl] = useState('')
  const [linkOpen, setLinkOpen] = useState(false)

  useEffect(() => {
    setSlots(slotsOf(item.links ?? []))
  }, [item.id, item.links])

  function saveSlot(label: SlotLabel, url: string) {
    if (slotUrl(item.links ?? [], label) === url.trim()) return
    patch.mutate({ body: { links: withSlot(item.links ?? [], label, url) }, field: `slot-${label}` })
  }

  function onAddLink(event: FormEvent) {
    event.preventDefault()
    const url = linkUrl.trim()
    if (!url) {
      mark('link', 'error', 'URL is required.')
      return
    }
    patch.mutate(
      { body: { links: [...links, { label: linkLabel.trim(), url }] }, field: 'link' },
      {
        onSuccess: () => {
          setLinkLabel('')
          setLinkUrl('')
          setLinkOpen(false)
        },
      },
    )
  }

  const source = (sources.data ?? []).find((entry) => entry.id === item.sourceId)
  const filledSlots = LINK_SLOTS.filter((label) => slots[label])
  const extraLinks = links.filter((link) => !isSlot(link.label))

  return (
    <>
      <section className="dossier-card">
        <div className="tasks-head">
          <h3>Links</h3>
          <button type="button" className="tasks-plus" aria-label="Add link" onClick={() => setLinkOpen((on) => !on)}>
            +
          </button>
        </div>
        <div className="tasks-form tasks-slots">
          {filledSlots.map((label) => (
            <UrlField
              key={label}
              label={label}
              value={slots[label]}
              onChange={(next) => setSlots((current) => ({ ...current, [label]: next }))}
              onBlur={(next) => saveSlot(label, next)}
            />
          ))}
        </div>
        {hint('slot-GitLab')}
        {hint('slot-Jira')}
        {hint('slot-Confluence')}
        {linkOpen ? (
          <form className="tasks-form" onSubmit={onAddLink}>
            <label>
              Label
              <input value={linkLabel} onChange={(e) => setLinkLabel(e.target.value)} placeholder="GitLab, Jira…" />
            </label>
            <label>
              URL
              <input value={linkUrl} onChange={(e) => setLinkUrl(e.target.value)} placeholder="https://" />
            </label>
            {hint('link')}
            <button type="submit">Add</button>
          </form>
        ) : null}
        {extraLinks.length > 0 ? (
          <table className="tasks-table">
            <thead>
              <tr>
                <th>Label</th>
                <th>URL</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {extraLinks.map((link) => (
                <tr key={`${link.label}-${link.url}`}>
                  <td>{linkCaption(link)}</td>
                  <td>
                    <span className="url-field-row">
                      <a href={link.url} target="_blank" rel="noreferrer">
                        {link.url}
                      </a>
                      <OpenUrl href={link.url} />
                    </span>
                  </td>
                  <td>
                    <button
                      type="button"
                      className="ghost"
                      onClick={() =>
                        patch.mutate({ body: { links: links.filter((entry) => entry !== link) }, field: 'link' })
                      }
                    >
                      Remove
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : null}
        {filledSlots.length === 0 && extraLinks.length === 0 && !linkOpen ? (
          <p className="dossier-empty">No links yet. Add Jira, GitLab or Confluence.</p>
        ) : null}
      </section>

      <section className="dossier-card">
        <header>
          <h3>Source</h3>
          <p>Where this task comes from.</p>
        </header>
        <div className="dossier-tile">
          <span className="dossier-tile-text">
            <p>{sourceKindLabel(item.sourceKind)}</p>
            <strong>{item.sourceName || sourceKindLabel(item.sourceKind)}</strong>
          </span>
          <span className="dossier-tile-icon" aria-hidden>
            <CloudArrowDown size={18} weight="light" />
          </span>
        </div>
        <p className="dossier-note">
          {source?.lastSyncAt ? `Synced ${createdLabel(source.lastSyncAt)}` : 'Never synced'}
        </p>
      </section>

      <section className="dossier-card">
        <header>
          <h3>Ownership</h3>
          <p>Project and people on the hook.</p>
        </header>
        <label>
          Project
          <select
            value={item.projectId ?? ''}
            onChange={(e) => patch.mutate({ body: { projectId: e.target.value }, field: 'project' })}
          >
            <option value="">No project</option>
            {(projects.data ?? [])
              .filter((entry) => !entry.archivedAt || entry.id === item.projectId)
              .map((entry) => (
                <option key={entry.id} value={entry.id}>
                  {entry.archivedAt ? `${entry.name} (archived)` : entry.name}
                </option>
              ))}
          </select>
          {hint('project')}
        </label>
        <PeoplePicker
          value={idsToRels(item.personIds ?? [])}
          onChange={(people) => patch.mutate({ body: { personIds: relIds(people) }, field: 'people' })}
        />
      </section>
    </>
  )
}
