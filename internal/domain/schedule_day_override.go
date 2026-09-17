package domain

import (
	"fmt"
	"strings"
	"time"
)

// Ymd is a calendar date in YYYY-MM-DD form, timezone-agnostic.
type Ymd string

const ymdLayout = "2006-01-02"

func ParseYmd(raw string) (Ymd, error) {
	raw = strings.TrimSpace(raw)
	if _, err := time.Parse(ymdLayout, raw); err != nil {
		return "", fmt.Errorf("%w: date", ErrInvalid)
	}
	return Ymd(raw), nil
}

func YmdOf(t time.Time, loc *time.Location) Ymd {
	return Ymd(t.In(loc).Format(ymdLayout))
}

// Midnight returns the local start of the day.
func (d Ymd) Midnight(loc *time.Location) time.Time {
	parsed, err := time.ParseInLocation(ymdLayout, string(d), loc)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// DayOverride replaces the default work window for one calendar day.
// Off means no work at all; a nil window keeps the default hours.
type DayOverride struct {
	Day          Ymd       `json:"day"`
	Off          bool      `json:"off"`
	WorkStartMin *int      `json:"workStartMin"`
	WorkEndMin   *int      `json:"workEndMin"`
	Note         string    `json:"note"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type DayOverrideDraft struct {
	Off          bool
	WorkStartMin *int
	WorkEndMin   *int
	Note         string
}

func NewDayOverride(day Ymd, draft DayOverrideDraft, now time.Time) (DayOverride, error) {
	if _, err := ParseYmd(string(day)); err != nil {
		return DayOverride{}, err
	}
	out := DayOverride{
		Day:       day,
		Off:       draft.Off,
		Note:      strings.TrimSpace(draft.Note),
		UpdatedAt: now.UTC(),
	}
	if draft.Off {
		return out, nil
	}
	if (draft.WorkStartMin == nil) != (draft.WorkEndMin == nil) {
		return DayOverride{}, fmt.Errorf("%w: day window", ErrInvalid)
	}
	if draft.WorkStartMin != nil {
		start, end := *draft.WorkStartMin, *draft.WorkEndMin
		if start < 0 || end > minutesPerDay || start >= end {
			return DayOverride{}, fmt.Errorf("%w: day window", ErrInvalid)
		}
		out.WorkStartMin = &start
		out.WorkEndMin = &end
	}
	if out.WorkStartMin == nil && out.Note == "" {
		return DayOverride{}, fmt.Errorf("%w: empty override", ErrInvalid)
	}
	return out, nil
}

// Window returns the override's own hours, if any.
func (o DayOverride) Window() (start, end int, ok bool) {
	if o.Off || o.WorkStartMin == nil || o.WorkEndMin == nil {
		return 0, 0, false
	}
	return *o.WorkStartMin, *o.WorkEndMin, true
}
