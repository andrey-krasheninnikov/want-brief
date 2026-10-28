package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// calendar answers "when can work happen": settings windows, day overrides,
// optional break, meeting buffers and the hard/soft busy split.
type calendar struct {
	settings  ScheduleSettings
	loc       *time.Location
	overrides map[Ymd]DayOverride
}

type softSpan struct {
	span
	title string
}

func newCalendar(settings ScheduleSettings, overrides []DayOverride, loc *time.Location) *calendar {
	c := &calendar{settings: settings, loc: loc, overrides: map[Ymd]DayOverride{}}
	for _, o := range overrides {
		c.overrides[o.Day] = o
	}
	return c
}

func (c *calendar) midnight(day time.Time) time.Time {
	y, m, d := day.In(c.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, c.loc)
}

func (c *calendar) at(day time.Time, minute int) time.Time {
	return c.midnight(day).Add(time.Duration(minute) * time.Minute)
}

func (c *calendar) ymd(t time.Time) Ymd {
	return YmdOf(t, c.loc)
}

func (c *calendar) nextDay(t time.Time) time.Time {
	return c.midnight(t).AddDate(0, 0, 1)
}

// window returns the raw work window for a day in minutes since midnight.
func (c *calendar) window(day time.Time) (int, int, bool) {
	if o, ok := c.overrides[c.ymd(day)]; ok {
		if o.Off {
			return 0, 0, false
		}
		if start, end, ok := o.Window(); ok {
			return start, end, true
		}
	}
	if !c.settings.isWorkday(day.In(c.loc).Weekday()) {
		return 0, 0, false
	}
	return c.settings.WorkStartMin, c.settings.WorkEndMin, true
}

func (c *calendar) isWorkday(day time.Time) bool {
	_, _, ok := c.window(day)
	return ok
}

// windows returns work spans for a day with the break carved out.
func (c *calendar) windows(day time.Time) []span {
	start, end, ok := c.window(day)
	if !ok {
		return nil
	}
	full := span{start: c.at(day, start), end: c.at(day, end)}
	if b := c.settings.Break; b != nil {
		bs, be := c.at(day, b.StartMin), c.at(day, b.EndMin)
		return subtractSpan([]span{full}, bs, be)
	}
	return []span{full}
}

// workSeconds sums the work-window time between two instants, so slack is
// measured in hours that can actually be worked rather than wall-clock.
func (c *calendar) workSeconds(from, to time.Time) int64 {
	if !to.After(from) {
		return 0
	}
	var total int64
	for day := c.midnight(from); day.Before(to); day = c.nextDay(day) {
		for _, window := range c.windows(day) {
			if start, end, ok := clipInterval(window.start, window.end, from, to); ok {
				total += spanSeconds(start, end)
			}
		}
	}
	return total
}

// daySeconds is the nominal capacity of a standard workday.
func (c *calendar) daySeconds() int64 {
	total := int64(c.settings.WorkEndMin-c.settings.WorkStartMin) * 60
	if b := c.settings.Break; b != nil {
		total -= int64(b.EndMin-b.StartMin) * 60
	}
	if total <= 0 {
		return 1
	}
	return total
}

func (c *calendar) breakSpan(day time.Time) (span, bool) {
	b := c.settings.Break
	if b == nil || !c.isWorkday(day) {
		return span{}, false
	}
	return span{start: c.at(day, b.StartMin), end: c.at(day, b.EndMin)}, true
}

// cursor is the first instant from now where work may be packed.
func (c *calendar) cursor(now time.Time) time.Time {
	now = now.In(c.loc).Truncate(time.Second)
	for d := 0; d < scheduleHorizonDays; d++ {
		anchor := now.AddDate(0, 0, d)
		for _, window := range c.windows(anchor) {
			if now.Before(window.start) {
				return window.start
			}
			if now.Before(window.end) {
				return now
			}
		}
	}
	return now
}

func (c *calendar) buffered(event EventOccurrence) span {
	buf := time.Duration(c.settings.MeetingBufferMin) * time.Minute
	start, end := event.ActiveSpan()
	return span{start: start.Add(-buf), end: end.Add(buf)}
}

func (c *calendar) isSoft(event EventOccurrence) bool {
	return c.settings.SoftBusy && event.CanSkip
}

// freeSpans returns free work time from now over the horizon (soft meetings
// still counted as free) and the soft meeting spans clipped to work windows.
func (c *calendar) freeSpans(now time.Time, events []EventOccurrence) ([]span, []softSpan) {
	cursor := c.cursor(now)
	var hard []span
	var soft []softSpan
	for _, event := range events {
		if c.isSoft(event) {
			soft = append(soft, softSpan{span: c.buffered(event), title: event.Title})
			continue
		}
		hard = append(hard, c.buffered(event))
	}
	hard = mergeSpans(hard)
	var free []span
	var softOut []softSpan
	for day := 0; day < scheduleHorizonDays; day++ {
		anchor := cursor.AddDate(0, 0, day)
		for _, window := range c.windows(anchor) {
			ws, we := window.start, window.end
			if cursor.After(ws) {
				ws = cursor
			}
			if !we.After(ws) {
				continue
			}
			free = append(free, complement(ws, we, clipSpans(hard, ws, we))...)
			for _, s := range soft {
				start, end, ok := clipInterval(s.start, s.end, ws, we)
				if ok {
					softOut = append(softOut, softSpan{span: span{start: start, end: end}, title: s.title})
				}
			}
		}
	}
	return free, softOut
}

func clipSpans(in []span, from, to time.Time) []span {
	var out []span
	for _, s := range in {
		start, end, ok := clipInterval(s.start, s.end, from, to)
		if ok {
			out = append(out, span{start: start, end: end})
		}
	}
	return out
}

// gridHours returns the hour range to draw: two hours before work start up
// to work end, widened by custom-hour overrides inside the range.
func (c *calendar) gridHours(from, to time.Time) (int, int) {
	start := c.settings.WorkStartMin/60 - 2
	if start < 0 {
		start = 0
	}
	end := (c.settings.WorkEndMin + 59) / 60
	if end > 24 {
		end = 24
	}
	for _, o := range c.overrides {
		day := o.Day.Midnight(c.loc)
		if day.Before(c.midnight(from)) || !day.Before(to) {
			continue
		}
		if s, e, ok := o.Window(); ok {
			if s/60 < start {
				start = s / 60
			}
			if (e+59)/60 > end {
				end = (e + 59) / 60
			}
		}
	}
	if end <= start {
		end = start + 1
	}
	return start, end
}

func (c *calendar) gridBounds(day time.Time) (time.Time, time.Time) {
	start, end := c.gridHours(c.midnight(day), c.nextDay(day))
	return c.at(day, start*60), c.at(day, end*60)
}

// busyInRange lists meetings (per day, clipped to grid hours) and breaks.
// Visual bounds stay the booked slot; Active* is the packed window.
func (c *calendar) busyInRange(events []EventOccurrence, from, to time.Time, names map[uuid.UUID]string) []ScheduleBusy {
	out := []ScheduleBusy{}
	for _, event := range events {
		if !event.EndsAt.After(from) || !event.StartsAt.Before(to) {
			continue
		}
		activeStart, activeEnd := event.ActiveSpan()
		people := namedPeople(event.People, names)
		for cursor := c.midnight(event.StartsAt); cursor.Before(event.EndsAt); cursor = cursor.AddDate(0, 0, 1) {
			gs, ge := c.gridBounds(cursor)
			start, end, ok := clipInterval(event.StartsAt, event.EndsAt, gs, ge)
			if !ok {
				continue
			}
			start, end, ok = clipInterval(start, end, from, to)
			if !ok {
				continue
			}
			busy := ScheduleBusy{
				StartsAt:   start.UTC(),
				EndsAt:     end.UTC(),
				Title:      event.Title,
				Soft:       c.isSoft(event),
				OriginalOn: event.OriginalOn,
				People:     people,
				CanSkip:    event.CanSkip,
			}
			if as, ae, ok := clipInterval(activeStart, activeEnd, start, end); ok {
				as, ae = as.UTC(), ae.UTC()
				busy.ActiveStartsAt = &as
				busy.ActiveEndsAt = &ae
			}
			if event.SeriesID != uuid.Nil {
				id := event.SeriesID
				busy.SeriesID = &id
			}
			out = append(out, busy)
		}
	}
	for cursor := c.midnight(from); cursor.Before(to); cursor = cursor.AddDate(0, 0, 1) {
		b, ok := c.breakSpan(cursor)
		if !ok {
			continue
		}
		start, end, ok := clipInterval(b.start, b.end, from, to)
		if !ok {
			continue
		}
		out = append(out, ScheduleBusy{StartsAt: start.UTC(), EndsAt: end.UTC(), Title: breakTitle})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartsAt.Equal(out[j].StartsAt) {
			return out[i].Title < out[j].Title
		}
		return out[i].StartsAt.Before(out[j].StartsAt)
	})
	return out
}

// roundUp moves t forward to the next grid tick (minutes since local midnight).
func (c *calendar) roundUp(t time.Time) time.Time {
	grid := c.settings.GridMin
	if grid <= 1 {
		return t.Truncate(time.Minute).Add(subMinute(t))
	}
	local := t.In(c.loc)
	mins := local.Hour()*60 + local.Minute()
	if mins%grid != 0 || local.Second() > 0 || local.Nanosecond() > 0 {
		mins = (mins/grid + 1) * grid
	}
	return c.midnight(local).Add(time.Duration(mins) * time.Minute)
}

func subMinute(t time.Time) time.Duration {
	if t.Second() > 0 || t.Nanosecond() > 0 {
		return time.Minute
	}
	return 0
}

// floorGrid rounds a duration down to the grid; ceilGrid rounds it up.
func (c *calendar) floorGrid(d time.Duration) time.Duration {
	step := time.Duration(c.settings.GridMin) * time.Minute
	if step <= 0 {
		return d
	}
	return d / step * step
}

func (c *calendar) ceilGrid(d time.Duration) time.Duration {
	step := time.Duration(c.settings.GridMin) * time.Minute
	if step <= 0 || d%step == 0 {
		return d
	}
	return (d/step + 1) * step
}

// workdaysBetween counts workdays strictly after a's day up to and including
// b's day; negative when b precedes a.
func (c *calendar) workdaysBetween(a, b time.Time) int {
	from, to := c.midnight(a), c.midnight(b)
	sign := 1
	if to.Before(from) {
		from, to = to, from
		sign = -1
	}
	count := 0
	for day := from.AddDate(0, 0, 1); !day.After(to); day = day.AddDate(0, 0, 1) {
		if c.isWorkday(day) {
			count++
		}
	}
	return sign * count
}

// backWorkdays moves t back by n workdays, keeping the clock.
func (c *calendar) backWorkdays(t time.Time, n int) time.Time {
	out := t.In(c.loc)
	for n > 0 {
		out = out.AddDate(0, 0, -1)
		if c.isWorkday(out) {
			n--
		}
	}
	return out
}

// workdaysFrom lists workday midnights from a's day through b's day.
func (c *calendar) workdaysFrom(a, b time.Time) []time.Time {
	var out []time.Time
	for day := c.midnight(a); !day.After(c.midnight(b)); day = day.AddDate(0, 0, 1) {
		if c.isWorkday(day) {
			out = append(out, day)
		}
	}
	return out
}

func (c *calendar) nextWorkday(after time.Time) time.Time {
	day := c.nextDay(after)
	for i := 0; i < scheduleHorizonDays; i++ {
		if c.isWorkday(day) {
			return day
		}
		day = day.AddDate(0, 0, 1)
	}
	return day
}
