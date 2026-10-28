package domain

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/schedule/*.golden.json")

// goldenBlock is the stable, human-readable projection of a block that the
// golden files store. Times are local to the schedule timezone.
type goldenBlock struct {
	Key      string   `json:"key"`
	Kind     string   `json:"kind"`
	Start    string   `json:"start"`
	End      string   `json:"end"`
	Lane     int      `json:"lane"`
	Late     bool     `json:"late,omitempty"`
	Reasons  []string `json:"reasons,omitempty"`
	Continue string   `json:"continue,omitempty"`
}

type goldenOutput struct {
	Blocks    []goldenBlock    `json:"blocks"`
	Unplanned []string         `json:"unplanned"`
	AtRisk    []string         `json:"atRisk"`
	Busy      []string         `json:"busy"`
	Overflow  ScheduleOverflow `json:"overflow"`
	Score     ScheduleScore    `json:"score"`
}

func stableID(name string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("want-brief/schedule/"+name))
}

func goldenItem(key, project, color string, plan int, due time.Time, occupancy Occupancy, status ItemStatus) Item {
	pid := stableID("project/" + project)
	item := Item{
		ID:             stableID("item/" + key),
		Title:          key + " " + project,
		ExternalKey:    key,
		Status:         status,
		Kind:           KindTask,
		Occupancy:      occupancy,
		ProjectID:      &pid,
		ProjectName:    project,
		ProjectColor:   color,
		DevDueAt:       &due,
		PlannedSeconds: plan,
		CreatedAt:      msk(2026, 9, 1, 12, 0).UTC(),
	}
	return item.WithQuadrant()
}

func goldenEvent(title string, start time.Time, minutes int, canSkip bool, people ...uuid.UUID) EventOccurrence {
	event := EventOccurrence{
		SeriesID: stableID("event/" + title),
		Title:    title,
		StartsAt: start,
		EndsAt:   start.Add(time.Duration(minutes) * time.Minute),
		CanSkip:  canSkip,
	}
	for _, id := range people {
		event.People = append(event.People, PersonRel{ID: id})
	}
	return event
}

func goldenCases() map[string]ScheduleInput {
	now := msk(2026, 9, 15, 9, 30)
	from := msk(2026, 9, 14, 0, 0)
	to := msk(2026, 9, 21, 0, 0)
	ann := stableID("person/ann")
	bob := stableID("person/bob")
	base := func() ScheduleInput {
		return ScheduleInput{Kind: ScheduleWork, Now: now, From: from, To: to, Settings: DefaultScheduleSettings()}
	}

	current := base()
	current.Items = []Item{
		goldenItem("HB-101", "Hamkor", "#0a7", 6*3600, msk(2026, 9, 18, 18, 0), OccupancyParallel, StatusInProgress),
		goldenItem("HB-102", "Hamkor", "#0a7", 4*3600, msk(2026, 9, 18, 18, 0), OccupancyParallel, StatusToDo),
		goldenItem("HB-103", "Hamkor", "#0a7", 3*3600, msk(2026, 9, 17, 18, 0), OccupancyWaiting, StatusToDo),
		goldenItem("AL-7", "Alpha", "#c33", 5*3600, msk(2026, 9, 17, 18, 0), OccupancySolo, StatusToDo),
		goldenItem("AL-9", "Alpha", "#c33", 2*3600, msk(2026, 9, 25, 18, 0), OccupancySolo, StatusToDo),
	}
	current.Items[0].TrackedSeconds = 3600
	current.Items[4].Important = true
	current.Events = []EventOccurrence{
		goldenEvent("Sprint review", msk(2026, 9, 16, 12, 0), 60, false),
		goldenEvent("Optional sync", msk(2026, 9, 17, 15, 0), 60, true),
	}
	hamkor := stableID("project/Hamkor")
	current.Factors = map[string]float64{laneKey(&hamkor): 1.25}

	overloaded := base()
	for i, key := range []string{"OV-1", "OV-2", "OV-3", "OV-4", "OV-5"} {
		item := goldenItem(key, "Overload", "#555", 8*3600, msk(2026, 9, 16, 18, 0), OccupancySolo, StatusToDo)
		item.CreatedAt = item.CreatedAt.Add(time.Duration(i) * time.Hour)
		overloaded.Items = append(overloaded.Items, item)
	}
	stress := 5
	overloaded.Items[2].Stress = &stress
	overloaded.Items[3].Urgent = true

	followup := base()
	followup.Kind = ScheduleFollowup
	followup.PeopleNames = map[uuid.UUID]string{ann: "Ann", bob: "Bob"}
	review := goldenItem("FU-1", "Hamkor", "#0a7", 3600, msk(2026, 9, 30, 18, 0), OccupancySolo, StatusReview)
	reviewDue := msk(2026, 9, 17, 18, 0)
	review.ReviewDueAt = &reviewDue
	waiting := goldenItem("FU-2", "Alpha", "#c33", 3600, msk(2026, 9, 30, 18, 0), OccupancySolo, StatusAwaitingDecision)
	waitingDue := msk(2026, 9, 16, 18, 0)
	waiting.DueAt = &waitingDue
	waiting.PersonIDs = []uuid.UUID{ann}
	overdue := goldenItem("FU-3", "Alpha", "#c33", 3600, msk(2026, 9, 30, 18, 0), OccupancySolo, StatusAwaitingDecision)
	overdueDue := msk(2026, 9, 14, 18, 0)
	overdue.DueAt = &overdueDue
	overdue.PersonIDs = []uuid.UUID{bob}
	followup.Items = []Item{review, waiting, overdue}
	followup.Events = []EventOccurrence{goldenEvent("Ann 1:1", msk(2026, 9, 16, 10, 0), 30, false, ann)}
	followup.Absences = []PersonAbsence{{
		ID: stableID("absence/bob"), PersonID: bob,
		StartsOn: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
		EndsOn:   time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC),
	}}

	overrides := base()
	overrides.Items = []Item{
		goldenItem("OR-1", "Alpha", "#c33", 4*3600, msk(2026, 9, 18, 18, 0), OccupancySolo, StatusToDo),
		goldenItem("OR-2", "Beta", "#36c", 3*3600, msk(2026, 9, 18, 18, 0), OccupancySolo, StatusToDo),
	}
	pin := msk(2026, 9, 17, 14, 0)
	overrides.Items[1].PinnedAt = &pin
	short, long := 12*60, 16*60
	overrides.Overrides = []DayOverride{
		{Day: "2026-09-15", Off: true, Note: "Day off"},
		{Day: "2026-09-16", WorkStartMin: &short, WorkEndMin: &long},
	}
	overrides.Settings.Break = &BreakWindow{StartMin: 13 * 60, EndMin: 14 * 60}
	overrides.Checkins = &LatestCheckins{Stress: 3, Focus: 2, Energy: 2, Interest: 3}

	return map[string]ScheduleInput{
		"current_week":       current,
		"overloaded_week":    overloaded,
		"followup_week":      followup,
		"overrides_absences": overrides,
	}
}

func goldenProjection(got Schedule) goldenOutput {
	loc := Moscow()
	stamp := func(t time.Time) string { return t.In(loc).Format("Mon 02 15:04") }
	out := goldenOutput{Overflow: got.Overflow, Score: got.Score, Blocks: []goldenBlock{}, Unplanned: []string{}, AtRisk: []string{}, Busy: []string{}}
	for _, block := range allLaneBlocks(got) {
		g := goldenBlock{Key: block.ExternalKey, Kind: block.Kind, Start: stamp(block.StartsAt), End: stamp(block.EndsAt), Lane: block.Lane, Late: block.Late, Reasons: block.Reasons}
		switch {
		case block.Continued && block.Continues:
			g.Continue = "both"
		case block.Continued:
			g.Continue = "from"
		case block.Continues:
			g.Continue = "to"
		}
		out.Blocks = append(out.Blocks, g)
	}
	for _, item := range got.Unplanned {
		out.Unplanned = append(out.Unplanned, item.Item.ExternalKey)
	}
	for _, risk := range got.AtRisk {
		out.AtRisk = append(out.AtRisk, risk.Key)
	}
	for _, busy := range got.Busy {
		out.Busy = append(out.Busy, busy.Title+" "+stamp(busy.StartsAt)+"-"+stamp(busy.EndsAt))
	}
	return out
}

func TestScheduleGolden(t *testing.T) {
	dir := filepath.Join("testdata", "schedule")
	for name, input := range goldenCases() {
		t.Run(name, func(t *testing.T) {
			got := goldenProjection(Plan(input))
			body, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			body = append(body, '\n')
			path := filepath.Join(dir, name+".golden.json")
			if *updateGolden {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, body, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v (run with -update)", err)
			}
			if !bytes.Equal(want, body) {
				t.Fatalf("golden mismatch for %s (run with -update)\n--- want\n%s\n--- got\n%s", name, want, body)
			}
		})
	}
}
