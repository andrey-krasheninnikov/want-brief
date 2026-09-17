package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	scheduleHorizonDays = 120
	breakTitle          = "Break"

	BlockKindWork = "work"
	BlockKindPing = "ping"
)

type ScheduleKind string

const (
	ScheduleWork     ScheduleKind = "work"
	ScheduleFollowup ScheduleKind = "followup"
)

// ScheduleInput is everything the packer needs. Only Items, Now, From and To
// are required; the rest defaults to "no extra knowledge".
type ScheduleInput struct {
	Kind          ScheduleKind
	Items         []Item
	Events        []EventOccurrence
	Now           time.Time
	From          time.Time
	To            time.Time
	Settings      ScheduleSettings
	Overrides     []DayOverride
	Absences      []PersonAbsence
	PeopleNames   map[uuid.UUID]string
	OpenIntervals []TimeInterval
	Factors       map[string]float64
	History       []TimeInterval
	Checkins      *LatestCheckins
	Previous      []ScheduleBlock
}

type Schedule struct {
	Lanes     []ScheduleLane   `json:"lanes"`
	Unplanned []UnplannedItem  `json:"unplanned"`
	Busy      []ScheduleBusy   `json:"busy"`
	Overflow  ScheduleOverflow `json:"overflow"`
	Capacity  ScheduleCapacity `json:"capacity"`
	Grid      ScheduleGrid     `json:"grid"`
	AtRisk    []AtRiskItem     `json:"atRisk"`
	Score     ScheduleScore    `json:"score"`
	// Horizon holds every packed block, not just the visible range; the
	// application keeps it as the stability snapshot.
	Horizon []ScheduleBlock `json:"-"`
}

type ScheduleGrid struct {
	Timezone     string `json:"timezone"`
	StartHour    int    `json:"startHour"`
	EndHour      int    `json:"endHour"`
	Workdays     []int  `json:"workdays"`
	WorkStartMin int    `json:"workStartMin"`
	WorkEndMin   int    `json:"workEndMin"`
}

type ScheduleLane struct {
	ProjectID    *uuid.UUID      `json:"projectId"`
	ProjectName  string          `json:"projectName"`
	ProjectColor string          `json:"projectColor"`
	Blocks       []ScheduleBlock `json:"blocks"`
}

type ScheduleBlock struct {
	ItemID           uuid.UUID        `json:"itemId"`
	Title            string           `json:"title"`
	ExternalKey      string           `json:"externalKey"`
	Kind             string           `json:"kind"`
	StartsAt         time.Time        `json:"startsAt"`
	EndsAt           time.Time        `json:"endsAt"`
	Late             bool             `json:"late"`
	Continued        bool             `json:"continued"`
	Continues        bool             `json:"continues"`
	Lane             int              `json:"lane"`
	Occupancy        Occupancy        `json:"occupancy"`
	Pinned           bool             `json:"pinned"`
	Quadrant         Quadrant         `json:"quadrant"`
	DueAt            time.Time        `json:"dueAt"`
	Stress           *int             `json:"stress"`
	RemainingSeconds int64            `json:"remainingSeconds"`
	EstimateFactor   float64          `json:"estimateFactor"`
	Reasons          []string         `json:"reasons"`
	People           []SchedulePerson `json:"people,omitempty"`
}

type UnplannedItem struct {
	Item          Item `json:"item"`
	MissingDevDue bool `json:"missingDevDue"`
	MissingPlan   bool `json:"missingPlan"`
}

type SchedulePerson struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type ScheduleBusy struct {
	StartsAt       time.Time        `json:"startsAt"`
	EndsAt         time.Time        `json:"endsAt"`
	Title          string           `json:"title"`
	Soft           bool             `json:"soft"`
	SeriesID       *uuid.UUID       `json:"seriesId,omitempty"`
	OriginalOn     Ymd              `json:"originalOn,omitempty"`
	ActiveStartsAt *time.Time       `json:"activeStartsAt,omitempty"`
	ActiveEndsAt   *time.Time       `json:"activeEndsAt,omitempty"`
	People         []SchedulePerson `json:"people,omitempty"`
	CanSkip        bool             `json:"canSkip,omitempty"`
}

type ScheduleOverflow struct {
	ItemCount  int        `json:"itemCount"`
	Seconds    int64      `json:"seconds"`
	FirstDueAt *time.Time `json:"firstDueAt"`
}

type ScheduleCapacity struct {
	FreeSeconds   int64 `json:"freeSeconds"`
	PackedSeconds int64 `json:"packedSeconds"`
	BusySeconds   int64 `json:"busySeconds"`
}

type AtRiskItem struct {
	ItemID       uuid.UUID `json:"itemId"`
	Key          string    `json:"key"`
	Title        string    `json:"title"`
	SlackSeconds int64     `json:"slackSeconds"`
}

type span struct {
	start time.Time
	end   time.Time
}

func ParseScheduleKind(raw string) (ScheduleKind, error) {
	kind := ScheduleKind(strings.TrimSpace(raw))
	if kind == "" {
		return ScheduleWork, nil
	}
	switch kind {
	case ScheduleWork, ScheduleFollowup:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: schedule kind", ErrInvalid)
	}
}

// BuildSchedule packs work tasks with default settings.
func BuildSchedule(items []Item, events []EventOccurrence, now, from, to time.Time) Schedule {
	return BuildKindSchedule(ScheduleWork, items, events, now, from, to)
}

// BuildKindSchedule packs with default settings; kept for callers and tests
// that do not care about knobs.
func BuildKindSchedule(kind ScheduleKind, items []Item, events []EventOccurrence, now, from, to time.Time) Schedule {
	return Plan(ScheduleInput{
		Kind:     kind,
		Items:    items,
		Events:   events,
		Now:      now,
		From:     from,
		To:       to,
		Settings: DefaultScheduleSettings(),
	})
}

// Plan runs the packing pipeline: calendar → order → reservations → pack →
// explain/score. It is pure: the same input always yields the same output.
func Plan(in ScheduleInput) Schedule {
	p := newPlanner(in)
	if !p.to.After(p.from) {
		return p.empty()
	}
	return p.run()
}

type planner struct {
	in       ScheduleInput
	kind     ScheduleKind
	settings ScheduleSettings
	loc      *time.Location
	now      time.Time
	from     time.Time
	to       time.Time
	cal      *calendar
	golden   *goldenHours

	queue     []*ranked
	rankOf    map[uuid.UUID]int
	byID      map[uuid.UUID]*ranked
	unplanned []UnplannedItem
	meta      map[string]ScheduleLane
	itemLane  map[uuid.UUID]string
	active    map[uuid.UUID]bool
}

func newPlanner(in ScheduleInput) *planner {
	settings := in.Settings
	if settings.Timezone == "" {
		settings = DefaultScheduleSettings()
	}
	kind := in.Kind
	if kind == "" {
		kind = ScheduleWork
	}
	loc := settings.Location()
	p := &planner{
		in:       in,
		kind:     kind,
		settings: settings,
		loc:      loc,
		now:      in.Now.In(loc).Truncate(time.Second),
		from:     in.From.In(loc),
		to:       in.To.In(loc),
		rankOf:   map[uuid.UUID]int{},
		byID:     map[uuid.UUID]*ranked{},
		meta:     map[string]ScheduleLane{},
		itemLane: map[uuid.UUID]string{},
		active:   map[uuid.UUID]bool{},
	}
	p.cal = newCalendar(settings, in.Overrides, loc)
	for _, interval := range in.OpenIntervals {
		if interval.EndedAt == nil {
			p.active[interval.ItemID] = true
		}
	}
	if settings.GoldenHours && len(in.History) > 0 {
		p.golden = newGoldenHours(in.History, loc)
	}
	return p
}

func (p *planner) empty() Schedule {
	return Schedule{
		Lanes:     []ScheduleLane{},
		Unplanned: []UnplannedItem{},
		Busy:      []ScheduleBusy{},
		Grid:      p.grid(),
		AtRisk:    []AtRiskItem{},
	}
}

func (p *planner) grid() ScheduleGrid {
	start, end := p.cal.gridHours(p.from, p.to)
	return ScheduleGrid{
		Timezone:     p.loc.String(),
		StartHour:    start,
		EndHour:      end,
		Workdays:     append([]int{}, p.settings.Workdays...),
		WorkStartMin: p.settings.WorkStartMin,
		WorkEndMin:   p.settings.WorkEndMin,
	}
}

func (p *planner) run() Schedule {
	p.collect()
	free, soft := p.cal.freeSpans(p.now, p.in.Events)

	var blocks []ScheduleBlock
	if p.kind == ScheduleFollowup {
		blocks = p.packFollowup(free)
	} else {
		blocks = p.packWork(free, soft)
	}
	sortBlocks(blocks)
	blocks = markContinuation(blocks)
	blocks = p.explain(blocks)

	overflow := p.overflow(blocks)
	visible := filterBlocks(blocks, p.from, p.to)
	out := Schedule{
		Lanes:     paintLanes(visible, p.itemLane, p.meta),
		Unplanned: p.unplanned,
		Busy:      p.cal.busyInRange(p.in.Events, p.from, p.to, p.in.PeopleNames),
		Overflow:  overflow,
		Capacity:  p.capacity(free, visible),
		Grid:      p.grid(),
		AtRisk:    p.atRisk(),
		Score:     p.score(blocks, overflow),
		Horizon:   blocks,
	}
	if out.Unplanned == nil {
		out.Unplanned = []UnplannedItem{}
	}
	return out
}

// collect splits items into the packing queue and the unplanned list, and
// records lane metadata per project.
func (p *planner) collect() {
	var queue []*ranked
	for _, item := range p.in.Items {
		if !scheduleEligible(item, p.kind) {
			continue
		}
		due := packingDue(item, p.kind)
		missingDue := due == nil
		missingPlan := p.kind == ScheduleWork && item.PlannedSeconds <= 0
		if missingDue || missingPlan {
			p.unplanned = append(p.unplanned, UnplannedItem{Item: item, MissingDevDue: missingDue, MissingPlan: missingPlan})
			continue
		}
		r := p.rank(item, *due)
		if p.kind == ScheduleWork && r.remaining <= 0 {
			continue
		}
		queue = append(queue, r)
		key := laneKey(item.ProjectID)
		p.itemLane[item.ID] = key
		if _, ok := p.meta[key]; !ok {
			name := item.ProjectName
			if name == "" {
				name = "No project"
			}
			p.meta[key] = ScheduleLane{ProjectID: item.ProjectID, ProjectName: name, ProjectColor: item.ProjectColor}
		}
	}
	sort.SliceStable(queue, func(i, j int) bool { return p.less(queue[i], queue[j]) })
	for i, r := range queue {
		p.rankOf[r.item.ID] = i
		p.byID[r.item.ID] = r
	}
	p.queue = queue
	sort.SliceStable(p.unplanned, func(i, j int) bool {
		a, b := p.unplanned[i].Item, p.unplanned[j].Item
		return p.lessItems(a, b)
	})
}

func scheduleEligible(item Item, kind ScheduleKind) bool {
	if item.Kind != KindTask || item.ArchivedAt != nil || item.DeletedAt != nil {
		return false
	}
	if kind == ScheduleFollowup {
		switch item.Status {
		case StatusReview, StatusQA, StatusAwaitingDecision:
			return true
		default:
			return false
		}
	}
	switch item.Status {
	case StatusToDo, StatusInProgress:
		return true
	default:
		return false
	}
}

func packingDue(item Item, kind ScheduleKind) *time.Time {
	if kind == ScheduleFollowup {
		switch item.Status {
		case StatusReview:
			return item.ReviewDueAt
		case StatusQA:
			return item.TestDueAt
		default:
			return item.DueAt
		}
	}
	return item.DevDueAt
}

func remainingSeconds(item Item) int64 {
	left := int64(item.PlannedSeconds) - item.TrackedSeconds
	if left < 0 {
		return 0
	}
	return left
}

func laneKey(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func newBlock(r *ranked, start, end time.Time, lane int, occupancy Occupancy, people []SchedulePerson) ScheduleBlock {
	return ScheduleBlock{
		ItemID:           r.item.ID,
		Title:            r.item.Title,
		ExternalKey:      r.item.ExternalKey,
		Kind:             BlockKindWork,
		StartsAt:         start.UTC(),
		EndsAt:           end.UTC(),
		Late:             end.After(r.due),
		Lane:             lane,
		Occupancy:        occupancy,
		Pinned:           r.item.Pinned,
		Quadrant:         r.item.Quadrant(),
		DueAt:            r.due.UTC(),
		Stress:           r.item.Stress,
		RemainingSeconds: r.remaining,
		EstimateFactor:   r.factor,
		Reasons:          append([]string{}, r.reasons...),
		People:           people,
	}
}

func sortBlocks(blocks []ScheduleBlock) {
	sort.SliceStable(blocks, func(i, j int) bool {
		if !blocks[i].StartsAt.Equal(blocks[j].StartsAt) {
			return blocks[i].StartsAt.Before(blocks[j].StartsAt)
		}
		if blocks[i].Lane != blocks[j].Lane {
			return blocks[i].Lane < blocks[j].Lane
		}
		return blocks[i].ItemID.String() < blocks[j].ItemID.String()
	})
}

func markContinuation(blocks []ScheduleBlock) []ScheduleBlock {
	counts := map[uuid.UUID]int{}
	for _, block := range blocks {
		counts[block.ItemID]++
	}
	seen := map[uuid.UUID]int{}
	for i := range blocks {
		id := blocks[i].ItemID
		seen[id]++
		blocks[i].Continued = seen[id] > 1
		blocks[i].Continues = seen[id] < counts[id]
	}
	return blocks
}

func filterBlocks(blocks []ScheduleBlock, from, to time.Time) []ScheduleBlock {
	out := []ScheduleBlock{}
	for _, block := range blocks {
		if !block.EndsAt.After(from) || !block.StartsAt.Before(to) {
			continue
		}
		out = append(out, block)
	}
	return out
}

func paintLanes(blocks []ScheduleBlock, itemLane map[uuid.UUID]string, meta map[string]ScheduleLane) []ScheduleLane {
	grouped := map[string][]ScheduleBlock{}
	for _, block := range blocks {
		key := itemLane[block.ItemID]
		grouped[key] = append(grouped[key], block)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := meta[keys[i]], meta[keys[j]]
		if a.ProjectID == nil && b.ProjectID != nil {
			return false
		}
		if a.ProjectID != nil && b.ProjectID == nil {
			return true
		}
		if a.ProjectName == b.ProjectName {
			return keys[i] < keys[j]
		}
		return a.ProjectName < b.ProjectName
	})
	lanes := make([]ScheduleLane, 0, len(keys))
	for _, key := range keys {
		lane := meta[key]
		lane.Blocks = grouped[key]
		if len(lane.Blocks) == 0 {
			continue
		}
		lanes = append(lanes, lane)
	}
	return lanes
}

// overflow sums late seconds of placed blocks plus whatever never fit.
func (p *planner) overflow(blocks []ScheduleBlock) ScheduleOverflow {
	packed := map[uuid.UUID]int64{}
	late := map[uuid.UUID]int64{}
	for _, block := range blocks {
		packed[block.ItemID] += spanSeconds(block.StartsAt, block.EndsAt)
		late[block.ItemID] += lateSeconds(block.StartsAt, block.EndsAt, block.DueAt)
	}
	var out ScheduleOverflow
	for _, r := range p.queue {
		left := int64(0)
		if p.kind == ScheduleWork {
			left = r.remaining - packed[r.item.ID]
			if left < 0 {
				left = 0
			}
		}
		over := late[r.item.ID] + left
		if over == 0 {
			continue
		}
		out.ItemCount++
		out.Seconds += over
		at := r.due.UTC()
		if out.FirstDueAt == nil || at.Before(*out.FirstDueAt) {
			out.FirstDueAt = &at
		}
	}
	return out
}

func (p *planner) capacity(free []span, blocks []ScheduleBlock) ScheduleCapacity {
	var out ScheduleCapacity
	for _, slot := range free {
		start, end, ok := clipInterval(slot.start, slot.end, p.from, p.to)
		if ok {
			out.FreeSeconds += spanSeconds(start, end)
		}
	}
	for _, block := range blocks {
		start, end, ok := clipInterval(block.StartsAt, block.EndsAt, p.from, p.to)
		if ok {
			out.PackedSeconds += spanSeconds(start, end)
		}
	}
	for _, event := range p.in.Events {
		start, end := event.ActiveSpan()
		start, end, ok := clipInterval(start, end, p.from, p.to)
		if ok {
			out.BusySeconds += spanSeconds(start, end)
		}
	}
	return out
}

func (p *planner) atRisk() []AtRiskItem {
	out := []AtRiskItem{}
	for _, r := range p.queue {
		if r.slack >= 0 {
			continue
		}
		out = append(out, AtRiskItem{ItemID: r.item.ID, Key: r.item.ExternalKey, Title: r.item.Title, SlackSeconds: r.slack})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SlackSeconds < out[j].SlackSeconds })
	return out
}

func mergeSpans(in []span) []span {
	if len(in) == 0 {
		return nil
	}
	sorted := cloneSpans(in)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].start.Before(sorted[j].start)
	})
	out := []span{sorted[0]}
	for _, next := range sorted[1:] {
		last := &out[len(out)-1]
		if next.start.After(last.end) {
			out = append(out, next)
			continue
		}
		if next.end.After(last.end) {
			last.end = next.end
		}
	}
	return out
}

func complement(from, to time.Time, busy []span) []span {
	cursor := from
	var free []span
	for _, hole := range busy {
		if hole.start.After(cursor) {
			free = append(free, span{start: cursor, end: hole.start})
		}
		if hole.end.After(cursor) {
			cursor = hole.end
		}
		if !cursor.Before(to) {
			return free
		}
	}
	if to.After(cursor) {
		free = append(free, span{start: cursor, end: to})
	}
	return free
}

func cloneSpans(in []span) []span {
	out := make([]span, len(in))
	copy(out, in)
	return out
}

func subtractSpan(slots []span, start, end time.Time) []span {
	if !end.After(start) {
		return slots
	}
	var out []span
	for _, slot := range slots {
		if !slot.end.After(start) || !end.After(slot.start) {
			out = append(out, slot)
			continue
		}
		if slot.start.Before(start) {
			out = append(out, span{start: slot.start, end: start})
		}
		if slot.end.After(end) {
			out = append(out, span{start: end, end: slot.end})
		}
	}
	return out
}

func subtractAll(slots []span, holes []span) []span {
	out := slots
	for _, hole := range holes {
		out = subtractSpan(out, hole.start, hole.end)
	}
	return out
}

// containsSpan reports whether [start, end) lies entirely inside one slot.
func containsSpan(slots []span, start, end time.Time) bool {
	for _, slot := range slots {
		if !slot.start.After(start) && !slot.end.Before(end) {
			return true
		}
	}
	return false
}

func spanSeconds(start, end time.Time) int64 {
	d := end.Sub(start)
	if d <= 0 {
		return 0
	}
	return int64(d / time.Second)
}

func lateSeconds(start, end, due time.Time) int64 {
	if !end.After(due) {
		return 0
	}
	lateStart := start
	if due.After(start) {
		lateStart = due
	}
	return spanSeconds(lateStart, end)
}
