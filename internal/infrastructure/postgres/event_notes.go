package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

func scanEventNote(scan func(dest ...any) error) (domain.EventNote, error) {
	var n domain.EventNote
	var on time.Time
	err := scan(&n.ID, &n.SeriesID, &on, &n.Body, &n.CreatedAt)
	if err != nil {
		return domain.EventNote{}, err
	}
	n.OriginalOn = domain.Ymd(on.Format("2006-01-02"))
	return n, nil
}

func (s *Store) GetEventNote(ctx context.Context, id uuid.UUID) (domain.EventNote, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, series_id, original_on, body, created_at FROM event_notes WHERE id=$1
	`, id)
	n, err := scanEventNote(row.Scan)
	return n, mapErr(err)
}

func (s *Store) ListEventNotes(ctx context.Context, seriesID uuid.UUID, originalOn *domain.Ymd) ([]domain.EventNote, error) {
	query := `
		SELECT id, series_id, original_on, body, created_at
		FROM event_notes WHERE series_id=$1
	`
	args := []any{seriesID}
	if originalOn != nil {
		query += ` AND original_on=$2::date`
		args = append(args, string(*originalOn))
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.EventNote{}
	for rows.Next() {
		n, err := scanEventNote(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreateEventNote(ctx context.Context, n domain.EventNote) (domain.EventNote, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO event_notes (id, series_id, original_on, body, created_at)
		VALUES ($1,$2,$3::date,$4,$5)
		RETURNING id, series_id, original_on, body, created_at
	`, n.ID, n.SeriesID, string(n.OriginalOn), n.Body, n.CreatedAt)
	created, err := scanEventNote(row.Scan)
	return created, err
}

func (s *Store) DeleteEventNote(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM event_notes WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type EventNoteRepo struct{ *Store }

func (r EventNoteRepo) Get(ctx context.Context, id uuid.UUID) (domain.EventNote, error) {
	return r.Store.GetEventNote(ctx, id)
}
func (r EventNoteRepo) ListBySeries(ctx context.Context, seriesID uuid.UUID, originalOn *domain.Ymd) ([]domain.EventNote, error) {
	return r.Store.ListEventNotes(ctx, seriesID, originalOn)
}
func (r EventNoteRepo) Create(ctx context.Context, note domain.EventNote) (domain.EventNote, error) {
	return r.Store.CreateEventNote(ctx, note)
}
func (r EventNoteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteEventNote(ctx, id)
}
