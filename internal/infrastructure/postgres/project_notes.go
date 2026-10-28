package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

func (s *Store) GetProjectNote(ctx context.Context, id uuid.UUID) (domain.ProjectNote, error) {
	var n domain.ProjectNote
	err := s.pool.QueryRow(ctx, `
		SELECT id, project_id, body, created_at FROM project_notes WHERE id=$1
	`, id).Scan(&n.ID, &n.ProjectID, &n.Body, &n.CreatedAt)
	return n, mapErr(err)
}

func (s *Store) ListProjectNotes(ctx context.Context, projectID uuid.UUID) ([]domain.ProjectNote, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, body, created_at
		FROM project_notes WHERE project_id=$1
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ProjectNote{}
	for rows.Next() {
		var n domain.ProjectNote
		if err := rows.Scan(&n.ID, &n.ProjectID, &n.Body, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreateProjectNote(ctx context.Context, n domain.ProjectNote) (domain.ProjectNote, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO project_notes (id, project_id, body, created_at)
		VALUES ($1,$2,$3,$4)
		RETURNING id, project_id, body, created_at
	`, n.ID, n.ProjectID, n.Body, n.CreatedAt).
		Scan(&n.ID, &n.ProjectID, &n.Body, &n.CreatedAt)
	return n, err
}

func (s *Store) DeleteProjectNote(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM project_notes WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type ProjectNoteRepo struct{ *Store }

func (r ProjectNoteRepo) Get(ctx context.Context, id uuid.UUID) (domain.ProjectNote, error) {
	return r.Store.GetProjectNote(ctx, id)
}
func (r ProjectNoteRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]domain.ProjectNote, error) {
	return r.Store.ListProjectNotes(ctx, projectID)
}
func (r ProjectNoteRepo) Create(ctx context.Context, note domain.ProjectNote) (domain.ProjectNote, error) {
	return r.Store.CreateProjectNote(ctx, note)
}
func (r ProjectNoteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteProjectNote(ctx, id)
}
