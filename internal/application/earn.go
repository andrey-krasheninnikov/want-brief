package application

import (
	"context"

	"github.com/pchkauu/want-brief/internal/domain"
)

func (s *Service) ListEarnDays(ctx context.Context, from, to domain.Ymd) ([]domain.EarnDay, error) {
	if s.EarnDays == nil {
		return []domain.EarnDay{}, nil
	}
	if from > to {
		from, to = to, from
	}
	return s.EarnDays.ListRange(ctx, from, to)
}

func (s *Service) UpsertEarnDay(ctx context.Context, day domain.Ymd, usd, rub float64) (domain.EarnDay, error) {
	row, err := domain.NewEarnDay(day, usd, rub, s.now())
	if err != nil {
		return domain.EarnDay{}, err
	}
	return s.EarnDays.Upsert(ctx, row)
}
