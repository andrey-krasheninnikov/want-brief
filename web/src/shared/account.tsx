import { createContext, useContext } from 'react'
import type { Account } from '../types'

export const AccountContext = createContext<Account | null>(null)

export function useAccount() {
  const account = useContext(AccountContext)
  if (!account) throw new Error('Account is unavailable')
  return account
}

const sessionChangeKey = 'want-brief.session-change'

export function notifySessionChange() {
  try {
    localStorage.setItem(sessionChangeKey, Date.now() + '-' + Math.random())
  } catch {
    // A browser may block storage in private mode.
  }
}

export function observeSessionChange() {
  const changed = (event: StorageEvent) => {
    if (event.key === sessionChangeKey) window.location.reload()
  }
  const restored = (event: PageTransitionEvent) => {
    if (event.persisted) window.location.reload()
  }
  window.addEventListener('storage', changed)
  window.addEventListener('pageshow', restored)
  return () => {
    window.removeEventListener('storage', changed)
    window.removeEventListener('pageshow', restored)
  }
}
