import {
  CalendarBlank,
  CalendarDots,
  ChartBar,
  CheckSquare,
  DotsThree,
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
import { Button, DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger, ThemeToggle } from '@want-foundation/desyst-react'
import { useEffect, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../api'
import { EventDossierSheet } from '../features/events/EventDossier'
import { TaskDossierSheet } from '../features/tasks/TaskPage'
import { TaskLogProvider } from '../features/tasks/TaskLogPrompt'
import { CheckinDialog } from '../features/today/CheckinDialog'
import { useTheme } from './theme'

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

function NavigationLink({ to, label, Icon }: (typeof links)[number]) {
  return (
    <NavLink to={to} className={({ isActive }) => (isActive ? 'active' : '')}>
      <Icon size={20} weight="light" aria-hidden />
      <span>{label}</span>
    </NavLink>
  )
}

export function Shell() {
  const navigate = useNavigate()
  const location = useLocation()
  const [searchParams, setSearchParams] = useSearchParams()
  const [checkinOpen, setCheckinOpen] = useState(false)
  const { theme, changeTheme } = useTheme()
  const taskId = searchParams.get('task')
  const eventId = searchParams.get('event')
  const eventOn = searchParams.get('on') ?? undefined
  const moreActive = links.slice(5).some(({ to }) => location.pathname === to || location.pathname.startsWith(`${to}/`))

  useEffect(() => {
    const heading = document.querySelector<HTMLElement>('.stage .ui-window-header h1')
    if (!heading) return
    heading.tabIndex = -1
    heading.focus({ preventScroll: true })
  }, [location.pathname])

  function closeTask() {
    const next = new URLSearchParams(searchParams)
    next.delete('task')
    setSearchParams(next, { replace: true })
  }

  function closeEvent() {
    const next = new URLSearchParams(searchParams)
    next.delete('event')
    next.delete('on')
    setSearchParams(next, { replace: true })
  }

  function logout() {
    void api.logout().finally(() => navigate('/login'))
  }

  return (
    <TaskLogProvider>
      <div className="app-shell">
        <a className="skip-link" href="#main-content">Skip to content</a>
        <aside className="sidebar" aria-label="Main navigation">
          <div className="brand">
            <img src="/logo.png" alt="" width={32} height={32} />
            <strong>Want Brief</strong>
          </div>
          <nav className="primary-nav" aria-label="Main">
            {links.map((link) => <NavigationLink key={link.to} {...link} />)}
          </nav>
          <div className="sidebar-foot">
            <ThemeToggle theme={theme} onThemeChange={changeTheme} label="Switch theme" />
            <Button type="button" variant="ghost" onClick={() => setCheckinOpen(true)}>
              <Heartbeat size={20} weight="light" aria-hidden />Check-in
            </Button>
            <Button type="button" variant="ghost" onClick={logout}>
              <SignOut size={20} weight="light" aria-hidden />Log out
            </Button>
          </div>
        </aside>
        <main id="main-content" className="stage">
          <Outlet />
        </main>
        <nav className="mobile-nav" aria-label="Main">
          {links.slice(0, 5).map((link) => <NavigationLink key={link.to} {...link} />)}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="ghost" className={moreActive ? 'mobile-more active' : 'mobile-more'} aria-label="More sections" aria-current={moreActive ? 'page' : undefined}>
                <DotsThree size={22} weight="light" aria-hidden />
                <span>More</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent side="top" align="end" className="mobile-more-menu">
              {links.slice(5).map((link) => (
                <DropdownMenuItem key={link.to} aria-current={location.pathname === link.to || location.pathname.startsWith(`${link.to}/`) ? 'page' : undefined} onSelect={() => navigate(link.to)}>
                  <link.Icon size={18} weight="light" aria-hidden />{link.label}
                </DropdownMenuItem>
              ))}
              <DropdownMenuItem onSelect={() => changeTheme(theme === 'dark' ? 'light' : 'dark')}>
                {theme === 'dark' ? 'Light theme' : 'Dark theme'}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setCheckinOpen(true)}>Check-in</DropdownMenuItem>
              <DropdownMenuItem onSelect={logout}>Log out</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </nav>
        <CheckinDialog open={checkinOpen} onClose={() => setCheckinOpen(false)} />
        {taskId ? <TaskDossierSheet id={taskId} onClose={closeTask} /> : null}
        {eventId ? <EventDossierSheet seriesId={eventId} originalOn={eventOn} onClose={closeEvent} /> : null}
      </div>
    </TaskLogProvider>
  )
}
