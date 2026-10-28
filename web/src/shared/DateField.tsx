import { DateField as DesystDateField, type DateFieldLabels, type DateFieldProps } from '@want-foundation/desyst-react'
import { useTimezone } from './timezone'
import { zonedLocalInput } from './zoned'

const labels: DateFieldLabels = {
  date: 'Pick date',
  datetime: 'Pick date and time',
  calendar: 'Calendar',
  year: 'Year',
  previousYear: 'Previous year',
  previousMonth: 'Previous month',
  nextMonth: 'Next month',
  nextYear: 'Next year',
  hours: 'Hours',
  minutes: 'Minutes',
  today: 'Today',
  clear: 'Clear',
  apply: 'Apply',
}

type Props = Pick<DateFieldProps, 'mode' | 'value' | 'onChange' | 'onCommit'> & { timeZone?: string }

export function DateField({ timeZone, ...props }: Props) {
  const contextTimeZone = useTimezone()
  const today = zonedLocalInput(new Date(), timeZone ?? contextTimeZone)
  return <DesystDateField {...props} labels={labels} locale="en-GB" today={today} />
}
