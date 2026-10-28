package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewEventRejectsEmptyTitle(t *testing.T) {
	_, err := NewEvent("  ", EventKindCall, time.Now(), nil)
	if err == nil {
		t.Fatal("expected invalid title")
	}
}

func TestNewEventRejectsZeroStart(t *testing.T) {
	_, err := NewEvent("Standup", EventKindCall, time.Time{}, nil)
	if err == nil {
		t.Fatal("expected invalid startsAt")
	}
}

func TestNewEventFromRejectsInvolvement(t *testing.T) {
	_, err := NewEventFrom(EventDraft{
		Title:           "Sync",
		Kind:            EventKindCall,
		Type:            EventTypeSync,
		StartsAt:        time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC),
		DurationSeconds: 1800,
		Involvement:     11,
	})
	if err == nil {
		t.Fatal("expected invalid involvement")
	}
}

func TestEventOccurrenceActiveSpan(t *testing.T) {
	start := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	event := EventOccurrence{StartsAt: start, EndsAt: start.Add(2 * time.Hour)}
	gotStart, gotEnd := event.ActiveSpan()
	if !gotStart.Equal(start) || !gotEnd.Equal(event.EndsAt) {
		t.Fatalf("full span %s %s", gotStart, gotEnd)
	}
	from, to := 30*60, 90*60
	event.ActiveStartOffset, event.ActiveEndOffset = &from, &to
	gotStart, gotEnd = event.ActiveSpan()
	if !gotStart.Equal(start.Add(30*time.Minute)) || !gotEnd.Equal(start.Add(90*time.Minute)) {
		t.Fatalf("active span %s %s", gotStart, gotEnd)
	}
}

func TestNewEventFromRejectsActiveOutsideDuration(t *testing.T) {
	start, end := 0, 1900
	_, err := NewEventFrom(EventDraft{
		Title:             "Sync",
		Kind:              EventKindCall,
		Type:              EventTypeSync,
		StartsAt:          time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC),
		DurationSeconds:   1800,
		ActiveStartOffset: &start,
		ActiveEndOffset:   &end,
	})
	if err == nil {
		t.Fatal("expected invalid active interval")
	}
}

func TestExpandOnceStaysSingle(t *testing.T) {
	start := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	event, err := NewEventFrom(EventDraft{
		Title:           "Retro",
		Kind:            EventKindEvent,
		Type:            EventTypeRetro,
		StartsAt:        start,
		DurationSeconds: 3600,
		Recurrence:      RecurrenceOnce,
	})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	got := event.Expand(from, to)
	if len(got) != 1 || !got[0].StartsAt.Equal(start) {
		t.Fatalf("got %+v", got)
	}
}

func TestExpandWeeklyHitsWeekdays(t *testing.T) {
	start := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC) // Wednesday
	event, err := NewEventFrom(EventDraft{
		Title:           "Grooming",
		Kind:            EventKindCall,
		Type:            EventTypeGrooming,
		StartsAt:        start,
		DurationSeconds: 1800,
		Recurrence:      RecurrenceWeekly,
		Weekdays:        []int{2, 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	got := event.Expand(from, to)
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
	if !got[0].StartsAt.Equal(start) || !got[1].StartsAt.Equal(start.AddDate(0, 0, 7)) {
		t.Fatalf("got %v %v", got[0].StartsAt, got[1].StartsAt)
	}
	if got[0].OriginalOn != "2026-09-16" || got[1].OriginalOn != "2026-09-23" {
		t.Fatalf("originalOn %s %s", got[0].OriginalOn, got[1].OriginalOn)
	}
}

func TestNewEventFromRejectsWeeklyWithoutWeekdays(t *testing.T) {
	_, err := NewEventFrom(EventDraft{
		Title:           "Daily",
		Kind:            EventKindCall,
		Type:            EventTypeDaily,
		StartsAt:        time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC),
		DurationSeconds: 1800,
		Recurrence:      RecurrenceWeekly,
	})
	if err == nil {
		t.Fatal("expected invalid weekdays")
	}
}

func TestExpandWeeklyFortnightSkipsWeekTwo(t *testing.T) {
	start := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	event, err := NewEventFrom(EventDraft{
		Title:           "Grooming",
		Kind:            EventKindCall,
		Type:            EventTypeGrooming,
		StartsAt:        start,
		DurationSeconds: 1800,
		Recurrence:      RecurrenceWeekly,
		Weekdays:        []int{2},
	})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	got := event.Expand(from, to)
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
	if !got[0].StartsAt.Equal(start) || !got[1].StartsAt.Equal(start.AddDate(0, 0, 14)) {
		t.Fatalf("got %v %v", got[0].StartsAt, got[1].StartsAt)
	}
}

func TestExpandWeeklyRespectsUntil(t *testing.T) {
	start := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	until := Ymd("2026-09-16")
	event, err := NewEventFrom(EventDraft{
		Title:           "Grooming",
		Kind:            EventKindCall,
		Type:            EventTypeGrooming,
		StartsAt:        start,
		DurationSeconds: 1800,
		Recurrence:      RecurrenceWeekly,
		Weekdays:        []int{2, 9},
		RepeatUntil:     &until,
	})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	got := event.Expand(from, to)
	if len(got) != 1 || !got[0].StartsAt.Equal(start) {
		t.Fatalf("got %+v", got)
	}
}

func TestExpandMonthlyHitsNextMonth(t *testing.T) {
	start := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	event, err := NewEventFrom(EventDraft{
		Title:           "Planning",
		Kind:            EventKindEvent,
		Type:            EventTypePlanning,
		StartsAt:        start,
		DurationSeconds: 3600,
		Recurrence:      RecurrenceMonthly,
	})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	got := event.Expand(from, to)
	if len(got) != 1 || got[0].StartsAt.Day() != 15 || got[0].StartsAt.Month() != time.September {
		t.Fatalf("got %+v", got)
	}
}

func TestExpandMonthlyRespectsUntil(t *testing.T) {
	start := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	until := Ymd("2026-08-31")
	event, err := NewEventFrom(EventDraft{
		Title:           "Planning",
		Kind:            EventKindEvent,
		Type:            EventTypePlanning,
		StartsAt:        start,
		DurationSeconds: 3600,
		Recurrence:      RecurrenceMonthly,
		RepeatUntil:     &until,
	})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	got := event.Expand(from, to)
	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeOccurrencesSkipsAndMoves(t *testing.T) {
	start := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	event, err := NewEventFrom(EventDraft{
		Title:           "Daily",
		Kind:            EventKindCall,
		Type:            EventTypeDaily,
		StartsAt:        start,
		DurationSeconds: 1800,
		Recurrence:      RecurrenceWeekly,
		Weekdays:        []int{2, 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	occs := event.Expand(from, to)
	moved := start.Add(2 * time.Hour)
	ovMove, err := NewEventOverride(event.ID, "2026-09-16", &moved, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	ovSkip, err := NewEventOverride(event.ID, "2026-09-23", nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	got := MergeOccurrences([]Event{event}, occs, []EventOverride{ovMove, ovSkip}, from, to)
	if len(got) != 1 || !got[0].StartsAt.Equal(moved) || !got[0].Overridden || got[0].OriginalOn != "2026-09-16" {
		t.Fatalf("got %+v", got)
	}
}

func TestNewEventNoteRejectsEmptyBody(t *testing.T) {
	_, err := NewEventNote(uuid.New(), "2026-09-16", "  ")
	if err == nil {
		t.Fatal("expected invalid note body")
	}
}

func TestEventRecurrenceKeepsOwnTimezoneAcrossDST(t *testing.T) {
    loc, err := time.LoadLocation("Europe/London")
    if err != nil { t.Fatal(err) }
    event := Event{
        StartsAt: time.Date(2026, 3, 23, 9, 0, 0, 0, loc),
        DurationSeconds: 1800,
        Recurrence: RecurrenceWeekly,
        Weekdays: []int{0, 7},
        Timezone: "Europe/London",
    }
    rows := event.Expand(time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))
    if len(rows) != 2 || !rows[0].StartsAt.Equal(time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)) ||
        !rows[1].StartsAt.Equal(time.Date(2026, 3, 30, 8, 0, 0, 0, time.UTC)) || string(rows[1].OriginalOn) != "2026-03-30" {
        t.Fatalf("occurrences=%+v", rows)
    }
}

func TestEventGapAndRepeatedHour(t *testing.T) {
    spring := Event{StartsAt: time.Date(2026, 3, 22, 1, 30, 0, 0, time.UTC), Timezone: "Europe/London"}
    gap := spring.OccurrenceOn(Ymd("2026-03-29"))
    if !gap.StartsAt.Equal(time.Date(2026, 3, 29, 1, 0, 0, 0, time.UTC)) { t.Fatalf("gap=%s", gap.StartsAt) }
    autumn := Event{StartsAt: time.Date(2026, 10, 18, 0, 30, 0, 0, time.UTC), Timezone: "Europe/London"}
    repeated := autumn.OccurrenceOn(Ymd("2026-10-25"))
    if !repeated.StartsAt.Equal(time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC)) { t.Fatalf("repeated=%s", repeated.StartsAt) }
}
