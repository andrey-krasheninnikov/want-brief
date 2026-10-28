import { useEffect, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

type OverlayProps = {
  open: boolean
  onCancel: () => void
  children: ReactNode
}

export function TaskOverlay({ open, onCancel, children }: OverlayProps) {
  useEffect(() => {
    if (!open) return
    function onKey(event: KeyboardEvent) {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopImmediatePropagation()
      onCancel()
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [open, onCancel])

  if (!open) return null
  return createPortal(
    <div className="tasks-confirm-root">
      <button type="button" className="tasks-confirm-back" aria-label="Cancel" onClick={onCancel} />
      {children}
    </div>,
    document.body,
  )
}

type Props = {
  open: boolean
  title: string
  body: string
  confirmLabel: string
  danger?: boolean
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}

export function TaskConfirm({
  open,
  title,
  body,
  confirmLabel,
  danger,
  busy,
  onConfirm,
  onCancel,
}: Props) {
  return (
    <TaskOverlay open={open} onCancel={onCancel}>
      <div className="tasks-confirm" role="dialog" aria-modal="true" aria-labelledby="tasks-confirm-title">
        <p className="tasks-kicker">Confirm</p>
        <h3 id="tasks-confirm-title">{title}</h3>
        <p className="muted">{body}</p>
        <div className="tasks-confirm-actions">
          <button type="button" className="ghost" onClick={onCancel} disabled={busy}>
            Cancel
          </button>
          <button type="button" className={danger ? 'danger' : undefined} onClick={onConfirm} disabled={busy}>
            {confirmLabel}
          </button>
        </div>
      </div>
    </TaskOverlay>
  )
}
