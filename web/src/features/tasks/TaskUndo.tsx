import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

type Extra = { label: string; run: () => void }

type Offer = (message: string, run: () => Promise<void>, extra?: Extra) => void

const UndoContext = createContext<Offer>(() => {})

export function useTaskUndo() {
  return useContext(UndoContext)
}

export function TaskUndoProvider({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState('')
  const [open, setOpen] = useState(false)
  const [extra, setExtra] = useState<Extra | null>(null)
  const runRef = useRef<(() => Promise<void>) | null>(null)
  const timer = useRef<number>(0)

  const clear = useCallback(() => {
    window.clearTimeout(timer.current)
    setOpen(false)
    setExtra(null)
    runRef.current = null
  }, [])

  const offer = useCallback<Offer>((text, run, more) => {
    window.clearTimeout(timer.current)
    runRef.current = run
    setMessage(text)
    setExtra(more ?? null)
    setOpen(true)
    timer.current = window.setTimeout(() => {
      setOpen(false)
      setExtra(null)
      runRef.current = null
    }, 8000)
  }, [])

  useEffect(() => () => window.clearTimeout(timer.current), [])

  async function undo() {
    const run = runRef.current
    clear()
    if (run) await run()
  }

  return (
    <UndoContext.Provider value={offer}>
      {children}
      {open
        ? createPortal(
            <div className="tasks-undo" role="status">
              <p>{message}</p>
              {extra ? (
                <button
                  type="button"
                  className="ghost"
                  onClick={() => {
                    extra.run()
                    clear()
                  }}
                >
                  {extra.label}
                </button>
              ) : null}
              <button type="button" className="ghost" onClick={() => void undo()}>
                Undo
              </button>
            </div>,
            document.body,
          )
        : null}
    </UndoContext.Provider>
  )
}
