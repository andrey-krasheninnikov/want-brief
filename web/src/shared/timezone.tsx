import { createContext, useContext } from 'react'

export const TimezoneContext = createContext<string | null>(null)

export function useTimezone(): string {
  const timezone = useContext(TimezoneContext)
  if (!timezone) throw new Error('Schedule timezone is unavailable')
  return timezone
}
