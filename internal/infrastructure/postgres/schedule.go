package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pchkauu/want-brief/internal/domain"
)

// Settings live in a single jsonb row. Missing keys fall back to defaults so
// older rows keep working when new knobs are added.
func (s *Store) GetScheduleSettings(ctx context.Context) (domain.ScheduleSettings, error) {
	settings := domain.DefaultScheduleSettings()
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT data FROM schedule_settings WHERE id=1`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return domain.DefaultScheduleSettings(), err
	}
	return settings, nil
}

func (s *Store) SaveScheduleSettings(ctx context.Context, settings domain.ScheduleSettings) (domain.ScheduleSettings, error) {
	raw, err := json.Marshal(settings)
	if err != nil {
		return settings, err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO schedule_settings (id, data, updated_at)
		VALUES (1, $1, now())
		ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = now()
	`, raw)
	return settings, err
}

type ScheduleSettingsRepo struct{ *Store }

func (r ScheduleSettingsRepo) Get(ctx context.Context) (domain.ScheduleSettings, error) {
	return r.Store.GetScheduleSettings(ctx)
}
func (r ScheduleSettingsRepo) Save(ctx context.Context, settings domain.ScheduleSettings) (domain.ScheduleSettings, error) {
	return r.Store.SaveScheduleSettings(ctx, settings)
}

const dayOverrideCols = `day, "off", work_start_min, work_end_min, note, updated_at`

func scanDayOverride(scan func(dest ...any) error) (domain.DayOverride, error) {
	var o domain.DayOverride
	var day time.Time
	err := scan(&day, &o.Off, &o.WorkStartMin, &o.WorkEndMin, &o.Note, &o.UpdatedAt)
	if err != nil {
		return o, err
	}
	o.Day = domain.Ymd(day.Format("2006-01-02"))
	return o, nil
}

func (s *Store) ListDayOverrides(ctx context.Context, from, to domain.Ymd) ([]domain.DayOverride, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+dayOverrideCols+`
		FROM schedule_day_overrides
		WHERE day >= $1::date AND day <= $2::date
		ORDER BY day
	`, string(from), string(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.DayOverride{}
	for rows.Next() {
		o, err := scanDayOverride(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) UpsertDayOverride(ctx context.Context, o domain.DayOverride) (domain.DayOverride, error) {
	o, err := scanDayOverride(s.pool.QueryRow(ctx, `
		INSERT INTO schedule_day_overrides (`+dayOverrideCols+`)
		VALUES ($1::date, $2, $3, $4, $5, $6)
		ON CONFLICT (day) DO UPDATE SET
			"off" = EXCLUDED."off",
			work_start_min = EXCLUDED.work_start_min,
			work_end_min = EXCLUDED.work_end_min,
			note = EXCLUDED.note,
			updated_at = EXCLUDED.updated_at
		RETURNING `+dayOverrideCols+`
	`, string(o.Day), o.Off, o.WorkStartMin, o.WorkEndMin, o.Note, o.UpdatedAt).Scan)
	return o, mapErr(err)
}

func (s *Store) DeleteDayOverride(ctx context.Context, day domain.Ymd) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM schedule_day_overrides WHERE day=$1::date`, string(day))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type DayOverrideRepo struct{ *Store }

func (r DayOverrideRepo) ListRange(ctx context.Context, from, to domain.Ymd) ([]domain.DayOverride, error) {
	return r.Store.ListDayOverrides(ctx, from, to)
}
func (r DayOverrideRepo) Upsert(ctx context.Context, override domain.DayOverride) (domain.DayOverride, error) {
	return r.Store.UpsertDayOverride(ctx, override)
}
func (r DayOverrideRepo) Delete(ctx context.Context, day domain.Ymd) error {
	return r.Store.DeleteDayOverride(ctx, day)
}

func (s *Store) GetScheduleSnapshot(ctx context.Context, kind domain.ScheduleKind) (domain.ScheduleSnapshot, error) {
	snap := domain.ScheduleSnapshot{Kind: kind, Blocks: []domain.ScheduleBlock{}}
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT taken_at, blocks FROM schedule_snapshots WHERE kind=$1`, string(kind)).
		Scan(&snap.TakenAt, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return snap, domain.ErrNotFound
	}
	if err != nil {
		return snap, err
	}
	if err := json.Unmarshal(raw, &snap.Blocks); err != nil {
		return snap, err
	}
	return snap, nil
}

func (s *Store) SaveScheduleSnapshot(ctx context.Context, snap domain.ScheduleSnapshot) error {
	raw, err := json.Marshal(snap.Blocks)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO schedule_snapshots (kind, taken_at, blocks)
		VALUES ($1, $2, $3)
		ON CONFLICT (kind) DO UPDATE SET taken_at = EXCLUDED.taken_at, blocks = EXCLUDED.blocks
	`, string(snap.Kind), snap.TakenAt, raw)
	return err
}

type ScheduleSnapshotRepo struct{ *Store }

func (r ScheduleSnapshotRepo) Get(ctx context.Context, kind domain.ScheduleKind) (domain.ScheduleSnapshot, error) {
	return r.Store.GetScheduleSnapshot(ctx, kind)
}
func (r ScheduleSnapshotRepo) Save(ctx context.Context, snapshot domain.ScheduleSnapshot) error {
	return r.Store.SaveScheduleSnapshot(ctx, snapshot)
}
