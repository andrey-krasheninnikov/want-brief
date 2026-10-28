package domain

import (
	"testing"
	"time"
)

func TestParseCheckinKind(t *testing.T) {
	if _, err := ParseCheckinKind("mood"); err == nil {
		t.Fatal("expected invalid kind")
	}
	kind, err := ParseCheckinKind("focus")
	if err != nil || kind != CheckinFocus {
		t.Fatalf("got %s %v", kind, err)
	}
	kind, err = ParseCheckinKind("happiness")
	if err != nil || kind != CheckinHappiness {
		t.Fatalf("got %s %v", kind, err)
	}
}

func TestLatestFromLogsPrefersFirstPerKind(t *testing.T) {
	logs := []StressLog{
		{Kind: CheckinFocus, Level: 5},
		{Kind: CheckinFocus, Level: 1},
		{Kind: CheckinEnergy, Level: 2},
	}
	got := LatestFromLogs(logs)
	if got.Focus != 5 || got.Energy != 2 || got.Stress != 3 || got.Happiness != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestNewCheckinRejectsLevel(t *testing.T) {
	_, err := NewCheckin(CheckinStress, 9, nil, time.Time{})
	if err == nil {
		t.Fatal("expected invalid level")
	}
}
