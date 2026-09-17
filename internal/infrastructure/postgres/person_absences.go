package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const personAbsenceCols = `id, person_id, starts_on, ends_on, note, created_at, updated_at`

func scanPersonAbsence(scan func(dest ...any) error) (domain.PersonAbsence, error) {
	var a domain.PersonAbsence
	err := scan(&a.ID, &a.PersonID, &a.StartsOn, &a.EndsOn, &a.Note, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return a, err
	}
	a.StartsOn = asUTCDate(a.StartsOn)
	a.EndsOn = asUTCDate(a.EndsOn)
	return a, nil
}

func asUTCDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *Store) GetPersonAbsence(ctx context.Context, id uuid.UUID) (domain.PersonAbsence, error) {
	a, err := scanPersonAbsence(s.pool.QueryRow(ctx, `SELECT `+personAbsenceCols+` FROM person_absences WHERE id=$1`, id).Scan)
	return a, mapErr(err)
}

func (s *Store) ListPersonAbsences(ctx context.Context, personID uuid.UUID) ([]domain.PersonAbsence, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+personAbsenceCols+`
		FROM person_absences WHERE person_id=$1
		ORDER BY starts_on DESC, id
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectPersonAbsences(rows)
}

// ListPersonAbsencesRange returns absences overlapping [from, to] by calendar day.
func (s *Store) ListPersonAbsencesRange(ctx context.Context, from, to time.Time) ([]domain.PersonAbsence, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+personAbsenceCols+`
		FROM person_absences
		WHERE starts_on <= $2::date AND ends_on >= $1::date
		ORDER BY starts_on, id
	`, asUTCDate(from), asUTCDate(to))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectPersonAbsences(rows)
}

func collectPersonAbsences(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]domain.PersonAbsence, error) {
	out := []domain.PersonAbsence{}
	for rows.Next() {
		a, err := scanPersonAbsence(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) absencesByPeople(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.PersonAbsence, error) {
	out := map[uuid.UUID][]domain.PersonAbsence{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+personAbsenceCols+`
		FROM person_absences WHERE person_id = ANY($1)
		ORDER BY starts_on DESC, id
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanPersonAbsence(rows.Scan)
		if err != nil {
			return nil, err
		}
		out[a.PersonID] = append(out[a.PersonID], a)
	}
	return out, rows.Err()
}

func (s *Store) CreatePersonAbsence(ctx context.Context, a domain.PersonAbsence) (domain.PersonAbsence, error) {
	a, err := scanPersonAbsence(s.pool.QueryRow(ctx, `
		INSERT INTO person_absences (`+personAbsenceCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+personAbsenceCols+`
	`, a.ID, a.PersonID, a.StartsOn, a.EndsOn, a.Note, a.CreatedAt, a.UpdatedAt).Scan)
	return a, mapErr(err)
}

func (s *Store) UpdatePersonAbsence(ctx context.Context, a domain.PersonAbsence) (domain.PersonAbsence, error) {
	a, err := scanPersonAbsence(s.pool.QueryRow(ctx, `
		UPDATE person_absences
		SET starts_on=$2, ends_on=$3, note=$4, updated_at=$5
		WHERE id=$1
		RETURNING `+personAbsenceCols+`
	`, a.ID, a.StartsOn, a.EndsOn, a.Note, a.UpdatedAt).Scan)
	return a, mapErr(err)
}

func (s *Store) DeletePersonAbsence(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM person_absences WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type PersonAbsenceRepo struct{ *Store }

func (r PersonAbsenceRepo) Get(ctx context.Context, id uuid.UUID) (domain.PersonAbsence, error) {
	return r.Store.GetPersonAbsence(ctx, id)
}
func (r PersonAbsenceRepo) ListByPerson(ctx context.Context, personID uuid.UUID) ([]domain.PersonAbsence, error) {
	return r.Store.ListPersonAbsences(ctx, personID)
}
func (r PersonAbsenceRepo) ListRange(ctx context.Context, from, to time.Time) ([]domain.PersonAbsence, error) {
	return r.Store.ListPersonAbsencesRange(ctx, from, to)
}
func (r PersonAbsenceRepo) Create(ctx context.Context, absence domain.PersonAbsence) (domain.PersonAbsence, error) {
	return r.Store.CreatePersonAbsence(ctx, absence)
}
func (r PersonAbsenceRepo) Update(ctx context.Context, absence domain.PersonAbsence) (domain.PersonAbsence, error) {
	return r.Store.UpdatePersonAbsence(ctx, absence)
}
func (r PersonAbsenceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeletePersonAbsence(ctx, id)
}
