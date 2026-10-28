package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const personSiteCols = `id, person_id, kind, url, comment, created_at, updated_at`

func scanPersonSite(scan func(dest ...any) error) (domain.PersonSite, error) {
	var site domain.PersonSite
	err := scan(&site.ID, &site.PersonID, &site.Kind, &site.URL, &site.Comment, &site.CreatedAt, &site.UpdatedAt)
	return site, err
}

func (s *Store) GetPersonSite(ctx context.Context, id uuid.UUID) (domain.PersonSite, error) {
	site, err := scanPersonSite(s.pool.QueryRow(ctx, `SELECT `+personSiteCols+` FROM person_sites WHERE id=$1`, id).Scan)
	return site, mapErr(err)
}

func (s *Store) ListPersonSites(ctx context.Context, personID uuid.UUID) ([]domain.PersonSite, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+personSiteCols+`
		FROM person_sites WHERE person_id=$1
		ORDER BY created_at, id
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PersonSite{}
	for rows.Next() {
		site, err := scanPersonSite(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, site)
	}
	return out, rows.Err()
}

func (s *Store) sitesByPeople(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.PersonSite, error) {
	out := map[uuid.UUID][]domain.PersonSite{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+personSiteCols+`
		FROM person_sites WHERE person_id = ANY($1)
		ORDER BY created_at, id
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		site, err := scanPersonSite(rows.Scan)
		if err != nil {
			return nil, err
		}
		out[site.PersonID] = append(out[site.PersonID], site)
	}
	return out, rows.Err()
}

func (s *Store) CreatePersonSite(ctx context.Context, site domain.PersonSite) (domain.PersonSite, error) {
	site, err := scanPersonSite(s.pool.QueryRow(ctx, `
		INSERT INTO person_sites (`+personSiteCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+personSiteCols+`
	`, site.ID, site.PersonID, site.Kind, site.URL, site.Comment, site.CreatedAt, site.UpdatedAt).Scan)
	return site, mapErr(err)
}

func (s *Store) UpdatePersonSite(ctx context.Context, site domain.PersonSite) (domain.PersonSite, error) {
	site, err := scanPersonSite(s.pool.QueryRow(ctx, `
		UPDATE person_sites
		SET kind=$2, url=$3, comment=$4, updated_at=$5
		WHERE id=$1
		RETURNING `+personSiteCols+`
	`, site.ID, site.Kind, site.URL, site.Comment, site.UpdatedAt).Scan)
	return site, mapErr(err)
}

func (s *Store) DeletePersonSite(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM person_sites WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type PersonSiteRepo struct{ *Store }

func (r PersonSiteRepo) Get(ctx context.Context, id uuid.UUID) (domain.PersonSite, error) {
	return r.Store.GetPersonSite(ctx, id)
}
func (r PersonSiteRepo) ListByPerson(ctx context.Context, personID uuid.UUID) ([]domain.PersonSite, error) {
	return r.Store.ListPersonSites(ctx, personID)
}
func (r PersonSiteRepo) Create(ctx context.Context, site domain.PersonSite) (domain.PersonSite, error) {
	return r.Store.CreatePersonSite(ctx, site)
}
func (r PersonSiteRepo) Update(ctx context.Context, site domain.PersonSite) (domain.PersonSite, error) {
	return r.Store.UpdatePersonSite(ctx, site)
}
func (r PersonSiteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeletePersonSite(ctx, id)
}
