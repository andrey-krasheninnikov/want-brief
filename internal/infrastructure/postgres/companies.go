package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

const companyCols = `id, name, description, links, started_on, ended_on, created_at, updated_at`
const companyTitleCols = `id, company_id, title, started_on, ended_on`
const companySalaryCols = `id, company_id, currency, amount, comment, started_on, ended_on`
const companyManagerCols = `id, company_id, person_id, started_on, ended_on`
const companyReportCols = `id, company_id, person_id, started_on, ended_on`
const companyContractCols = `id, company_id, person_id, kind, started_on, ended_on`

func scanCompany(scan func(dest ...any) error) (domain.Company, error) {
	var c domain.Company
	var raw []byte
	err := scan(&c.ID, &c.Name, &c.Description, &raw, &c.StartedOn, &c.EndedOn, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, err
	}
	c.Links, err = decodeProjectLinks(raw)
	return c, err
}

func (s *Store) GetCompany(ctx context.Context, id uuid.UUID) (domain.Company, error) {
	c, err := scanCompany(s.pool.QueryRow(ctx, `SELECT `+companyCols+` FROM companies WHERE id=$1`, id).Scan)
	if err != nil {
		return domain.Company{}, mapErr(err)
	}
	out := []domain.Company{c}
	if err := s.attachCompanies(ctx, out); err != nil {
		return domain.Company{}, err
	}
	return out[0].WithTenure(time.Now().UTC()), nil
}

func (s *Store) ListCompanies(ctx context.Context) ([]domain.Company, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+companyCols+` FROM companies ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Company{}
	for rows.Next() {
		c, err := scanCompany(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachCompanies(ctx, out); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for i := range out {
		out[i] = out[i].WithTenure(now)
	}
	return out, nil
}

func (s *Store) CreateCompany(ctx context.Context, c domain.Company) (domain.Company, error) {
	return s.saveCompany(ctx, c, true)
}

func (s *Store) UpdateCompany(ctx context.Context, c domain.Company) (domain.Company, error) {
	return s.saveCompany(ctx, c, false)
}

func (s *Store) saveCompany(ctx context.Context, c domain.Company, insert bool) (domain.Company, error) {
	raw, err := encodeProjectLinks(c.Links)
	if err != nil {
		return domain.Company{}, err
	}
	snapshot := c
	if insert {
		c, err = scanCompany(s.pool.QueryRow(ctx, `
			INSERT INTO companies (`+companyCols+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			RETURNING `+companyCols+`
		`, c.ID, c.Name, c.Description, raw, c.StartedOn, c.EndedOn, c.CreatedAt, c.UpdatedAt).Scan)
	} else {
		c, err = scanCompany(s.pool.QueryRow(ctx, `
			UPDATE companies
			SET name=$2, description=$3, links=$4, started_on=$5, ended_on=$6, updated_at=$7
			WHERE id=$1
			RETURNING `+companyCols+`
		`, c.ID, c.Name, c.Description, raw, c.StartedOn, c.EndedOn, c.UpdatedAt).Scan)
	}
	if err != nil {
		return domain.Company{}, mapErr(err)
	}
	c.Projects = snapshot.Projects
	c.Events = snapshot.Events
	c.People = snapshot.People
	c.Titles = snapshot.Titles
	c.Salaries = snapshot.Salaries
	c.Managers = snapshot.Managers
	c.Reports = snapshot.Reports
	c.Contracts = snapshot.Contracts
	if err := s.replaceCompanyLinks(ctx, c); err != nil {
		return domain.Company{}, err
	}
	if err := s.replaceCompanyCareer(ctx, c); err != nil {
		return domain.Company{}, err
	}
	return s.GetCompany(ctx, c.ID)
}

func (s *Store) DeleteCompany(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM companies WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) attachCompanies(ctx context.Context, rows []domain.Company) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
		index[row.ID] = i
		rows[i].Projects = []domain.PersonRel{}
		rows[i].Events = []domain.PersonRel{}
		rows[i].People = []domain.PersonRel{}
		rows[i].Titles = []domain.CompanyTitle{}
		rows[i].Salaries = []domain.CompanySalary{}
		rows[i].Managers = []domain.CompanyManager{}
		rows[i].Reports = []domain.CompanyReport{}
		rows[i].Contracts = []domain.CompanyContract{}
	}
	projects, err := s.relsByOwners(ctx, "company_projects", "company_id", "project_id", ids)
	if err != nil {
		return err
	}
	events, err := s.relsByOwners(ctx, "company_events", "company_id", "event_id", ids)
	if err != nil {
		return err
	}
	people, err := s.relsByOwners(ctx, "company_people", "company_id", "person_id", ids)
	if err != nil {
		return err
	}
	for id, list := range projects {
		rows[index[id]].Projects = list
	}
	for id, list := range events {
		rows[index[id]].Events = list
	}
	for id, list := range people {
		rows[index[id]].People = list
	}
	return s.attachCompanyCareer(ctx, rows, index, ids)
}

func (s *Store) attachCompanyCareer(ctx context.Context, rows []domain.Company, index map[uuid.UUID]int, ids []uuid.UUID) error {
	titles, err := s.pool.Query(ctx, `
		SELECT `+companyTitleCols+` FROM company_titles
		WHERE company_id = ANY($1)
		ORDER BY started_on, id
	`, ids)
	if err != nil {
		return err
	}
	defer titles.Close()
	for titles.Next() {
		var row domain.CompanyTitle
		if err := titles.Scan(&row.ID, &row.CompanyID, &row.Title, &row.StartedOn, &row.EndedOn); err != nil {
			return err
		}
		rows[index[row.CompanyID]].Titles = append(rows[index[row.CompanyID]].Titles, row)
	}
	if err := titles.Err(); err != nil {
		return err
	}

	salaries, err := s.pool.Query(ctx, `
		SELECT `+companySalaryCols+` FROM company_salaries
		WHERE company_id = ANY($1)
		ORDER BY started_on, id
	`, ids)
	if err != nil {
		return err
	}
	defer salaries.Close()
	for salaries.Next() {
		var row domain.CompanySalary
		if err := salaries.Scan(&row.ID, &row.CompanyID, &row.Currency, &row.Amount, &row.Comment, &row.StartedOn, &row.EndedOn); err != nil {
			return err
		}
		rows[index[row.CompanyID]].Salaries = append(rows[index[row.CompanyID]].Salaries, row)
	}
	if err := salaries.Err(); err != nil {
		return err
	}

	managers, err := s.pool.Query(ctx, `
		SELECT `+companyManagerCols+` FROM company_managers
		WHERE company_id = ANY($1)
		ORDER BY started_on, id
	`, ids)
	if err != nil {
		return err
	}
	defer managers.Close()
	for managers.Next() {
		var row domain.CompanyManager
		if err := managers.Scan(&row.ID, &row.CompanyID, &row.PersonID, &row.StartedOn, &row.EndedOn); err != nil {
			return err
		}
		rows[index[row.CompanyID]].Managers = append(rows[index[row.CompanyID]].Managers, row)
	}
	if err := managers.Err(); err != nil {
		return err
	}

	reports, err := s.pool.Query(ctx, `
		SELECT `+companyReportCols+` FROM company_reports
		WHERE company_id = ANY($1)
		ORDER BY started_on, id
	`, ids)
	if err != nil {
		return err
	}
	defer reports.Close()
	for reports.Next() {
		var row domain.CompanyReport
		if err := reports.Scan(&row.ID, &row.CompanyID, &row.PersonID, &row.StartedOn, &row.EndedOn); err != nil {
			return err
		}
		rows[index[row.CompanyID]].Reports = append(rows[index[row.CompanyID]].Reports, row)
	}
	if err := reports.Err(); err != nil {
		return err
	}

	contracts, err := s.pool.Query(ctx, `
		SELECT `+companyContractCols+` FROM company_contracts
		WHERE company_id = ANY($1)
		ORDER BY started_on, id
	`, ids)
	if err != nil {
		return err
	}
	defer contracts.Close()
	for contracts.Next() {
		var row domain.CompanyContract
		if err := contracts.Scan(&row.ID, &row.CompanyID, &row.PersonID, &row.Kind, &row.StartedOn, &row.EndedOn); err != nil {
			return err
		}
		rows[index[row.CompanyID]].Contracts = append(rows[index[row.CompanyID]].Contracts, row)
	}
	return contracts.Err()
}

func (s *Store) replaceCompanyLinks(ctx context.Context, c domain.Company) error {
	if err := s.replaceRels(ctx, "company_projects", "company_id", "project_id", c.ID, c.Projects); err != nil {
		return err
	}
	if err := s.replaceRels(ctx, "company_events", "company_id", "event_id", c.ID, c.Events); err != nil {
		return err
	}
	return s.replaceRels(ctx, "company_people", "company_id", "person_id", c.ID, c.People)
}

func (s *Store) replaceCompanyCareer(ctx context.Context, c domain.Company) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM company_titles WHERE company_id=$1`, c.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM company_salaries WHERE company_id=$1`, c.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM company_managers WHERE company_id=$1`, c.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM company_reports WHERE company_id=$1`, c.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM company_contracts WHERE company_id=$1`, c.ID); err != nil {
		return err
	}
	for _, row := range c.Titles {
		if row.ID == uuid.Nil {
			row.ID = uuid.New()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO company_titles (`+companyTitleCols+`) VALUES ($1,$2,$3,$4,$5)
		`, row.ID, c.ID, row.Title, row.StartedOn, row.EndedOn); err != nil {
			return mapErr(err)
		}
	}
	for _, row := range c.Salaries {
		if row.ID == uuid.Nil {
			row.ID = uuid.New()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO company_salaries (`+companySalaryCols+`) VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, row.ID, c.ID, row.Currency, row.Amount, row.Comment, row.StartedOn, row.EndedOn); err != nil {
			return mapErr(err)
		}
	}
	for _, row := range c.Managers {
		if row.ID == uuid.Nil {
			row.ID = uuid.New()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO company_managers (`+companyManagerCols+`) VALUES ($1,$2,$3,$4,$5)
		`, row.ID, c.ID, row.PersonID, row.StartedOn, row.EndedOn); err != nil {
			return mapErr(err)
		}
	}
	for _, row := range c.Reports {
		if row.ID == uuid.Nil {
			row.ID = uuid.New()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO company_reports (`+companyReportCols+`) VALUES ($1,$2,$3,$4,$5)
		`, row.ID, c.ID, row.PersonID, row.StartedOn, row.EndedOn); err != nil {
			return mapErr(err)
		}
	}
	for _, row := range c.Contracts {
		if row.ID == uuid.Nil {
			row.ID = uuid.New()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO company_contracts (`+companyContractCols+`) VALUES ($1,$2,$3,$4,$5,$6)
		`, row.ID, c.ID, row.PersonID, row.Kind, row.StartedOn, row.EndedOn); err != nil {
			return mapErr(err)
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) GetCompanyNote(ctx context.Context, id uuid.UUID) (domain.CompanyNote, error) {
	var n domain.CompanyNote
	err := s.pool.QueryRow(ctx, `
		SELECT id, company_id, body, created_at FROM company_notes WHERE id=$1
	`, id).Scan(&n.ID, &n.CompanyID, &n.Body, &n.CreatedAt)
	return n, mapErr(err)
}

func (s *Store) ListCompanyNotes(ctx context.Context, companyID uuid.UUID) ([]domain.CompanyNote, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, company_id, body, created_at
		FROM company_notes WHERE company_id=$1
		ORDER BY created_at DESC
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.CompanyNote{}
	for rows.Next() {
		var n domain.CompanyNote
		if err := rows.Scan(&n.ID, &n.CompanyID, &n.Body, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CreateCompanyNote(ctx context.Context, n domain.CompanyNote) (domain.CompanyNote, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO company_notes (id, company_id, body, created_at)
		VALUES ($1,$2,$3,$4)
		RETURNING id, company_id, body, created_at
	`, n.ID, n.CompanyID, n.Body, n.CreatedAt).
		Scan(&n.ID, &n.CompanyID, &n.Body, &n.CreatedAt)
	return n, err
}

func (s *Store) UpdateCompanyNote(ctx context.Context, n domain.CompanyNote) (domain.CompanyNote, error) {
	err := s.pool.QueryRow(ctx, `
		UPDATE company_notes SET body=$2 WHERE id=$1
		RETURNING id, company_id, body, created_at
	`, n.ID, n.Body).Scan(&n.ID, &n.CompanyID, &n.Body, &n.CreatedAt)
	return n, mapErr(err)
}

func (s *Store) DeleteCompanyNote(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM company_notes WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type CompanyRepo struct{ *Store }

func (r CompanyRepo) Get(ctx context.Context, id uuid.UUID) (domain.Company, error) {
	return r.Store.GetCompany(ctx, id)
}
func (r CompanyRepo) List(ctx context.Context) ([]domain.Company, error) {
	return r.Store.ListCompanies(ctx)
}
func (r CompanyRepo) Create(ctx context.Context, company domain.Company) (domain.Company, error) {
	return r.Store.CreateCompany(ctx, company)
}
func (r CompanyRepo) Update(ctx context.Context, company domain.Company) (domain.Company, error) {
	return r.Store.UpdateCompany(ctx, company)
}
func (r CompanyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteCompany(ctx, id)
}

type CompanyNoteRepo struct{ *Store }

func (r CompanyNoteRepo) Get(ctx context.Context, id uuid.UUID) (domain.CompanyNote, error) {
	return r.Store.GetCompanyNote(ctx, id)
}
func (r CompanyNoteRepo) ListByCompany(ctx context.Context, companyID uuid.UUID) ([]domain.CompanyNote, error) {
	return r.Store.ListCompanyNotes(ctx, companyID)
}
func (r CompanyNoteRepo) Create(ctx context.Context, note domain.CompanyNote) (domain.CompanyNote, error) {
	return r.Store.CreateCompanyNote(ctx, note)
}
func (r CompanyNoteRepo) Update(ctx context.Context, note domain.CompanyNote) (domain.CompanyNote, error) {
	return r.Store.UpdateCompanyNote(ctx, note)
}
func (r CompanyNoteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteCompanyNote(ctx, id)
}
