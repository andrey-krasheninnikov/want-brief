import { useAccount } from '../../shared/account'

export function AssigneeField({ value, onChange }: { value: string | null; onChange: (id: string | null) => void }) {
  const { workspace, members } = useAccount()
  if (workspace.kind !== 'team') return null
  return <label>Assigned member
    <select value={value ?? ''} onChange={(event) => onChange(event.target.value || null)}>
      <option value="">Unassigned</option>
      {members.map((member) => <option key={member.id} value={member.id}>{member.label}</option>)}
    </select>
  </label>
}

export function ParticipantField({ value, onChange }: { value: string[]; onChange: (ids: string[]) => void }) {
  const { workspace, members } = useAccount()
  if (workspace.kind !== 'team') return null
  return <fieldset className="member-choices">
    <legend>Workspace participants</legend>
    <p className="muted">This event reserves time for the selected members.</p>
    {members.map((member) => <label key={member.id} className="row">
      <input type="checkbox" checked={value.includes(member.id)} onChange={(event) => {
        onChange(event.target.checked ? [...value, member.id] : value.filter((id) => id !== member.id))
      }} />{member.label}
    </label>)}
  </fieldset>
}
