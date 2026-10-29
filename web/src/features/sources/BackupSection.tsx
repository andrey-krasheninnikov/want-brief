import { Button } from '@want-foundation/desyst-react'
import { useQueryClient } from '@tanstack/react-query'
import { useState, type ChangeEvent } from 'react'
import { api } from '../../api'

type Summary = {
  exportedAt: string
  records: number
}

const maxBytes = 64 * 1024 * 1024

export function BackupSection() {
  const queryClient = useQueryClient()
  const [file, setFile] = useState<File | null>(null)
  const [summary, setSummary] = useState<Summary | null>(null)
  const [confirmed, setConfirmed] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)

  async function download() {
    setError('')
    setBusy(true)
    try {
      const blob = await api.downloadBackup()
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `want-brief-${new Date().toISOString().slice(0, 10)}.json`
      document.body.append(link)
      link.click()
      link.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not download backup.')
    } finally {
      setBusy(false)
    }
  }

  async function choose(event: ChangeEvent<HTMLInputElement>) {
    const selected = event.target.files?.[0]
    event.target.value = ''
    setFile(null)
    setSummary(null)
    setConfirmed(false)
    setDone(false)
    setError('')
    if (!selected) return
    if (selected.size > maxBytes) {
      setError('Backup exceeds 64 MiB.')
      return
    }
    try {
      const parsed: unknown = JSON.parse(await selected.text())
      if (!parsed || typeof parsed !== 'object' || !('tables' in parsed) ||
        !parsed.tables || typeof parsed.tables !== 'object' || Array.isArray(parsed.tables) ||
        !('exportedAt' in parsed) || typeof parsed.exportedAt !== 'string' ||
        !('formatVersion' in parsed) || parsed.formatVersion !== 1 ||
        !Object.values(parsed.tables).every(Array.isArray)) {
        throw new Error('Invalid backup file.')
      }
      setSummary({
        exportedAt: parsed.exportedAt,
        records: Object.values(parsed.tables).reduce((total, rows) => total + (rows as unknown[]).length, 0),
      })
      setFile(selected)
    } catch {
      setError('Invalid backup file.')
    }
  }

  async function restore() {
    if (!file || !confirmed) return
    setError('')
    setBusy(true)
    try {
      await api.restoreBackup(file)
      await queryClient.invalidateQueries()
      setFile(null)
      setSummary(null)
      setConfirmed(false)
      setDone(true)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not restore backup.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="stack" aria-labelledby="backup-heading">
      <h3 id="backup-heading">Backup</h3>
      <p>Download your records and settings. Source tokens, account password, and sessions are excluded.</p>
      <div><Button type="button" onClick={() => void download()} disabled={busy}>Download backup</Button></div>
      <label htmlFor="backup-file">Restore from a backup file</label>
      <input id="backup-file" type="file" accept=".json,application/json" onChange={(event) => void choose(event)} disabled={busy} />
      {summary ? (
        <>
          <p>{file?.name}: {summary.records} records, exported {summary.exportedAt}.</p>
          <p>Restoring replaces all current records. Download a current backup before continuing. Source tokens must be entered again.</p>
          <label className="row">
            <input type="checkbox" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} disabled={busy} />
            I understand that current records will be replaced.
          </label>
          <div><Button type="button" onClick={() => void restore()} disabled={!confirmed || busy}>Restore backup</Button></div>
        </>
      ) : null}
      {error ? <p className="error" role="alert">{error}</p> : null}
      {done ? <p role="status">Backup restored. Re-enter source tokens before syncing.</p> : null}
    </section>
  )
}
