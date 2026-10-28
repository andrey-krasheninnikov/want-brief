package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const personAddressCols = `id, person_id, kind, label, value, note, created_at, updated_at`

func scanPersonAddress(scan func(dest ...any) error) (domain.PersonAddress, error) {
	var a domain.PersonAddress
	err := scan(&a.ID, &a.PersonID, &a.Kind, &a.Label, &a.Value, &a.Note, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func (s *Store) GetPersonAddress(ctx context.Context, id uuid.UUID) (domain.PersonAddress, error) {
	a, err := scanPersonAddress(s.pool.QueryRow(ctx, `SELECT `+personAddressCols+` FROM person_addresses WHERE id=$1`, id).Scan)
	return a, mapErr(err)
}

func (s *Store) ListPersonAddresses(ctx context.Context, personID uuid.UUID) ([]domain.PersonAddress, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+personAddressCols+`
		FROM person_addresses WHERE person_id=$1
		ORDER BY created_at, id
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PersonAddress{}
	for rows.Next() {
		a, err := scanPersonAddress(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) addressesByPeople(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.PersonAddress, error) {
	out := map[uuid.UUID][]domain.PersonAddress{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+personAddressCols+`
		FROM person_addresses WHERE person_id = ANY($1)
		ORDER BY created_at, id
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanPersonAddress(rows.Scan)
		if err != nil {
			return nil, err
		}
		out[a.PersonID] = append(out[a.PersonID], a)
	}
	return out, rows.Err()
}

func (s *Store) CreatePersonAddress(ctx context.Context, a domain.PersonAddress) (domain.PersonAddress, error) {
	a, err := scanPersonAddress(s.pool.QueryRow(ctx, `
		INSERT INTO person_addresses (`+personAddressCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+personAddressCols+`
	`, a.ID, a.PersonID, a.Kind, a.Label, a.Value, a.Note, a.CreatedAt, a.UpdatedAt).Scan)
	return a, mapErr(err)
}

func (s *Store) UpdatePersonAddress(ctx context.Context, a domain.PersonAddress) (domain.PersonAddress, error) {
	a, err := scanPersonAddress(s.pool.QueryRow(ctx, `
		UPDATE person_addresses
		SET kind=$2, label=$3, value=$4, note=$5, updated_at=$6
		WHERE id=$1
		RETURNING `+personAddressCols+`
	`, a.ID, a.Kind, a.Label, a.Value, a.Note, a.UpdatedAt).Scan)
	return a, mapErr(err)
}

func (s *Store) DeletePersonAddress(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM person_addresses WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type PersonAddressRepo struct{ *Store }

func (r PersonAddressRepo) Get(ctx context.Context, id uuid.UUID) (domain.PersonAddress, error) {
	return r.Store.GetPersonAddress(ctx, id)
}
func (r PersonAddressRepo) ListByPerson(ctx context.Context, personID uuid.UUID) ([]domain.PersonAddress, error) {
	return r.Store.ListPersonAddresses(ctx, personID)
}
func (r PersonAddressRepo) Create(ctx context.Context, address domain.PersonAddress) (domain.PersonAddress, error) {
	return r.Store.CreatePersonAddress(ctx, address)
}
func (r PersonAddressRepo) Update(ctx context.Context, address domain.PersonAddress) (domain.PersonAddress, error) {
	return r.Store.UpdatePersonAddress(ctx, address)
}
func (r PersonAddressRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeletePersonAddress(ctx, id)
}
