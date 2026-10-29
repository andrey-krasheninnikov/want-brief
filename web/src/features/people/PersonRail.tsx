import { Button } from '@want-foundation/desyst-react'
import type { ReactNode } from 'react'

type RailProps = {
  children: ReactNode
}

export function PersonRail({ children }: RailProps) {
  return <div className="people-rail">{children}</div>
}

type AddProps = {
  label: string
  onClick: () => void
}

export function PersonRailAdd({ label, onClick }: AddProps) {
  return (
    <Button type="button" className="people-rail-add" onClick={onClick}>
      {label}
    </Button>
  )
}
