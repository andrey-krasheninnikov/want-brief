import { Button } from '@want-foundation/desyst-react'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { BrowserRouter, Navigate, Route, Routes, useLocation, useParams, useSearchParams } from 'react-router-dom'
import { useEffect } from 'react'
import { api, ApiError } from './api'
import { AccountContext, observeSessionChange } from './shared/account'
import { workspaceFromPath, workspaceUrl } from './shared/workspacePath'
import { AccountScreen } from './features/account/AccountScreen'
import { LoginScreen } from './features/login/LoginScreen'
import { MatrixScreen } from './features/matrix/MatrixScreen'
import { ProjectsScreen } from './features/projects/ProjectsScreen'
import { ProjectNotesPage } from './features/projects/ProjectNotesPage'
import { ProjectPage } from './features/projects/ProjectPage'
import { SettingsScreen } from './features/sources/SettingsScreen'
import { TasksScreen } from './features/tasks/TasksScreen'
import { EventsScreen } from './features/events/EventsScreen'
import { PeopleScreen } from './features/people/PeopleScreen'
import { CompaniesScreen } from './features/companies/CompaniesScreen'
import { TodayScreen } from './features/today/TodayScreen'
import { PulseScreen } from './features/pulse/PulseScreen'
import { ScheduleScreen } from './features/schedule/ScheduleScreen'
import { ScheduleSettingsScreen } from './features/schedule/settings/ScheduleSettingsScreen'
import { Shell } from './shared/Shell'
import { TimezoneContext } from './shared/timezone'

const client = new QueryClient()

function TaskNotesRedirect() {
  const { id } = useParams()
  return <Navigate to={id ? `/tasks/${id}` : '/tasks'} replace />
}

function EventPathRedirect() {
  const { id } = useParams()
  const [params] = useSearchParams()
  const next = new URLSearchParams(params)
  if (id) next.set('event', id)
  const suffix = next.toString()
  return <Navigate to={suffix ? `/events?${suffix}` : '/events'} replace />
}

function WorkspaceRedirect({ href }: { href: string }) {
  useEffect(() => { window.location.replace(href) }, [href])
  return <div className="login-stage"><a href={href}>Open workspace</a></div>
}

function Gate() {
  const location = useLocation()
  const selected = workspaceFromPath(window.location.pathname)
  const me = useQuery({
    queryKey: ['me'],
    queryFn: api.me,
    retry: false,
    refetchInterval: 60_000,
  })
  const settings = useQuery({
    queryKey: ['schedule-settings', me.data?.capabilities.canAnalyze],
    queryFn: api.scheduleSettings,
    enabled: me.isSuccess && Boolean(selected),
  })
  if (me.isLoading) {
    return <div className="login-stage">booting…</div>
  }
  if (me.isError) {
    if (me.error instanceof ApiError && me.error.status === 401) {
      return <Navigate to="/login" replace />
    }
    return <div className="login-stage"><p role="alert">Could not open this workspace.</p><a href="/today">Open personal workspace</a><Button onClick={() => void me.refetch()}>Retry</Button></div>
  }
  if (!me.data) return <div className="login-stage" role="status">Loading account…</div>
  if (!selected) {
    const route = window.location.pathname + window.location.search + window.location.hash
    return <WorkspaceRedirect href={workspaceUrl(me.data.workspace.id, route)} />
  }
  if (location.pathname === '/today' && me.data.workspace.kind === 'team') {
    const personal = me.data.workspaces.find((space) => space.kind === 'personal')
    if (personal) return <WorkspaceRedirect href={workspaceUrl(personal.id, '/today')} />
  }
  if (!settings.data) {
    return settings.isError
      ? <div className="login-stage"><p>Could not load schedule settings.</p><Button onClick={() => void settings.refetch()}>Retry</Button></div>
      : <div className="login-stage">Loading schedule settings…</div>
  }
  return <AccountContext.Provider value={me.data}><TimezoneContext.Provider value={settings.data.timezone}><Shell /></TimezoneContext.Provider></AccountContext.Provider>
}

export default function App() {
  useEffect(observeSessionChange, [])
  const workspace = workspaceFromPath(window.location.pathname)
  return (
    <QueryClientProvider client={client}>
      <BrowserRouter basename={workspace ? `/w/${workspace}` : '/'}>
        <Routes>
          <Route path="/login" element={<LoginScreen />} />
          <Route element={<Gate />}>
            <Route path="/" element={<Navigate to="/today" replace />} />
            <Route path="/today" element={<TodayScreen />} />
            <Route path="/events" element={<EventsScreen />} />
            <Route path="/events/:id" element={<EventPathRedirect />} />
            <Route path="/schedule" element={<ScheduleScreen />} />
            <Route path="/schedule/settings" element={<ScheduleSettingsScreen />} />
            <Route path="/projects" element={<ProjectsScreen />} />
            <Route path="/projects/:id/notes" element={<ProjectNotesPage />} />
            <Route path="/projects/:id" element={<ProjectPage />} />
            <Route path="/people" element={<PeopleScreen />} />
            <Route path="/people/:id" element={<PeopleScreen />} />
            <Route path="/companies" element={<CompaniesScreen />} />
            <Route path="/companies/:id" element={<CompaniesScreen />} />
            <Route path="/tasks" element={<TasksScreen />} />
            <Route path="/tasks/:id" element={<TasksScreen />} />
            <Route path="/tasks/:id/notes" element={<TaskNotesRedirect />} />
            <Route path="/inbox" element={<Navigate to="/tasks" replace />} />
            <Route path="/matrix" element={<MatrixScreen />} />
            <Route path="/track" element={<Navigate to="/tasks" replace />} />
            <Route path="/pulse" element={<PulseScreen />} />
            <Route path="/notes" element={<Navigate to="/pulse" replace />} />
            <Route path="/load" element={<Navigate to="/pulse" replace />} />
            <Route path="/account" element={<AccountScreen />} />
            <Route path="/settings" element={<SettingsScreen />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
