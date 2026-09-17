import { Play, Stop } from '@phosphor-icons/react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api'
import { elapsed } from '../../shared/format'
import type { TimeInterval } from '../../types'
import { useTaskLogPrompt } from './TaskLogPrompt'

type Props = {
  itemId: string
  running?: TimeInterval
  prominent?: boolean
}

export function TaskTimer({ itemId, running, prominent }: Props) {
  const promptLog = useTaskLogPrompt()
  const queryClient = useQueryClient()
  const start = useMutation({
    mutationFn: () => api.startInterval(itemId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['intervals'] })
    },
  })
  const stop = useMutation({
    mutationFn: () => api.stopInterval(running!.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['intervals'] })
      void queryClient.invalidateQueries({ queryKey: ['items'] })
      void queryClient.invalidateQueries({ queryKey: ['load'] })
      promptLog(itemId, 'timer_stop')
    },
  })

  if (!running && !prominent) return null

  return (
    <div
      className={['tasks-timer', running ? 'on' : ''].filter(Boolean).join(' ')}
      onPointerDown={(event) => event.stopPropagation()}
      onClick={(event) => event.stopPropagation()}
    >
      {running ? <span className="mono">{elapsed(running.startedAt)}</span> : null}
      {running ? (
        <button type="button" className="on" disabled={stop.isPending} aria-label="Stop" onClick={() => stop.mutate()}>
          <Stop size={18} weight="light" />
        </button>
      ) : (
        <button type="button" className="ghost" disabled={start.isPending} aria-label="Start" onClick={() => start.mutate()}>
          <Play size={18} weight="light" />
        </button>
      )}
    </div>
  )
}
