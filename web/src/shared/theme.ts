import { useEffect, useRef, useState } from 'react'

export type Theme = 'light' | 'dark'

const KEY = 'wantbrief.theme'

export function resolveTheme(saved: string | null, systemDark: boolean): Theme {
  return saved === 'light' || saved === 'dark' ? saved : systemDark ? 'dark' : 'light'
}

function savedTheme(): string | null {
  try {
    return window.localStorage.getItem(KEY)
  } catch {
    return null
  }
}

export function useTheme() {
  const explicitChoice = useRef(['light', 'dark'].includes(savedTheme() ?? ''))
  const [theme, setTheme] = useState<Theme>(() => resolveTheme(savedTheme(), window.matchMedia('(prefers-color-scheme: dark)').matches))

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    document.documentElement.classList.toggle('dark', theme === 'dark')
  }, [theme])

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => {
      if (!explicitChoice.current) setTheme(media.matches ? 'dark' : 'light')
    }
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [])

  function changeTheme(next: Theme) {
    explicitChoice.current = true
    try {
      window.localStorage.setItem(KEY, next)
    } catch {
      // Keep the selected theme for this visit when storage is unavailable.
    }
    setTheme(next)
  }

  return { theme, changeTheme }
}
