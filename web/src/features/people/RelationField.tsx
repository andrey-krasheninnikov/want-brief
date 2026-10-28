import { Plus } from '@phosphor-icons/react'
import { useEffect, useMemo, useState } from 'react'
import { TaskSheet } from '../tasks/TaskSheet'
import type { PersonRel } from '../../types'

type Option = { id: string; name: string }

type Props = {
  label: string
  options: Option[]
  value: PersonRel[]
  onChange: (next: PersonRel[]) => void
  requireComment?: boolean
}

export function idsToRels(ids: string[]): PersonRel[] {
  return ids.map((id) => ({ id, comment: '' }))
}

export function relIds(rels: PersonRel[]): string[] {
  return rels.map((rel) => rel.id)
}

export function RelationField({ label, options, value, onChange, requireComment = false }: Props) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [draft, setDraft] = useState<PersonRel[]>(value)

  useEffect(() => {
    if (!open) return
    setDraft(value)
    setQuery('')
  }, [open])

  const names = useMemo(() => {
    const byId = new Map(options.map((row) => [row.id, row.name]))
    return value.map((rel) => byId.get(rel.id) ?? rel.id)
  }, [options, value])

  const filtered = options.filter((row) => row.name.toLowerCase().includes(query.trim().toLowerCase()))
  const selected = new Map(draft.map((rel) => [rel.id, rel]))
  const incomplete = requireComment && draft.some((rel) => !rel.comment.trim())

  function toggle(id: string) {
    const current = selected.get(id)
    if (current) {
      setDraft(draft.filter((rel) => rel.id !== id))
      return
    }
    setDraft([...draft, { id, comment: '' }])
  }

  function setComment(id: string, comment: string) {
    setDraft(draft.map((rel) => (rel.id === id ? { ...rel, comment } : rel)))
  }

  function apply() {
    if (incomplete) return
    onChange(draft)
    setOpen(false)
  }

  return (
    <div className="people-block">
      <p className="people-block-label">{label}</p>
      <div className="rel-chips">
        {names.length === 0 ? <p className="muted">None</p> : null}
        {names.map((name, index) => (
          <span key={`${value[index]?.id ?? name}-${index}`} className="rel-chip">
            {name}
          </span>
        ))}
        <button type="button" className="ghost rel-edit" aria-label={`Edit ${label}`} onClick={() => setOpen(true)}>
          <Plus size={16} weight="light" aria-hidden />
        </button>
      </div>
      <TaskSheet open={open} kicker="Links" title={label} onClose={() => setOpen(false)}>
        <div className="rel-sheet">
          <input
            className="people-search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search"
            autoFocus
          />
          {filtered.length === 0 ? <p className="muted">None.</p> : null}
          <ul className="rel-list">
            {filtered.map((row) => {
              const rel = selected.get(row.id)
              return (
                <li key={row.id}>
                  <label>
                    <input type="checkbox" checked={Boolean(rel)} onChange={() => toggle(row.id)} />
                    {row.name}
                  </label>
                  {requireComment && rel ? (
                    <textarea
                      rows={2}
                      value={rel.comment}
                      placeholder="Comment required"
                      onChange={(event) => setComment(row.id, event.target.value)}
                    />
                  ) : null}
                </li>
              )
            })}
          </ul>
          {incomplete ? <p className="error">Comment is required for each selected item.</p> : null}
          <button type="button" onClick={apply} disabled={incomplete}>
            Done
          </button>
        </div>
      </TaskSheet>
    </div>
  )
}
