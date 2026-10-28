import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { api } from '../../api'
import { Window } from '../../shared/Window'

export function SettingsScreen() {
  const queryClient = useQueryClient()
  const sources = useQuery({ queryKey: ['sources'], queryFn: api.sources })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const [kind, setKind] = useState<'jira' | 'todoist'>('jira')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [token, setToken] = useState('')
  const [projectId, setProjectId] = useState('')
  const [insecureTls, setInsecureTls] = useState(false)
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
    mutationFn: ({ id, body }: { id: string; body: Record<string, string | boolean> }) => api.patchSource(id, body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['sources'] }),
    onError: (err: Error) => setError(err.message),
  })
  const removeSource = useMutation({
    mutationFn: api.deleteSource,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['sources'] }),
  })

  const projectName = new Map((projects.data ?? []).map((project) => [project.id, project.name]))
  const openProjects = (projects.data ?? []).filter((project) => !project.archivedAt)

  function onAddSource(event: FormEvent) {
    event.preventDefault()
    setError('')
    createSource.mutate()
  }

  return (
    <Window title="Sources">
      {error ? <p className="error">{error}</p> : null}
      <h3>Sources</h3>
      <ul className="list dense">
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
                {' · '}
                {source.lastSyncAt ? new Date(source.lastSyncAt).toLocaleString() : 'never synced'}
              </small>
            </div>
            {source.kind === 'manual' ? null : (
              <>
                <button type="button" onClick={() => sync.mutate(source.id)} disabled={sync.isPending}>
                  Sync
                </button>
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
                    <button
                      type="button"
                      className="ghost"
                      onClick={() => {
                        const next = window.prompt('Email', source.email ?? '')
                        if (next) patchSource.mutate({ id: source.id, body: { email: next } })
                      }}
                    >
                      Email
                    </button>
                  </>
                ) : null}
                <button
                  type="button"
                  className="ghost"
                  onClick={() => {
                    const next = window.prompt('API token')
                    if (next) patchSource.mutate({ id: source.id, body: { token: next } })
                  }}
                >
                  Token
                </button>
                <button type="button" className="ghost" onClick={() => removeSource.mutate(source.id)}>
                  Del
                </button>
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
          <input placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <select value={projectId} onChange={(e) => setProjectId(e.target.value)} required>
          <option value="">Project</option>
          {openProjects.map((project) => (
            <option key={project.id} value={project.id}>
              {project.name}
            </option>
          ))}
        </select>
        <input
          placeholder={kind === 'jira' ? 'https://your-domain.atlassian.net' : 'https://api.todoist.com'}
          value={baseUrl}
          onChange={(e) => setBaseUrl(e.target.value)}
        />
        {kind === 'jira' ? (
          <>
            <input
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
        <input
          placeholder="API token"
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
        />
        <button type="submit" disabled={!projectId || createSource.isPending}>
          Add source
        </button>
      </form>
    </Window>
  )
}
