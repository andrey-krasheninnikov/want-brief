import { Input } from '@want-foundation/desyst-react'
import { Button } from '@want-foundation/desyst-react'
import { useTimezone } from '../../shared/timezone'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { api } from '../../api'
import { Window } from '../../shared/Window'
import { BackupSection } from './BackupSection'

const syncIntervals = [
  { minutes: 0, label: 'Manual' },
  { minutes: 15, label: 'Every 15 minutes' },
  { minutes: 60, label: 'Every hour' },
  { minutes: 360, label: 'Every 6 hours' },
  { minutes: 1440, label: 'Every day' },
]

export function SettingsScreen() {
  const tz = useTimezone()
  const queryClient = useQueryClient()
  const sources = useQuery({ queryKey: ['sources'], queryFn: api.sources, refetchInterval: 60_000 })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const [kind, setKind] = useState<'jira' | 'todoist'>('jira')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [token, setToken] = useState('')
  const [projectId, setProjectId] = useState('')
  const [insecureTls, setInsecureTls] = useState(false)
  const [syncIntervalMinutes, setSyncIntervalMinutes] = useState(0)
  const [error, setError] = useState('')

  const createSource = useMutation({
    mutationFn: () =>
      api.createSource({
        kind,
        name,
        baseUrl,
        token,
        email: kind === 'jira' ? email : undefined,
        projectId,
        insecureTls: kind === 'jira' ? insecureTls : undefined,
        syncIntervalMinutes,
      }),
    onSuccess: () => {
      setName('')
      setToken('')
      setEmail('')
      setInsecureTls(false)
      void queryClient.invalidateQueries({ queryKey: ['sources'] })
    },
    onError: (err: Error) => setError(err.message),
  })
  const sync = useMutation({
    mutationFn: api.syncSource,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['sources'] })
      void queryClient.invalidateQueries({ queryKey: ['items'] })
    },
    onError: (err: Error) => setError(err.message),
  })
  const patchSource = useMutation({
    mutationFn: ({ id, body }: { id: string; body: Record<string, string | boolean | number> }) => api.patchSource(id, body),
    onSuccess: () => {
      setError('')
      void queryClient.invalidateQueries({ queryKey: ['sources'] })
    },
    onError: (err: Error) => setError(err.message),
  })
  const removeSource = useMutation({
    mutationFn: api.deleteSource,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['sources'] }),
  })

  const projectName = new Map((projects.data ?? []).map((project) => [project.id, project.name]))
  const openProjects = (projects.data ?? []).filter((project) => !project.archivedAt)
  const syncTime = (value: string | null) => value
    ? new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: tz }).format(new Date(value))
    : 'never'

  function onAddSource(event: FormEvent) {
    event.preventDefault()
    setError('')
    createSource.mutate()
  }

  return (
    <Window title="Sources">
      {error ? <p className="error">{error}</p> : null}
      <h3>Sources</h3>
      <ul className="list dense sources-list">
        {(sources.data ?? []).map((source) => (
          <li key={source.id}>
            <div className="grow">
              <strong>{source.name}</strong>
              <small>
                {source.kind}
                {source.projectId ? ` · ${projectName.get(source.projectId) ?? 'project'}` : ''}
                {source.email ? ` · ${source.email}` : ''}
                {' · '}
                {source.connected ? 'connected' : 'no connection'}
                {source.lastError ? ` · ${source.lastError}` : ''}
              </small>
              {source.kind === 'manual' ? null : (
                <small>
                  Last attempt: {syncTime(source.lastAttemptAt)} · Last successful sync: {syncTime(source.lastSyncAt)}
                </small>
              )}
              {source.kind !== 'manual' && !source.hasToken ? <small>Add a token to enable sync.</small> : null}
            </div>
            {source.kind === 'manual' ? null : (
              <>
                <label>
                  Sync schedule
                  <select
                    value={source.syncIntervalMinutes}
                    onChange={(event) => patchSource.mutate({ id: source.id, body: { syncIntervalMinutes: Number(event.target.value) } })}
                    disabled={patchSource.isPending}
                  >
                    {syncIntervals.map((interval) => (
                      <option key={interval.minutes} value={interval.minutes}>{interval.label}</option>
                    ))}
                  </select>
                </label>
                <Button type="button" onClick={() => sync.mutate(source.id)} disabled={sync.isPending}>
                  Sync
                </Button>
                {source.kind === 'jira' ? (
                  <>
                    <label className="row">
                      <input
                        type="checkbox"
                        checked={Boolean(source.insecureTls)}
                        onChange={(e) =>
                          patchSource.mutate({ id: source.id, body: { insecureTls: e.target.checked } })
                        }
                      />
                      Skip TLS verify
                    </label>
                    <Button
                      type="button"
                      className="ghost"
                      onClick={() => {
                        const next = window.prompt('Email', source.email ?? '')
                        if (next) patchSource.mutate({ id: source.id, body: { email: next } })
                      }}
                    >
                      Email
                    </Button>
                  </>
                ) : null}
                <Button
                  type="button"
                  className="ghost"
                  onClick={() => {
                    const next = window.prompt('API token')
                    if (next) patchSource.mutate({ id: source.id, body: { token: next } })
                  }}
                >
                  Token
                </Button>
                <Button type="button" className="ghost" onClick={() => removeSource.mutate(source.id)}>
                  Del
                </Button>
              </>
            )}
          </li>
        ))}
      </ul>
      <form className="stack" onSubmit={onAddSource}>
        <div className="row">
          <select value={kind} onChange={(e) => setKind(e.target.value as 'jira' | 'todoist')}>
            <option value="jira">Jira</option>
            <option value="todoist">Todoist</option>
          </select>
          <Input placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <select value={projectId} onChange={(e) => setProjectId(e.target.value)} required>
          <option value="">Project</option>
          {openProjects.map((project) => (
            <option key={project.id} value={project.id}>
              {project.name}
            </option>
          ))}
        </select>
        <Input
          placeholder={kind === 'jira' ? 'https://your-domain.atlassian.net' : 'https://api.todoist.com'}
          value={baseUrl}
          onChange={(e) => setBaseUrl(e.target.value)}
        />
        {kind === 'jira' ? (
          <>
            <Input
              placeholder="Atlassian email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
            <label className="row">
              <input type="checkbox" checked={insecureTls} onChange={(e) => setInsecureTls(e.target.checked)} />
              Skip TLS verify
            </label>
          </>
        ) : null}
        <Input
          placeholder="API token"
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
        />
        <label>
          Sync schedule
          <select value={syncIntervalMinutes} onChange={(event) => setSyncIntervalMinutes(Number(event.target.value))}>
            {syncIntervals.map((interval) => (
              <option key={interval.minutes} value={interval.minutes}>{interval.label}</option>
            ))}
          </select>
        </label>
        <Button type="submit" disabled={!projectId || createSource.isPending}>
          Add source
        </Button>
      </form>
      <BackupSection />
    </Window>
  )
}
