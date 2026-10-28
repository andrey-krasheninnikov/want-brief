import { useEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

type Props = {
  open: boolean
  title: string
  kicker?: string
  titleSlot?: ReactNode
  onClose: () => void
  wide?: boolean
  children: ReactNode
}

const closers: Array<() => void> = []

function onEscape(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  event.preventDefault()
  closers[closers.length - 1]?.()
}

function focusables(root: HTMLElement) {
  return [
    ...root.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  ].filter((node) => !node.closest('.tasks-sheet-back'))
}

export function TaskSheet({ open, title, kicker = 'New', titleSlot, wide, onClose, children }: Props) {
  const dialog = useRef<HTMLElement>(null)
  const prior = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!open) return
    closers.push(onClose)
    if (closers.length === 1) window.addEventListener('keydown', onEscape)
    return () => {
      const index = closers.lastIndexOf(onClose)
      if (index >= 0) closers.splice(index, 1)
      if (closers.length === 0) window.removeEventListener('keydown', onEscape)
    }
  }, [open, onClose])

  useEffect(() => {
    if (!open) return
    prior.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const root = dialog.current
    const first = root ? focusables(root)[0] : null
    first?.focus()

    function onKey(event: KeyboardEvent) {
      if (event.key !== 'Tab' || !root) return
      const items = focusables(root)
      if (items.length === 0) return
      const head = items[0]
      const tail = items[items.length - 1]
      if (event.shiftKey && document.activeElement === head) {
        event.preventDefault()
        tail.focus()
      } else if (!event.shiftKey && document.activeElement === tail) {
        event.preventDefault()
        head.focus()
      }
    }

    root?.addEventListener('keydown', onKey)
    return () => {
      root?.removeEventListener('keydown', onKey)
      prior.current?.focus()
    }
  }, [open])

  if (!open) return null

  return createPortal(
    <div className="tasks-sheet-root">
      <button type="button" className="tasks-sheet-back" aria-label="Close" onClick={onClose} />
      <aside
        ref={dialog}
        className={wide ? 'tasks-sheet wide' : 'tasks-sheet'}
        role="dialog"
        aria-modal="true"
        aria-label={titleSlot ? title : undefined}
        aria-labelledby={titleSlot ? undefined : 'tasks-sheet-title'}
      >
        <div className="tasks-sheet-core">
          <header>
            <p className="tasks-kicker">{kicker}</p>
            {titleSlot ?? <h2 id="tasks-sheet-title">{title}</h2>}
          </header>
          {children}
        </div>
      </aside>
    </div>,
    document.body,
  )
}
