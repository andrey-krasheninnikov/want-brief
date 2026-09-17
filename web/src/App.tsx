import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { BrowserRouter, Navigate, Route, Routes, useParams } from 'react-router-dom'
import { api } from './api'
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

const client = new QueryClient()

function TaskNotesRedirect() {
  const { id } = useParams()
  return <Navigate to={id ? `/tasks/${id}` : '/tasks'} replace />
}

function Gate() {
  const me = useQuery({
    queryKey: ['me'],
    queryFn: api.me,
    retry: false,
  })
  if (me.isLoading) {
    return <div className="login-stage">booting…</div>
  }
  if (me.isError) {
    return <Navigate to="/login" replace />
  }
  return <Shell />
}

export default function App() {
  return (
    <QueryClientProvider client={client}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginScreen />} />
          <Route element={<Gate />}>
            <Route path="/" element={<Navigate to="/today" replace />} />
            <Route path="/today" element={<TodayScreen />} />
            <Route path="/events" element={<EventsScreen />} />
            <Route path="/events/:id" element={<EventsScreen />} />
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
            <Route path="/settings" element={<SettingsScreen />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
