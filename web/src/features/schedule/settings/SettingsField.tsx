import type { ReactNode } from 'react'

// Section groups related knobs under a short title and one-line rationale.
export function Section({ title, hint, children }: { title: string; hint: string; children: ReactNode }) {
  return (
    <section className="sset-section">
      <header>
        <h3>{title}</h3>
        <p>{hint}</p>
      </header>
      <div className="sset-grid">{children}</div>
    </section>
  )
}

export function NumberField({
  label,
  value,
  min,
  max,
  step = 1,
  unit,
  onChange,
}: {
  label: string
  value: number
  min?: number
  max?: number
  step?: number
  unit?: string
  onChange: (next: number) => void
}) {
  return (
    <label className="sset-field">
      <span>{label}</span>
      <span className="sset-input">
        <input
          type="number"
          value={Number.isFinite(value) ? value : ''}
          min={min}
          max={max}
          step={step}
          onChange={(e) => onChange(e.target.value === '' ? Number.NaN : Number(e.target.value))}
        />
        {unit ? <small>{unit}</small> : null}
      </span>
    </label>
  )
}

export function Toggle({ label, hint, value, onChange }: { label: string; hint?: string; value: boolean; onChange: (next: boolean) => void }) {
  return (
    <label className="sset-toggle">
      <input type="checkbox" checked={value} onChange={(e) => onChange(e.target.checked)} />
      <span>
        <b>{label}</b>
        {hint ? <small>{hint}</small> : null}
      </span>
    </label>
  )
}

export function TimeField({ label, value, onChange }: { label: string; value: number; onChange: (minutes: number) => void }) {
  const clock = `${String(Math.floor(value / 60)).padStart(2, '0')}:${String(value % 60).padStart(2, '0')}`
  return (
    <label className="sset-field">
      <span>{label}</span>
      <span className="sset-input">
        <input
          type="time"
          step={300}
          value={clock}
          onChange={(e) => {
            const [h, m] = e.target.value.split(':').map(Number)
            if (Number.isFinite(h) && Number.isFinite(m)) onChange(h * 60 + m)
          }}
        />
      </span>
    </label>
  )
}
