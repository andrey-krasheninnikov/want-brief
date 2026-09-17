package domain

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func msk(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, Moscow())
}

func taskItem(title string, project *uuid.UUID, name, color string, due time.Time, plan int, tracked int64, status ItemStatus) Item {
	item := Item{
		ID:             uuid.New(),
		Title:          title,
		ExternalKey:    title,
		Status:         status,
		Kind:           KindTask,
		Occupancy:      OccupancySolo,
		ProjectID:      project,
		ProjectName:    name,
		ProjectColor:   color,
		DevDueAt:       &due,
		PlannedSeconds: plan,
		TrackedSeconds: tracked,
		CreatedAt:      msk(2026, 1, 1, 12, 0).UTC(),
	}
	return item.WithQuadrant()
}

func planWith(items []Item, events []EventOccurrence, now, from, to time.Time, tweak func(*ScheduleInput)) Schedule {
	in := ScheduleInput{
		Kind:     ScheduleWork,
		Items:    items,
		Events:   events,
		Now:      now,
		From:     from,
		To:       to,
		Settings: DefaultScheduleSettings(),
	}
	if tweak != nil {
		tweak(&in)
	}
	return Plan(in)
}

func allLaneBlocks(got Schedule) []ScheduleBlock {
	var out []ScheduleBlock
	for _, lane := range got.Lanes {
		out = append(out, lane.Blocks...)
	}
	sortBlocks(out)
	return out
}

func blocksOf(got Schedule, id uuid.UUID) []ScheduleBlock {
	var out []ScheduleBlock
	for _, block := range allLaneBlocks(got) {
		if block.ItemID == id {
			out = append(out, block)
		}
	}
	return out
}

func firstBlock(t *testing.T, got Schedule, id uuid.UUID) ScheduleBlock {
	t.Helper()
	blocks := blocksOf(got, id)
	if len(blocks) == 0 {
		t.Fatalf("no blocks for %s", id)
	}
	return blocks[0]
}

func wantTime(t *testing.T, label string, got, want time.Time) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("%s %s want %s", label, got.In(Moscow()).Format("Mon 02 15:04"), want.In(Moscow()).Format("Mon 02 15:04"))
	}
}

func hasReasonPrefix(block ScheduleBlock, prefix string) bool {
	for _, reason := range block.Reasons {
		if len(reason) >= len(prefix) && reason[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func TestBuildScheduleSkipsReviewOnWork(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	item := taskItem("Review", &pid, "A", "#f00", due, 3600, 0, StatusReview)
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	if len(got.Lanes) != 0 || len(got.Unplanned) != 0 {
		t.Fatalf("lanes=%d unplanned=%d", len(got.Lanes), len(got.Unplanned))
	}
}

func TestBuildScheduleSkipsNonPackableStatuses(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	for _, status := range []ItemStatus{StatusNeedsGrooming, StatusAwaitingDecision, StatusBacklog, StatusBlocked} {
		item := taskItem(string(status), &pid, "A", "#f00", due, 3600, 0, status)
		got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
		if len(got.Lanes) != 0 || len(got.Unplanned) != 0 {
			t.Fatalf("%s: lanes=%d unplanned=%d", status, len(got.Lanes), len(got.Unplanned))
		}
	}
}

func TestBuildScheduleMissingDevDueIsUnplanned(t *testing.T) {
	pid := uuid.New()
	item := taskItem("Need due", &pid, "A", "#f00", msk(2026, 9, 20, 18, 0), 3600, 0, StatusToDo)
	item.DevDueAt = nil
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	if len(got.Unplanned) != 1 || !got.Unplanned[0].MissingDevDue || got.Unplanned[0].MissingPlan {
		t.Fatalf("%+v", got.Unplanned)
	}
}

func TestBuildScheduleMissingPlanIsUnplanned(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	item := taskItem("Need plan", &pid, "A", "#f00", due, 0, 0, StatusToDo)
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	if len(got.Unplanned) != 1 || !got.Unplanned[0].MissingPlan || got.Unplanned[0].MissingDevDue {
		t.Fatalf("%+v", got.Unplanned)
	}
	if len(got.Lanes) != 0 {
		t.Fatalf("lanes %d", len(got.Lanes))
	}
}

func TestBuildScheduleSerialProjects(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	a, b := uuid.New(), uuid.New()
	first := taskItem("A1", &a, "Alpha", "#111", due, 2*3600, 0, StatusToDo)
	first.CreatedAt = msk(2026, 1, 1, 12, 0).UTC()
	second := taskItem("B1", &b, "Beta", "#222", due, 2*3600, 0, StatusToDo)
	second.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{first, second}, nil, now, now, now.Add(24*time.Hour))
	if len(got.Lanes) != 2 {
		t.Fatalf("lanes %d", len(got.Lanes))
	}
	alpha := laneNamed(got, "Alpha")
	beta := laneNamed(got, "Beta")
	wantTime(t, "alpha start", alpha.Blocks[0].StartsAt, msk(2026, 9, 15, 10, 0))
	wantTime(t, "alpha end", alpha.Blocks[0].EndsAt, msk(2026, 9, 15, 12, 0))
	wantTime(t, "beta start", beta.Blocks[0].StartsAt, alpha.Blocks[0].EndsAt)
}

func TestBuildScheduleBlockCarriesWhy(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	stress := 4
	item := taskItem("Why", &pid, "Alpha", "#111", due, 4*3600, 3600, StatusToDo)
	item.Pinned = true
	item.Urgent = true
	item.Important = true
	item.Stress = &stress
	item = item.WithQuadrant()
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	block := laneNamed(got, "Alpha").Blocks[0]
	if !block.Pinned || block.Quadrant != QuadrantDo {
		t.Fatalf("pinned=%v quadrant=%s", block.Pinned, block.Quadrant)
	}
	if !block.DueAt.Equal(due.UTC()) {
		t.Fatalf("dueAt %s want %s", block.DueAt, due.UTC())
	}
	if block.Stress == nil || *block.Stress != stress {
		t.Fatalf("stress %v", block.Stress)
	}
	if block.RemainingSeconds != 3*3600 {
		t.Fatalf("remaining %d", block.RemainingSeconds)
	}
	if !hasReason(block.Reasons, reasonPinned, reasonInProgress) {
		t.Fatalf("reasons %v", block.Reasons)
	}
	if block.Kind != BlockKindWork || block.EstimateFactor != 1 {
		t.Fatalf("kind=%s factor=%v", block.Kind, block.EstimateFactor)
	}
}

func TestBuildScheduleMeetingThenNextProject(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	a, b := uuid.New(), uuid.New()
	first := taskItem("A1", &a, "Alpha", "#111", due, 3*3600, 0, StatusToDo)
	second := taskItem("B1", &b, "Beta", "#222", due, 3*3600, 0, StatusToDo)
	second.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	meetStart := msk(2026, 9, 15, 10, 0)
	sid := uuid.New()
	events := []EventOccurrence{{SeriesID: sid, Title: "Standup", StartsAt: meetStart, EndsAt: meetStart.Add(time.Hour)}}
	now := msk(2026, 9, 15, 8, 0)
	got := planWith([]Item{first, second}, events, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Settings.Break = &BreakWindow{StartMin: 13 * 60, EndMin: 14 * 60}
	})
	if !hasBusyTitle(got, "Standup") || !hasBusyTitle(got, breakTitle) {
		t.Fatalf("busy %+v", got.Busy)
	}
	alpha := laneNamed(got, "Alpha")
	wantTime(t, "alpha start", alpha.Blocks[0].StartsAt, meetStart.Add(time.Hour+10*time.Minute))
	beta := laneNamed(got, "Beta")
	lastAlpha := alpha.Blocks[len(alpha.Blocks)-1]
	if beta.Blocks[0].StartsAt.Before(lastAlpha.EndsAt) {
		t.Fatalf("beta %s overlaps alpha end %s", beta.Blocks[0].StartsAt, lastAlpha.EndsAt)
	}
}

func TestBuildScheduleSplitsLongTicket(t *testing.T) {
	due := msk(2026, 9, 30, 18, 0)
	pid := uuid.New()
	item := taskItem("Long", &pid, "Alpha", "#111", due, 20*3600, 0, StatusToDo)
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{item}, nil, now, now, msk(2026, 9, 17, 0, 0))
	blocks := laneNamed(got, "Alpha").Blocks
	// Two-hour slices with ten-minute pauses, six focus hours per day.
	if len(blocks) != 6 {
		t.Fatalf("blocks %d", len(blocks))
	}
	if dur := blocks[0].EndsAt.Sub(blocks[0].StartsAt); dur != 2*time.Hour {
		t.Fatalf("first slice %s", dur)
	}
	wantTime(t, "second start", blocks[1].StartsAt, msk(2026, 9, 15, 12, 10))
	wantTime(t, "day two start", blocks[3].StartsAt, msk(2026, 9, 16, 10, 0))
	if !blocks[0].Continues || blocks[0].Continued {
		t.Fatalf("first continued=%v continues=%v", blocks[0].Continued, blocks[0].Continues)
	}
	if !blocks[5].Continued || !blocks[5].Continues {
		t.Fatalf("last continued=%v continues=%v", blocks[5].Continued, blocks[5].Continues)
	}
	var total time.Duration
	for _, block := range blocks {
		total += block.EndsAt.Sub(block.StartsAt)
	}
	if total != 12*time.Hour {
		t.Fatalf("visible %s", total)
	}
}

func TestBuildScheduleLateAfterDevDue(t *testing.T) {
	due := msk(2026, 9, 15, 9, 0)
	pid := uuid.New()
	item := taskItem("Late", &pid, "Alpha", "#111", due, 2*3600, 0, StatusToDo)
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	block := got.Lanes[0].Blocks[0]
	if !block.Late || !hasReason(block.Reasons, reasonNoSlack) {
		t.Fatalf("late=%v reasons=%v", block.Late, block.Reasons)
	}
	if len(got.AtRisk) != 1 || got.AtRisk[0].SlackSeconds >= 0 {
		t.Fatalf("atRisk %+v", got.AtRisk)
	}
}

func TestBuildScheduleSkipsPastHoursToday(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	item := taskItem("Now", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	now := msk(2026, 9, 15, 15, 0)
	got := BuildSchedule([]Item{item}, nil, now, msk(2026, 9, 15, 0, 0), msk(2026, 9, 16, 0, 0))
	wantTime(t, "start", got.Lanes[0].Blocks[0].StartsAt, now)
}

func TestBuildScheduleSkipsFullyTracked(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	item := taskItem("Done hours", &pid, "Alpha", "#111", due, 3600, 3600, StatusInProgress)
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	if len(got.Lanes) != 0 || len(got.Unplanned) != 0 {
		t.Fatalf("lanes=%d unplanned=%d", len(got.Lanes), len(got.Unplanned))
	}
}

func TestPlanNoBreakByDefault(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	item := taskItem("Across noon", &pid, "Alpha", "#111", due, 4*3600, 0, StatusToDo)
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	blocks := laneNamed(got, "Alpha").Blocks
	if len(blocks) != 2 {
		t.Fatalf("blocks %d", len(blocks))
	}
	wantTime(t, "second end", blocks[1].EndsAt, msk(2026, 9, 15, 14, 10))
	if hasBusyTitle(got, "Lunch") || hasBusyTitle(got, breakTitle) {
		t.Fatalf("unexpected break %+v", got.Busy)
	}
}

func TestPlanBreakSplitsDay(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	item := taskItem("Across break", &pid, "Alpha", "#111", due, 3*3600, 0, StatusToDo)
	now := msk(2026, 9, 15, 8, 0)
	got := planWith([]Item{item}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Settings.Break = &BreakWindow{StartMin: 13 * 60, EndMin: 14 * 60}
	})
	blocks := laneNamed(got, "Alpha").Blocks
	if len(blocks) != 2 {
		t.Fatalf("blocks %+v", blocks)
	}
	// 10:00-12:00 is the max slice; the remaining hour does not fit before
	// the break (12:10-13:00) so whole-fit takes the gap after it.
	wantTime(t, "pre-break end", blocks[0].EndsAt, msk(2026, 9, 15, 12, 0))
	wantTime(t, "post-break start", blocks[1].StartsAt, msk(2026, 9, 15, 14, 0))
	for _, block := range blocks {
		if block.StartsAt.Before(msk(2026, 9, 15, 14, 0)) && block.EndsAt.After(msk(2026, 9, 15, 13, 0)) {
			t.Fatalf("block overlaps break %+v", block)
		}
	}
	if !hasBusyTitle(got, breakTitle) {
		t.Fatalf("busy %+v", got.Busy)
	}
}

func TestBuildScheduleSkipsWeekend(t *testing.T) {
	due := msk(2026, 9, 30, 18, 0)
	pid := uuid.New()
	item := taskItem("Weekend", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	now := msk(2026, 9, 19, 8, 0)
	got := BuildSchedule([]Item{item}, nil, now, msk(2026, 9, 19, 0, 0), msk(2026, 9, 21, 0, 0))
	if len(got.Lanes) != 0 {
		t.Fatalf("lanes %d", len(got.Lanes))
	}
}

func TestPlanWorkdaysFromSettings(t *testing.T) {
	due := msk(2026, 9, 30, 18, 0)
	pid := uuid.New()
	item := taskItem("Saturday", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	now := msk(2026, 9, 19, 8, 0)
	got := planWith([]Item{item}, nil, now, msk(2026, 9, 19, 0, 0), msk(2026, 9, 21, 0, 0), func(in *ScheduleInput) {
		in.Settings.Workdays = []int{1, 2, 3, 4, 5, 6}
	})
	wantTime(t, "saturday start", firstBlock(t, got, item.ID).StartsAt, msk(2026, 9, 19, 10, 0))
}

func TestPlanTimezoneFromSettings(t *testing.T) {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	due := time.Date(2026, 9, 20, 18, 0, 0, 0, berlin)
	pid := uuid.New()
	item := taskItem("Berlin", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, berlin)
	got := planWith([]Item{item}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Settings.Timezone = "Europe/Berlin"
	})
	if !firstBlock(t, got, item.ID).StartsAt.Equal(time.Date(2026, 9, 15, 10, 0, 0, 0, berlin)) {
		t.Fatalf("start %s", firstBlock(t, got, item.ID).StartsAt.In(berlin))
	}
	if got.Grid.Timezone != "Europe/Berlin" || got.Grid.StartHour != 8 || got.Grid.EndHour != 19 {
		t.Fatalf("grid %+v", got.Grid)
	}
}

func TestBuildScheduleOverflowAfterDue(t *testing.T) {
	due := msk(2026, 9, 15, 11, 0)
	pid := uuid.New()
	item := taskItem("Late", &pid, "Alpha", "#111", due, 4*3600, 0, StatusToDo)
	now := msk(2026, 9, 15, 10, 0)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	if got.Overflow.ItemCount != 1 || got.Overflow.Seconds <= 0 {
		t.Fatalf("%+v", got.Overflow)
	}
	if got.Overflow.FirstDueAt == nil || !got.Overflow.FirstDueAt.Equal(due.UTC()) {
		t.Fatalf("due %+v", got.Overflow.FirstDueAt)
	}
	if got.Score.LateSeconds != got.Overflow.Seconds {
		t.Fatalf("score %+v overflow %+v", got.Score, got.Overflow)
	}
}

func TestBuildSchedulePinnedFirst(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	a, b := uuid.New(), uuid.New()
	plain := taskItem("Plain", &a, "Alpha", "#111", due, 3600, 0, StatusToDo)
	pinned := taskItem("Pinned", &b, "Beta", "#222", due, 3600, 0, StatusToDo)
	pinned.Pinned = true
	pinned.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	now := msk(2026, 9, 15, 8, 0)
	got := BuildSchedule([]Item{plain, pinned}, nil, now, now, now.Add(24*time.Hour))
	wantTime(t, "pinned start", laneNamed(got, "Beta").Blocks[0].StartsAt, msk(2026, 9, 15, 10, 0))
	wantTime(t, "plain start", laneNamed(got, "Alpha").Blocks[0].StartsAt, msk(2026, 9, 15, 11, 0))
}

// #1 #2: a soon-due Schedule task beats a far-due Do task.
func TestPlanSlackBeatsQuadrant(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	do := taskItem("Do later", &pid, "Alpha", "#111", msk(2026, 10, 15, 18, 0), 2*3600, 0, StatusToDo)
	do.Urgent, do.Important = true, true
	soon := taskItem("Schedule soon", &pid, "Alpha", "#111", msk(2026, 9, 16, 18, 0), 2*3600, 0, StatusToDo)
	soon.Important = true
	soon.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{do, soon}, nil, now, now, now.Add(24*time.Hour))
	wantTime(t, "soon start", firstBlock(t, got, soon.ID).StartsAt, msk(2026, 9, 15, 10, 0))
	wantTime(t, "do start", firstBlock(t, got, do.ID).StartsAt, msk(2026, 9, 15, 12, 0))
	if !hasReason(firstBlock(t, got, soon.ID).Reasons, reasonDueSoon) {
		t.Fatalf("reasons %v", firstBlock(t, got, soon.ID).Reasons)
	}
	if len(got.AtRisk) != 0 {
		t.Fatalf("atRisk %+v", got.AtRisk)
	}
}

// #3: started work goes before fresh work.
func TestPlanInProgressFirst(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 20, 18, 0)
	fresh := taskItem("Fresh", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	started := taskItem("Started", &pid, "Alpha", "#111", due, 3600, 0, StatusInProgress)
	started.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{fresh, started}, nil, now, now, now.Add(24*time.Hour))
	wantTime(t, "started", firstBlock(t, got, started.ID).StartsAt, msk(2026, 9, 15, 10, 0))
	wantTime(t, "fresh", firstBlock(t, got, fresh.ID).StartsAt, msk(2026, 9, 15, 11, 0))
}

// #4: OldestFirst is a setting.
func TestPlanOldestFirstToggle(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 20, 18, 0)
	old := taskItem("Old", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	young := taskItem("Young", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	young.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{young, old}, nil, now, now, now.Add(24*time.Hour))
	wantTime(t, "old first", firstBlock(t, got, old.ID).StartsAt, msk(2026, 9, 15, 10, 0))
	got = planWith([]Item{young, old}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Settings.OldestFirst = false
	})
	wantTime(t, "young first", firstBlock(t, got, young.ID).StartsAt, msk(2026, 9, 15, 10, 0))
}

// #5: stress pulls the effective due earlier.
func TestPlanStressPullsDueEarlier(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	calm := taskItem("Calm", &pid, "Alpha", "#111", msk(2026, 9, 17, 18, 0), 3600, 0, StatusToDo)
	tense := taskItem("Tense", &pid, "Alpha", "#111", msk(2026, 9, 18, 12, 0), 3600, 0, StatusToDo)
	five := 5
	tense.Stress = &five
	tense.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{calm, tense}, nil, now, now, now.Add(24*time.Hour))
	wantTime(t, "tense first", firstBlock(t, got, tense.ID).StartsAt, msk(2026, 9, 15, 10, 0))
	got = planWith([]Item{calm, tense}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Settings.StressShiftHours = 0
	})
	wantTime(t, "calm first without shift", firstBlock(t, got, calm.ID).StartsAt, msk(2026, 9, 15, 10, 0))
}

// #6: urgent tightens the deadline independently of the quadrant.
func TestPlanUrgentTightensDue(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	plain := taskItem("Plain", &pid, "Alpha", "#111", msk(2026, 9, 17, 18, 0), 3600, 0, StatusToDo)
	urgent := taskItem("Urgent", &pid, "Alpha", "#111", msk(2026, 9, 18, 18, 0), 3600, 0, StatusToDo)
	urgent.Urgent = true
	urgent.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{plain, urgent}, nil, now, now, now.Add(24*time.Hour))
	wantTime(t, "urgent first", firstBlock(t, got, urgent.ID).StartsAt, msk(2026, 9, 15, 10, 0))
}

// #7: PinnedAt lands on the first free slot at or after the pin.
func TestPlanPinnedAt(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 20, 18, 0)
	filler := taskItem("Filler", &pid, "Alpha", "#111", due, 9*3600, 0, StatusToDo)
	pinned := taskItem("Pinned at", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	at := msk(2026, 9, 16, 14, 0)
	pinned.PinnedAt = &at
	pinned.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{filler, pinned}, nil, now, now, now.Add(72*time.Hour))
	block := firstBlock(t, got, pinned.ID)
	wantTime(t, "pinned at", block.StartsAt, at)
	if !hasReason(block.Reasons, reasonPinnedAt) {
		t.Fatalf("reasons %v", block.Reasons)
	}
	for _, other := range blocksOf(got, filler.ID) {
		if other.StartsAt.Before(block.EndsAt) && other.EndsAt.After(block.StartsAt) {
			t.Fatalf("filler %s-%s overlaps pin", other.StartsAt, other.EndsAt)
		}
	}
}

// #8: gaps shorter than the minimum slice are skipped.
func TestPlanMinSliceSkipsShortGaps(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	item := taskItem("Two hours", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 2*3600, 0, StatusToDo)
	events := []EventOccurrence{{SeriesID: uuid.New(), Title: "Sync", StartsAt: msk(2026, 9, 15, 10, 30), EndsAt: msk(2026, 9, 15, 11, 30)}}
	got := BuildSchedule([]Item{item}, events, now, now, now.Add(24*time.Hour))
	wantTime(t, "start after sync", firstBlock(t, got, item.ID).StartsAt, msk(2026, 9, 15, 11, 40))
}

// #9: long work is cut into slices with a pause between them.
func TestPlanMaxSliceInsertsBreak(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	item := taskItem("Four hours", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 4*3600, 0, StatusToDo)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	blocks := blocksOf(got, item.ID)
	if len(blocks) != 2 {
		t.Fatalf("blocks %+v", blocks)
	}
	wantTime(t, "first end", blocks[0].EndsAt, msk(2026, 9, 15, 12, 0))
	wantTime(t, "second start", blocks[1].StartsAt, msk(2026, 9, 15, 12, 10))
}

// #10: a task that fits whole later today is not split now.
func TestPlanWholeFitPrefersUnbrokenGap(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 11, 30)
	item := taskItem("One hour", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 3600, 0, StatusToDo)
	events := []EventOccurrence{{SeriesID: uuid.New(), Title: "Call", StartsAt: msk(2026, 9, 15, 12, 30), EndsAt: msk(2026, 9, 15, 13, 0)}}
	got := BuildSchedule([]Item{item}, events, now, msk(2026, 9, 15, 0, 0), msk(2026, 9, 16, 0, 0))
	blocks := blocksOf(got, item.ID)
	if len(blocks) != 1 {
		t.Fatalf("blocks %+v", blocks)
	}
	wantTime(t, "whole fit", blocks[0].StartsAt, msk(2026, 9, 15, 13, 10))
}

// #11: within one priority class the packer continues the same project.
func TestPlanSameProjectContinues(t *testing.T) {
	x, y := uuid.New(), uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 20, 18, 0)
	a := taskItem("A", &x, "X", "#111", due, 3600, 0, StatusToDo)
	b := taskItem("B", &y, "Y", "#222", due, 3600, 0, StatusToDo)
	b.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	c := taskItem("C", &x, "X", "#111", due, 3600, 0, StatusToDo)
	c.CreatedAt = msk(2026, 1, 3, 12, 0).UTC()
	got := BuildSchedule([]Item{a, b, c}, nil, now, now, now.Add(24*time.Hour))
	wantTime(t, "a", firstBlock(t, got, a.ID).StartsAt, msk(2026, 9, 15, 10, 0))
	wantTime(t, "c continues project", firstBlock(t, got, c.ID).StartsAt, msk(2026, 9, 15, 11, 0))
	wantTime(t, "b", firstBlock(t, got, b.ID).StartsAt, msk(2026, 9, 15, 12, 0))
	if !hasReason(firstBlock(t, got, c.ID).Reasons, reasonSameProj) {
		t.Fatalf("reasons %v", firstBlock(t, got, c.ID).Reasons)
	}
	if got.Score.Switches != 1 {
		t.Fatalf("switches %d", got.Score.Switches)
	}
}

// #12: starts snap to the grid and the last slice absorbs the odd minutes.
func TestPlanGridRounding(t *testing.T) {
	pid := uuid.New()
	now := time.Date(2026, 9, 15, 10, 7, 30, 0, Moscow())
	item := taskItem("Odd", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 58*60, 0, StatusToDo)
	got := BuildSchedule([]Item{item}, nil, now, msk(2026, 9, 15, 0, 0), msk(2026, 9, 16, 0, 0))
	block := firstBlock(t, got, item.ID)
	wantTime(t, "start", block.StartsAt, msk(2026, 9, 15, 10, 10))
	wantTime(t, "end", block.EndsAt, msk(2026, 9, 15, 11, 10))
}

// #13: project estimate factors inflate the remaining work.
func TestPlanEstimateFactor(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	item := taskItem("Underestimated", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 2*3600, 0, StatusToDo)
	got := planWith([]Item{item}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Factors = map[string]float64{pid.String(): 1.5}
	})
	block := firstBlock(t, got, item.ID)
	if block.RemainingSeconds != 3*3600 || block.EstimateFactor != 1.5 {
		t.Fatalf("remaining=%d factor=%v", block.RemainingSeconds, block.EstimateFactor)
	}
	if !hasReasonPrefix(block, reasonEstimate) {
		t.Fatalf("reasons %v", block.Reasons)
	}
	got = planWith([]Item{item}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Factors = map[string]float64{pid.String(): 9}
	})
	if firstBlock(t, got, item.ID).EstimateFactor != 2 {
		t.Fatalf("clamp %v", firstBlock(t, got, item.ID).EstimateFactor)
	}
}

// #14: meetings get a buffer on both sides.
func TestPlanMeetingBuffer(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 15, 0)
	item := taskItem("Around meeting", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 3*3600, 0, StatusToDo)
	events := []EventOccurrence{{SeriesID: uuid.New(), Title: "Review", StartsAt: msk(2026, 9, 15, 17, 0), EndsAt: msk(2026, 9, 15, 18, 0)}}
	got := BuildSchedule([]Item{item}, events, now, now, now.Add(24*time.Hour))
	blocks := blocksOf(got, item.ID)
	if len(blocks) != 3 {
		t.Fatalf("blocks %+v", blocks)
	}
	wantTime(t, "before buffer", blocks[0].EndsAt, msk(2026, 9, 15, 16, 50))
	wantTime(t, "after buffer", blocks[1].StartsAt, msk(2026, 9, 15, 18, 10))
	if len(got.Busy) != 1 || !got.Busy[0].StartsAt.Equal(msk(2026, 9, 15, 17, 0)) {
		t.Fatalf("busy keeps real meeting bounds %+v", got.Busy)
	}
}

// #15: day overrides switch a day off or change its hours.
func TestPlanDayOverrides(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	item := taskItem("Override", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 3600, 0, StatusToDo)
	got := planWith([]Item{item}, nil, now, now, now.Add(48*time.Hour), func(in *ScheduleInput) {
		in.Overrides = []DayOverride{{Day: "2026-09-15", Off: true}}
	})
	wantTime(t, "day off", firstBlock(t, got, item.ID).StartsAt, msk(2026, 9, 16, 10, 0))
	start, end := 12*60, 16*60
	got = planWith([]Item{item}, nil, now, now, now.Add(48*time.Hour), func(in *ScheduleInput) {
		in.Overrides = []DayOverride{{Day: "2026-09-15", WorkStartMin: &start, WorkEndMin: &end}}
	})
	wantTime(t, "custom hours", firstBlock(t, got, item.ID).StartsAt, msk(2026, 9, 15, 12, 0))
}

// #17: the daily focus budget caps how much lands on one day.
func TestPlanDailyFocusBudget(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	item := taskItem("Big", &pid, "Alpha", "#111", msk(2026, 9, 30, 18, 0), 4*3600, 0, StatusToDo)
	got := planWith([]Item{item}, nil, now, now, now.Add(72*time.Hour), func(in *ScheduleInput) {
		in.Settings.DailyFocusMin = 180
	})
	blocks := blocksOf(got, item.ID)
	if len(blocks) != 3 {
		t.Fatalf("blocks %+v", blocks)
	}
	wantTime(t, "second end", blocks[1].EndsAt, msk(2026, 9, 15, 13, 10))
	wantTime(t, "third day", blocks[2].StartsAt, msk(2026, 9, 16, 10, 0))
}

// #18: skippable meetings are used only when the task would be late.
func TestPlanSoftBusyRetry(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	events := []EventOccurrence{{SeriesID: uuid.New(), Title: "Optional", CanSkip: true, StartsAt: msk(2026, 9, 15, 10, 0), EndsAt: msk(2026, 9, 15, 19, 0)}}
	urgent := taskItem("Due today", &pid, "Alpha", "#111", msk(2026, 9, 15, 18, 0), 2*3600, 0, StatusToDo)
	relaxed := taskItem("Due later", &pid, "Alpha", "#111", msk(2026, 9, 30, 18, 0), 2*3600, 0, StatusToDo)
	relaxed.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{urgent, relaxed}, events, now, now, now.Add(48*time.Hour))
	block := firstBlock(t, got, urgent.ID)
	wantTime(t, "into soft meeting", block.StartsAt, msk(2026, 9, 15, 10, 0))
	if !hasReasonPrefix(block, reasonSoftBusy) {
		t.Fatalf("reasons %v", block.Reasons)
	}
	wantTime(t, "relaxed waits", firstBlock(t, got, relaxed.ID).StartsAt, msk(2026, 9, 16, 10, 0))
	if len(got.Busy) != 1 || !got.Busy[0].Soft {
		t.Fatalf("busy %+v", got.Busy)
	}
}

// #19: the task being tracked stays where it is, even outside work hours.
func TestPlanActiveIntervalStaysNow(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 4, 30)
	due := msk(2026, 9, 20, 18, 0)
	first := taskItem("First", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	tracked := taskItem("Tracked", &pid, "Alpha", "#111", due, 3*3600, 0, StatusToDo)
	tracked.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := planWith([]Item{first, tracked}, nil, now, msk(2026, 9, 15, 0, 0), msk(2026, 9, 16, 0, 0), func(in *ScheduleInput) {
		in.OpenIntervals = []TimeInterval{{ID: uuid.New(), ItemID: tracked.ID, StartedAt: now.Add(-10 * time.Minute)}}
	})
	blocks := blocksOf(got, tracked.ID)
	if len(blocks) != 2 {
		t.Fatalf("tracked blocks %+v", blocks)
	}
	active := blocks[0]
	wantTime(t, "active start", active.StartsAt, now)
	wantTime(t, "active end", active.EndsAt, now.Add(2*time.Hour))
	if !hasReason(active.Reasons, reasonActive) {
		t.Fatalf("reasons %v", active.Reasons)
	}
	// The active task keeps its head start: its last hour opens the day and
	// only then the other task follows.
	wantTime(t, "tail opens the day", blocks[1].StartsAt, msk(2026, 9, 15, 10, 0))
	wantTime(t, "first waits", firstBlock(t, got, first.ID).StartsAt, msk(2026, 9, 15, 11, 0))
}

// #20: one active track plus waiting tracks.
func TestPlanActivePlusWaitingSideBySide(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	active := taskItem("Code", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	active.Occupancy = OccupancyParallel
	var items []Item
	items = append(items, active)
	for i := 0; i < 2; i++ {
		item := taskItem(fmt.Sprintf("Wait%d", i), &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
		item.Occupancy = OccupancyWaiting
		item.CreatedAt = msk(2026, 1, 1, 12, i+1).UTC()
		items = append(items, item)
	}
	got := BuildSchedule(items, nil, now, now, now.Add(24*time.Hour))
	blocks := laneNamed(got, "Alpha").Blocks
	if len(blocks) != 3 {
		t.Fatalf("blocks %d", len(blocks))
	}
	lanes := map[int]bool{}
	for _, block := range blocks {
		wantTime(t, "start", block.StartsAt, msk(2026, 9, 15, 10, 0))
		lanes[block.Lane] = true
	}
	if !lanes[0] || !lanes[1] || !lanes[2] {
		t.Fatalf("lanes %+v", lanes)
	}
}

func TestPlanTwoParallelSerialize(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	a := taskItem("Code A", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	a.Occupancy = OccupancyParallel
	b := taskItem("Code B", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	b.Occupancy = OccupancyParallel
	b.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{a, b}, nil, now, now, now.Add(24*time.Hour))
	wantTime(t, "b after a", firstBlock(t, got, b.ID).StartsAt, msk(2026, 9, 15, 11, 0))
}

func TestPlanThirdWaitingQueues(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	var items []Item
	for i := 0; i < 3; i++ {
		item := taskItem(fmt.Sprintf("Wait%d", i), &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
		item.Occupancy = OccupancyWaiting
		item.CreatedAt = msk(2026, 1, 1, 12, i).UTC()
		items = append(items, item)
	}
	got := BuildSchedule(items, nil, now, now, now.Add(24*time.Hour))
	late := 0
	for _, block := range laneNamed(got, "Alpha").Blocks {
		if block.StartsAt.Equal(msk(2026, 9, 15, 11, 0).UTC()) {
			late++
		}
		if block.Lane == 0 {
			t.Fatalf("waiting block on active track %+v", block)
		}
	}
	if late != 1 {
		t.Fatalf("queued blocks %d", late)
	}
}

func TestBuildScheduleParallelNeverOverlapsItself(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	item := taskItem("Long parallel", &pid, "Alpha", "#111", due, 4*3600, 0, StatusToDo)
	item.Occupancy = OccupancyParallel
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	blocks := laneNamed(got, "Alpha").Blocks
	if len(blocks) != 2 {
		t.Fatalf("blocks %+v", blocks)
	}
	wantTime(t, "first end", blocks[0].EndsAt, msk(2026, 9, 15, 12, 0))
	wantTime(t, "second start", blocks[1].StartsAt, msk(2026, 9, 15, 12, 10))
	if blocks[0].Lane != 0 || blocks[1].Lane != 0 {
		t.Fatalf("lanes %d %d", blocks[0].Lane, blocks[1].Lane)
	}
}

func TestBuildScheduleSoloExcludesParallel(t *testing.T) {
	due := msk(2026, 9, 20, 18, 0)
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	solo := taskItem("Solo", &pid, "Alpha", "#111", due, 2*3600, 0, StatusToDo)
	solo.Pinned = true
	par := taskItem("Par", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	par.Occupancy = OccupancyParallel
	got := BuildSchedule([]Item{solo, par}, nil, now, now, now.Add(24*time.Hour))
	wantTime(t, "solo start", firstBlock(t, got, solo.ID).StartsAt, msk(2026, 9, 15, 10, 0))
	wantTime(t, "parallel after solo", firstBlock(t, got, par.ID).StartsAt, firstBlock(t, got, solo.ID).EndsAt)
}

// #21: follow-ups become short pings in the check windows.
func TestPlanFollowupPings(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	reviewDue := msk(2026, 9, 17, 18, 0)
	item := taskItem("Review me", &pid, "Alpha", "#111", msk(2026, 9, 30, 18, 0), 8*3600, 0, StatusReview)
	item.ReviewDueAt = &reviewDue
	got := BuildKindSchedule(ScheduleFollowup, []Item{item}, nil, now, now, msk(2026, 9, 19, 0, 0))
	blocks := blocksOf(got, item.ID)
	if len(blocks) != 3 {
		t.Fatalf("pings %+v", blocks)
	}
	for i, block := range blocks {
		wantTime(t, fmt.Sprintf("ping %d", i), block.StartsAt, msk(2026, 9, 15+i, 10, 0))
		if block.Kind != BlockKindPing || block.EndsAt.Sub(block.StartsAt) != 20*time.Minute || block.Late {
			t.Fatalf("ping %+v", block)
		}
	}
}

func TestPlanFollowupLatePingsEveryDay(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 11, 0)
	due := msk(2026, 9, 14, 18, 0)
	item := taskItem("Overdue", &pid, "Alpha", "#111", msk(2026, 9, 30, 18, 0), 3600, 0, StatusAwaitingDecision)
	item.DueAt = &due
	got := BuildKindSchedule(ScheduleFollowup, []Item{item}, nil, now, msk(2026, 9, 15, 0, 0), msk(2026, 9, 17, 0, 0))
	blocks := blocksOf(got, item.ID)
	if len(blocks) != 2 {
		t.Fatalf("pings %+v", blocks)
	}
	wantTime(t, "today second window", blocks[0].StartsAt, msk(2026, 9, 15, 14, 0))
	if !blocks[0].Late || got.Overflow.ItemCount != 1 {
		t.Fatalf("late=%v overflow=%+v", blocks[0].Late, got.Overflow)
	}
}

func TestBuildFollowupScheduleMissingDueIsUnplanned(t *testing.T) {
	pid := uuid.New()
	item := taskItem("Wait", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 3600, 0, StatusAwaitingDecision)
	item.DueAt = nil
	now := msk(2026, 9, 15, 8, 0)
	got := BuildKindSchedule(ScheduleFollowup, []Item{item}, nil, now, now, now.Add(24*time.Hour))
	if len(got.Unplanned) != 1 || !got.Unplanned[0].MissingDevDue {
		t.Fatalf("%+v", got.Unplanned)
	}
}

// #22: pings wait for the people involved.
func TestPlanFollowupWaitsForPerson(t *testing.T) {
	pid := uuid.New()
	person := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 17, 18, 0)
	item := taskItem("Ask Ann", &pid, "Alpha", "#111", msk(2026, 9, 30, 18, 0), 3600, 0, StatusAwaitingDecision)
	item.DueAt = &due
	item.PersonIDs = []uuid.UUID{person}
	got := planWith([]Item{item}, []EventOccurrence{{
		SeriesID: uuid.New(), Title: "Ann 1:1", StartsAt: msk(2026, 9, 16, 10, 0), EndsAt: msk(2026, 9, 16, 10, 30),
		People: []PersonRel{{ID: person}},
	}}, now, now, msk(2026, 9, 18, 0, 0), func(in *ScheduleInput) {
		in.Kind = ScheduleFollowup
		in.PeopleNames = map[uuid.UUID]string{person: "Ann"}
		day := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
		in.Absences = []PersonAbsence{{ID: uuid.New(), PersonID: person, StartsOn: day, EndsOn: day}}
	})
	blocks := blocksOf(got, item.ID)
	if len(blocks) != 2 {
		t.Fatalf("pings %+v", blocks)
	}
	wantTime(t, "skips absence day, slides past 1:1", blocks[0].StartsAt, msk(2026, 9, 16, 10, 40))
	if !hasReason(blocks[0].Reasons, reasonPersonAway+":Ann") {
		t.Fatalf("absence reason %v", blocks[0].Reasons)
	}
}

func TestPlanActiveWindowFreesShoulders(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	start, end := 30*60, 90*60
	person := uuid.New()
	events := []EventOccurrence{{
		SeriesID:          uuid.New(),
		Title:             "Workshop",
		StartsAt:          msk(2026, 9, 15, 10, 0),
		EndsAt:            msk(2026, 9, 15, 12, 0),
		ActiveStartOffset: &start,
		ActiveEndOffset:   &end,
		People:            []PersonRel{{ID: person}},
	}}
	item := taskItem("Around active", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 90*60, 0, StatusToDo)
	got := planWith([]Item{item}, events, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.PeopleNames = map[uuid.UUID]string{person: "Ann"}
	})
	block := firstBlock(t, got, item.ID)
	wantTime(t, "after active core", block.StartsAt, msk(2026, 9, 15, 11, 40))
	if len(got.Busy) != 1 {
		t.Fatalf("busy %+v", got.Busy)
	}
	busy := got.Busy[0]
	wantTime(t, "visual start", busy.StartsAt, msk(2026, 9, 15, 10, 0))
	wantTime(t, "visual end", busy.EndsAt, msk(2026, 9, 15, 12, 0))
	if busy.ActiveStartsAt == nil || busy.ActiveEndsAt == nil {
		t.Fatalf("missing active bounds %+v", busy)
	}
	wantTime(t, "active start", *busy.ActiveStartsAt, msk(2026, 9, 15, 10, 30))
	wantTime(t, "active end", *busy.ActiveEndsAt, msk(2026, 9, 15, 11, 30))
	if !busy.CanSkip && len(busy.People) != 1 {
		t.Fatalf("people %+v", busy.People)
	}
	if busy.People[0].Name != "Ann" {
		t.Fatalf("people %+v", busy.People)
	}
}

func TestPlanWorkPacksThroughSkippablePersonMeeting(t *testing.T) {
	pid := uuid.New()
	person := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	events := []EventOccurrence{{
		SeriesID: uuid.New(), Title: "Optional", CanSkip: true,
		StartsAt: msk(2026, 9, 15, 10, 0), EndsAt: msk(2026, 9, 15, 19, 0),
		People: []PersonRel{{ID: person}},
	}}
	item := taskItem("Due today", &pid, "Alpha", "#111", msk(2026, 9, 15, 18, 0), 2*3600, 0, StatusToDo)
	item.PersonIDs = []uuid.UUID{person}
	got := planWith([]Item{item}, events, now, now, now.Add(48*time.Hour), func(in *ScheduleInput) {
		in.PeopleNames = map[uuid.UUID]string{person: "Ann"}
	})
	block := firstBlock(t, got, item.ID)
	wantTime(t, "into skippable meeting", block.StartsAt, msk(2026, 9, 15, 10, 0))
	if len(block.People) != 1 || block.People[0].Name != "Ann" {
		t.Fatalf("people %+v", block.People)
	}
}

func TestPlanWorkWaitsForHardPersonMeeting(t *testing.T) {
	pid := uuid.New()
	person := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	events := []EventOccurrence{{
		SeriesID: uuid.New(), Title: "Ann 1:1",
		StartsAt: msk(2026, 9, 15, 10, 0), EndsAt: msk(2026, 9, 15, 12, 0),
		People: []PersonRel{{ID: person}},
	}}
	item := taskItem("Needs Ann", &pid, "Alpha", "#111", msk(2026, 9, 20, 18, 0), 3600, 0, StatusToDo)
	item.PersonIDs = []uuid.UUID{person}
	got := planWith([]Item{item}, events, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.PeopleNames = map[uuid.UUID]string{person: "Ann"}
	})
	block := firstBlock(t, got, item.ID)
	wantTime(t, "after hard meeting", block.StartsAt, msk(2026, 9, 15, 12, 10))
	if !hasReason(block.Reasons, reasonPersonBusy+":Ann") {
		t.Fatalf("reasons %v", block.Reasons)
	}
}

func TestPlanFollowupSkipsSoftPersonBusy(t *testing.T) {
	pid := uuid.New()
	person := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 15, 18, 0)
	item := taskItem("Ask Ann", &pid, "Alpha", "#111", msk(2026, 9, 30, 18, 0), 3600, 0, StatusAwaitingDecision)
	item.DueAt = &due
	item.PersonIDs = []uuid.UUID{person}
	got := planWith([]Item{item}, []EventOccurrence{{
		SeriesID: uuid.New(), Title: "Optional 1:1", CanSkip: true,
		StartsAt: msk(2026, 9, 15, 10, 0), EndsAt: msk(2026, 9, 15, 10, 30),
		People: []PersonRel{{ID: person}},
	}}, now, now, msk(2026, 9, 16, 0, 0), func(in *ScheduleInput) {
		in.Kind = ScheduleFollowup
		in.PeopleNames = map[uuid.UUID]string{person: "Ann"}
	})
	block := firstBlock(t, got, item.ID)
	wantTime(t, "ping during skippable", block.StartsAt, msk(2026, 9, 15, 10, 0))
	if hasReasonPrefix(block, reasonPersonBusy) {
		t.Fatalf("skippable tagged busy %v", block.Reasons)
	}
}

// #23: low energy mornings take light tasks first.
func TestPlanLowEnergyMorning(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 20, 18, 0)
	heavy := taskItem("Heavy", &pid, "Alpha", "#111", due, 3*3600, 0, StatusToDo)
	light := taskItem("Light", &pid, "Alpha", "#111", due, 30*60, 0, StatusToDo)
	light.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := planWith([]Item{heavy, light}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Checkins = &LatestCheckins{Stress: 3, Focus: 3, Energy: 1, Interest: 3}
	})
	wantTime(t, "light in the morning", firstBlock(t, got, light.ID).StartsAt, msk(2026, 9, 15, 10, 0))
	wantTime(t, "heavy after zone", firstBlock(t, got, heavy.ID).StartsAt, msk(2026, 9, 15, 12, 0))
	if !hasReason(firstBlock(t, got, light.ID).Reasons, reasonLight) {
		t.Fatalf("reasons %v", firstBlock(t, got, light.ID).Reasons)
	}
	got = planWith([]Item{heavy, light}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Checkins = &LatestCheckins{Stress: 3, Focus: 4, Energy: 4, Interest: 3}
	})
	wantTime(t, "heavy first when fresh", firstBlock(t, got, heavy.ID).StartsAt, msk(2026, 9, 15, 10, 0))
}

// #24: Do tasks move into historically productive hours.
func TestPlanGoldenHours(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	item := taskItem("Important", &pid, "Alpha", "#111", msk(2026, 9, 30, 18, 0), 3600, 0, StatusToDo)
	item.Urgent, item.Important = true, true
	var history []TimeInterval
	for d := 1; d <= 20; d++ {
		start := msk(2026, 8, d, 15, 0)
		end := start.Add(2 * time.Hour)
		history = append(history, TimeInterval{ID: uuid.New(), ItemID: item.ID, StartedAt: start, EndedAt: &end})
	}
	got := planWith([]Item{item}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.History = history
	})
	block := firstBlock(t, got, item.ID)
	wantTime(t, "golden start", block.StartsAt, msk(2026, 9, 15, 15, 0))
	if !hasReason(block.Reasons, reasonGolden) {
		t.Fatalf("reasons %v", block.Reasons)
	}
	got = planWith([]Item{item}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.History = history
		in.Settings.GoldenHours = false
	})
	wantTime(t, "plain start", firstBlock(t, got, item.ID).StartsAt, msk(2026, 9, 15, 10, 0))
}

// #25: no more than N different tasks per day.
func TestPlanMaxTasksPerDay(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 30, 18, 0)
	var items []Item
	for i := 0; i < 3; i++ {
		item := taskItem(fmt.Sprintf("T%d", i), &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
		item.CreatedAt = msk(2026, 1, 1, 12, i).UTC()
		items = append(items, item)
	}
	got := planWith(items, nil, now, now, now.Add(72*time.Hour), func(in *ScheduleInput) {
		in.Settings.MaxTasksPerDay = 2
	})
	wantTime(t, "third task tomorrow", firstBlock(t, got, items[2].ID).StartsAt, msk(2026, 9, 16, 10, 0))
}

// #26: the target lead makes tasks at risk a day before their due date.
func TestPlanTargetLeadFlagsRisk(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	item := taskItem("Tomorrow morning", &pid, "Alpha", "#111", msk(2026, 9, 16, 10, 0), 3*3600, 0, StatusToDo)
	got := BuildSchedule([]Item{item}, nil, now, now, now.Add(24*time.Hour))
	if len(got.AtRisk) != 1 || got.AtRisk[0].ItemID != item.ID {
		t.Fatalf("atRisk %+v", got.AtRisk)
	}
	got = planWith([]Item{item}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Settings.TargetLeadWorkdays = 0
	})
	if len(got.AtRisk) != 0 {
		t.Fatalf("atRisk without lead %+v", got.AtRisk)
	}
}

// #27: a block keeps its previous place unless moving gains the threshold.
func TestPlanStickyKeepsPrevious(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 20, 18, 0)
	a := taskItem("A", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	b := taskItem("B", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	b.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	previous := []ScheduleBlock{{ItemID: b.ID, Kind: BlockKindWork, StartsAt: msk(2026, 9, 15, 11, 20).UTC(), EndsAt: msk(2026, 9, 15, 12, 20).UTC()}}
	got := planWith([]Item{a, b}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Previous = previous
	})
	block := firstBlock(t, got, b.ID)
	wantTime(t, "sticky", block.StartsAt, msk(2026, 9, 15, 11, 20))
	if !hasReason(block.Reasons, reasonSticky) {
		t.Fatalf("reasons %v", block.Reasons)
	}
	far := []ScheduleBlock{{ItemID: b.ID, Kind: BlockKindWork, StartsAt: msk(2026, 9, 16, 10, 0).UTC(), EndsAt: msk(2026, 9, 16, 11, 0).UTC()}}
	got = planWith([]Item{a, b}, nil, now, now, now.Add(24*time.Hour), func(in *ScheduleInput) {
		in.Previous = far
	})
	wantTime(t, "moved when gain is large", firstBlock(t, got, b.ID).StartsAt, msk(2026, 9, 15, 11, 0))
}

// #28: a block names the higher-ranked block that pushed it.
func TestPlanPushedBy(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 20, 18, 0)
	a := taskItem("MB-1", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	b := taskItem("MB-2", &pid, "Alpha", "#111", due, 3600, 0, StatusToDo)
	b.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{a, b}, nil, now, now, now.Add(24*time.Hour))
	if !hasReason(firstBlock(t, got, b.ID).Reasons, reasonPushedBy+":MB-1") {
		t.Fatalf("reasons %v", firstBlock(t, got, b.ID).Reasons)
	}
}

// #29: what-if reports the delta of a scenario.
func TestWhatIfScenarios(t *testing.T) {
	pid := uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	late := taskItem("Late", &pid, "Alpha", "#111", msk(2026, 9, 15, 9, 0), 2*3600, 0, StatusToDo)
	in := ScheduleInput{Kind: ScheduleWork, Items: []Item{late}, Now: now, From: now, To: now.Add(24 * time.Hour), Settings: DefaultScheduleSettings()}
	got := WhatIf(in, WhatIfScenario{DropItems: []uuid.UUID{late.ID}})
	if got.Base.LateSeconds <= 0 || got.Variant.LateSeconds != 0 || got.Delta.LateSeconds >= 0 {
		t.Fatalf("%+v", got)
	}
	got = WhatIf(in, WhatIfScenario{MoveDue: []WhatIfMove{{ItemID: late.ID, DueAt: msk(2026, 9, 25, 18, 0)}}})
	if got.Variant.LateSeconds != 0 || got.Variant.AtRisk != 0 {
		t.Fatalf("%+v", got)
	}
	meeting := uuid.New()
	in.Events = []EventOccurrence{{SeriesID: meeting, Title: "Blocker", StartsAt: msk(2026, 9, 15, 10, 0), EndsAt: msk(2026, 9, 15, 19, 0)}}
	in.Items = []Item{taskItem("Today", &pid, "Alpha", "#111", msk(2026, 9, 15, 18, 0), 3600, 0, StatusToDo)}
	got = WhatIf(in, WhatIfScenario{SkipEvents: []uuid.UUID{meeting}})
	if got.Base.LateSeconds <= 0 || got.Variant.LateSeconds != 0 {
		t.Fatalf("%+v", got)
	}
}

// #30: the score counts fragments, switches and late time.
func TestScoreCountsFragmentsAndSwitches(t *testing.T) {
	x, y := uuid.New(), uuid.New()
	now := msk(2026, 9, 15, 8, 0)
	due := msk(2026, 9, 20, 18, 0)
	long := taskItem("Long", &x, "X", "#111", due, 4*3600, 0, StatusToDo)
	other := taskItem("Other", &y, "Y", "#222", due, 3600, 0, StatusToDo)
	other.CreatedAt = msk(2026, 1, 2, 12, 0).UTC()
	got := BuildSchedule([]Item{long, other}, nil, now, now, now.Add(24*time.Hour))
	if got.Score.Fragments != 1 || got.Score.Switches != 1 || got.Score.LateSeconds != 0 {
		t.Fatalf("score %+v", got.Score)
	}
	if got.Score.Total != 1.5 {
		t.Fatalf("total %v", got.Score.Total)
	}
}

func TestScheduleSettingsValidate(t *testing.T) {
	ok := DefaultScheduleSettings()
	if err := ok.Validate(); err != nil {
		t.Fatalf("defaults invalid: %v", err)
	}
	bad := DefaultScheduleSettings()
	bad.Break = &BreakWindow{StartMin: 9 * 60, EndMin: 10 * 60}
	if err := bad.Validate(); err == nil {
		t.Fatal("break outside window accepted")
	}
	bad = DefaultScheduleSettings()
	bad.CheckWindows = []string{"25:00"}
	if err := bad.Validate(); err == nil {
		t.Fatal("bad clock accepted")
	}
	bad = DefaultScheduleSettings()
	bad.Timezone = "Mars/Olympus"
	if err := bad.Validate(); err == nil {
		t.Fatal("bad timezone accepted")
	}
}

func laneNamed(got Schedule, name string) ScheduleLane {
	for _, lane := range got.Lanes {
		if lane.ProjectName == name {
			return lane
		}
	}
	panic(name)
}

func hasBusyTitle(got Schedule, title string) bool {
	for _, row := range got.Busy {
		if row.Title == title {
			return true
		}
	}
	return false
}
