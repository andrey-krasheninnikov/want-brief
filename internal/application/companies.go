package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

type CompanyWrite struct {
	Name        string
	Description string
	Links       []domain.ProjectLink
	Projects    []domain.PersonRel
	Events      []domain.PersonRel
	People      []domain.PersonRel
	StartedOn   *time.Time
	EndedOn     *time.Time
}

func (s *Service) ListCompanies(ctx context.Context) ([]domain.Company, error) {
	return s.Companies.List(ctx)
}

func (s *Service) GetCompany(ctx context.Context, id uuid.UUID) (domain.Company, error) {
	return s.Companies.Get(ctx, id)
}

func (s *Service) CreateCompany(ctx context.Context, write CompanyWrite) (domain.Company, error) {
	company, err := domain.NewCompany(draftFromCompany(write))
	if err != nil {
		return domain.Company{}, err
	}
	return s.Companies.Create(ctx, company)
}

func (s *Service) ReplaceCompany(ctx context.Context, id uuid.UUID, write CompanyWrite) (domain.Company, error) {
	company, err := s.Companies.Get(ctx, id)
	if err != nil {
		return domain.Company{}, err
	}
	if err := company.Apply(draftFromCompany(write)); err != nil {
		return domain.Company{}, err
	}
	return s.Companies.Update(ctx, company)
}

func (s *Service) DeleteCompany(ctx context.Context, id uuid.UUID) error {
	return s.Companies.Delete(ctx, id)
}

func (s *Service) AddCompanyTitle(ctx context.Context, id uuid.UUID, title string, startedOn *time.Time) (domain.Company, error) {
	company, err := s.Companies.Get(ctx, id)
	if err != nil {
		return domain.Company{}, err
	}
	if err := company.SetTitle(title, s.dayOrToday(startedOn)); err != nil {
		return domain.Company{}, err
	}
	return s.Companies.Update(ctx, company)
}

func (s *Service) AddCompanySalary(ctx context.Context, id uuid.UUID, currency string, amount float64, startedOn *time.Time, comment string) (domain.Company, error) {
	company, err := s.Companies.Get(ctx, id)
	if err != nil {
		return domain.Company{}, err
	}
	kind, err := domain.ParseSalaryCurrency(currency)
	if err != nil {
		return domain.Company{}, err
	}
	if err := company.SetSalary(kind, amount, s.dayOrToday(startedOn), comment); err != nil {
		return domain.Company{}, err
	}
	return s.Companies.Update(ctx, company)
}

func (s *Service) SetCompanyManager(ctx context.Context, id uuid.UUID, personID *uuid.UUID, startedOn *time.Time) (domain.Company, error) {
	company, err := s.Companies.Get(ctx, id)
	if err != nil {
		return domain.Company{}, err
	}
	if personID != nil {
		if _, err := s.People.Get(ctx, *personID); err != nil {
			return domain.Company{}, err
		}
	}
	if err := company.SetManager(personID, s.dayOrToday(startedOn)); err != nil {
		return domain.Company{}, err
	}
	return s.Companies.Update(ctx, company)
}

func (s *Service) AddCompanyReport(ctx context.Context, id, personID uuid.UUID, startedOn *time.Time) (domain.Company, error) {
	company, err := s.Companies.Get(ctx, id)
	if err != nil {
		return domain.Company{}, err
	}
	if _, err := s.People.Get(ctx, personID); err != nil {
		return domain.Company{}, err
	}
	if err := company.AddReport(personID, s.dayOrToday(startedOn)); err != nil {
		return domain.Company{}, err
	}
	return s.Companies.Update(ctx, company)
}

func (s *Service) EndCompanyReport(ctx context.Context, id, reportID uuid.UUID, endedOn *time.Time) (domain.Company, error) {
	company, err := s.Companies.Get(ctx, id)
	if err != nil {
		return domain.Company{}, err
	}
	if err := company.EndReport(reportID, s.dayOrToday(endedOn)); err != nil {
		return domain.Company{}, err
	}
	return s.Companies.Update(ctx, company)
}

func (s *Service) SetCompanyContract(ctx context.Context, id uuid.UUID, personID *uuid.UUID, kind string, startedOn *time.Time) (domain.Company, error) {
	company, err := s.Companies.Get(ctx, id)
	if err != nil {
		return domain.Company{}, err
	}
	parsed, err := domain.ParseContractKind(kind)
	if err != nil {
		return domain.Company{}, err
	}
	if personID != nil {
		if _, err := s.People.Get(ctx, *personID); err != nil {
			return domain.Company{}, err
		}
	}
	if err := company.SetContract(personID, parsed, s.dayOrToday(startedOn)); err != nil {
		return domain.Company{}, err
	}
	return s.Companies.Update(ctx, company)
}

func (s *Service) ListCompanyNotes(ctx context.Context, companyID uuid.UUID) ([]domain.CompanyNote, error) {
	if _, err := s.Companies.Get(ctx, companyID); err != nil {
		return nil, err
	}
	return s.CompanyNotes.ListByCompany(ctx, companyID)
}

func (s *Service) CreateCompanyNote(ctx context.Context, companyID uuid.UUID, body string) (domain.CompanyNote, error) {
	if _, err := s.Companies.Get(ctx, companyID); err != nil {
		return domain.CompanyNote{}, err
	}
	note, err := domain.NewCompanyNote(companyID, body)
	if err != nil {
		return domain.CompanyNote{}, err
	}
	return s.CompanyNotes.Create(ctx, note)
}

func (s *Service) ReplaceCompanyNote(ctx context.Context, companyID, noteID uuid.UUID, body string) (domain.CompanyNote, error) {
	note, err := s.CompanyNotes.Get(ctx, noteID)
	if err != nil {
		return domain.CompanyNote{}, err
	}
	if note.CompanyID != companyID {
		return domain.CompanyNote{}, domain.ErrNotFound
	}
	if err := note.Apply(body); err != nil {
		return domain.CompanyNote{}, err
	}
	return s.CompanyNotes.Update(ctx, note)
}

func (s *Service) DeleteCompanyNote(ctx context.Context, companyID, noteID uuid.UUID) error {
	note, err := s.CompanyNotes.Get(ctx, noteID)
	if err != nil {
		return err
	}
	if note.CompanyID != companyID {
		return domain.ErrNotFound
	}
	return s.CompanyNotes.Delete(ctx, noteID)
}

func (s *Service) dayOrToday(day *time.Time) time.Time {
	if day != nil {
		return *day
	}
	return domain.MoscowDate(s.now())
}

func draftFromCompany(write CompanyWrite) domain.CompanyDraft {
	return domain.CompanyDraft{
		Name:        write.Name,
		Description: write.Description,
		Links:       write.Links,
		Projects:    write.Projects,
		Events:      write.Events,
		People:      write.People,
		StartedOn:   write.StartedOn,
		EndedOn:     write.EndedOn,
	}
}
