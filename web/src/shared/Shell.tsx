import {
  CalendarBlank,
  CalendarDots,
  ChartBar,
  CheckSquare,
  GridFour,
  Heartbeat,
  Plugs,
  SignOut,
  SquaresFour,
  Buildings,
  Sun,
  Users,
  type Icon,
} from '@phosphor-icons/react'
import { useState } from 'react'
import { NavLink, Outlet, useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../api'
import { TaskDossierSheet } from '../features/tasks/TaskPage'
import { TaskLogProvider } from '../features/tasks/TaskLogPrompt'
import { CheckinDialog } from '../features/today/CheckinDialog'

const links: { to: string; label: string; Icon: Icon }[] = [
  { to: '/today', label: 'Today', Icon: Sun },
  { to: '/tasks', label: 'Tasks', Icon: CheckSquare },
  { to: '/events', label: 'Events', Icon: CalendarBlank },
  { to: '/schedule', label: 'Schedule', Icon: CalendarDots },
  { to: '/matrix', label: 'Priorities', Icon: GridFour },
  { to: '/projects', label: 'Projects', Icon: SquaresFour },
  { to: '/people', label: 'People', Icon: Users },
  { to: '/companies', label: 'Companies', Icon: Buildings },
  { to: '/pulse', label: 'Pulse', Icon: ChartBar },
  { to: '/settings', label: 'Sources', Icon: Plugs },
]

export function Shell() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [checkinOpen, setCheckinOpen] = useState(false)
  const taskId = searchParams.get('task')

  function closeTask() {
    const next = new URLSearchParams(searchParams)
    next.delete('task')
    setSearchParams(next, { replace: true })
  }

  return (
    <TaskLogProvider>
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <img src="/logo.png" alt="Want Brief" width={32} height={32} />
        </div>
        <nav>
          {links.map((link) => (
            <NavLink
              key={link.to}
              to={link.to}
              title={link.label}
              aria-label={link.label}
              className={({ isActive }) => (isActive ? 'active' : '')}
            >
              <link.Icon size={22} weight="light" aria-hidden />
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-foot">
          <button
            type="button"
            className="ghost"
            title="Check-in"
            aria-label="Check-in"
            onClick={() => setCheckinOpen(true)}
          >
            <Heartbeat size={22} weight="light" aria-hidden />
          </button>
          <button
            type="button"
            className="ghost"
            title="Log out"
            aria-label="Log out"
            onClick={() => {
              void api.logout().finally(() => navigate('/login'))
            }}
          >
            <SignOut size={22} weight="light" aria-hidden />
          </button>
        </div>
      </aside>
      <main className="stage">
        <Outlet />
      </main>
      <CheckinDialog open={checkinOpen} onClose={() => setCheckinOpen(false)} />
      {taskId ? <TaskDossierSheet id={taskId} onClose={closeTask} /> : null}
    </div>
    </TaskLogProvider>
  )
}
