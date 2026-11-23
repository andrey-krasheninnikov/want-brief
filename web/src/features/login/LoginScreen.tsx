import { Button, Input, ThemeToggle } from '@want-foundation/desyst-react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type FormEvent } from 'react'
import { api, ApiError } from '../../api'
import { notifySessionChange } from '../../shared/account'
import { Window } from '../../shared/Window'
import { useTheme } from '../../shared/theme'
import { workspaceUrl } from '../../shared/workspacePath'

type Mode = 'signin' | 'legacy' | 'register' | 'forgot'
type LinkAction = 'verify' | 'reset' | 'bind' | 'invite'

function emailLink(): { action: LinkAction; token: string } | null {
  const params = new URLSearchParams(window.location.hash.slice(1))
  for (const action of ['verify', 'reset', 'bind', 'invite'] as const) {
    const token = params.get(action)
    if (token) return { action, token }
  }
  return null
}

const titles = {
  signin: 'Sign in', legacy: 'Installation sign-in', register: 'Create account', forgot: 'Recover password',
  verify: 'Create your password', reset: 'Choose a new password', bind: 'Confirm your email', invite: 'Workspace invitation',
}

const submitLabels = {
  signin: 'Sign in', legacy: 'Sign in', register: 'Send confirmation link', forgot: 'Send recovery link',
  verify: 'Confirm and create account', bind: 'Confirm email', reset: 'Save password', invite: 'Accept invitation',
}

export function LoginScreen() {
  const client = useQueryClient()
  const { theme, changeTheme } = useTheme()
  const [link, setLink] = useState(emailLink)
  const [mode, setMode] = useState<Mode>('signin')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const inviting = link?.action === 'invite'
  const session = useQuery({ queryKey: ['invite-session'], queryFn: api.me, enabled: inviting, retry: false })
  const choosingAccount = inviting && session.isSuccess
  const tokenForm = link && !inviting ? link.action : null
  const form = tokenForm ?? mode
  const newPassword = form === 'verify' || form === 'reset'
  const needsEmail = !choosingAccount && (form === 'signin' || form === 'forgot' || form === 'register')
  const needsPassword = !choosingAccount && (newPassword || form === 'signin' || form === 'legacy')

  useEffect(() => {
    const heading = document.querySelector<HTMLElement>('.login-stage h1')
    if (heading) { heading.tabIndex = -1; heading.focus({ preventScroll: true }) }
  }, [form, choosingAccount])

  useEffect(() => {
    function readLink() {
      setLink(emailLink())
      setNotice('')
      setError('')
      setPassword('')
      setMode('signin')
    }
    window.addEventListener('hashchange', readLink)
    return () => window.removeEventListener('hashchange', readLink)
  }, [])

  function changeMode(next: Mode) {
    setMode(next)
    setPassword('')
    setError('')
    setNotice('')
  }

  function finishLink(message: string) {
    window.history.replaceState(null, '', window.location.pathname)
    setLink(null)
    changeMode('signin')
    setNotice(message)
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (pending) return
    setError(''); setNotice(''); setPending(true)
    try {
      if (newPassword && Array.from(password).length < 12) throw new Error('Use at least 12 characters.')
      if (newPassword && new TextEncoder().encode(password).length > 72) throw new Error('Use a password of up to 72 UTF-8 bytes.')
      if (choosingAccount && link) {
        const result = await api.acceptInvitation(link.token)
        window.location.replace(workspaceUrl(result.workspaceId, '/account'))
      } else if (link && (form === 'verify' || form === 'bind')) {
        await api.confirmEmail(link.token, form === 'verify' ? password : undefined)
        finishLink('Email confirmed. You can now sign in with your email and password.')
      } else if (link && form === 'reset') {
        await api.resetPassword(link.token, password)
        notifySessionChange()
        finishLink('Password changed. Sign in with your new password.')
      } else if (form === 'register' || form === 'forgot') {
        if (form === 'register') await api.register(email.trim())
        else await api.forgotPassword(email.trim())
        setNotice(form === 'register' ? 'Check your inbox for the link to set your password.' : 'If this email has an account, a recovery link will arrive shortly.')
      } else {
        await api.login(password, form === 'legacy' ? undefined : email.trim())
        notifySessionChange()
        setPassword('')
        if (inviting) await client.invalidateQueries({ queryKey: ['invite-session'] })
        else window.location.replace('/today')
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Request failed. Try again.')
    } finally {
      setPending(false)
    }
  }

  async function switchAccount() {
    setPending(true); setError('')
    try {
      await api.logout()
      notifySessionChange()
      await client.invalidateQueries({ queryKey: ['invite-session'] })
      changeMode('signin')
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not sign out.') }
    finally { setPending(false) }
  }

  const sessionError = session.isError && !(session.error instanceof ApiError && session.error.status === 401)
  return (
    <div className="login-stage">
      <Window title={titles[inviting ? 'invite' : form]} actions={<ThemeToggle theme={theme} onThemeChange={changeTheme} label="Switch theme" />}>
        {inviting && session.isPending ? <p role="status">Checking your account…</p> : inviting && sessionError ? (
          <><p role="alert">Could not check your account.</p><Button onClick={() => void session.refetch()}>Retry</Button></>
        ) : (
          <form className="stack" onSubmit={submit}>
            <img src="/logo.png" alt="" width={48} height={48} />
            {choosingAccount ? <p>Accept this invitation as <strong>{session.data.user.email || 'the installation owner'}</strong>.</p> : null}
            {inviting && !choosingAccount ? <p>Sign in with the email that received the invitation. If you create an account, reopen this invitation after confirming your address.</p> : null}
            {form === 'legacy' ? <p>Use the original installation password until you confirm an email in Account.</p> : null}
            {form === 'bind' ? <p>Use this address to sign in to your existing account.</p> : null}
            {needsEmail ? <label>Email<Input type="email" autoComplete="email" name="email" value={email} onChange={(e) => setEmail(e.target.value)} required /></label> : null}
            {needsPassword ? <label>{newPassword ? 'New password' : 'Password'}<Input type="password" name="password" autoComplete={newPassword ? 'new-password' : 'current-password'} minLength={newPassword ? 12 : undefined} aria-describedby={newPassword ? 'password-hint' : undefined} value={password} onChange={(e) => setPassword(e.target.value)} required /></label> : null}
            {newPassword ? <small id="password-hint">Use at least 12 characters.</small> : null}
            {error ? <p className="error" role="alert">{error}</p> : null}
            {notice ? <p role="status">{notice}</p> : null}
            <Button type="submit" disabled={pending || (choosingAccount && !session.data.user.email)}>{submitLabels[choosingAccount ? 'invite' : form]}</Button>
            {choosingAccount ? <Button type="button" variant="ghost" disabled={pending} onClick={() => void switchAccount()}>Use another account</Button> : null}
            {!tokenForm && !choosingAccount ? <div className="account-links">
              {mode !== 'signin' ? <Button type="button" variant="ghost" onClick={() => changeMode('signin')}>Back to sign in</Button> : null}
              {mode === 'signin' ? <>
                <Button type="button" variant="ghost" onClick={() => changeMode('register')}>Create account</Button>
                <Button type="button" variant="ghost" onClick={() => changeMode('forgot')}>Forgot password?</Button>
                {!inviting ? <Button type="button" variant="ghost" onClick={() => changeMode('legacy')}>Use installation password</Button> : null}
              </> : null}
            </div> : null}
            {tokenForm ? <a href="/login">Back to sign in</a> : null}
          </form>
        )}
      </Window>
    </div>
  )
}
