package domain

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestNewEarnDay(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	row, err := NewEarnDay("2026-09-17", 100, 1000, 85, now)
	if err != nil {
		t.Fatal(err)
	}
	if row.Day != "2026-09-17" || row.USD != 100 || row.RUB != 1000 || row.Rate != 85 || !row.UpdatedAt.Equal(now) {
		t.Fatalf("got %+v", row)
	}

	if _, err := NewEarnDay("2026-09-17", 0, 0, 85, now); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEarnDay("nope", 1, 1, 85, now); err == nil {
		t.Fatal("expected invalid date")
	}
	if _, err := NewEarnDay("2026-09-17", -1, 0, 85, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("neg usd: %v", err)
	}
	if _, err := NewEarnDay("2026-09-17", 0, math.NaN(), 85, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nan rub: %v", err)
	}
	if _, err := NewEarnDay("2026-09-17", 0, 0, 0, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zero rate: %v", err)
	}
}
