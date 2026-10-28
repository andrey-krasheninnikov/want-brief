package domain

import (
	"math"
	"strconv"
	"time"
)

const (
	lightTaskSeconds = 3600
	reasonPinned     = "pinned"
	reasonPinnedAt   = "pinned_at"
	reasonActive     = "active"
	reasonInProgress = "in_progress"
	reasonNoSlack    = "no_slack"
	reasonDueSoon    = "due_soon"
	reasonSameProj   = "same_project"
	reasonSticky     = "sticky"
	reasonGolden     = "golden_hour"
	reasonLight      = "light_task"
	reasonSoftBusy   = "soft_busy"
	reasonPersonAway = "person_away"
	reasonPersonBusy = "person_busy"
	reasonEstimate   = "estimate"
	reasonPushedBy   = "pushed_by"
)

// ranked is an item with everything the order and the packer need
// precomputed: effective due, buffered remaining, slack bucket.
type ranked struct {
	item         Item
	due          time.Time
	effectiveDue time.Time
	factor       float64
	remaining    int64
	slack        int64
	bucket       int
	active       bool
	inProgress   bool
	light        bool
	project      string
	occupancy    Occupancy
	reasons      []string
}

func (p *planner) estimateFactor(item Item) float64 {
	if !p.settings.EstimateBuffer {
		return 1
	}
	k, ok := p.in.Factors[laneKey(item.ProjectID)]
	if !ok || k < 1 {
		return 1
	}
	if max := p.settings.EstimateMaxK; max >= 1 && k > max {
		k = max
	}
	return math.Round(k*100) / 100
}

func (p *planner) rank(item Item, due time.Time) *ranked {
	factor := p.estimateFactor(item)
	remaining := int64(float64(item.PlannedSeconds)*factor) - item.TrackedSeconds
	if remaining < 0 {
		remaining = 0
	}
	r := &ranked{
		item:       item,
		due:        due.In(p.loc),
		factor:     factor,
		remaining:  remaining,
		active:     p.active[item.ID],
		inProgress: item.Status == StatusInProgress || item.TrackedSeconds > 0,
		project:    laneKey(item.ProjectID),
		occupancy:  item.EffectiveOccupancy(),
	}
	r.effectiveDue = p.effectiveDue(r)
	r.slack = p.cal.workSeconds(p.now, r.effectiveDue) - remaining
	if r.effectiveDue.Before(p.now) {
		r.slack = int64(r.effectiveDue.Sub(p.now)/time.Second) - remaining
	}
	r.bucket = p.slackBucket(r.slack)
	r.light = remaining <= lightTaskSeconds || item.Quadrant() == QuadrantSchedule || item.Quadrant() == QuadrantDelegate
	r.reasons = p.staticReasons(r)
	return r
}

// effectiveDue pulls the due date earlier by the target lead (#26), by one
// more workday for urgent tasks (#6) and by stress (#5): stress 5 with a 6h
// shift lands a full day earlier.
func (p *planner) effectiveDue(r *ranked) time.Time {
	due := r.due
	lead := p.settings.TargetLeadWorkdays
	if r.item.Urgent {
		lead++
	}
	if lead > 0 {
		due = p.cal.backWorkdays(due, lead)
	}
	if r.item.Stress != nil && *r.item.Stress > 1 && p.settings.StressShiftHours > 0 {
		hours := (*r.item.Stress - 1) * p.settings.StressShiftHours
		due = due.Add(-time.Duration(hours) * time.Hour)
	}
	return due
}

// slackBucket groups slack by workdays of spare capacity: 0 = already late,
// 1 = less than a day, 2 = one day, 3 = 2–3 days, 4 = 4–7 days, 5 = later.
func (p *planner) slackBucket(slack int64) int {
	if slack < 0 {
		return 0
	}
	days := int(slack / p.cal.daySeconds())
	switch {
	case days <= 0:
		return 1
	case days == 1:
		return 2
	case days <= 3:
		return 3
	case days <= 7:
		return 4
	default:
		return 5
	}
}

func (p *planner) staticReasons(r *ranked) []string {
	var out []string
	if r.active {
		out = append(out, reasonActive)
	}
	if r.item.Pinned {
		out = append(out, reasonPinned)
	}
	if r.inProgress && !r.active {
		out = append(out, reasonInProgress)
	}
	switch r.bucket {
	case 0:
		out = append(out, reasonNoSlack)
	case 1, 2:
		out = append(out, reasonDueSoon)
	}
	if r.factor > 1.001 {
		out = append(out, reasonEstimate+"_x"+strconv.FormatFloat(r.factor, 'f', -1, 64))
	}
	return out
}

// less is the packing order: what is happening now, then pinned, then
// started work, then by slack bucket, then important before urgent (which
// together spell the Eisenhower order), then effective due, stress and age.
func (p *planner) less(a, b *ranked) bool {
	if a.active != b.active {
		return a.active
	}
	if a.item.Pinned != b.item.Pinned {
		return a.item.Pinned
	}
	if a.inProgress != b.inProgress {
		return a.inProgress
	}
	if a.bucket != b.bucket {
		return a.bucket < b.bucket
	}
	if a.item.Important != b.item.Important {
		return a.item.Important
	}
	if a.item.Urgent != b.item.Urgent {
		return a.item.Urgent
	}
	if !a.effectiveDue.Equal(b.effectiveDue) {
		return a.effectiveDue.Before(b.effectiveDue)
	}
	as, bs := stressOf(a.item), stressOf(b.item)
	if as != bs {
		return as > bs
	}
	if p.settings.OldestFirst {
		return a.item.CreatedAt.Before(b.item.CreatedAt)
	}
	return a.item.CreatedAt.After(b.item.CreatedAt)
}

// lessItems orders items that were never ranked (unplanned list).
func (p *planner) lessItems(a, b Item) bool {
	if a.Pinned != b.Pinned {
		return a.Pinned
	}
	qa, qb := quadrantRank(a.Quadrant()), quadrantRank(b.Quadrant())
	if qa != qb {
		return qa < qb
	}
	ad, bd := packingDue(a, p.kind), packingDue(b, p.kind)
	if ad == nil && bd != nil {
		return false
	}
	if ad != nil && bd == nil {
		return true
	}
	if ad != nil && bd != nil && !ad.Equal(*bd) {
		return ad.Before(*bd)
	}
	as, bs := stressOf(a), stressOf(b)
	if as != bs {
		return as > bs
	}
	if p.settings.OldestFirst {
		return a.CreatedAt.Before(b.CreatedAt)
	}
	return a.CreatedAt.After(b.CreatedAt)
}

// sameClass reports whether two ranked items tie on every key before the
// Eisenhower quadrant, so swapping them costs nothing deadline-wise (#11).
func sameClass(a, b *ranked) bool {
	return a.active == b.active && a.item.Pinned == b.item.Pinned && a.inProgress == b.inProgress && a.bucket == b.bucket
}

// pickNext chooses the next item to pack: the head of the queue unless a
// same-class item continues the project of the last placed block.
func pickNext(queue []*ranked, lastProject string) int {
	if len(queue) == 0 {
		return -1
	}
	head := queue[0]
	if lastProject == "" || head.project == lastProject {
		return 0
	}
	for i, r := range queue[1:] {
		if !sameClass(head, r) {
			break
		}
		if r.project == lastProject {
			return i + 1
		}
	}
	return 0
}

func stressOf(item Item) int {
	if item.Stress == nil {
		return -1
	}
	return *item.Stress
}

func quadrantRank(q Quadrant) int {
	switch q {
	case QuadrantDo:
		return 0
	case QuadrantSchedule:
		return 1
	case QuadrantDelegate:
		return 2
	default:
		return 3
	}
}
