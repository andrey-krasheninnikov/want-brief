package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

type SourceInput struct {
	Kind        string     `json:"kind"`
	Name        string     `json:"name"`
	BaseURL     string     `json:"baseUrl"`
	Token       string     `json:"token"`
	Email       string     `json:"email"`
	Query       string     `json:"query"`
	ProjectID   *uuid.UUID `json:"projectId"`
	InsecureTLS *bool      `json:"insecureTls"`
}

func (s *Service) ListSources(ctx context.Context) ([]domain.Source, error) {
	sources, err := s.Sources.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Source, 0, len(sources))
	for _, source := range sources {
		source.HasToken = source.TokenSealed != ""
		out = append(out, source.Public())
	}
	return out, nil
}

func (s *Service) CreateSource(ctx context.Context, in SourceInput) (domain.Source, error) {
	kind, err := domain.ParseSourceKind(in.Kind)
	if err != nil {
		return domain.Source{}, err
	}
	if kind == domain.SourceManual {
		return domain.Source{}, fmt.Errorf("%w: manual source is built-in", domain.ErrConflict)
	}
	projectID := uuid.Nil
	if in.ProjectID != nil {
		projectID = *in.ProjectID
	}
	source, err := domain.NewSource(kind, in.Name, in.BaseURL, in.Query, projectID, in.Email)
	if err != nil {
		return domain.Source{}, err
	}
	if in.InsecureTLS != nil {
		source.InsecureTLS = *in.InsecureTLS
	}
	if err := s.requireProject(ctx, source.ProjectID); err != nil {
		return domain.Source{}, err
	}
	token := strings.TrimSpace(in.Token)
	if token != "" {
		sealed, err := s.Tokens.Seal(token)
		if err != nil {
			return domain.Source{}, err
		}
		source.TokenSealed = sealed
		s.probeSource(ctx, &source, token)
	}
	created, err := s.Sources.Create(ctx, source)
	if err != nil {
		return domain.Source{}, err
	}
	created.HasToken = created.TokenSealed != ""
	return created.Public(), nil
}

func (s *Service) PatchSource(ctx context.Context, id uuid.UUID, in SourceInput) (domain.Source, error) {
	source, err := s.Sources.Get(ctx, id)
	if err != nil {
		return domain.Source{}, err
	}
	if source.Kind == domain.SourceManual {
		return domain.Source{}, fmt.Errorf("%w: manual source", domain.ErrForbidden)
	}
	urlChanged := false
	emailChanged := false
	tlsChanged := false
	if strings.TrimSpace(in.Name) != "" {
		source.Name = strings.TrimSpace(in.Name)
	}
	if in.BaseURL != "" {
		next := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
		urlChanged = next != source.BaseURL
		source.BaseURL = next
	}
	if source.Kind != domain.SourceJira && in.Query != "" {
		source.QueryFilter = in.Query
	}
	if in.ProjectID != nil {
		if err := s.requireProject(ctx, in.ProjectID); err != nil {
			return domain.Source{}, err
		}
		source.ProjectID = in.ProjectID
	}
	if strings.TrimSpace(in.Email) != "" {
		if source.Kind == domain.SourceJira {
			email, err := domain.NormalizeJiraEmail(in.Email)
			if err != nil {
				return domain.Source{}, err
			}
			emailChanged = email != source.Email
			source.Email = email
		}
	}
	if in.InsecureTLS != nil && *in.InsecureTLS != source.InsecureTLS {
		source.InsecureTLS = *in.InsecureTLS
		tlsChanged = true
	}
	token := strings.TrimSpace(in.Token)
	if token != "" {
		sealed, err := s.Tokens.Seal(token)
		if err != nil {
			return domain.Source{}, err
		}
		source.TokenSealed = sealed
	}
	source.UpdatedAt = s.now()
	if token != "" || urlChanged || emailChanged || tlsChanged {
		plain := token
		if plain == "" && source.TokenSealed != "" {
			opened, err := s.Tokens.Open(source.TokenSealed)
			if err != nil {
				return domain.Source{}, fmt.Errorf("open token: %w", err)
			}
			plain = opened
		}
		if plain != "" {
			s.probeSource(ctx, &source, plain)
		}
	}
	updated, err := s.Sources.Update(ctx, source)
	if err != nil {
		return domain.Source{}, err
	}
	updated.HasToken = updated.TokenSealed != ""
	return updated.Public(), nil
}

func (s *Service) DeleteSource(ctx context.Context, id uuid.UUID) error {
	source, err := s.Sources.Get(ctx, id)
	if err != nil {
		return err
	}
	if source.Kind == domain.SourceManual {
		return fmt.Errorf("%w: manual source", domain.ErrForbidden)
	}
	return s.Sources.Delete(ctx, id)
}

func (s *Service) SyncSource(ctx context.Context, id uuid.UUID) (int, error) {
	source, err := s.Sources.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	if source.Kind == domain.SourceManual {
		return 0, fmt.Errorf("%w: manual source", domain.ErrForbidden)
	}
	if source.ProjectID == nil {
		return 0, fmt.Errorf("%w: source project", domain.ErrInvalid)
	}
	puller, ok := s.Pullers[source.Kind]
	if !ok {
		return 0, domain.ErrUnsupported
	}
	if source.TokenSealed == "" {
		return 0, domain.ErrNoToken
	}
	token, err := s.Tokens.Open(source.TokenSealed)
	if err != nil {
		return 0, fmt.Errorf("open token: %w", err)
	}
	if err := puller.Probe(ctx, source, token); err != nil {
		s.markDisconnected(ctx, &source, err)
		return 0, err
	}
	remote, err := puller.Pull(ctx, source, token)
	if err != nil {
		s.markDisconnected(ctx, &source, err)
		return 0, err
	}
	count := 0
	for _, remoteItem := range remote {
		item := domain.Item{
			SourceID:    source.ID,
			ExternalKey: remoteItem.ExternalKey,
			Title:       remoteItem.Title,
			Status:      domain.StatusBacklog,
			Kind:        domain.KindTask,
			ProjectID:   source.ProjectID,
			Urgent:      remoteItem.HintUrgent,
			Important:   remoteItem.HintImportant,
			DueAt:       remoteItem.DueAt,
			Links:       remoteLinks(source.Kind, remoteItem.URL),
			UpdatedAt:   s.now(),
			CreatedAt:   s.now(),
		}
		if remoteItem.CreatedAt != nil {
			item.CreatedAt = remoteItem.CreatedAt.UTC()
		}
		upserted, err := s.Items.UpsertSynced(ctx, item)
		if err != nil {
			return count, err
		}
		if upserted.DeletedAt != nil {
			continue
		}
		count++
	}
	now := s.now()
	source.LastSyncAt = &now
	source.Connected = true
	source.LastError = ""
	source.UpdatedAt = now
	if _, err := s.Sources.Update(ctx, source); err != nil {
		return count, err
	}
	return count, nil
}

func (s *Service) requireProject(ctx context.Context, id *uuid.UUID) error {
	if id == nil || *id == uuid.Nil {
		return fmt.Errorf("%w: source project", domain.ErrInvalid)
	}
	_, err := s.Projects.Get(ctx, *id)
	return err
}

func (s *Service) probeSource(ctx context.Context, source *domain.Source, token string) {
	puller, ok := s.Pullers[source.Kind]
	if !ok {
		source.Connected = false
		source.LastError = domain.ErrUnsupported.Error()
		return
	}
	if err := puller.Probe(ctx, *source, token); err != nil {
		source.Connected = false
		source.LastError = err.Error()
		return
	}
	source.Connected = true
	source.LastError = ""
}

func (s *Service) markDisconnected(ctx context.Context, source *domain.Source, cause error) {
	source.Connected = false
	source.LastError = cause.Error()
	source.UpdatedAt = s.now()
	_, _ = s.Sources.Update(ctx, *source)
}

func remoteLinks(kind domain.SourceKind, remoteURL string) []domain.ProjectLink {
	if strings.TrimSpace(remoteURL) == "" {
		return []domain.ProjectLink{}
	}
	label := "Jira"
	if kind == domain.SourceTodoist {
		label = "Todoist"
	}
	links, err := domain.NormalizeLinks([]domain.ProjectLink{{Label: label, URL: remoteURL}})
	if err != nil {
		return []domain.ProjectLink{}
	}
	return links
}
