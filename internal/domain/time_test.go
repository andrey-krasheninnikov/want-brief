package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAllocatedAndWallSeconds(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	from := now.Add(-2 * time.Hour)
	to := now
	itemA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	itemB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	endA := now.Add(-30 * time.Minute)
	endB := now.Add(-20 * time.Minute)
	intervals := []TimeInterval{
		{ItemID: itemA, StartedAt: now.Add(-90 * time.Minute), EndedAt: &endA},
		{ItemID: itemB, StartedAt: now.Add(-70 * time.Minute), EndedAt: &endB},
	}

	allocated := AllocatedSeconds(intervals, from, to, now)
	wall := WallSeconds(intervals, from, to, now)

	if allocated != 3600+3000 {
		t.Fatalf("allocated=%d", allocated)
	}
	if wall != 70*60 {
		t.Fatalf("wall=%d want %d", wall, 70*60)
	}
}

func TestNewClosedInterval(t *testing.T) {
	start := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	end := start.Add(45 * time.Minute)
	interval, err := NewClosedInterval(uuid.New(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if interval.EndedAt == nil || !interval.EndedAt.Equal(end) {
		t.Fatalf("ended=%v", interval.EndedAt)
	}
	if _, err := NewClosedInterval(uuid.New(), end, start); err == nil {
		t.Fatal("expected invalid")
	}
}

func TestStopInterval(t *testing.T) {
	start := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	interval := NewOpenInterval(uuid.New(), start)
	stopped, err := interval.Stop(start.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if stopped.EndedAt == nil {
		t.Fatal("expected end")
	}
	_, err = stopped.Stop(start.Add(2 * time.Minute))
	if err == nil {
		t.Fatal("expected conflict")
	}
}
