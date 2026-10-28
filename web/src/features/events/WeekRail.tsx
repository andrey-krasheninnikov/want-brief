type Props = {
  days: string[]
  selected: string
  today: string
  onSelect: (ymd: string) => void
  onPrev: () => void
  onNext: () => void
}

function dayParts(ymd: string) {
  const date = new Date(`${ymd}T12:00:00+03:00`)
  return {
    week: new Intl.DateTimeFormat('en-GB', { weekday: 'short', timeZone: 'Europe/Moscow' }).format(date),
    num: new Intl.DateTimeFormat('en-GB', { day: 'numeric', timeZone: 'Europe/Moscow' }).format(date),
  }
}

export function WeekRail({ days, selected, today, onSelect, onPrev, onNext }: Props) {
  return (
    <div className="events-rail">
      <button type="button" className="ghost" aria-label="Previous week" onClick={onPrev}>
        ‹
      </button>
      <div className="events-days">
        {days.map((ymd) => {
          const parts = dayParts(ymd)
          const classes = [
            'events-day',
            ymd === selected ? 'on' : '',
            ymd === today ? 'today' : '',
          ]
            .filter(Boolean)
            .join(' ')
          return (
            <button key={ymd} type="button" className={classes} onClick={() => onSelect(ymd)}>
              <small>{parts.week}</small>
              <strong>{parts.num}</strong>
            </button>
          )
        })}
      </div>
      <button type="button" className="ghost" aria-label="Next week" onClick={onNext}>
        ›
      </button>
    </div>
  )
}
