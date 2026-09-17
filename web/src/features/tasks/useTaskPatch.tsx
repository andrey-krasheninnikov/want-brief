import { useState, type ReactNode } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'

type FieldState = 'idle' | 'saving' | 'saved' | 'error'

function FieldHint({ state, error }: { state: FieldState; error?: string }) {
  if (state === 'saving') return <span className="tasks-field-hint">Saving</span>
  if (state === 'saved') return <span className="tasks-field-hint ok">Saved</span>
  if (state === 'error') return <span className="error">{error || 'Could not save.'}</span>
  return null
}

export function useTaskPatch(id: string) {
  const queryClient = useQueryClient()
  const [fields, setFields] = useState<Record<string, { state: FieldState; error?: string }>>({})

  function mark(field: string, state: FieldState, error?: string) {
    setFields((current) => ({ ...current, [field]: { state, error } }))
  }

  const patch = useMutation({
    mutationFn: ({ body }: { body: Record<string, unknown>; field?: string }) => api.patchItem(id, body),
    onMutate: ({ field }) => {
      if (field) mark(field, 'saving')
    },
    onSuccess: (_data, { field }) => {
      if (field) {
        mark(field, 'saved')
        window.setTimeout(() => mark(field, 'idle'), 1200)
      }
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      void queryClient.invalidateQueries({ queryKey: ['load'] })
      void queryClient.invalidateQueries({ queryKey: ['people'] })
      void queryClient.invalidateQueries({ queryKey: ['person'] })
    },
    onError: (err, { field }) => {
      const message = err instanceof Error ? err.message : 'Could not save.'
      if (field) mark(field, 'error', message)
    },
  })

  function hint(field: string): ReactNode {
    const entry = fields[field]
    if (!entry) return null
    return <FieldHint state={entry.state} error={entry.error} />
  }

  return { patch, mark, hint }
}

export type TaskPatch = ReturnType<typeof useTaskPatch>
