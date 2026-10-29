import { Button, Input, ThemeToggle } from '@want-foundation/desyst-react'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api'
import { Window } from '../../shared/Window'
import { useTheme } from '../../shared/theme'

export function LoginScreen() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { theme, changeTheme } = useTheme()
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    const heading = document.querySelector<HTMLElement>('.login-stage .ui-window-header h1')
    if (!heading) return
    heading.tabIndex = -1
    heading.focus({ preventScroll: true })
  }, [])

  async function submit() {
    setError('')
    try {
      await api.login(password)
      queryClient.setQueryData(['me'], { ok: true })
      navigate('/today')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not sign in. Try again.')
    }
  }

  return (
    <div className="login-stage">
      <Window title="Welcome back" actions={<ThemeToggle theme={theme} onThemeChange={changeTheme} label="Switch theme" />}>
        <form className="stack" onSubmit={(event) => { event.preventDefault(); void submit() }}>
          <img src="/logo.png" alt="" width={56} height={56} />
          <h2>Want Brief</h2>
          <p className="muted">Sign in to see your work.</p>
          <label>
            Password
            <Input type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} aria-invalid={Boolean(error)} aria-describedby={error ? 'login-error' : undefined} required />
          </label>
          {error ? <p className="error" id="login-error" role="alert">{error}</p> : null}
          <Button type="submit">Sign in</Button>
        </form>
      </Window>
    </div>
  )
}
