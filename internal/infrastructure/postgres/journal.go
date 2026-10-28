package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

func (s *Store) ListJournal(ctx context.Context) ([]domain.JournalEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, source, owner_id, owner_name, body, created_at
		FROM (
			SELECT n.id, 'loose'::text AS source, n.item_id AS owner_id,
				COALESCE(i.title, 'Loose') AS owner_name, n.body, n.created_at
			FROM notes n
			LEFT JOIN items i ON i.id = n.item_id
			UNION ALL
			SELECT n.id, 'project', n.project_id, p.name, n.body, n.created_at
			FROM project_notes n
			JOIN projects p ON p.id = n.project_id
			UNION ALL
			SELECT n.id, 'item', n.item_id, i.title, n.body, n.created_at
			FROM item_notes n
			JOIN items i ON i.id = n.item_id AND i.deleted_at IS NULL
			UNION ALL
			SELECT n.id, 'person', n.person_id, pe.name, n.body, n.created_at
			FROM person_notes n
			JOIN people pe ON pe.id = n.person_id
		) feed
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.JournalEntry{}
	for rows.Next() {
		var entry domain.JournalEntry
		var owner *uuid.UUID
		if err := rows.Scan(&entry.ID, &entry.Source, &owner, &entry.OwnerName, &entry.Body, &entry.CreatedAt); err != nil {
			return nil, err
		}
		if owner != nil {
			id := owner.String()
			entry.OwnerID = &id
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

type JournalRepo struct{ *Store }

func (r JournalRepo) List(ctx context.Context) ([]domain.JournalEntry, error) {
	return r.Store.ListJournal(ctx)
}
