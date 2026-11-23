import { expect, test } from 'bun:test'
import { api } from '../src/api'
import { schedulePage } from '../src/shared/zoned'
import { workspaceFromPath } from '../src/shared/workspacePath'

test('an unscoped page uses the personal workspace; a scoped URL binds requests to its workspace', () => {
  const id = 'fea0715a-ce93-45f3-8780-b525a82ca347'
  expect(workspaceFromPath('/settings')).toBeUndefined()
  expect(workspaceFromPath(`/w/${id}/tasks`)).toBe(id)
  expect(workspaceFromPath('/w/not-a-workspace/today')).toBeUndefined()
})

test('a free day spans the local clock change and cannot advance to tomorrow', () => {
  const page = schedulePage('2026-11-01', 'America/New_York', 'free', 1)
  expect(page.days).toEqual(['2026-11-01'])
  expect(page.from).toBe('2026-11-01T04:00:00.000Z')
  expect(page.to).toBe('2026-11-02T05:00:00.000Z')
})

test('four paid pages end at the midnight following day 28', () => {
  const page = schedulePage('2026-12-18', 'Europe/Moscow', 'paid', 3)
  expect(page.days[0]).toBe('2027-01-08')
  expect(page.days.at(-1)).toBe('2027-01-14')
  expect(page.to).toBe('2027-01-14T21:00:00.000Z')
})

test('workspace data, backups and edits to a team task use the intended workspace', async () => {
  const personal = 'fea0715a-ce93-45f3-8780-b525a82ca347'
  const team = '5ca48f61-109e-444a-8f7f-08ba66b58141'
  const windowProperty = Object.getOwnPropertyDescriptor(globalThis, 'window')
  const originalFetch = globalThis.fetch
  const requests: { path: string; workspace: string | null }[] = []
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { location: { pathname: '/w/' + personal + '/schedule' } } })
  globalThis.fetch = (async (url, init) => {
    requests.push({ path: String(url), workspace: new Headers(init?.headers).get('X-Workspace-Id') })
    return new Response('{}')
  }) as typeof fetch
  try {
    await api.sources()
    await api.downloadBackup()
    await api.restoreBackup(new File(['{}'], 'backup.json'))
    await api.patchItem('task', { plannedSeconds: 600 }, team)
    expect(requests).toEqual([
      { path: '/api/sources', workspace: personal },
      { path: '/api/backup', workspace: personal },
      { path: '/api/restore?mode=replace', workspace: personal },
      { path: '/api/items/task', workspace: team },
    ])
  } finally {
    globalThis.fetch = originalFetch
    if (windowProperty) Object.defineProperty(globalThis, 'window', windowProperty)
    else Reflect.deleteProperty(globalThis, 'window')
  }
})
