package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/pchkauu/want-brief/internal/domain"
)

func (s *Service) Schedule(ctx context.Context, from, to time.Time, kind domain.ScheduleKind) (domain.Schedule, error) {
	in, err := s.scheduleInput(ctx, from, to, kind)
	if err != nil {
		return domain.Schedule{}, err
	}
	if s.Snapshots != nil {
		if prev, err := s.Snapshots.Get(ctx, in.Kind); err == nil {
			in.Previous = prev.Blocks
		}
	}
	report := domain.Plan(in)
	if s.Snapshots != nil {
		blocks := report.Horizon
		if blocks == nil {
			blocks = []domain.ScheduleBlock{}
		}
		_ = s.Snapshots.Save(ctx, domain.ScheduleSnapshot{Kind: in.Kind, TakenAt: in.Now, Blocks: blocks})
	}
	return report, nil
}

func (s *Service) ScheduleWhatIf(ctx context.Context, from, to time.Time, kind domain.ScheduleKind, scenario domain.WhatIfScenario) (domain.WhatIfResult, error) {
	in, err := s.scheduleInput(ctx, from, to, kind)
	if err != nil {
		return domain.WhatIfResult{}, err
	}
	return domain.WhatIf(in, scenario), nil
}

// scheduleInput gathers everything the packer reads: open tasks, meetings
// over the horizon, settings, day overrides, absences, open intervals,
// estimate factors, tracked history and the latest check-ins.
func (s *Service) scheduleInput(ctx context.Context, from, to time.Time, kind domain.ScheduleKind) (domain.ScheduleInput, error) {
	if from.IsZero() || to.IsZero() || !to.After(from) {
		return domain.ScheduleInput{}, fmt.Errorf("%w: range", domain.ErrInvalid)
	}
	parsed, err := domain.ParseScheduleKind(string(kind))
	if err != nil {
		return domain.ScheduleInput{}, err
	}
	settings, err := s.scheduleSettings(ctx)
	if err != nil {
		return domain.ScheduleInput{}, err
	}
	taskKind := domain.KindTask
	items, err := s.Items.List(ctx, domain.ItemFilter{Kind: &taskKind, OpenOnly: true})
	if err != nil {
		return domain.ScheduleInput{}, err
	}
	now := s.now()
	horizon := now.Add(120 * 24 * time.Hour)
	if to.After(horizon) {
		horizon = to
	}
	eventFrom := now
	if from.Before(eventFrom) {
		eventFrom = from
	}
	events, err := s.ListEventOccurrences(ctx, eventFrom, horizon)
	if err != nil {
		return domain.ScheduleInput{}, err
	}
	open, err := s.ListOpenIntervals(ctx)
	if err != nil {
		return domain.ScheduleInput{}, err
	}
	in := domain.ScheduleInput{
		Kind:          parsed,
		Items:         applyOpenTrack(items, open, now),
		Events:        events,
		Now:           now,
		From:          from,
		To:            to,
		Settings:      settings,
		OpenIntervals: open,
	}
	loc := settings.Location()
	if s.DayOverrides != nil {
		in.Overrides, err = s.DayOverrides.ListRange(ctx, domain.YmdOf(eventFrom, loc), domain.YmdOf(horizon, loc))
		if err != nil {
			return domain.ScheduleInput{}, err
		}
	}
	if s.PersonAbsences != nil {
		in.Absences, err = s.PersonAbsences.ListRange(ctx, eventFrom, horizon)
		if err != nil {
			return domain.ScheduleInput{}, err
		}
	}
	in.PeopleNames, err = s.peopleNames(ctx)
	if err != nil {
		return domain.ScheduleInput{}, err
	}
	if settings.EstimateBuffer {
		in.Factors, err = s.estimateFactors(ctx)
		if err != nil {
			return domain.ScheduleInput{}, err
		}
	}
	if settings.GoldenHours {
		in.History, err = s.Intervals.ListRange(ctx, now.AddDate(0, 0, -7*domain.GoldenHistoryWeeks), now)
		if err != nil {
			return domain.ScheduleInput{}, err
		}
	}
	if settings.EnergyAware {
		checkins, err := s.LatestCheckins(ctx)
		if err != nil {
			return domain.ScheduleInput{}, err
		}
		in.Checkins = &checkins
	}
	return in, nil
}

func (s *Service) peopleNames(ctx context.Context) (map[uuid.UUID]string, error) {
	people, err := s.People.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(people))
	for _, person := range people {
		out[person.ID] = person.Name
	}
	return out, nil
}

// estimateFactors returns, per project, the median tracked/planned ratio of
// finished tasks (#13). Projects without history get no entry (factor 1).
func (s *Service) estimateFactors(ctx context.Context) (map[string]float64, error) {
	done := domain.StatusDone
	taskKind := domain.KindTask
	items, err := s.Items.List(ctx, domain.ItemFilter{Kind: &taskKind, Status: &done, IncludeArchived: true})
	if err != nil {
		return nil, err
	}
	ratios := map[string][]float64{}
	for _, item := range items {
		if item.PlannedSeconds <= 0 || item.TrackedSeconds <= 0 {
			continue
		}
		key := ""
		if item.ProjectID != nil {
			key = item.ProjectID.String()
		}
		ratios[key] = append(ratios[key], float64(item.TrackedSeconds)/float64(item.PlannedSeconds))
	}
	out := map[string]float64{}
	for key, list := range ratios {
		if len(list) < 3 {
			continue
		}
		sort.Float64s(list)
		mid := len(list) / 2
		median := list[mid]
		if len(list)%2 == 0 {
			median = (list[mid-1] + list[mid]) / 2
		}
		if median > 1 {
			out[key] = median
		}
	}
	return out, nil
}

func applyOpenTrack(items []domain.Item, open []domain.TimeInterval, now time.Time) []domain.Item {
	extra := map[uuid.UUID]int64{}
	for _, interval := range open {
		sec := int64(now.Sub(interval.StartedAt) / time.Second)
		if sec < 0 {
			continue
		}
		extra[interval.ItemID] += sec
	}
	if len(extra) == 0 {
		return items
	}
	out := append([]domain.Item(nil), items...)
	for i := range out {
		out[i].TrackedSeconds += extra[out[i].ID]
	}
	return out
}
