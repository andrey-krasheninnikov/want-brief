package domain

import "time"

// GoldenHistoryWeeks is how far back callers should load tracked intervals
// when feeding ScheduleInput.History.
const GoldenHistoryWeeks = 8

const (
	lowCheckinLevel  = 2
	energyZoneHours  = 2
	goldenMargin     = 0.1
	goldenMinSeconds = 3600
)

// energyZone returns the first hours of today's window when the latest
// check-ins report low energy or focus, so heavy tasks wait until later (#23).
// No zone when nothing light is queued: blocking mornings would only delay.
func (p *planner) energyZone() *span {
	if !p.settings.EnergyAware || p.in.Checkins == nil {
		return nil
	}
	if p.in.Checkins.Energy > lowCheckinLevel && p.in.Checkins.Focus > lowCheckinLevel {
		return nil
	}
	hasLight := false
	for _, r := range p.queue {
		if r.light {
			hasLight = true
			break
		}
	}
	if !hasLight {
		return nil
	}
	windows := p.cal.windows(p.now)
	if len(windows) == 0 {
		return nil
	}
	zone := span{start: windows[0].start, end: windows[0].start.Add(energyZoneHours * time.Hour)}
	if !p.now.Before(zone.end) {
		return nil
	}
	return &zone
}

// pastZone pushes a heavy task's start past the low-energy zone.
func (s *packState) pastZone(r *ranked, start time.Time) time.Time {
	if s.zone == nil || r.light {
		return start
	}
	if start.Before(s.zone.end) && !start.Before(s.zone.start) {
		return s.p.cal.roundUp(s.zone.end)
	}
	return start
}

// goldenHours weights hours of the day by how much tracked work historically
// landed there (#24). Weights are normalised to [0, 1].
type goldenHours struct {
	weights [24]float64
	loc     *time.Location
}

func newGoldenHours(history []TimeInterval, loc *time.Location) *goldenHours {
	g := &goldenHours{loc: loc}
	var total float64
	for _, interval := range history {
		if interval.EndedAt == nil {
			continue
		}
		cursor := interval.StartedAt.In(loc)
		end := interval.EndedAt.In(loc)
		for cursor.Before(end) {
			next := cursor.Truncate(time.Hour).Add(time.Hour)
			if next.After(end) {
				next = end
			}
			sec := next.Sub(cursor).Seconds()
			g.weights[cursor.Hour()] += sec
			total += sec
			cursor = next
		}
	}
	if total < goldenMinSeconds {
		return nil
	}
	var max float64
	for _, w := range g.weights {
		if w > max {
			max = w
		}
	}
	if max <= 0 {
		return nil
	}
	for i := range g.weights {
		g.weights[i] /= max
	}
	return g
}

func (g *goldenHours) weight(at time.Time) float64 {
	if g == nil {
		return 0
	}
	return g.weights[at.In(g.loc).Hour()]
}
