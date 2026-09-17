package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	defaultPingMinutes = 20
	pingSearchDays     = 30
	pingRetries        = 8
)

// packFollowup turns review/QA/awaiting tasks into short pings inside the
// check windows (#21): one per workday until due, one per day after it, and
// always at least one. Pings wait for people to be available (#22).
func (p *planner) packFollowup(free []span) []ScheduleBlock {
	pingDur := time.Duration(p.settings.FollowupPingMin) * time.Minute
	if pingDur <= 0 {
		pingDur = defaultPingMinutes * time.Minute
	}
	windows := p.settings.CheckWindowMinutes()
	if len(windows) == 0 {
		windows = []int{p.settings.WorkStartMin}
	}
	state := cloneSpans(free)
	var blocks []ScheduleBlock
	for _, r := range p.queue {
		placed := 0
		var carry []string
		for _, day := range p.pingDays(r) {
			if away := p.awayOn(r, day); len(away) > 0 {
				carry = appendUnique(carry, away...)
				continue
			}
			block, ok := p.ping(r, day, windows, &state, pingDur, carry)
			if !ok {
				continue
			}
			carry = nil
			blocks = append(blocks, block)
			placed++
		}
		if placed > 0 {
			continue
		}
		// Nothing landed (people away, day full): find the first day that works.
		day := p.cal.nextWorkday(p.now)
		for i := 0; i < pingSearchDays; i++ {
			if away := p.awayOn(r, day); len(away) > 0 {
				carry = appendUnique(carry, away...)
				day = p.cal.nextWorkday(day)
				continue
			}
			if block, ok := p.ping(r, day, windows, &state, pingDur, carry); ok {
				blocks = append(blocks, block)
				break
			}
			day = p.cal.nextWorkday(day)
		}
	}
	return blocks
}

// pingDays lists the workdays that should get a ping: today through the due
// day, or today through the visible range end once the task is late.
func (p *planner) pingDays(r *ranked) []time.Time {
	today := p.cal.midnight(p.now)
	if r.due.Before(p.now) {
		end := p.to
		if end.Before(p.now) {
			end = p.now
		}
		return p.cal.workdaysFrom(today, end)
	}
	return p.cal.workdaysFrom(today, r.due)
}

// ping places one check for the item on the given day at the first check
// window that is still ahead, sliding past meetings of involved people.
func (p *planner) ping(r *ranked, day time.Time, windows []int, state *[]span, dur time.Duration, carry []string) (ScheduleBlock, bool) {
	at, ok := p.firstWindow(r, day, windows)
	if !ok {
		return ScheduleBlock{}, false
	}
	dayEnd := p.cal.nextDay(day)
	for try := 0; try < pingRetries; try++ {
		start, found := firstStartFrom(*state, at)
		if !found || !start.Before(dayEnd) {
			return ScheduleBlock{}, false
		}
		start = p.cal.roundUp(start)
		end := start.Add(dur)
		if !containsSpan(*state, start, end) {
			at = start.Add(time.Minute)
			continue
		}
		if until, name, busy := p.personBusy(r, start, end); busy {
			carry = appendUnique(carry, reasonPersonBusy+":"+name)
			at = until
			continue
		}
		block := newBlock(r, start, end, 0, OccupancySolo, p.itemPeople(r.item))
		block.Kind = BlockKindPing
		block.RemainingSeconds = 0
		block.Reasons = appendUnique(block.Reasons, carry...)
		*state = subtractSpan(*state, start, end)
		return block, true
	}
	return ScheduleBlock{}, false
}

// firstWindow picks the check window for a day. Today only windows still
// ahead count; if all passed and the task is due today or late, ping now.
func (p *planner) firstWindow(r *ranked, day time.Time, windows []int) (time.Time, bool) {
	isToday := p.cal.midnight(p.now).Equal(day)
	for _, minute := range windows {
		at := p.cal.at(day, minute)
		if !isToday || !at.Before(p.now) {
			return at, true
		}
	}
	if isToday && !r.due.After(p.cal.nextDay(day)) {
		return p.cal.roundUp(p.now), true
	}
	return time.Time{}, false
}

// awayOn lists person_away reasons for people absent on the day.
func (p *planner) awayOn(r *ranked, day time.Time) []string {
	var out []string
	for _, id := range r.item.PersonIDs {
		for _, absence := range p.in.Absences {
			if absence.PersonID == id && absence.Covers(day.Add(12*time.Hour), p.loc) {
				out = append(out, reasonPersonAway+":"+p.personName(id))
				break
			}
		}
	}
	return out
}

// personBusy reports whether any involved person sits in a hard meeting during
// [start, end); it returns when the buffered active window ends.
func (p *planner) personBusy(r *ranked, start, end time.Time) (time.Time, string, bool) {
	for _, hole := range p.personBusyNamed(r) {
		if !hole.start.Before(end) || !hole.end.After(start) {
			continue
		}
		return hole.end, hole.name, true
	}
	return time.Time{}, "", false
}

func (p *planner) personBusySpans(r *ranked) []span {
	holes := p.personBusyNamed(r)
	out := make([]span, 0, len(holes))
	for _, hole := range holes {
		out = append(out, hole.span)
	}
	return mergeSpans(out)
}

func (p *planner) personBusyReasons(r *ranked, start time.Time) []string {
	var out []string
	for _, hole := range p.personBusyNamed(r) {
		if p.cal.roundUp(hole.end).Equal(start) {
			out = appendUnique(out, reasonPersonBusy+":"+hole.name)
		}
	}
	return out
}

type namedSpan struct {
	span
	name string
}

func (p *planner) personBusyNamed(r *ranked) []namedSpan {
	var out []namedSpan
	for _, id := range r.item.PersonIDs {
		for _, event := range p.in.Events {
			if p.cal.isSoft(event) {
				continue
			}
			if !eventHasPerson(event, id) {
				continue
			}
			out = append(out, namedSpan{span: p.cal.buffered(event), name: p.personName(id)})
		}
	}
	return out
}

func (p *planner) itemPeople(item Item) []SchedulePerson {
	return namedIDs(item.PersonIDs, p.in.PeopleNames)
}

func eventHasPerson(event EventOccurrence, id uuid.UUID) bool {
	for _, rel := range event.People {
		if rel.ID == id {
			return true
		}
	}
	return false
}

func (p *planner) personName(id uuid.UUID) string {
	if name, ok := p.in.PeopleNames[id]; ok && name != "" {
		return name
	}
	return id.String()[:8]
}

func namedPeople(rels []PersonRel, names map[uuid.UUID]string) []SchedulePerson {
	ids := make([]uuid.UUID, 0, len(rels))
	for _, rel := range rels {
		ids = append(ids, rel.ID)
	}
	return namedIDs(ids, names)
}

func namedIDs(ids []uuid.UUID, names map[uuid.UUID]string) []SchedulePerson {
	if len(ids) == 0 {
		return nil
	}
	out := make([]SchedulePerson, 0, len(ids))
	for _, id := range ids {
		name := ""
		if names != nil {
			name = names[id]
		}
		if name == "" {
			name = id.String()[:8]
		}
		out = append(out, SchedulePerson{ID: id, Name: name})
	}
	return out
}

func appendUnique(list []string, values ...string) []string {
	for _, value := range values {
		seen := false
		for _, existing := range list {
			if existing == value {
				seen = true
				break
			}
		}
		if !seen {
			list = append(list, value)
		}
	}
	return list
}
