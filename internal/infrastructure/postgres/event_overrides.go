package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

func scanEventOverride(scan func(dest ...any) error) (domain.EventOverride, error) {
	var o domain.EventOverride
	var on time.Time
	err := scan(&o.SeriesID, &on, &o.StartsAt, &o.DurationSeconds, &o.Skipped)
	if err != nil {
		return domain.EventOverride{}, err
	}
	o.OriginalOn = domain.Ymd(on.Format("2006-01-02"))
	return o, nil
}

func (s *Store) GetEventOverride(ctx context.Context, seriesID uuid.UUID, originalOn domain.Ymd) (domain.EventOverride, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT series_id, original_on, starts_at, duration_seconds, skipped
		FROM event_overrides WHERE series_id=$1 AND original_on=$2::date
	`, seriesID, string(originalOn))
	o, err := scanEventOverride(row.Scan)
	return o, mapErr(err)
}

func (s *Store) ListEventOverrides(ctx context.Context, seriesIDs []uuid.UUID) ([]domain.EventOverride, error) {
	if len(seriesIDs) == 0 {
		return []domain.EventOverride{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT series_id, original_on, starts_at, duration_seconds, skipped
		FROM event_overrides WHERE series_id = ANY($1)
	`, seriesIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.EventOverride{}
	for rows.Next() {
		o, err := scanEventOverride(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) UpsertEventOverride(ctx context.Context, o domain.EventOverride) (domain.EventOverride, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO event_overrides (series_id, original_on, starts_at, duration_seconds, skipped)
		VALUES ($1,$2::date,$3,$4,$5)
		ON CONFLICT (series_id, original_on) DO UPDATE SET
			starts_at = EXCLUDED.starts_at,
			duration_seconds = EXCLUDED.duration_seconds,
			skipped = EXCLUDED.skipped
		RETURNING series_id, original_on, starts_at, duration_seconds, skipped
	`, o.SeriesID, string(o.OriginalOn), o.StartsAt, o.DurationSeconds, o.Skipped)
	return scanEventOverride(row.Scan)
}

func (s *Store) DeleteEventOverride(ctx context.Context, seriesID uuid.UUID, originalOn domain.Ymd) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM event_overrides WHERE series_id=$1 AND original_on=$2::date
	`, seriesID, string(originalOn))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type EventOverrideRepo struct{ *Store }

func (r EventOverrideRepo) Get(ctx context.Context, seriesID uuid.UUID, originalOn domain.Ymd) (domain.EventOverride, error) {
	return r.Store.GetEventOverride(ctx, seriesID, originalOn)
}
func (r EventOverrideRepo) ListBySeries(ctx context.Context, seriesIDs []uuid.UUID) ([]domain.EventOverride, error) {
	return r.Store.ListEventOverrides(ctx, seriesIDs)
}
func (r EventOverrideRepo) Upsert(ctx context.Context, override domain.EventOverride) (domain.EventOverride, error) {
	return r.Store.UpsertEventOverride(ctx, override)
}
func (r EventOverrideRepo) Delete(ctx context.Context, seriesID uuid.UUID, originalOn domain.Ymd) error {
	return r.Store.DeleteEventOverride(ctx, seriesID, originalOn)
}
