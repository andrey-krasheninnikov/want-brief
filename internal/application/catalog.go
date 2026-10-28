package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

type ProjectWrite struct {
	Name             string
	Color            string
	Description      string
	MonthlyIncomeUSD float64
	MonthlyIncomeRUB float64
	TargetHoursDay   float64
	Links            []domain.ProjectLink
	People           []domain.PersonRel
	Companies        []domain.PersonRel
}

type ProjectPatch struct {
	Name             *string
	Color            *string
	Description      *string
	MonthlyIncomeUSD *float64
	MonthlyIncomeRUB *float64
	TargetHoursDay   *float64
	Links            *[]domain.ProjectLink
	People           *[]domain.PersonRel
	Companies        *[]domain.PersonRel
	Archived         *bool
}

func (s *Service) ListProjects(ctx context.Context) ([]domain.Project, error) {
	return s.Projects.List(ctx)
}

func (s *Service) GetProject(ctx context.Context, id uuid.UUID) (domain.Project, error) {
	return s.Projects.Get(ctx, id)
}

func (s *Service) CreateProject(ctx context.Context, in ProjectWrite) (domain.Project, error) {
	project, err := domain.NewProject(in.Name, in.Color, in.TargetHoursDay)
	if err != nil {
		return domain.Project{}, err
	}
	if err := project.SetMonthlyIncomeUSD(in.MonthlyIncomeUSD); err != nil {
		return domain.Project{}, err
	}
	if err := project.SetMonthlyIncomeRUB(in.MonthlyIncomeRUB); err != nil {
		return domain.Project{}, err
	}
	project.Description = strings.TrimSpace(in.Description)
	project.Links, err = domain.NormalizeLinks(in.Links)
	if err != nil {
		return domain.Project{}, err
	}
	people, err := domain.NormalizePersonRels(in.People)
	if err != nil {
		return domain.Project{}, err
	}
	project.People = people
	companies, err := domain.NormalizePersonRels(in.Companies)
	if err != nil {
		return domain.Project{}, err
	}
	project.Companies = companies
	return s.Projects.Create(ctx, project)
}

func (s *Service) PatchProject(ctx context.Context, id uuid.UUID, in ProjectPatch) (domain.Project, error) {
	project, err := s.Projects.Get(ctx, id)
	if err != nil {
		return domain.Project{}, err
	}
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return domain.Project{}, fmt.Errorf("%w: project name", domain.ErrInvalid)
		}
		project.Name = trimmed
	}
	if in.Color != nil && *in.Color != "" {
		project.Color = *in.Color
	}
	if in.Description != nil {
		project.Description = strings.TrimSpace(*in.Description)
	}
	if in.MonthlyIncomeUSD != nil {
		if err := project.SetMonthlyIncomeUSD(*in.MonthlyIncomeUSD); err != nil {
			return domain.Project{}, err
		}
	}
	if in.MonthlyIncomeRUB != nil {
		if err := project.SetMonthlyIncomeRUB(*in.MonthlyIncomeRUB); err != nil {
			return domain.Project{}, err
		}
	}
	if in.TargetHoursDay != nil {
		if err := project.SetTargetHoursDay(*in.TargetHoursDay); err != nil {
			return domain.Project{}, err
		}
	}
	if in.Links != nil {
		project.Links, err = domain.NormalizeLinks(*in.Links)
		if err != nil {
			return domain.Project{}, err
		}
	}
	if in.People != nil {
		people, err := domain.NormalizePersonRels(*in.People)
		if err != nil {
			return domain.Project{}, err
		}
		project.People = people
	}
	if in.Companies != nil {
		companies, err := domain.NormalizePersonRels(*in.Companies)
		if err != nil {
			return domain.Project{}, err
		}
		project.Companies = companies
	}
	now := s.now()
	if in.Archived != nil {
		if *in.Archived {
			project.Archive(now)
		} else {
			project.Restore(now)
		}
	}
	project.UpdatedAt = now
	return s.Projects.Update(ctx, project)
}

func (s *Service) ListProjectNotes(ctx context.Context, projectID uuid.UUID) ([]domain.ProjectNote, error) {
	if _, err := s.Projects.Get(ctx, projectID); err != nil {
		return nil, err
	}
	return s.ProjectNotes.ListByProject(ctx, projectID)
}

func (s *Service) CreateProjectNote(ctx context.Context, projectID uuid.UUID, body string) (domain.ProjectNote, error) {
	if _, err := s.Projects.Get(ctx, projectID); err != nil {
		return domain.ProjectNote{}, err
	}
	note, err := domain.NewProjectNote(projectID, body)
	if err != nil {
		return domain.ProjectNote{}, err
	}
	return s.ProjectNotes.Create(ctx, note)
}

func (s *Service) DeleteProjectNote(ctx context.Context, projectID, noteID uuid.UUID) error {
	note, err := s.ProjectNotes.Get(ctx, noteID)
	if err != nil {
		return err
	}
	if note.ProjectID != projectID {
		return domain.ErrNotFound
	}
	return s.ProjectNotes.Delete(ctx, noteID)
}

func (s *Service) DeleteProject(ctx context.Context, id uuid.UUID) error {
	return s.Projects.Delete(ctx, id)
}

func (s *Service) ListNotes(ctx context.Context, itemID *uuid.UUID) ([]domain.Note, error) {
	return s.Notes.List(ctx, itemID)
}

func (s *Service) CreateNote(ctx context.Context, body string, itemID *uuid.UUID) (domain.Note, error) {
	note, err := domain.NewNote(body, itemID)
	if err != nil {
		return domain.Note{}, err
	}
	return s.Notes.Create(ctx, note)
}

func (s *Service) PatchNote(ctx context.Context, id uuid.UUID, body string) (domain.Note, error) {
	note, err := s.Notes.Get(ctx, id)
	if err != nil {
		return domain.Note{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return domain.Note{}, fmt.Errorf("%w: note body", domain.ErrInvalid)
	}
	note.Body = body
	note.UpdatedAt = s.now()
	return s.Notes.Update(ctx, note)
}

func (s *Service) DeleteNote(ctx context.Context, id uuid.UUID) error {
	return s.Notes.Delete(ctx, id)
}
