package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const personContactCols = `id, person_id, kind, label, value, note, created_at, updated_at`

func scanPersonContact(scan func(dest ...any) error) (domain.PersonContact, error) {
	var c domain.PersonContact
	err := scan(&c.ID, &c.PersonID, &c.Kind, &c.Label, &c.Value, &c.Note, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) GetPersonContact(ctx context.Context, id uuid.UUID) (domain.PersonContact, error) {
	c, err := scanPersonContact(s.pool.QueryRow(ctx, `SELECT `+personContactCols+` FROM person_contacts WHERE id=$1`, id).Scan)
	return c, mapErr(err)
}

func (s *Store) ListPersonContacts(ctx context.Context, personID uuid.UUID) ([]domain.PersonContact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+personContactCols+`
		FROM person_contacts WHERE person_id=$1
		ORDER BY created_at, id
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PersonContact{}
	for rows.Next() {
		c, err := scanPersonContact(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) contactsByPeople(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.PersonContact, error) {
	out := map[uuid.UUID][]domain.PersonContact{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+personContactCols+`
		FROM person_contacts WHERE person_id = ANY($1)
		ORDER BY created_at, id
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanPersonContact(rows.Scan)
		if err != nil {
			return nil, err
		}
		out[c.PersonID] = append(out[c.PersonID], c)
	}
	return out, rows.Err()
}

func (s *Store) CreatePersonContact(ctx context.Context, c domain.PersonContact) (domain.PersonContact, error) {
	c, err := scanPersonContact(s.pool.QueryRow(ctx, `
		INSERT INTO person_contacts (`+personContactCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+personContactCols+`
	`, c.ID, c.PersonID, c.Kind, c.Label, c.Value, c.Note, c.CreatedAt, c.UpdatedAt).Scan)
	return c, mapErr(err)
}

func (s *Store) UpdatePersonContact(ctx context.Context, c domain.PersonContact) (domain.PersonContact, error) {
	c, err := scanPersonContact(s.pool.QueryRow(ctx, `
		UPDATE person_contacts
		SET kind=$2, label=$3, value=$4, note=$5, updated_at=$6
		WHERE id=$1
		RETURNING `+personContactCols+`
	`, c.ID, c.Kind, c.Label, c.Value, c.Note, c.UpdatedAt).Scan)
	return c, mapErr(err)
}

func (s *Store) DeletePersonContact(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM person_contacts WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type PersonContactRepo struct{ *Store }

func (r PersonContactRepo) Get(ctx context.Context, id uuid.UUID) (domain.PersonContact, error) {
	return r.Store.GetPersonContact(ctx, id)
}
func (r PersonContactRepo) ListByPerson(ctx context.Context, personID uuid.UUID) ([]domain.PersonContact, error) {
	return r.Store.ListPersonContacts(ctx, personID)
}
func (r PersonContactRepo) Create(ctx context.Context, contact domain.PersonContact) (domain.PersonContact, error) {
	return r.Store.CreatePersonContact(ctx, contact)
}
func (r PersonContactRepo) Update(ctx context.Context, contact domain.PersonContact) (domain.PersonContact, error) {
	return r.Store.UpdatePersonContact(ctx, contact)
}
func (r PersonContactRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeletePersonContact(ctx, id)
}
