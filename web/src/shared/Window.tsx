import type { ReactNode } from 'react'

type Props = {
  title: string
  kicker?: string
  className?: string
  children: ReactNode
  actions?: ReactNode
}

export function Window({ title, kicker, className, children, actions }: Props) {
  return (
    <section className={className ? `window ${className}` : 'window'}>
      <header className="titlebar">
        <div>
          {kicker ? <p className="title-kicker">{kicker}</p> : null}
          <h1>{title}</h1>
        </div>
        <div className="title-actions">{actions}</div>
      </header>
      <div className="window-body">{children}</div>
    </section>
  )
}
