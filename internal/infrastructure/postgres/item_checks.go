package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const itemCheckCols = `id, item_id, body, done, position, created_at, updated_at`

func scanItemCheck(scan func(dest ...any) error) (domain.ItemCheck, error) {
	var c domain.ItemCheck
	err := scan(&c.ID, &c.ItemID, &c.Body, &c.Done, &c.Position, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) GetItemCheck(ctx context.Context, id uuid.UUID) (domain.ItemCheck, error) {
	c, err := scanItemCheck(s.pool.QueryRow(ctx, `SELECT `+itemCheckCols+` FROM item_checks WHERE id=$1`, id).Scan)
	return c, mapErr(err)
}

func (s *Store) ListItemChecks(ctx context.Context, itemID uuid.UUID) ([]domain.ItemCheck, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+itemCheckCols+`
		FROM item_checks WHERE item_id=$1
		ORDER BY position, created_at, id
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ItemCheck{}
	for rows.Next() {
		c, err := scanItemCheck(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateItemCheck(ctx context.Context, c domain.ItemCheck) (domain.ItemCheck, error) {
	c, err := scanItemCheck(s.pool.QueryRow(ctx, `
		INSERT INTO item_checks (`+itemCheckCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+itemCheckCols+`
	`, c.ID, c.ItemID, c.Body, c.Done, c.Position, c.CreatedAt, c.UpdatedAt).Scan)
	return c, mapErr(err)
}

func (s *Store) UpdateItemCheck(ctx context.Context, c domain.ItemCheck) (domain.ItemCheck, error) {
	c, err := scanItemCheck(s.pool.QueryRow(ctx, `
		UPDATE item_checks
		SET body=$2, done=$3, position=$4, updated_at=$5
		WHERE id=$1
		RETURNING `+itemCheckCols+`
	`, c.ID, c.Body, c.Done, c.Position, c.UpdatedAt).Scan)
	return c, mapErr(err)
}

func (s *Store) DeleteItemCheck(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM item_checks WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type ItemCheckRepo struct{ *Store }

func (r ItemCheckRepo) Get(ctx context.Context, id uuid.UUID) (domain.ItemCheck, error) {
	return r.Store.GetItemCheck(ctx, id)
}
func (r ItemCheckRepo) ListByItem(ctx context.Context, itemID uuid.UUID) ([]domain.ItemCheck, error) {
	return r.Store.ListItemChecks(ctx, itemID)
}
func (r ItemCheckRepo) Create(ctx context.Context, check domain.ItemCheck) (domain.ItemCheck, error) {
	return r.Store.CreateItemCheck(ctx, check)
}
func (r ItemCheckRepo) Update(ctx context.Context, check domain.ItemCheck) (domain.ItemCheck, error) {
	return r.Store.UpdateItemCheck(ctx, check)
}
func (r ItemCheckRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteItemCheck(ctx, id)
}
