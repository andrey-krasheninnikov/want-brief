import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import type { TaskLogKind } from '../../types'
import { TaskLogDialog } from './TaskLogDialog'
import { TaskUndoProvider } from './TaskUndo'

type Prompt = (itemId: string, kind: TaskLogKind) => void

const TaskLogContext = createContext<Prompt>(() => {})

export function useTaskLogPrompt() {
  return useContext(TaskLogContext)
}

export function TaskLogProvider({ children }: { children: ReactNode }) {
  const [itemId, setItemId] = useState<string | null>(null)
  const [kind, setKind] = useState<TaskLogKind>('status')
  const prompt = useCallback<Prompt>((id, nextKind) => {
    setKind(nextKind)
    setItemId(id)
  }, [])
  return (
    <TaskUndoProvider>
      <TaskLogContext.Provider value={prompt}>
        {children}
        <TaskLogDialog itemId={itemId} kind={kind} onClose={() => setItemId(null)} />
      </TaskLogContext.Provider>
    </TaskUndoProvider>
  )
}
