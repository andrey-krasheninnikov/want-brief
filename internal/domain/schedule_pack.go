package domain

import (
	"time"

	"github.com/google/uuid"
)

// dayBudget tracks how much focus work and how many distinct tasks landed on
// a calendar day (#17, #25).
type dayBudget struct {
	focus int64
	items map[uuid.UUID]bool
}

// packState is the mutable free-time model during one packing run. Track 0
// is the single active track; tracks 1..N are waiting tracks (#20). free is
// the intersection of all tracks (what a solo task needs).
type packState struct {
	p       *planner
	free    []span
	tracks  [][]span
	soft    []softSpan
	budgets map[Ymd]*dayBudget
	left    map[uuid.UUID]int64
	blocks  []ScheduleBlock
	last    string
	zone    *span
}

type slice struct {
	start     time.Time
	end       time.Time
	track     int
	continues bool
	reasons   []string
}

func (p *planner) newPackState(free []span, soft []softSpan) *packState {
	s := &packState{
		p:       p,
		free:    cloneSpans(free),
		soft:    soft,
		budgets: map[Ymd]*dayBudget{},
		left:    map[uuid.UUID]int64{},
	}
	s.tracks = make([][]span, 1+p.settings.WaitingTracks)
	for i := range s.tracks {
		s.tracks[i] = cloneSpans(free)
	}
	for _, r := range p.queue {
		s.left[r.item.ID] = r.remaining
	}
	s.zone = p.energyZone()
	return s
}

func (s *packState) clone() *packState {
	out := &packState{
		p:       s.p,
		free:    cloneSpans(s.free),
		soft:    s.soft,
		budgets: make(map[Ymd]*dayBudget, len(s.budgets)),
		left:    make(map[uuid.UUID]int64, len(s.left)),
		blocks:  append([]ScheduleBlock{}, s.blocks...),
		last:    s.last,
		zone:    s.zone,
	}
	out.tracks = make([][]span, len(s.tracks))
	for i := range s.tracks {
		out.tracks[i] = cloneSpans(s.tracks[i])
	}
	for day, b := range s.budgets {
		items := make(map[uuid.UUID]bool, len(b.items))
		for id := range b.items {
			items[id] = true
		}
		out.budgets[day] = &dayBudget{focus: b.focus, items: items}
	}
	for id, left := range s.left {
		out.left[id] = left
	}
	return out
}

// packWork runs reservations, the greedy pass and, when a snapshot is
// available, a second pass that keeps blocks close to where they were (#27).
func (p *planner) packWork(free []span, soft []softSpan) []ScheduleBlock {
	greedy := p.packWith(free, soft, nil)
	if len(p.in.Previous) == 0 || p.settings.StabilityThresholdMin <= 0 {
		return greedy.blocks
	}
	sticky := p.stickyReservations(greedy.blocks)
	if len(sticky) == 0 {
		return greedy.blocks
	}
	return p.packWith(free, soft, sticky).blocks
}

func (p *planner) packWith(free []span, soft []softSpan, sticky []reservation) *packState {
	s := p.newPackState(free, soft)
	s.reserveActive()
	for _, res := range sticky {
		s.reserve(res)
	}
	s.placePinnedAt()
	s.greedy()
	return s
}

func (s *packState) greedy() {
	var remaining []*ranked
	for _, r := range s.p.queue {
		if s.left[r.item.ID] > 0 {
			remaining = append(remaining, r)
		}
	}
	// Reservations are not "the previous block" in packing order.
	s.last = ""
	for len(remaining) > 0 {
		i := pickNext(remaining, s.last)
		r := remaining[i]
		remaining = append(remaining[:i], remaining[i+1:]...)
		var extra []string
		if i > 0 {
			extra = []string{reasonSameProj}
		}
		s.placeWithRetry(r, time.Time{}, extra)
	}
}

// placeWithRetry packs against hard busy first; if the task ends up late or
// unplaced and soft meetings exist, it retries over soft-busy time and keeps
// whichever result is better (#18).
func (s *packState) placeWithRetry(r *ranked, from time.Time, extra []string) {
	trial := s.clone()
	trial.placeItem(r, from, false, extra)
	if s.p.settings.SoftBusy && len(s.soft) > 0 && (trial.left[r.item.ID] > 0 || trial.lateFor(r.item.ID) > 0) {
		alt := s.clone()
		alt.placeItem(r, from, true, extra)
		if alt.betterFor(trial, r.item.ID) {
			trial = alt
		}
	}
	*s = *trial
}

func (s *packState) lateFor(id uuid.UUID) int64 {
	var late int64
	for _, block := range s.blocks {
		if block.ItemID == id {
			late += lateSeconds(block.StartsAt, block.EndsAt, block.DueAt)
		}
	}
	return late
}

func (s *packState) betterFor(other *packState, id uuid.UUID) bool {
	a, b := s.lateFor(id), other.lateFor(id)
	if a != b {
		return a < b
	}
	return s.left[id] < other.left[id]
}

// placeItem carves slices for one item until nothing is left or no slot fits.
func (s *packState) placeItem(r *ranked, from time.Time, includeSoft bool, extra []string) {
	cursor := from
	for s.left[r.item.ID] > 0 {
		view, track := s.viewFor(r, cursor, includeSoft)
		sl, ok := s.pickSlice(r, view, cursor, s.left[r.item.ID])
		if !ok {
			return
		}
		sl.track = track
		sl.reasons = append(sl.reasons, extra...)
		s.commit(r, sl)
		cursor = sl.end
		if sl.continues && s.left[r.item.ID] > 0 {
			cursor = s.insertBreak(sl.end)
		}
	}
}

// viewFor returns the free spans this item may use from the cursor on, and
// the track index it would occupy.
func (s *packState) viewFor(r *ranked, from time.Time, includeSoft bool) ([]span, int) {
	var view []span
	track := 0
	switch r.occupancy {
	case OccupancyParallel:
		view = s.tracks[0]
	case OccupancyWaiting:
		if len(s.tracks) > 1 {
			track = s.earliestWaitingTrack(from)
			view = s.tracks[track]
		} else {
			view = s.tracks[0]
		}
	default:
		view = s.free
	}
	if r.occupancy != OccupancySolo {
		view = subtractAll(view, s.ownSpans(r.item.ID))
	}
	if !includeSoft {
		for _, soft := range s.soft {
			view = subtractSpan(view, soft.start, soft.end)
		}
	}
	if len(r.item.PersonIDs) > 0 {
		view = subtractAll(view, s.p.personBusySpans(r))
	}
	return view, track
}

func (s *packState) ownSpans(id uuid.UUID) []span {
	var out []span
	for _, block := range s.blocks {
		if block.ItemID == id {
			out = append(out, span{start: block.StartsAt, end: block.EndsAt})
		}
	}
	return out
}

func (s *packState) earliestWaitingTrack(from time.Time) int {
	best := 1
	var bestStart time.Time
	found := false
	for i := 1; i < len(s.tracks); i++ {
		start, ok := firstStartFrom(s.tracks[i], from)
		if !ok {
			continue
		}
		if !found || start.Before(bestStart) {
			best, bestStart, found = i, start, true
		}
	}
	return best
}

func firstStartFrom(slots []span, from time.Time) (time.Time, bool) {
	for _, slot := range slots {
		if !slot.end.After(slot.start) || !slot.end.After(from) {
			continue
		}
		if slot.start.Before(from) {
			return from, true
		}
		return slot.start, true
	}
	return time.Time{}, false
}

func firstSlotEndingAfter(slots []span, from time.Time) (span, bool) {
	for _, slot := range slots {
		if slot.end.After(from) && slot.end.After(slot.start) {
			return slot, true
		}
	}
	return span{}, false
}

// pickSlice finds the next slice for the item: grid-aligned start (#12),
// minimum gap (#8), maximum length (#9), whole-fit preference (#10), day
// budgets (#17 #25), low-energy mornings (#23) and golden hours (#24).
func (s *packState) pickSlice(r *ranked, view []span, from time.Time, left int64) (slice, bool) {
	c := s.p.cal
	set := s.p.settings
	minSlice := time.Duration(set.MinSliceMin) * time.Minute
	maxSlice := time.Duration(set.MaxSliceMin) * time.Minute
	grid := time.Duration(set.GridMin) * time.Minute
	need := time.Duration(left) * time.Second
	cursor := from
	for guard := 0; guard < scheduleHorizonDays*8; guard++ {
		slot, ok := firstSlotEndingAfter(view, cursor)
		if !ok {
			return slice{}, false
		}
		start := slot.start
		if cursor.After(start) {
			start = cursor
		}
		start = c.roundUp(start)
		if !start.Before(slot.end) {
			cursor = slot.end
			continue
		}
		if !s.dayAllows(c.ymd(start), r) {
			cursor = c.nextDay(start)
			continue
		}
		start = s.pastZone(r, start)
		if !start.Before(slot.end) {
			cursor = slot.end
			continue
		}
		avail := slot.end.Sub(start)
		if minSlice > 0 && avail < minSlice && need >= minSlice {
			cursor = slot.end
			continue
		}
		var reasons []string
		if alt, ok := s.betterGap(r, view, start, avail, need); ok {
			start, avail, reasons = alt.start, alt.avail, alt.reasons
		}
		day := c.ymd(start)
		take := avail
		if take > need {
			take = need
		}
		if maxSlice > 0 && take > maxSlice {
			take = maxSlice
		}
		take = s.capBudget(day, r, take, minSlice)
		if take >= need {
			take = c.ceilGrid(need)
			if take > avail {
				take = c.floorGrid(avail)
			}
		} else {
			take = c.floorGrid(take)
		}
		if take <= 0 {
			cursor = start.Add(avail)
			continue
		}
		end := start.Add(take)
		return slice{
			start:     start,
			end:       end,
			continues: take < need && avail-take >= grid,
			reasons:   reasons,
		}, true
	}
	return slice{}, false
}

type gapChoice struct {
	start   time.Time
	avail   time.Duration
	reasons []string
}

// betterGap looks at other gaps on the same day: a gap that holds the whole
// remaining work beats splitting (#10); for Do tasks a golden-hour gap beats
// an ordinary one when it does not make the task late (#24).
func (s *packState) betterGap(r *ranked, view []span, start time.Time, avail, need time.Duration) (gapChoice, bool) {
	c := s.p.cal
	set := s.p.settings
	maxSlice := time.Duration(set.MaxSliceMin) * time.Minute
	minSlice := time.Duration(set.MinSliceMin) * time.Minute
	dayEnd := c.nextDay(start)
	want := need
	if maxSlice > 0 && want > maxSlice {
		want = maxSlice
	}
	var candidates []gapChoice
	for _, slot := range view {
		if !slot.end.After(start) || !slot.start.Before(dayEnd) {
			continue
		}
		gs := slot.start
		if start.After(gs) {
			gs = start
		}
		gs = s.pastZone(r, c.roundUp(gs))
		if !gs.Before(slot.end) {
			continue
		}
		candidates = append(candidates, gapChoice{start: gs, avail: slot.end.Sub(gs)})
	}
	if len(candidates) == 0 {
		return gapChoice{}, false
	}
	current := candidates[0]
	// Whole-fit: the current gap splits the task but a later gap today holds it.
	if (maxSlice == 0 || need <= maxSlice) && avail < need {
		for _, cand := range candidates[1:] {
			if cand.avail >= need {
				current = cand
				break
			}
		}
	}
	if s.p.golden == nil || r.item.Quadrant() != QuadrantDo {
		if current.start.Equal(start) {
			return gapChoice{}, false
		}
		return current, true
	}
	best := current
	bestWeight := s.p.golden.weight(current.start)
	for _, cand := range hourStarts(candidates, want, c.loc) {
		if cand.avail < want && cand.avail < current.avail {
			continue
		}
		if minSlice > 0 && cand.avail < minSlice && need >= minSlice {
			continue
		}
		if cand.start.Add(minDuration(cand.avail, want)).After(r.due) {
			continue
		}
		w := s.p.golden.weight(cand.start)
		if w > bestWeight+goldenMargin {
			best, bestWeight = cand, w
		}
	}
	if best.start.Equal(start) {
		return gapChoice{}, false
	}
	if !best.start.Equal(current.start) {
		best.reasons = append(best.reasons, reasonGolden)
	}
	return best, true
}

// hourStarts expands gaps into candidate starts at every full hour inside
// them (plus the gap start itself), so golden hours can pick a time inside a
// long free stretch rather than only between meetings.
func hourStarts(gaps []gapChoice, want time.Duration, loc *time.Location) []gapChoice {
	var out []gapChoice
	for _, gap := range gaps {
		out = append(out, gap)
		end := gap.start.Add(gap.avail)
		local := gap.start.In(loc)
		hour := time.Date(local.Year(), local.Month(), local.Day(), local.Hour()+1, 0, 0, 0, loc)
		for ; hour.Add(want).Before(end) || hour.Add(want).Equal(end); hour = hour.Add(time.Hour) {
			out = append(out, gapChoice{start: hour, avail: end.Sub(hour)})
		}
	}
	return out
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// dayAllows checks the day budgets: the task cap only matters for a task
// not yet on that day (#25); the focus cap applies to everyone (#17).
func (s *packState) dayAllows(day Ymd, r *ranked) bool {
	b := s.budgets[day]
	if b == nil {
		return true
	}
	set := s.p.settings
	if set.MaxTasksPerDay > 0 && !b.items[r.item.ID] && len(b.items) >= set.MaxTasksPerDay {
		return false
	}
	if r.occupancy != OccupancyWaiting && set.DailyFocusMin > 0 && b.focus >= int64(set.DailyFocusMin)*60 {
		return false
	}
	return true
}

// capBudget shrinks a slice to the remaining daily focus budget, never
// below the minimum slice so it does not create slivers.
func (s *packState) capBudget(day Ymd, r *ranked, take, minSlice time.Duration) time.Duration {
	set := s.p.settings
	if r.occupancy == OccupancyWaiting || set.DailyFocusMin <= 0 {
		return take
	}
	var used int64
	if b := s.budgets[day]; b != nil {
		used = b.focus
	}
	budgetLeft := time.Duration(int64(set.DailyFocusMin)*60-used) * time.Second
	if budgetLeft <= 0 {
		return take
	}
	if take > budgetLeft {
		// A small overshoot beats leaving a sliver for another day.
		if take-budgetLeft < minSlice {
			return take
		}
		if budgetLeft < minSlice {
			return minSlice
		}
		return budgetLeft
	}
	return take
}

func (s *packState) commit(r *ranked, sl slice) {
	lane := sl.track
	block := newBlock(r, sl.start, sl.end, lane, r.occupancy, s.p.itemPeople(r.item))
	block.Reasons = append(block.Reasons, sl.reasons...)
	block.Reasons = appendUnique(block.Reasons, s.p.personBusyReasons(r, sl.start)...)
	for _, soft := range s.soft {
		if soft.start.Before(sl.end) && soft.end.After(sl.start) {
			block.Reasons = append(block.Reasons, reasonSoftBusy+":"+soft.title)
		}
	}
	if s.zone != nil && r.light && sl.start.Before(s.zone.end) && sl.end.After(s.zone.start) {
		block.Reasons = append(block.Reasons, reasonLight)
	}
	s.occupy(r.occupancy, sl.track, sl.start, sl.end)
	dur := spanSeconds(sl.start, sl.end)
	day := s.p.cal.ymd(sl.start)
	b := s.budgets[day]
	if b == nil {
		b = &dayBudget{items: map[uuid.UUID]bool{}}
		s.budgets[day] = b
	}
	b.items[r.item.ID] = true
	if r.occupancy != OccupancyWaiting {
		b.focus += dur
	}
	s.left[r.item.ID] -= dur
	if s.left[r.item.ID] < 0 {
		s.left[r.item.ID] = 0
	}
	s.blocks = append(s.blocks, block)
	s.last = r.project
}

// occupy removes a span from the sets an occupancy consumes.
func (s *packState) occupy(occupancy Occupancy, track int, start, end time.Time) {
	switch occupancy {
	case OccupancyParallel:
		s.tracks[0] = subtractSpan(s.tracks[0], start, end)
		s.free = subtractSpan(s.free, start, end)
	case OccupancyWaiting:
		if track < 0 || track >= len(s.tracks) {
			track = 0
		}
		s.tracks[track] = subtractSpan(s.tracks[track], start, end)
		s.free = subtractSpan(s.free, start, end)
	default:
		s.free = subtractSpan(s.free, start, end)
		for i := range s.tracks {
			s.tracks[i] = subtractSpan(s.tracks[i], start, end)
		}
	}
}

// insertBreak blocks a short pause after a long slice on every track (#9)
// and returns where the next slice may start.
func (s *packState) insertBreak(end time.Time) time.Time {
	pause := time.Duration(s.p.settings.SliceBreakMin) * time.Minute
	if pause <= 0 {
		return end
	}
	until := end.Add(pause)
	s.occupy(OccupancySolo, 0, end, until)
	return until
}
