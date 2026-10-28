package domain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimezone  = "Europe/Moscow"
	minutesPerDay    = 24 * 60
	maxWaitingTracks = 5
	maxCheckWindows  = 6
)

// BreakWindow is an optional daily pause (formerly the hardcoded lunch).
type BreakWindow struct {
	StartMin int `json:"startMin"`
	EndMin   int `json:"endMin"`
}

// ScheduleSettings drives the packer. Zero values are never used directly:
// callers start from DefaultScheduleSettings and overlay stored JSON on top.
type ScheduleSettings struct {
	Timezone              string       `json:"timezone"`
	WorkStartMin          int          `json:"workStartMin"`
	WorkEndMin            int          `json:"workEndMin"`
	Workdays              []int        `json:"workdays"`
	Break                 *BreakWindow `json:"break"`
	MeetingBufferMin      int          `json:"meetingBufferMin"`
	DailyFocusMin         int          `json:"dailyFocusMin"`
	MaxTasksPerDay        int          `json:"maxTasksPerDay"`
	MinSliceMin           int          `json:"minSliceMin"`
	MaxSliceMin           int          `json:"maxSliceMin"`
	SliceBreakMin         int          `json:"sliceBreakMin"`
	GridMin               int          `json:"gridMin"`
	EstimateBuffer        bool         `json:"estimateBuffer"`
	EstimateMaxK          float64      `json:"estimateMaxK"`
	OldestFirst           bool         `json:"oldestFirst"`
	StressShiftHours      int          `json:"stressShiftHours"`
	TargetLeadWorkdays    int          `json:"targetLeadWorkdays"`
	SoftBusy              bool         `json:"softBusy"`
	WaitingTracks         int          `json:"waitingTracks"`
	FollowupPingMin       int          `json:"followupPingMin"`
	CheckWindows          []string     `json:"checkWindows"`
	EnergyAware           bool         `json:"energyAware"`
	GoldenHours           bool         `json:"goldenHours"`
	StabilityThresholdMin int          `json:"stabilityThresholdMin"`
}

func DefaultScheduleSettings() ScheduleSettings {
	return ScheduleSettings{
		Timezone:              defaultTimezone,
		WorkStartMin:          10 * 60,
		WorkEndMin:            19 * 60,
		Workdays:              []int{1, 2, 3, 4, 5},
		Break:                 nil,
		MeetingBufferMin:      10,
		DailyFocusMin:         6 * 60,
		MaxTasksPerDay:        5,
		MinSliceMin:           45,
		MaxSliceMin:           120,
		SliceBreakMin:         10,
		GridMin:               5,
		EstimateBuffer:        true,
		EstimateMaxK:          2,
		OldestFirst:           true,
		StressShiftHours:      6,
		TargetLeadWorkdays:    1,
		SoftBusy:              true,
		WaitingTracks:         2,
		FollowupPingMin:       20,
		CheckWindows:          []string{"10:00", "14:00"},
		EnergyAware:           true,
		GoldenHours:           true,
		StabilityThresholdMin: 30,
	}
}

// Validate checks ranges and cross-field rules; it does not mutate.
func (s ScheduleSettings) Validate() error {
	if _, err := time.LoadLocation(strings.TrimSpace(s.Timezone)); err != nil || strings.TrimSpace(s.Timezone) == "" {
		return fmt.Errorf("%w: timezone", ErrInvalid)
	}
	if s.WorkStartMin < 0 || s.WorkEndMin > minutesPerDay || s.WorkStartMin >= s.WorkEndMin {
		return fmt.Errorf("%w: work window", ErrInvalid)
	}
	if len(s.Workdays) == 0 {
		return fmt.Errorf("%w: workdays", ErrInvalid)
	}
	for _, day := range s.Workdays {
		if day < 0 || day > 6 {
			return fmt.Errorf("%w: workdays", ErrInvalid)
		}
	}
	if s.Break != nil {
		if s.Break.StartMin < s.WorkStartMin || s.Break.EndMin > s.WorkEndMin || s.Break.StartMin >= s.Break.EndMin {
			return fmt.Errorf("%w: break window", ErrInvalid)
		}
	}
	for name, value := range map[string]int{
		"meeting buffer":      s.MeetingBufferMin,
		"daily focus":         s.DailyFocusMin,
		"max tasks per day":   s.MaxTasksPerDay,
		"min slice":           s.MinSliceMin,
		"max slice":           s.MaxSliceMin,
		"slice break":         s.SliceBreakMin,
		"stress shift":        s.StressShiftHours,
		"target lead":         s.TargetLeadWorkdays,
		"waiting tracks":      s.WaitingTracks,
		"followup ping":       s.FollowupPingMin,
		"stability threshold": s.StabilityThresholdMin,
	} {
		if value < 0 {
			return fmt.Errorf("%w: %s", ErrInvalid, name)
		}
	}
	if s.MinSliceMin > 0 && s.MaxSliceMin > 0 && s.MinSliceMin > s.MaxSliceMin {
		return fmt.Errorf("%w: min slice above max slice", ErrInvalid)
	}
	switch s.GridMin {
	case 1, 5, 10, 15, 30:
	default:
		return fmt.Errorf("%w: grid", ErrInvalid)
	}
	if s.EstimateMaxK < 1 || s.EstimateMaxK > 5 {
		return fmt.Errorf("%w: estimate max factor", ErrInvalid)
	}
	if s.WaitingTracks > maxWaitingTracks {
		return fmt.Errorf("%w: waiting tracks", ErrInvalid)
	}
	if len(s.CheckWindows) == 0 || len(s.CheckWindows) > maxCheckWindows {
		return fmt.Errorf("%w: check windows", ErrInvalid)
	}
	for _, raw := range s.CheckWindows {
		minute, err := parseClockMinutes(raw)
		if err != nil {
			return err
		}
		if minute < s.WorkStartMin || minute >= s.WorkEndMin {
			return fmt.Errorf("%w: check window outside work hours", ErrInvalid)
		}
	}
	return nil
}

// Normalized returns a copy with trimmed strings, sorted unique workdays and
// sorted check windows.
func (s ScheduleSettings) Normalized() ScheduleSettings {
	out := s
	out.Timezone = strings.TrimSpace(s.Timezone)
	seen := map[int]bool{}
	out.Workdays = nil
	for _, day := range s.Workdays {
		if !seen[day] {
			seen[day] = true
			out.Workdays = append(out.Workdays, day)
		}
	}
	sort.Ints(out.Workdays)
	out.CheckWindows = make([]string, 0, len(s.CheckWindows))
	for _, raw := range s.CheckWindows {
		out.CheckWindows = append(out.CheckWindows, strings.TrimSpace(raw))
	}
	sort.Strings(out.CheckWindows)
	return out
}

// Location resolves the configured timezone, falling back to Moscow.
func (s ScheduleSettings) Location() *time.Location {
	loc, err := time.LoadLocation(strings.TrimSpace(s.Timezone))
	if err != nil || loc == nil {
		return Moscow()
	}
	return loc
}

func (s ScheduleSettings) isWorkday(day time.Weekday) bool {
	for _, wd := range s.Workdays {
		if time.Weekday(wd) == day {
			return true
		}
	}
	return false
}

// CheckWindowMinutes returns the follow-up check windows as minutes since
// midnight, sorted ascending. Invalid entries are skipped.
func (s ScheduleSettings) CheckWindowMinutes() []int {
	out := make([]int, 0, len(s.CheckWindows))
	for _, raw := range s.CheckWindows {
		minute, err := parseClockMinutes(raw)
		if err != nil {
			continue
		}
		out = append(out, minute)
	}
	sort.Ints(out)
	return out
}

func parseClockMinutes(raw string) (int, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("%w: clock %q", ErrInvalid, raw)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, fmt.Errorf("%w: clock %q", ErrInvalid, raw)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("%w: clock %q", ErrInvalid, raw)
	}
	return hour*60 + minute, nil
}
