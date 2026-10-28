package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

func (s *Store) GetNote(ctx context.Context, id uuid.UUID) (domain.Note, error) {
	var n domain.Note
	err := s.pool.QueryRow(ctx, `
		SELECT id, item_id, body, created_at, updated_at FROM notes WHERE id=$1
	`, id).Scan(&n.ID, &n.ItemID, &n.Body, &n.CreatedAt, &n.UpdatedAt)
	return n, mapErr(err)
}

func (s *Store) ListNotes(ctx context.Context, itemID *uuid.UUID) ([]domain.Note, error) {
	query := `SELECT id, item_id, body, created_at, updated_at FROM notes`
	args := []any{}
	if itemID != nil {
		query += ` WHERE item_id=$1`
		args = append(args, *itemID)
	} else {
		query += ` WHERE item_id IS NULL`
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Note
	for rows.Next() {
		var n domain.Note
		if err := rows.Scan(&n.ID, &n.ItemID, &n.Body, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreateNote(ctx context.Context, n domain.Note) (domain.Note, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO notes (id, item_id, body, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, item_id, body, created_at, updated_at
	`, n.ID, n.ItemID, n.Body, n.CreatedAt, n.UpdatedAt).
		Scan(&n.ID, &n.ItemID, &n.Body, &n.CreatedAt, &n.UpdatedAt)
	return n, err
}

func (s *Store) UpdateNote(ctx context.Context, n domain.Note) (domain.Note, error) {
	err := s.pool.QueryRow(ctx, `
		UPDATE notes SET body=$2, updated_at=$3 WHERE id=$1
		RETURNING id, item_id, body, created_at, updated_at
	`, n.ID, n.Body, n.UpdatedAt).
		Scan(&n.ID, &n.ItemID, &n.Body, &n.CreatedAt, &n.UpdatedAt)
	return n, mapErr(err)
}

func (s *Store) DeleteNote(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM notes WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type NoteRepo struct{ *Store }

func (r NoteRepo) Get(ctx context.Context, id uuid.UUID) (domain.Note, error) {
	return r.Store.GetNote(ctx, id)
}
func (r NoteRepo) List(ctx context.Context, itemID *uuid.UUID) ([]domain.Note, error) {
	return r.Store.ListNotes(ctx, itemID)
}
func (r NoteRepo) Create(ctx context.Context, note domain.Note) (domain.Note, error) {
	return r.Store.CreateNote(ctx, note)
}
func (r NoteRepo) Update(ctx context.Context, note domain.Note) (domain.Note, error) {
	return r.Store.UpdateNote(ctx, note)
}
func (r NoteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteNote(ctx, id)
}

func scanInterval(scanner interface{ Scan(dest ...any) error }) (domain.TimeInterval, error) {
	var t domain.TimeInterval
	err := scanner.Scan(&t.ID, &t.ItemID, &t.StartedAt, &t.EndedAt)
	return t, err
}

func (s *Store) GetInterval(ctx context.Context, id uuid.UUID) (domain.TimeInterval, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, item_id, started_at, ended_at FROM time_intervals WHERE id=$1
	`, id)
	t, err := scanInterval(row)
	return t, mapErr(err)
}

func (s *Store) ListOpenIntervals(ctx context.Context) ([]domain.TimeInterval, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, item_id, started_at, ended_at
		FROM time_intervals WHERE ended_at IS NULL
		ORDER BY started_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.TimeInterval
	for rows.Next() {
		t, err := scanInterval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ListRangeIntervals(ctx context.Context, from, to time.Time) ([]domain.TimeInterval, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, item_id, started_at, ended_at
		FROM time_intervals
		WHERE started_at < $2
		  AND (ended_at IS NULL OR ended_at > $1)
		ORDER BY started_at
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.TimeInterval
	for rows.Next() {
		t, err := scanInterval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateInterval(ctx context.Context, t domain.TimeInterval) (domain.TimeInterval, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO time_intervals (id, item_id, started_at, ended_at)
		VALUES ($1,$2,$3,$4)
		RETURNING id, item_id, started_at, ended_at
	`, t.ID, t.ItemID, t.StartedAt, t.EndedAt).
		Scan(&t.ID, &t.ItemID, &t.StartedAt, &t.EndedAt)
	return t, err
}

func (s *Store) UpdateInterval(ctx context.Context, t domain.TimeInterval) (domain.TimeInterval, error) {
	err := s.pool.QueryRow(ctx, `
		UPDATE time_intervals SET ended_at=$2 WHERE id=$1
		RETURNING id, item_id, started_at, ended_at
	`, t.ID, t.EndedAt).
		Scan(&t.ID, &t.ItemID, &t.StartedAt, &t.EndedAt)
	return t, mapErr(err)
}

type IntervalRepo struct{ *Store }

func (r IntervalRepo) Get(ctx context.Context, id uuid.UUID) (domain.TimeInterval, error) {
	return r.Store.GetInterval(ctx, id)
}
func (r IntervalRepo) ListOpen(ctx context.Context) ([]domain.TimeInterval, error) {
	return r.Store.ListOpenIntervals(ctx)
}
func (r IntervalRepo) ListRange(ctx context.Context, from, to time.Time) ([]domain.TimeInterval, error) {
	return r.Store.ListRangeIntervals(ctx, from, to)
}
func (r IntervalRepo) Create(ctx context.Context, interval domain.TimeInterval) (domain.TimeInterval, error) {
	return r.Store.CreateInterval(ctx, interval)
}
func (r IntervalRepo) Update(ctx context.Context, interval domain.TimeInterval) (domain.TimeInterval, error) {
	return r.Store.UpdateInterval(ctx, interval)
}

func (s *Store) ListStressRange(ctx context.Context, from, to time.Time) ([]domain.StressLog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, item_id, kind, level, logged_at
		FROM stress_logs
		WHERE logged_at >= $1 AND logged_at < $2
		ORDER BY logged_at
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.StressLog
	for rows.Next() {
		var log domain.StressLog
		if err := rows.Scan(&log.ID, &log.ItemID, &log.Kind, &log.Level, &log.LoggedAt); err != nil {
			return nil, err
		}
		out = append(out, log)
	}
	return out, rows.Err()
}

func (s *Store) LatestStress(ctx context.Context) ([]domain.StressLog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (kind) id, item_id, kind, level, logged_at
		FROM stress_logs
		ORDER BY kind, logged_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.StressLog
	for rows.Next() {
		var log domain.StressLog
		if err := rows.Scan(&log.ID, &log.ItemID, &log.Kind, &log.Level, &log.LoggedAt); err != nil {
			return nil, err
		}
		out = append(out, log)
	}
	return out, rows.Err()
}

func (s *Store) CreateStress(ctx context.Context, log domain.StressLog) (domain.StressLog, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO stress_logs (id, item_id, kind, level, logged_at)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, item_id, kind, level, logged_at
	`, log.ID, log.ItemID, log.Kind, log.Level, log.LoggedAt).
		Scan(&log.ID, &log.ItemID, &log.Kind, &log.Level, &log.LoggedAt)
	return log, err
}

type StressRepo struct{ *Store }

func (r StressRepo) ListRange(ctx context.Context, from, to time.Time) ([]domain.StressLog, error) {
	return r.Store.ListStressRange(ctx, from, to)
}
func (r StressRepo) Latest(ctx context.Context) ([]domain.StressLog, error) {
	return r.Store.LatestStress(ctx)
}
func (r StressRepo) Create(ctx context.Context, log domain.StressLog) (domain.StressLog, error) {
	return r.Store.CreateStress(ctx, log)
}
