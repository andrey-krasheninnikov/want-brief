import { resolveTheme } from './theme'

function equal(actual: string, expected: string) {
  if (actual !== expected) throw new Error(`theme: got ${actual}, want ${expected}`)
}

equal(resolveTheme('light', true), 'light')
equal(resolveTheme('dark', false), 'dark')
equal(resolveTheme(null, true), 'dark')
equal(resolveTheme('other', false), 'light')
