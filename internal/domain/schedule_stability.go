package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// reservation is a block decided before the greedy pass: the task being
// tracked right now (#19), a task pinned to an instant (#7) or a block kept
// from the previous layout (#27).
type reservation struct {
	item   *ranked
	start  time.Time
	end    time.Time
	reason string
}

// reserveActive pins the task with an open interval to [now, now+slice]
// even outside work windows: it is happening regardless of the calendar.
func (s *packState) reserveActive() {
	maxSlice := time.Duration(s.p.settings.MaxSliceMin) * time.Minute
	for _, r := range s.p.queue {
		if !r.active || s.left[r.item.ID] <= 0 {
			continue
		}
		take := time.Duration(s.left[r.item.ID]) * time.Second
		if maxSlice > 0 && take > maxSlice {
			take = maxSlice
		}
		take = s.p.cal.ceilGrid(take)
		start := s.p.now
		end := start.Add(take)
		s.commit(r, slice{start: start, end: end, reasons: []string{reasonActive}})
	}
}

// placePinnedAt puts tasks with a pin instant at the first free slot on or
// after it, before anything else competes for that time.
func (s *packState) placePinnedAt() {
	var pinned []*ranked
	for _, r := range s.p.queue {
		if r.item.PinnedAt == nil || s.left[r.item.ID] <= 0 {
			continue
		}
		if r.item.PinnedAt.Before(s.p.now) {
			continue
		}
		pinned = append(pinned, r)
	}
	sort.SliceStable(pinned, func(i, j int) bool {
		return pinned[i].item.PinnedAt.Before(*pinned[j].item.PinnedAt)
	})
	for _, r := range pinned {
		s.placeItem(r, r.item.PinnedAt.In(s.p.loc), false, []string{reasonPinnedAt})
	}
}

// reserve places a previous-layout block if its slot is still free.
func (s *packState) reserve(res reservation) {
	r := res.item
	left := s.left[r.item.ID]
	if left <= 0 {
		return
	}
	end := res.end
	if want := res.start.Add(time.Duration(left) * time.Second); want.Before(end) {
		end = want
	}
	view, track := s.viewFor(r, res.start, false)
	if !containsSpan(view, res.start, end) {
		return
	}
	if !s.dayAllows(s.p.cal.ymd(res.start), r) {
		return
	}
	s.commit(r, slice{start: res.start, end: end, track: track, reasons: []string{res.reason}})
}

// stickyReservations compares the fresh greedy layout with the snapshot and
// keeps future, on-time blocks whose greedy alternative is not clearly better.
func (p *planner) stickyReservations(greedy []ScheduleBlock) []reservation {
	threshold := time.Duration(p.settings.StabilityThresholdMin) * time.Minute
	prevByItem := map[uuid.UUID][]ScheduleBlock{}
	for _, block := range p.in.Previous {
		if block.Kind == BlockKindPing || !block.StartsAt.After(p.now) {
			continue
		}
		prevByItem[block.ItemID] = append(prevByItem[block.ItemID], block)
	}
	greedyFirst := map[uuid.UUID]time.Time{}
	for _, block := range greedy {
		if at, ok := greedyFirst[block.ItemID]; !ok || block.StartsAt.Before(at) {
			greedyFirst[block.ItemID] = block.StartsAt
		}
	}
	var out []reservation
	for _, r := range p.queue {
		prev := prevByItem[r.item.ID]
		if len(prev) == 0 || r.active {
			continue
		}
		sort.Slice(prev, func(i, j int) bool { return prev[i].StartsAt.Before(prev[j].StartsAt) })
		fresh, placed := greedyFirst[r.item.ID]
		if !placed {
			continue
		}
		first := prev[0]
		if first.EndsAt.After(r.due) {
			continue
		}
		if gain := first.StartsAt.Sub(fresh); gain >= threshold {
			continue
		}
		for _, block := range prev {
			if block.EndsAt.After(r.due) {
				break
			}
			out = append(out, reservation{item: r, start: block.StartsAt.In(p.loc), end: block.EndsAt.In(p.loc), reason: reasonSticky})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].start.Before(out[j].start) })
	return out
}
