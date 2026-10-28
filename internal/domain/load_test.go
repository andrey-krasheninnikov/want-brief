package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCheckinAveragesFrom(t *testing.T) {
	got := CheckinAveragesFrom([]StressLog{
		{Kind: CheckinStress, Level: 2},
		{Kind: CheckinStress, Level: 4},
		{Kind: CheckinFocus, Level: 5},
		{Kind: CheckinHappiness, Level: 2},
		{Kind: CheckinHappiness, Level: 4},
		{Kind: "", Level: 1},
	})
	if got.Stress == nil || *got.Stress != 7.0/3.0 {
		t.Fatalf("stress=%v", got.Stress)
	}
	if got.Focus == nil || *got.Focus != 5 {
		t.Fatalf("focus=%v", got.Focus)
	}
	if got.Happiness == nil || *got.Happiness != 3 {
		t.Fatalf("happiness=%v", got.Happiness)
	}
	if got.Energy != nil || got.Interest != nil {
		t.Fatalf("empty kinds should stay nil")
	}
}

func TestLoadByDaySplitsMoscowMidnight(t *testing.T) {
	loc := Moscow()
	from := time.Date(2026, 9, 14, 22, 0, 0, 0, loc)
	to := time.Date(2026, 9, 15, 2, 0, 0, 0, loc)
	end := to
	intervals := []TimeInterval{{
		ItemID:    uuid.New(),
		StartedAt: from,
		EndedAt:   &end,
	}}
	days := LoadByDay(intervals, from, to, to, loc)
	if len(days) != 2 {
		t.Fatalf("days=%d %+v", len(days), days)
	}
	if days[0].Date != "2026-09-14" || days[0].AllocatedSeconds != 2*3600 {
		t.Fatalf("day0=%+v", days[0])
	}
	if days[1].Date != "2026-09-15" || days[1].AllocatedSeconds != 2*3600 {
		t.Fatalf("day1=%+v", days[1])
	}
}

func TestLoadByDayAtDSTBoundary(t *testing.T) {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	end := to
	rows := LoadByDay([]TimeInterval{{ItemID: uuid.New(), StartedAt: from, EndedAt: &end}}, from, to, to, loc)
	if len(rows) != 2 || rows[0].Date != "2026-03-29" || rows[0].AllocatedSeconds != 23*3600 || rows[1].AllocatedSeconds != 3600 {
		t.Fatalf("spring load=%+v", rows)
	}
	from = time.Date(2026, 10, 24, 23, 0, 0, 0, time.UTC)
	to = time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	end = to
	rows = LoadByDay([]TimeInterval{{ItemID: uuid.New(), StartedAt: from, EndedAt: &end}}, from, to, to, loc)
	if len(rows) != 1 || rows[0].Date != "2026-10-25" || rows[0].AllocatedSeconds != 25*3600 {
		t.Fatalf("autumn load=%+v", rows)
	}
}
