package application

import (
	"context"

	"github.com/pchkauu/want-brief/internal/domain"
)

// scheduleSettings loads stored settings, falling back to defaults when the
// repository is not wired (tests) or the row does not exist yet.
func (s *Service) scheduleSettings(ctx context.Context) (domain.ScheduleSettings, error) {
	if s.ScheduleSettings == nil {
		return domain.DefaultScheduleSettings(), nil
	}
	return s.ScheduleSettings.Get(ctx)
}

func (s *Service) GetScheduleSettings(ctx context.Context) (domain.ScheduleSettings, error) {
	return s.scheduleSettings(ctx)
}

func (s *Service) SaveScheduleSettings(ctx context.Context, settings domain.ScheduleSettings) (domain.ScheduleSettings, error) {
	settings = settings.Normalized()
	if err := settings.Validate(); err != nil {
		return domain.ScheduleSettings{}, err
	}
	return s.ScheduleSettings.Save(ctx, settings)
}

func (s *Service) ListDayOverrides(ctx context.Context, from, to domain.Ymd) ([]domain.DayOverride, error) {
	if s.DayOverrides == nil {
		return []domain.DayOverride{}, nil
	}
	if from > to {
		from, to = to, from
	}
	return s.DayOverrides.ListRange(ctx, from, to)
}

func (s *Service) UpsertDayOverride(ctx context.Context, day domain.Ymd, draft domain.DayOverrideDraft) (domain.DayOverride, error) {
	override, err := domain.NewDayOverride(day, draft, s.now())
	if err != nil {
		return domain.DayOverride{}, err
	}
	return s.DayOverrides.Upsert(ctx, override)
}

func (s *Service) DeleteDayOverride(ctx context.Context, day domain.Ymd) error {
	return s.DayOverrides.Delete(ctx, day)
}
