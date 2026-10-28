package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const personProfessionCols = `id, person_id, title, comment, started_on, ended_on, created_at, updated_at`
const personSalaryCols = `id, profession_id, started_on, ended_on, monthly_salary_usd, monthly_salary_rub`

func scanPersonProfession(scan func(dest ...any) error) (domain.PersonProfession, error) {
	var p domain.PersonProfession
	err := scan(&p.ID, &p.PersonID, &p.Title, &p.Comment, &p.StartedOn, &p.EndedOn, &p.CreatedAt, &p.UpdatedAt)
	if p.Salaries == nil {
		p.Salaries = []domain.SalaryPeriod{}
	}
	return p, err
}

func scanSalaryPeriod(scan func(dest ...any) error) (domain.SalaryPeriod, error) {
	var s domain.SalaryPeriod
	err := scan(&s.ID, &s.ProfessionID, &s.StartedOn, &s.EndedOn, &s.MonthlySalaryUSD, &s.MonthlySalaryRUB)
	return s, err
}

func (s *Store) GetPersonProfession(ctx context.Context, id uuid.UUID) (domain.PersonProfession, error) {
	p, err := scanPersonProfession(s.pool.QueryRow(ctx, `SELECT `+personProfessionCols+` FROM person_professions WHERE id=$1`, id).Scan)
	if err != nil {
		return domain.PersonProfession{}, mapErr(err)
	}
	salaries, err := s.salariesByProfessions(ctx, []uuid.UUID{p.ID}, false)
	if err != nil {
		return domain.PersonProfession{}, err
	}
	p.Salaries = salaries[p.ID]
	if p.Salaries == nil {
		p.Salaries = []domain.SalaryPeriod{}
	}
	return p, nil
}

func (s *Store) ListPersonProfessions(ctx context.Context, personID uuid.UUID) ([]domain.PersonProfession, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+personProfessionCols+`
		FROM person_professions WHERE person_id=$1
		ORDER BY ended_on NULLS FIRST, started_on DESC, id
	`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.PersonProfession{}
	ids := []uuid.UUID{}
	for rows.Next() {
		p, err := scanPersonProfession(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	salaries, err := s.salariesByProfessions(ctx, ids, false)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Salaries = salaries[out[i].ID]
		if out[i].Salaries == nil {
			out[i].Salaries = []domain.SalaryPeriod{}
		}
	}
	return out, nil
}

func (s *Store) professionsByPeople(ctx context.Context, ids []uuid.UUID, currentOnly bool) (map[uuid.UUID][]domain.PersonProfession, error) {
	out := map[uuid.UUID][]domain.PersonProfession{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+personProfessionCols+`
		FROM person_professions WHERE person_id = ANY($1)
		ORDER BY ended_on NULLS FIRST, started_on DESC, id
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []domain.PersonProfession{}
	profIDs := []uuid.UUID{}
	for rows.Next() {
		p, err := scanPersonProfession(rows.Scan)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
		profIDs = append(profIDs, p.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	salaries, err := s.salariesByProfessions(ctx, profIDs, currentOnly)
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		p.Salaries = salaries[p.ID]
		if p.Salaries == nil {
			p.Salaries = []domain.SalaryPeriod{}
		}
		out[p.PersonID] = append(out[p.PersonID], p)
	}
	return out, nil
}

func (s *Store) salariesByProfessions(ctx context.Context, ids []uuid.UUID, currentOnly bool) (map[uuid.UUID][]domain.SalaryPeriod, error) {
	out := map[uuid.UUID][]domain.SalaryPeriod{}
	if len(ids) == 0 {
		return out, nil
	}
	q := `
		SELECT ` + personSalaryCols + `
		FROM person_profession_salaries
		WHERE profession_id = ANY($1)
	`
	if currentOnly {
		q += ` AND ended_on IS NULL`
	}
	q += ` ORDER BY started_on, id`
	rows, err := s.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		period, err := scanSalaryPeriod(rows.Scan)
		if err != nil {
			return nil, err
		}
		out[period.ProfessionID] = append(out[period.ProfessionID], period)
	}
	return out, rows.Err()
}

func (s *Store) CreatePersonProfession(ctx context.Context, p domain.PersonProfession) (domain.PersonProfession, error) {
	return s.savePersonProfession(ctx, p, true)
}

func (s *Store) UpdatePersonProfession(ctx context.Context, p domain.PersonProfession) (domain.PersonProfession, error) {
	return s.savePersonProfession(ctx, p, false)
}

func (s *Store) savePersonProfession(ctx context.Context, p domain.PersonProfession, insert bool) (domain.PersonProfession, error) {
	salaries := p.Salaries
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PersonProfession{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if insert {
		p, err = scanPersonProfession(tx.QueryRow(ctx, `
			INSERT INTO person_professions (`+personProfessionCols+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			RETURNING `+personProfessionCols+`
		`, p.ID, p.PersonID, p.Title, p.Comment, p.StartedOn, p.EndedOn, p.CreatedAt, p.UpdatedAt).Scan)
	} else {
		p, err = scanPersonProfession(tx.QueryRow(ctx, `
			UPDATE person_professions
			SET title=$2, comment=$3, started_on=$4, ended_on=$5, updated_at=$6
			WHERE id=$1
			RETURNING `+personProfessionCols+`
		`, p.ID, p.Title, p.Comment, p.StartedOn, p.EndedOn, p.UpdatedAt).Scan)
	}
	if err != nil {
		return domain.PersonProfession{}, mapErr(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM person_profession_salaries WHERE profession_id=$1`, p.ID); err != nil {
		return domain.PersonProfession{}, err
	}
	for _, period := range salaries {
		if period.ID == uuid.Nil {
			period.ID = uuid.New()
		}
		if period.ProfessionID == uuid.Nil {
			period.ProfessionID = p.ID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO person_profession_salaries (`+personSalaryCols+`)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, period.ID, p.ID, period.StartedOn, period.EndedOn, period.MonthlySalaryUSD, period.MonthlySalaryRUB); err != nil {
			return domain.PersonProfession{}, mapErr(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PersonProfession{}, err
	}
	return s.GetPersonProfession(ctx, p.ID)
}

func (s *Store) DeletePersonProfession(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM person_professions WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type PersonProfessionRepo struct{ *Store }

func (r PersonProfessionRepo) Get(ctx context.Context, id uuid.UUID) (domain.PersonProfession, error) {
	return r.Store.GetPersonProfession(ctx, id)
}
func (r PersonProfessionRepo) ListByPerson(ctx context.Context, personID uuid.UUID) ([]domain.PersonProfession, error) {
	return r.Store.ListPersonProfessions(ctx, personID)
}
func (r PersonProfessionRepo) Create(ctx context.Context, profession domain.PersonProfession) (domain.PersonProfession, error) {
	return r.Store.CreatePersonProfession(ctx, profession)
}
func (r PersonProfessionRepo) Update(ctx context.Context, profession domain.PersonProfession) (domain.PersonProfession, error) {
	return r.Store.UpdatePersonProfession(ctx, profession)
}
func (r PersonProfessionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeletePersonProfession(ctx, id)
}
