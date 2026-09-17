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
      className="tasks-timer"
      onPointerDown={(event) => event.stopPropagation()}
      onClick={(event) => event.stopPropagation()}
    >
      {running && prominent ? <span className="tasks-timer-dot" aria-hidden /> : null}
      {running ? <span className="mono">{elapsed(running.startedAt)}</span> : null}
      {running ? (
        <button type="button" disabled={stop.isPending} onClick={() => stop.mutate()}>
          Stop
        </button>
      ) : (
        <button
          type="button"
          className={prominent ? undefined : 'ghost'}
          disabled={start.isPending}
          onClick={() => start.mutate()}
        >
          Start
        </button>
      )}
    </div>
  )
}
