import { CalendarBlank, Lightning, MinusCircle, UserSwitch, type Icon } from '@phosphor-icons/react'
import { type CSSProperties } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { Window } from '../../shared/Window'
import { openTask } from '../../shared/taskOverlay'
import type { Item, Quadrant } from '../../types'
import { PriorityList } from './PriorityList'
import { itemsInQuadrant, priorityItems } from './rank'
import './matrix.css'

const cells: { id: Quadrant; title: string; hint: string; Icon: Icon }[] = [
  { id: 'do', title: 'Do', hint: 'urgent, important', Icon: Lightning },
  { id: 'schedule', title: 'Schedule', hint: 'not urgent, important', Icon: CalendarBlank },
  { id: 'delegate', title: 'Delegate', hint: 'urgent, not important', Icon: UserSwitch },
  { id: 'drop', title: 'Drop', hint: 'not urgent, not important', Icon: MinusCircle },
]

export function MatrixScreen() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const items = useQuery({ queryKey: ['items'], queryFn: () => api.items({ openOnly: true }) })
  const patch = useMutation({
    mutationFn: ({ id, body }: { id: string; body: Record<string, unknown> }) => api.patchItem(id, body),
    onSuccess: () => {
      window.setTimeout(() => {
        void queryClient.invalidateQueries({ queryKey: ['items'] })
      }, 0)
    },
  })
  const rows = priorityItems(items.data ?? [])

  function move(item: Item, quadrant: Quadrant) {
    patch.mutate({
      id: item.id,
      body: {
        urgent: quadrant === 'do' || quadrant === 'delegate',
        important: quadrant === 'do' || quadrant === 'schedule',
      },
    })
  }

  return (
    <Window className="priorities-window" kicker="Focus" title="Priorities">
      <div className="prio-stack">
        {items.isLoading ? <p className="muted">Loading…</p> : null}
        {items.isError ? <p className="error">{items.error.message}</p> : null}
        <div className="prio-matrix">
          {cells.map((cell, index) => {
            const listed = itemsInQuadrant(rows, cell.id)
            return (
              <section
                key={cell.id}
                className={`prio-shell ${cell.id}`}
                style={{ '--d': index } as CSSProperties}
                onDragOver={(event) => event.preventDefault()}
                onDrop={(event) => {
                  const id = event.dataTransfer.getData('text/plain')
                  const item = rows.find((row) => row.id === id)
                  if (item) move(item, cell.id)
                }}
              >
                <div className="prio-core">
                  <header>
                    <span className="prio-icon" aria-hidden>
                      <cell.Icon size={18} weight="light" />
                    </span>
                    <div>
                      <h3>{cell.title}</h3>
                      <small>{cell.hint}</small>
                    </div>
                    <span className="mono">{listed.length}</span>
                  </header>
                  {listed.length === 0 ? <p className="muted">Empty</p> : null}
                  <ul>
                    {listed.map((item) => (
                      <li
                        key={item.id}
                        draggable
                        onDragStart={(event) => event.dataTransfer.setData('text/plain', item.id)}
                      >
                        <button type="button" className="ghost" onClick={() => openTask(navigate, item.id)}>
                          {item.title}
                        </button>
                        <small>{item.sourceName}</small>
                      </li>
                    ))}
                  </ul>
                </div>
              </section>
            )
          })}
        </div>
        <PriorityList items={rows} />
      </div>
    </Window>
  )
}
