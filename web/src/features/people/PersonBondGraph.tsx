import type { Person, PersonBond } from '../../types'
import { bondKindLabel } from './peopleModel'

type Props = {
  person: Person
  bonds: PersonBond[]
}

type Node = { id: string; label: string; x: number; y: number }
type Edge = { from: string; to: string; kind: string }

export function PersonBondGraph({ person, bonds }: Props) {
  const open = bonds.filter((bond) => !bond.endedOn)
  const me = open.find((bond) => !bond.otherId)
  const others = open.filter((bond) => bond.otherId).slice(0, 3)
  const height = Math.max(120, 36 + others.length * 44)
  const nodes: Node[] = [
    { id: 'me', label: 'Me', x: 48, y: height / 2 },
    { id: person.id, label: person.name || 'Person', x: 168, y: height / 2 },
    ...others.map((bond, index) => ({
      id: bond.id,
      label: bond.otherName || 'Person',
      x: 318,
      y: others.length === 1 ? height / 2 : 28 + index * ((height - 56) / Math.max(others.length - 1, 1)),
    })),
  ]
  const edges: Edge[] = []
  if (me) edges.push({ from: 'me', to: person.id, kind: bondKindLabel(me.kind) })
  else edges.push({ from: 'me', to: person.id, kind: '' })
  for (const bond of others) {
    edges.push({ from: person.id, to: bond.id, kind: bondKindLabel(bond.kind) })
  }

  return (
    <svg className="people-graph" viewBox={`0 0 380 ${height}`} role="img" aria-label="Relations to Me">
      {edges.map((edge) => {
        const from = nodes.find((node) => node.id === edge.from)
        const to = nodes.find((node) => node.id === edge.to)
        if (!from || !to) return null
        const midX = (from.x + to.x) / 2
        const midY = (from.y + to.y) / 2
        return (
          <g key={`${edge.from}-${edge.to}`}>
            <line x1={from.x + 28} y1={from.y} x2={to.x - 28} y2={to.y} />
            {edge.kind ? (
              <text x={midX} y={midY - 6} textAnchor="middle">
                {edge.kind}
              </text>
            ) : null}
          </g>
        )
      })}
      {nodes.map((node) => (
        <g key={node.id}>
          <circle cx={node.x} cy={node.y} r={22} />
          <text x={node.x} y={node.y + 4} textAnchor="middle">
            {truncate(node.label)}
          </text>
        </g>
      ))}
    </svg>
  )
}

function truncate(value: string): string {
  return value.length > 10 ? `${value.slice(0, 9)}…` : value
}
