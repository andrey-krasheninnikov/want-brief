import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../api'
import { hours } from '../../shared/format'
import { moscowClocks, moscowRange } from '../../shared/moscow'

export function ClockRow() {
  const [now, setNow] = useState(() => new Date())
  const load = useQuery({
    queryKey: ['load', 'day'],
    queryFn: () => {
      const bounds = moscowRange('day')
      return api.load(bounds.from, bounds.to)
    },
    refetchInterval: 5000,
  })
  const clocks = moscowClocks(now)

  useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), 1000)
    return () => window.clearInterval(id)
  }, [])

  return (
    <section className="clocks">
      <article>
        <div className="today-clock-core">
          <small>Moscow</small>
          <strong className="mono">{clocks.moscow}</strong>
        </div>
      </article>
      <article>
        <div className="today-clock-core">
          <small>Tashkent</small>
          <strong className="mono">{clocks.tashkent}</strong>
        </div>
      </article>
      <article>
        <div className="today-clock-core">
          <small>Tracked today</small>
          <strong className="mono">{hours(load.data?.allocatedSeconds ?? 0)}</strong>
        </div>
      </article>
    </section>
  )
}
