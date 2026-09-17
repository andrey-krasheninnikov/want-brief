package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseItemEventKind(t *testing.T) {
	if _, err := ParseItemEventKind("status"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseItemEventKind("nope"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestNewItemEventRejectsNilItem(t *testing.T) {
	if _, err := NewItemEvent(uuid.Nil, ItemEventStatus, "status", "to_do", "done", time.Time{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestItemEventAnnotate(t *testing.T) {
	event, err := NewItemEvent(uuid.New(), ItemEventStatus, "status", "to_do", "done", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	level := 4
	got, err := event.Annotate("  shipped  ", &level)
	if err != nil {
		t.Fatal(err)
	}
	if got.Note != "shipped" || got.Stress == nil || *got.Stress != 4 {
		t.Fatalf("%+v", got)
	}
	if event.Note != "" || event.Stress != nil {
		t.Fatal("mutated source")
	}
}

func TestItemEventAnnotateRejectsStress(t *testing.T) {
	event, err := NewItemEvent(uuid.New(), ItemEventTimerStop, "", "", "60", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	level := 9
	if _, err := event.Annotate("", &level); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}
