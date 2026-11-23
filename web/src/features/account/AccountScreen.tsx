import { Button, Input } from '@want-foundation/desyst-react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '../../api'
import { notifySessionChange, useAccount } from '../../shared/account'
import { useTimezone } from '../../shared/timezone'
import { Window } from '../../shared/Window'
import { workspaceUrl } from '../../shared/workspacePath'

export function WorkspacePicker() {
  const account = useAccount()
  function switchWorkspace(id: string) {
    const next = account.workspaces.find((space) => space.id === id)
    if (next) window.location.assign(workspaceUrl(id, next.kind === 'team' ? '/schedule' : '/today'))
  }
  return (
    <label className="workspace-picker">Workspace
      <select value={account.workspace.id} onChange={(e) => switchWorkspace(e.target.value)}>
        {account.workspaces.map((space) => <option key={space.id} value={space.id}>{space.name}</option>)}
      </select>
    </label>
  )
}

export function AccountScreen() {
  const account = useAccount()
  const { workspace, user, capabilities } = account
  const client = useQueryClient()
  const timeZone = useTimezone()
  const [teamName, setTeamName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [inviteEmail, setInviteEmail] = useState('')
  const stamp = (raw: string) => new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone }).format(new Date(raw))
  const canInvite = workspace.kind === 'team' && workspace.role === 'owner' && capabilities.canCollaborate
  const team = useMutation({
    mutationFn: () => api.createWorkspace(teamName.trim()),
    onSuccess: ({ id }) => window.location.assign(workspaceUrl(id, '/account')),
  })
  const binding = useMutation({ mutationFn: () => api.bindEmail(email.trim(), password), onSuccess: () => setPassword('') })
  const sessions = useQuery({ queryKey: ['sessions'], queryFn: api.sessions })
  const revoke = useMutation({
    mutationFn: api.revokeSession,
    onSuccess: (_, id) => {
      if (sessions.data?.find((session) => session.id === id)?.current) {
        notifySessionChange()
        window.location.replace('/login')
      } else void client.invalidateQueries({ queryKey: ['sessions'] })
    },
  })
  const invitationKey = ['invitations', workspace.id]
  const invitations = useQuery({
    queryKey: invitationKey,
    queryFn: () => api.invitations(workspace.id),
    enabled: workspace.kind === 'team' && workspace.role === 'owner',
  })
  const invite = useMutation({
    mutationFn: () => api.invite(workspace.id, inviteEmail.trim()),
    onSuccess: () => { setInviteEmail(''); void client.invalidateQueries({ queryKey: invitationKey }) },
  })
  const cancel = useMutation({
    mutationFn: (id: string) => api.revokeInvitation(workspace.id, id),
    onSuccess: () => client.invalidateQueries({ queryKey: invitationKey }),
  })
  return (
    <Window title="Account and workspaces">
      <div className="account-settings stack">
        <section className="stack" aria-labelledby="account-workspace">
          <h2 id="account-workspace">{workspace.name}</h2>
          <WorkspacePicker />
          <p>{workspace.kind === 'personal' ? 'Personal workspace' : 'Team workspace'} · {workspace.role === 'owner' ? 'Owner' : 'Editor'}</p>
          <p><strong>{workspace.plan === 'paid' ? 'Paid' : 'Free'}</strong> · {workspace.plan === 'paid' ? 'Unlimited sources and 28 days of planning.' : `${account.sourceCount} of 2 external sources used. Planning for today.`}</p>
          <p>{capabilities.canCollaborate ? 'Team collaboration is available on this plan.' : 'Shared editing requires a paid plan.'} {capabilities.canAnalyze ? 'Intelligent analysis is included.' : 'Intelligent analysis requires a paid plan.'}</p>
          {workspace.kind === 'team' ? <><h3>Members</h3><ul className="account-list">{account.members.map((member) => <li key={member.id}><span>{member.label}</span><small>{member.role === 'owner' ? 'Owner' : 'Editor'}</small></li>)}</ul></> : null}
        </section>

        {workspace.kind === 'team' && workspace.role === 'owner' ? <section className="stack" aria-labelledby="invite-heading">
          <h2 id="invite-heading">Invitations</h2>
          {canInvite ? <form className="stack" onSubmit={(e) => { e.preventDefault(); invite.mutate() }}>
            <label>Member email<Input type="email" value={inviteEmail} required onChange={(e) => { setInviteEmail(e.target.value); invite.reset() }} /></label>
            <div><Button type="submit" disabled={invite.isPending}>Send invitation</Button></div>
            {invite.isSuccess ? <p role="status">Invitation sent.</p> : null}
            {invite.error ? <p className="error" role="alert">{invite.error.message}</p> : null}
          </form> : <p>Invitations and shared editing require a paid workspace.</p>}
          {invitations.isLoading ? <p role="status">Loading invitations…</p> : null}
          {invitations.error ? <p role="alert" className="error">{invitations.error.message}</p> : null}
          {invitations.data?.length === 0 ? <p className="muted">No pending invitations.</p> : null}
          <ul className="account-list">{invitations.data?.map((entry) => <li key={entry.id}>
            <span>{entry.email}<small>Expires {stamp(entry.expiresAt)}</small></span>
            <Button type="button" variant="ghost" disabled={cancel.isPending} onClick={() => cancel.mutate(entry.id)} aria-label={`Revoke invitation to ${entry.email}`}>Revoke</Button>
          </li>)}</ul>
          {cancel.error ? <p className="error" role="alert">{cancel.error.message}</p> : null}
        </section> : null}

        <section className="stack" aria-labelledby="create-team">
          <h2 id="create-team">New team workspace</h2>
          <form className="stack" onSubmit={(e) => { e.preventDefault(); team.mutate() }}>
            <label>Workspace name<Input value={teamName} required onChange={(e) => setTeamName(e.target.value)} /></label>
            <div><Button type="submit" disabled={!teamName.trim() || team.isPending}>Create workspace</Button></div>
            {team.error ? <p className="error" role="alert">{team.error.message}</p> : null}
          </form>
        </section>

        <section className="stack" aria-labelledby="account-email">
          <h2 id="account-email">Sign-in email</h2>
          {user.email ? <p>{user.email}</p> : <form className="stack" onSubmit={(e) => { e.preventDefault(); binding.mutate() }}>
            <label>Email<Input type="email" autoComplete="email" required value={email} onChange={(e) => { setEmail(e.target.value); binding.reset() }} /></label>
            <label>Installation password<Input type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} /></label>
            <div><Button type="submit" disabled={binding.isPending}>Send confirmation link</Button></div>
            {binding.isSuccess ? <p role="status">Open the confirmation link in your inbox to finish linking this email.</p> : null}
            {binding.error ? <p className="error" role="alert">{binding.error.message}</p> : null}
          </form>}
        </section>

        <section className="stack" aria-labelledby="account-sessions">
          <h2 id="account-sessions">Active sessions</h2>
          {sessions.isLoading ? <p role="status">Loading sessions…</p> : null}
          {sessions.error ? <p className="error" role="alert">{sessions.error.message}</p> : null}
          <ul className="account-list">{sessions.data?.map((entry) => <li key={entry.id}>
            <span>{entry.current ? 'This session' : 'Session'} · {stamp(entry.createdAt)}<small>Expires {stamp(entry.expiresAt)}</small></span>
            <Button type="button" variant="ghost" disabled={revoke.isPending} onClick={() => revoke.mutate(entry.id)} aria-label={`End ${entry.current ? 'this session' : 'session from ' + stamp(entry.createdAt)}`}>End session</Button>
          </li>)}</ul>
          {revoke.error ? <p className="error" role="alert">{revoke.error.message}</p> : null}
        </section>
      </div>
    </Window>
  )
}
