import { Input } from '@want-foundation/desyst-react'
import { Button } from '@want-foundation/desyst-react'
import { useAccount } from '../../shared/account'
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
  const { workspace, sourceCount, capabilities } = useAccount()
  const owner = workspace.role === 'owner'
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

  const count = sources.data?.filter((source) => source.kind !== 'manual').length ?? sourceCount
  const limit = capabilities.externalSourceLimit
  const atLimit = limit != null && count >= limit
  function refreshCount() {
    void queryClient.invalidateQueries({ queryKey: ['sources'] })
    void queryClient.invalidateQueries({ queryKey: ['me'] })
  }

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
      refreshCount()
    },
    onError: (err: Error) => { setError(err.message); refreshCount() },
  })
  const sync = useMutation({
    mutationFn: api.syncSource,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['sources'] })
      void queryClient.invalidateQueries({ queryKey: ['items'] })
    },
    onError: (err: Error) => {
      setError(err.message)
      void queryClient.invalidateQueries({ queryKey: ['sources'] })
    },
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
    onSuccess: refreshCount,
    onError: (err: Error) => setError(err.message),
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
      {error ? <p className="error" role="alert">{error}</p> : null}
      <p>{workspace.name} · {limit != null ? `Free · ${count} of ${limit} external sources used.` : `Paid · ${count} external sources. No limit.`} Manual tasks use no source slots.</p>
      {sources.isLoading ? <p role="status">Loading sources…</p> : null}
      {sources.error ? <p className="error" role="alert">{sources.error.message}</p> : null}
      <ul className="list dense sources-list">
        {(sources.data ?? []).map((source) => (
          <li key={source.id}>
            <div className="grow">
              <strong>{source.name}</strong>
              <small>
                {source.kind}
                {source.projectId ? ` · ${projectName.get(source.projectId) ?? 'project'}` : ''}
                {owner && source.email ? ` · ${source.email}` : ''}
                {' · '}
                {source.connected ? 'connected' : 'no connection'}
                {source.lastError ? ` · ${source.lastError}` : ''}
              </small>
              {source.kind === 'manual' ? null : (
                <small>
                  Last attempt: {syncTime(source.lastAttemptAt)} · Last successful sync: {syncTime(source.lastSyncAt)}
                </small>
              )}
              {owner && source.kind !== 'manual' && !source.hasToken ? <small>Add a token to enable sync.</small> : null}
            </div>
            {source.kind === 'manual' || !owner ? null : (
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
      {owner && atLimit ? <p role="status">Both source slots are in use. Remove a source to connect another.</p> : null}
      {owner && !atLimit ? <form className="stack" onSubmit={onAddSource}>
        <div className="row">
          <select aria-label="Source type" value={kind} onChange={(e) => setKind(e.target.value as 'jira' | 'todoist')}>
            <option value="jira">Jira</option>
            <option value="todoist">Todoist</option>
          </select>
          <Input aria-label="Source name" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <select aria-label="Project" value={projectId} onChange={(e) => setProjectId(e.target.value)} required>
          <option value="">Project</option>
          {openProjects.map((project) => (
            <option key={project.id} value={project.id}>
              {project.name}
            </option>
          ))}
        </select>
        <Input
          aria-label="Service URL"
          placeholder={kind === 'jira' ? 'https://your-domain.atlassian.net' : 'https://api.todoist.com'}
          value={baseUrl}
          onChange={(e) => setBaseUrl(e.target.value)}
        />
        {kind === 'jira' ? (
          <>
            <Input
              aria-label="Atlassian email"
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
          aria-label="API token"
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
      </form> : null}
      {owner ? <BackupSection /> : <p className="muted">The workspace owner manages connections and backups.</p>}
    </Window>
  )
}
