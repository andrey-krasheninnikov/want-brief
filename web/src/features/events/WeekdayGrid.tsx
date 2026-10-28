const DAYS = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']

type Props = {
  value: number[]
  onChange: (next: number[]) => void
}

export function WeekdayGrid({ value, onChange }: Props) {
  function toggle(slot: number) {
    onChange(value.includes(slot) ? value.filter((item) => item !== slot) : [...value, slot].sort((a, b) => a - b))
  }

  return (
    <div className="events-weeks">
      {[0, 1].map((week) => (
        <div key={week} className="events-week-row">
          <span>Week {week + 1}</span>
          {DAYS.map((label, index) => {
            const slot = week * 7 + index
            return (
              <button
                key={slot}
                type="button"
                className={value.includes(slot) ? 'on' : ''}
                aria-pressed={value.includes(slot)}
                onClick={() => toggle(slot)}
              >
                {label}
              </button>
            )
          })}
        </div>
      ))}
    </div>
  )
}
