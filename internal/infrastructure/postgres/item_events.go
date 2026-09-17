package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const itemEventColumns = `id, item_id, kind, field, from_value, to_value, note, stress, created_at`

func scanItemEvent(scan func(dest ...any) error) (domain.ItemEvent, error) {
	var e domain.ItemEvent
	err := scan(&e.ID, &e.ItemID, &e.Kind, &e.Field, &e.From, &e.To, &e.Note, &e.Stress, &e.CreatedAt)
	return e, err
}

func (s *Store) CreateItemEvent(ctx context.Context, e domain.ItemEvent) (domain.ItemEvent, error) {
	e, err := scanItemEvent(s.pool.QueryRow(ctx, `
		INSERT INTO item_events (`+itemEventColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+itemEventColumns+`
	`, e.ID, e.ItemID, e.Kind, e.Field, e.From, e.To, e.Note, e.Stress, e.CreatedAt).Scan)
	return e, mapErr(err)
}

func (s *Store) UpdateItemEvent(ctx context.Context, e domain.ItemEvent) (domain.ItemEvent, error) {
	e, err := scanItemEvent(s.pool.QueryRow(ctx, `
		UPDATE item_events
		SET note=$2, stress=$3
		WHERE id=$1
		RETURNING `+itemEventColumns+`
	`, e.ID, e.Note, e.Stress).Scan)
	return e, mapErr(err)
}

func (s *Store) ListItemEvents(ctx context.Context, itemID uuid.UUID) ([]domain.ItemEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+itemEventColumns+`
		FROM item_events WHERE item_id=$1
		ORDER BY created_at DESC, id DESC
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ItemEvent{}
	for rows.Next() {
		e, err := scanItemEvent(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) LatestItemEventByKinds(ctx context.Context, itemID uuid.UUID, kinds []domain.ItemEventKind) (domain.ItemEvent, error) {
	if len(kinds) == 0 {
		return domain.ItemEvent{}, domain.ErrNotFound
	}
	labels := make([]string, len(kinds))
	for i, kind := range kinds {
		labels[i] = string(kind)
	}
	e, err := scanItemEvent(s.pool.QueryRow(ctx, `
		SELECT `+itemEventColumns+`
		FROM item_events
		WHERE item_id=$1 AND kind = ANY($2)
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, itemID, labels).Scan)
	return e, mapErr(err)
}

type ItemEventRepo struct{ *Store }

func (r ItemEventRepo) Create(ctx context.Context, event domain.ItemEvent) (domain.ItemEvent, error) {
	return r.Store.CreateItemEvent(ctx, event)
}
func (r ItemEventRepo) Update(ctx context.Context, event domain.ItemEvent) (domain.ItemEvent, error) {
	return r.Store.UpdateItemEvent(ctx, event)
}
func (r ItemEventRepo) ListByItem(ctx context.Context, itemID uuid.UUID) ([]domain.ItemEvent, error) {
	return r.Store.ListItemEvents(ctx, itemID)
}
func (r ItemEventRepo) LatestByItemAndKinds(ctx context.Context, itemID uuid.UUID, kinds []domain.ItemEventKind) (domain.ItemEvent, error) {
	return r.Store.LatestItemEventByKinds(ctx, itemID, kinds)
}
