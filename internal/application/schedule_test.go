package application

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pchkauu/want-brief/internal/domain"
)

func TestApplyOpenTrackAddsElapsed(t *testing.T) {
	id := uuid.New()
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	items := []domain.Item{{ID: id, TrackedSeconds: 60}}
	open := []domain.TimeInterval{{ItemID: id, StartedAt: now.Add(-90 * time.Second)}}
	got := applyOpenTrack(items, open, now)
	if got[0].TrackedSeconds != 150 {
		t.Fatalf("tracked %d", got[0].TrackedSeconds)
	}
	if items[0].TrackedSeconds != 60 {
		t.Fatalf("mutated source %d", items[0].TrackedSeconds)
	}
}
