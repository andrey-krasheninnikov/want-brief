import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../api'
import { Window } from '../../../shared/Window'
import type { ScheduleSettings } from '../../../types'
import { DayOverridesEditor } from './DayOverridesEditor'
import { NumberField, Section, TimeField, Toggle } from './SettingsField'
import './settings.css'

const WEEKDAYS = [
  { value: 1, label: 'Mon' },
  { value: 2, label: 'Tue' },
  { value: 3, label: 'Wed' },
  { value: 4, label: 'Thu' },
  { value: 5, label: 'Fri' },
  { value: 6, label: 'Sat' },
  { value: 0, label: 'Sun' },
]

const TIMEZONES = ['Europe/Moscow', 'Asia/Tashkent', 'Europe/Belgrade', 'Europe/London', 'UTC']

function validate(s: ScheduleSettings): string {
  const numbers: [string, number][] = [
    ['Meeting buffer', s.meetingBufferMin],
    ['Daily focus', s.dailyFocusMin],
    ['Tasks per day', s.maxTasksPerDay],
    ['Min slice', s.minSliceMin],
    ['Max slice', s.maxSliceMin],
    ['Slice break', s.sliceBreakMin],
    ['Grid', s.gridMin],
    ['Estimate cap', s.estimateMaxK],
    ['Stress shift', s.stressShiftHours],
    ['Target lead', s.targetLeadWorkdays],
    ['Waiting tracks', s.waitingTracks],
    ['Ping length', s.followupPingMin],
    ['Stability threshold', s.stabilityThresholdMin],
  ]
  for (const [label, value] of numbers) {
    if (!Number.isFinite(value) || value < 0) return `${label} must be a number`
  }
  if (s.workStartMin >= s.workEndMin) return 'Work start must be before work end'
  if (s.workdays.length === 0) return 'Pick at least one workday'
  if (s.break && s.break.startMin >= s.break.endMin) return 'Break start must be before break end'
  if (s.gridMin <= 0) return 'Grid must be at least one minute'
  if (s.maxSliceMin > 0 && s.minSliceMin > s.maxSliceMin) return 'Min slice must not exceed max slice'
  if (s.checkWindows.length === 0) return 'Add at least one check window'
  return ''
}

// ScheduleSettingsScreen loads the stored settings once and hands them to the form as its initial draft.
export function ScheduleSettingsScreen() {
  const stored = useQuery({ queryKey: ['schedule-settings'], queryFn: api.scheduleSettings })
  if (!stored.data) {
    return (
      <Window className="sset-window" kicker="Schedule" title="Packer settings">
        {stored.isError ? <p className="error">{stored.error.message}</p> : <p className="muted">Loading…</p>}
      </Window>
    )
  }
  return <SettingsForm initial={stored.data} />
}

function SettingsForm({ initial }: { initial: ScheduleSettings }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<ScheduleSettings>(initial)
  const [windowsRaw, setWindowsRaw] = useState(initial.checkWindows.join(', '))
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  const save = useMutation({
    mutationFn: (body: ScheduleSettings) => api.saveScheduleSettings(body),
    onSuccess: (next) => {
      setDraft(next)
      setWindowsRaw(next.checkWindows.join(', '))
      setSaved(true)
      void queryClient.invalidateQueries({ queryKey: ['schedule-settings'] })
      void queryClient.invalidateQueries({ queryKey: ['schedule'] })
    },
    onError: (err: Error) => setError(err.message),
  })

  function patch(next: Partial<ScheduleSettings>) {
    setSaved(false)
    setDraft((prev) => ({ ...prev, ...next }))
  }

  function submit() {
    setError('')
    const windows = windowsRaw
      .split(/[,\s]+/)
      .map((raw) => raw.trim())
      .filter(Boolean)
    const body: ScheduleSettings = { ...draft, checkWindows: windows }
    const problem = validate(body)
    if (problem) {
      setError(problem)
      return
    }
    save.mutate(body)
  }

  function toggleWorkday(value: number) {
    const has = draft.workdays.includes(value)
    const next = has ? draft.workdays.filter((d) => d !== value) : [...draft.workdays, value]
    patch({ workdays: next.sort((a, b) => ((a + 6) % 7) - ((b + 6) % 7)) })
  }

  return (
    <Window
      className="sset-window"
      kicker="Schedule"
      title="Packer settings"
      actions={
        <div className="sset-actions">
          <button type="button" className="ghost" onClick={() => navigate('/schedule')}>
            Back to schedule
          </button>
          <button type="button" disabled={save.isPending} onClick={submit}>
            {save.isPending ? 'Saving…' : saved ? 'Saved' : 'Save'}
          </button>
        </div>
      }
    >
      {error ? <p className="error">{error}</p> : null}
      <div className="sset-body">
          <Section title="Hours" hint="The daily window the packer may fill. The break is optional and blocks every workday.">
            <label className="sset-field">
              <span>Timezone</span>
              <span className="sset-input">
                <input list="sset-tz" value={draft.timezone} onChange={(e) => patch({ timezone: e.target.value })} />
                <datalist id="sset-tz">
                  {TIMEZONES.map((tz) => (
                    <option key={tz} value={tz} />
                  ))}
                </datalist>
              </span>
            </label>
            <TimeField label="Work starts" value={draft.workStartMin} onChange={(v) => patch({ workStartMin: v })} />
            <TimeField label="Work ends" value={draft.workEndMin} onChange={(v) => patch({ workEndMin: v })} />
            <div className="sset-field sset-span">
              <span>Workdays</span>
              <span className="sset-chips">
                {WEEKDAYS.map((day) => (
                  <button
                    key={day.value}
                    type="button"
                    className={draft.workdays.includes(day.value) ? 'chip on' : 'chip'}
                    onClick={() => toggleWorkday(day.value)}
                  >
                    {day.label}
                  </button>
                ))}
              </span>
            </div>
            <Toggle
              label="Daily break"
              hint="Off by default: no fixed lunch."
              value={draft.break != null}
              onChange={(on) => patch({ break: on ? { startMin: 13 * 60, endMin: 14 * 60 } : null })}
            />
            {draft.break ? (
              <>
                <TimeField label="Break from" value={draft.break.startMin} onChange={(v) => patch({ break: { ...draft.break!, startMin: v } })} />
                <TimeField label="Break to" value={draft.break.endMin} onChange={(v) => patch({ break: { ...draft.break!, endMin: v } })} />
              </>
            ) : null}
          </Section>

          <Section title="Meetings" hint="Meetings block time with a buffer on both sides; skippable ones give way only when a task would run late.">
            <NumberField label="Buffer around meetings" value={draft.meetingBufferMin} min={0} max={60} unit="min" onChange={(v) => patch({ meetingBufferMin: v })} />
            <Toggle label="Use skippable meetings" hint="Pack over meetings marked can-skip when the alternative is being late." value={draft.softBusy} onChange={(v) => patch({ softBusy: v })} />
          </Section>

          <Section title="Slicing" hint="How work is cut: grid rounding, the shortest gap worth using, the longest stretch before a pause.">
            <NumberField label="Grid" value={draft.gridMin} min={1} max={60} unit="min" onChange={(v) => patch({ gridMin: v })} />
            <NumberField label="Min slice" value={draft.minSliceMin} min={0} max={240} unit="min" onChange={(v) => patch({ minSliceMin: v })} />
            <NumberField label="Max slice" value={draft.maxSliceMin} min={0} max={480} unit="min" onChange={(v) => patch({ maxSliceMin: v })} />
            <NumberField label="Pause after a slice" value={draft.sliceBreakMin} min={0} max={60} unit="min" onChange={(v) => patch({ sliceBreakMin: v })} />
          </Section>

          <Section title="Limits" hint="Caps per day. Zero disables a cap.">
            <NumberField label="Focus per day" value={draft.dailyFocusMin} min={0} max={900} step={15} unit="min" onChange={(v) => patch({ dailyFocusMin: v })} />
            <NumberField label="Tasks per day" value={draft.maxTasksPerDay} min={0} max={30} onChange={(v) => patch({ maxTasksPerDay: v })} />
          </Section>

          <Section title="Ordering" hint="What goes first: slack against the effective due date, then importance and urgency, then age.">
            <NumberField label="Target lead" value={draft.targetLeadWorkdays} min={0} max={10} unit="workdays" onChange={(v) => patch({ targetLeadWorkdays: v })} />
            <NumberField label="Stress shift" value={draft.stressShiftHours} min={0} max={48} unit="h per stress point" onChange={(v) => patch({ stressShiftHours: v })} />
            <Toggle label="Oldest first" hint="Among equals, the older task goes first." value={draft.oldestFirst} onChange={(v) => patch({ oldestFirst: v })} />
            <Toggle label="Estimate buffer" hint="Scale estimates by each project's tracked-to-planned history." value={draft.estimateBuffer} onChange={(v) => patch({ estimateBuffer: v })} />
            <NumberField label="Estimate cap" value={draft.estimateMaxK} min={1} max={5} step={0.1} unit="×" onChange={(v) => patch({ estimateMaxK: v })} />
          </Section>

          <Section title="Parallel work" hint="One active track plus waiting tracks for tasks that mostly wait on someone else.">
            <NumberField label="Waiting tracks" value={draft.waitingTracks} min={0} max={5} onChange={(v) => patch({ waitingTracks: v })} />
          </Section>

          <Section title="Follow-ups" hint="Reviews and decisions become short pings inside check windows, one per workday until due.">
            <NumberField label="Ping length" value={draft.followupPingMin} min={5} max={120} unit="min" onChange={(v) => patch({ followupPingMin: v })} />
            <label className="sset-field">
              <span>Check windows</span>
              <span className="sset-input">
                <input
                  type="text"
                  value={windowsRaw}
                  placeholder="10:00, 14:00"
                  onChange={(e) => {
                    setSaved(false)
                    setWindowsRaw(e.target.value)
                  }}
                />
              </span>
            </label>
          </Section>

          <Section title="Human factor" hint="Signals from check-ins and tracked history shape where tasks land.">
            <Toggle label="Energy aware" hint="Low energy or focus keeps the first two hours for light tasks." value={draft.energyAware} onChange={(v) => patch({ energyAware: v })} />
            <Toggle label="Golden hours" hint="Do-first tasks prefer the hours where the last eight weeks show the most tracked work." value={draft.goldenHours} onChange={(v) => patch({ goldenHours: v })} />
          </Section>

          <Section title="Stability" hint="Blocks stay put unless a fresh layout wins by more than the threshold.">
            <NumberField label="Threshold" value={draft.stabilityThresholdMin} min={0} max={480} unit="min" onChange={(v) => patch({ stabilityThresholdMin: v })} />
          </Section>

          <DayOverridesEditor tz={draft.timezone} workStartMin={draft.workStartMin} workEndMin={draft.workEndMin} />
      </div>
    </Window>
  )
}
