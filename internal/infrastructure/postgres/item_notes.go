package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const itemNoteColumns = `id, item_id, body, source_kind, external_id, author_name, url, created_at`

func (s *Store) GetItemNote(ctx context.Context, id uuid.UUID) (domain.ItemNote, error) {
	var n domain.ItemNote
	err := s.pool.QueryRow(ctx, `
		SELECT `+itemNoteColumns+` FROM item_notes WHERE id=$1
	`, id).Scan(&n.ID, &n.ItemID, &n.Body, &n.SourceKind, &n.ExternalID, &n.AuthorName, &n.URL, &n.CreatedAt)
	return n, mapErr(err)
}

func (s *Store) ListItemNotes(ctx context.Context, itemID uuid.UUID) ([]domain.ItemNote, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+itemNoteColumns+`
		FROM item_notes WHERE item_id=$1
		ORDER BY created_at DESC
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ItemNote{}
	for rows.Next() {
		var n domain.ItemNote
		if err := rows.Scan(&n.ID, &n.ItemID, &n.Body, &n.SourceKind, &n.ExternalID, &n.AuthorName, &n.URL, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreateItemNote(ctx context.Context, n domain.ItemNote) (domain.ItemNote, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO item_notes (id, item_id, body, source_kind, external_id, author_name, url, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+itemNoteColumns+`
	`, n.ID, n.ItemID, n.Body, n.SourceKind, n.ExternalID, n.AuthorName, n.URL, n.CreatedAt).
		Scan(&n.ID, &n.ItemID, &n.Body, &n.SourceKind, &n.ExternalID, &n.AuthorName, &n.URL, &n.CreatedAt)
	return n, err
}

func (s *Store) DeleteItemNote(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM item_notes WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type ItemNoteRepo struct{ *Store }

func (r ItemNoteRepo) Get(ctx context.Context, id uuid.UUID) (domain.ItemNote, error) {
	return r.Store.GetItemNote(ctx, id)
}
func (r ItemNoteRepo) ListByItem(ctx context.Context, itemID uuid.UUID) ([]domain.ItemNote, error) {
	return r.Store.ListItemNotes(ctx, itemID)
}
func (r ItemNoteRepo) Create(ctx context.Context, note domain.ItemNote) (domain.ItemNote, error) {
	return r.Store.CreateItemNote(ctx, note)
}
func (r ItemNoteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteItemNote(ctx, id)
}
