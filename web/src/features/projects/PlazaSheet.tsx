import { useEffect, type ReactNode } from 'react'

type Props = {
  open: boolean
  title: string
  onClose: () => void
  children: ReactNode
}

export function PlazaSheet({ open, title, onClose, children }: Props) {
  useEffect(() => {
    if (!open) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  return (
    <div className="plaza-sheet-root">
      <button type="button" className="plaza-sheet-back" aria-label="Close" onClick={onClose} />
      <aside className="plaza-sheet" role="dialog" aria-modal="true" aria-labelledby="plaza-sheet-title">
        <div className="plaza-sheet-core">
          <header>
            <p className="plaza-kicker">New</p>
            <h2 id="plaza-sheet-title">{title}</h2>
          </header>
          {children}
        </div>
      </aside>
    </div>
  )
}
