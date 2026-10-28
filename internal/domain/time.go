package domain

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

type TimeInterval struct {
	ID        uuid.UUID  `json:"id"`
	ItemID    uuid.UUID  `json:"itemId"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt"`
}

func NewOpenInterval(itemID uuid.UUID, at time.Time) TimeInterval {
	return TimeInterval{
		ID:        uuid.New(),
		ItemID:    itemID,
		StartedAt: at.UTC(),
	}
}

func NewClosedInterval(itemID uuid.UUID, started, ended time.Time) (TimeInterval, error) {
	start := started.UTC()
	end := ended.UTC()
	if !end.After(start) {
		return TimeInterval{}, fmt.Errorf("%w: end before start", ErrInvalid)
	}
	return TimeInterval{
		ID:        uuid.New(),
		ItemID:    itemID,
		StartedAt: start,
		EndedAt:   &end,
	}, nil
}

func (t TimeInterval) Stop(at time.Time) (TimeInterval, error) {
	if t.EndedAt != nil {
		return TimeInterval{}, fmt.Errorf("%w: interval already stopped", ErrConflict)
	}
	end := at.UTC()
	if end.Before(t.StartedAt) {
		return TimeInterval{}, fmt.Errorf("%w: end before start", ErrInvalid)
	}
	t.EndedAt = &end
	return t, nil
}

func (t TimeInterval) Duration(now time.Time) time.Duration {
	end := now
	if t.EndedAt != nil {
		end = *t.EndedAt
	}
	if end.Before(t.StartedAt) {
		return 0
	}
	return end.Sub(t.StartedAt)
}

func clipInterval(started, ended, from, to time.Time) (time.Time, time.Time, bool) {
	if ended.Before(from) || !started.Before(to) {
		return time.Time{}, time.Time{}, false
	}
	if started.Before(from) {
		started = from
	}
	if ended.After(to) {
		ended = to
	}
	if !ended.After(started) {
		return time.Time{}, time.Time{}, false
	}
	return started, ended, true
}

func Moscow() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.FixedZone("MSK", 3*3600)
	}
	return loc
}

func AllocatedSeconds(intervals []TimeInterval, from, to, now time.Time) int64 {
	var total time.Duration
	for _, interval := range intervals {
		end := now
		if interval.EndedAt != nil {
			end = *interval.EndedAt
		}
		start, stop, ok := clipInterval(interval.StartedAt, end, from, to)
		if !ok {
			continue
		}
		total += stop.Sub(start)
	}
	return int64(total / time.Second)
}

func WallSeconds(intervals []TimeInterval, from, to, now time.Time) int64 {
	type span struct {
		start time.Time
		end   time.Time
	}
	spans := make([]span, 0, len(intervals))
	for _, interval := range intervals {
		end := now
		if interval.EndedAt != nil {
			end = *interval.EndedAt
		}
		start, stop, ok := clipInterval(interval.StartedAt, end, from, to)
		if !ok {
			continue
		}
		spans = append(spans, span{start: start, end: stop})
	}
	if len(spans) == 0 {
		return 0
	}
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].start.Before(spans[j].start)
	})
	merged := spans[0]
	var total time.Duration
	for _, next := range spans[1:] {
		if next.start.After(merged.end) {
			total += merged.end.Sub(merged.start)
			merged = next
			continue
		}
		if next.end.After(merged.end) {
			merged.end = next.end
		}
	}
	total += merged.end.Sub(merged.start)
	return int64(total / time.Second)
}
