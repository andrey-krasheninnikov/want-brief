package domain

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

type EventOverride struct {
	SeriesID        uuid.UUID  `json:"seriesId"`
	OriginalOn      Ymd        `json:"originalOn"`
	StartsAt        *time.Time `json:"startsAt"`
	DurationSeconds *int       `json:"durationSeconds"`
	Skipped         bool       `json:"skipped"`
}

func NewEventOverride(seriesID uuid.UUID, originalOn Ymd, startsAt *time.Time, duration *int, skipped bool) (EventOverride, error) {
	if seriesID == uuid.Nil {
		return EventOverride{}, fmt.Errorf("%w: series", ErrInvalid)
	}
	on, err := ParseYmd(string(originalOn))
	if err != nil {
		return EventOverride{}, err
	}
	if duration != nil && *duration < 1 {
		return EventOverride{}, fmt.Errorf("%w: duration", ErrInvalid)
	}
	if !skipped && startsAt == nil && duration == nil {
		return EventOverride{}, fmt.Errorf("%w: empty override", ErrInvalid)
	}
	var start *time.Time
	if startsAt != nil {
		utc := startsAt.UTC()
		start = &utc
	}
	return EventOverride{
		SeriesID:        seriesID,
		OriginalOn:      on,
		StartsAt:        start,
		DurationSeconds: duration,
		Skipped:         skipped,
	}, nil
}

func (o EventOverride) Apply(occ EventOccurrence) EventOccurrence {
	occ.Overridden = true
	if o.DurationSeconds != nil {
		occ.DurationSeconds = *o.DurationSeconds
	}
	if o.StartsAt != nil {
		occ.StartsAt = o.StartsAt.UTC()
	}
	occ.EndsAt = occ.StartsAt.Add(time.Duration(occ.DurationSeconds) * time.Second)
	return occ
}

func MergeOccurrences(series []Event, occs []EventOccurrence, overrides []EventOverride, from, to time.Time) []EventOccurrence {
	from, to = clampEventRange(from, to)
	byID := make(map[uuid.UUID]Event, len(series))
	for _, event := range series {
		byID[event.ID] = event
	}
	type key struct {
		id uuid.UUID
		on Ymd
	}
	index := make(map[key]EventOccurrence, len(occs)+len(overrides))
	for _, occ := range occs {
		index[key{occ.SeriesID, occ.OriginalOn}] = occ
	}
	for _, override := range overrides {
		event, ok := byID[override.SeriesID]
		if !ok {
			continue
		}
		k := key{override.SeriesID, override.OriginalOn}
		occ, have := index[k]
		if !have {
			if !event.HasSlot(override.OriginalOn) {
				continue
			}
			occ = event.OccurrenceOn(override.OriginalOn)
		}
		if override.Skipped {
			delete(index, k)
			continue
		}
		index[k] = override.Apply(occ)
	}
	out := make([]EventOccurrence, 0, len(index))
	for _, occ := range index {
		if occ.StartsAt.Before(from) || !occ.StartsAt.Before(to) {
			continue
		}
		out = append(out, occ)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartsAt.Equal(out[j].StartsAt) {
			return out[i].SeriesID.String() < out[j].SeriesID.String()
		}
		return out[i].StartsAt.Before(out[j].StartsAt)
	})
	return out
}
