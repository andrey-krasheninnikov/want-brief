package postgres

import (
	"context"
	"time"

	"github.com/pchkauu/want-brief/internal/domain"
)

const earnDayCols = `day, usd, rub, updated_at`

func scanEarnDay(scan func(dest ...any) error) (domain.EarnDay, error) {
	var row domain.EarnDay
	var day time.Time
	err := scan(&day, &row.USD, &row.RUB, &row.UpdatedAt)
	if err != nil {
		return row, err
	}
	row.Day = domain.Ymd(day.Format("2006-01-02"))
	return row, nil
}

func (s *Store) ListEarnDays(ctx context.Context, from, to domain.Ymd) ([]domain.EarnDay, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+earnDayCols+`
		FROM earn_days
		WHERE day >= $1::date AND day <= $2::date
		ORDER BY day
	`, string(from), string(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.EarnDay{}
	for rows.Next() {
		row, err := scanEarnDay(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) UpsertEarnDay(ctx context.Context, row domain.EarnDay) (domain.EarnDay, error) {
	row, err := scanEarnDay(s.pool.QueryRow(ctx, `
		INSERT INTO earn_days (`+earnDayCols+`)
		VALUES ($1::date, $2, $3, $4)
		ON CONFLICT (day) DO UPDATE SET
			usd = EXCLUDED.usd,
			rub = EXCLUDED.rub,
			updated_at = EXCLUDED.updated_at
		RETURNING `+earnDayCols+`
	`, string(row.Day), row.USD, row.RUB, row.UpdatedAt).Scan)
	return row, mapErr(err)
}

type EarnDayRepo struct{ *Store }

func (r EarnDayRepo) ListRange(ctx context.Context, from, to domain.Ymd) ([]domain.EarnDay, error) {
	return r.Store.ListEarnDays(ctx, from, to)
}

func (r EarnDayRepo) Upsert(ctx context.Context, row domain.EarnDay) (domain.EarnDay, error) {
	return r.Store.UpsertEarnDay(ctx, row)
}
