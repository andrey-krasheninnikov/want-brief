import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../api'
import { Window } from '../../shared/Window'

export function LoginScreen() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  async function submit() {
    setError('')
    try {
      await api.login(password)
      queryClient.setQueryData(['me'], { ok: true })
      navigate('/today')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'login failed')
    }
  }

  return (
    <div className="login-stage">
      <Window title="Welcome back">
        <form
          className="stack"
          onSubmit={(event) => {
            event.preventDefault()
            void submit()
          }}
        >
          <img src="/logo.png" alt="Want Brief" width={56} height={56} />
          <h2>Want Brief</h2>
          <p className="muted">Small briefs, big progress.</p>
          <label>
            Password
            <input
              type="password"
              autoFocus
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>
          {error ? <p className="error">{error}</p> : null}
          <button type="button" onClick={() => void submit()}>
            Enter
          </button>
        </form>
      </Window>
    </div>
  )
}
