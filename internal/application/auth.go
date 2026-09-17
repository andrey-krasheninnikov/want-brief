package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	Users             domain.UserRepository
	Sessions          domain.SessionRepository
	Projects          domain.ProjectRepository
	Sources           domain.SourceRepository
	Items             domain.ItemRepository
	Notes             domain.NoteRepository
	Intervals         domain.IntervalRepository
	Stress            domain.StressRepository
	Events            domain.EventRepository
	EventOverrides    domain.EventOverrideRepository
	EventNotes        domain.EventNoteRepository
	Companies         domain.CompanyRepository
	CompanyNotes      domain.CompanyNoteRepository
	People            domain.PersonRepository
	PersonNotes       domain.PersonNoteRepository
	PersonContacts    domain.PersonContactRepository
	PersonSites       domain.PersonSiteRepository
	PersonBonds       domain.PersonBondRepository
	PersonProfessions domain.PersonProfessionRepository
	PersonAbsences    domain.PersonAbsenceRepository
	ScheduleSettings  domain.ScheduleSettingsRepository
	DayOverrides      domain.DayOverrideRepository
	Snapshots         domain.ScheduleSnapshotRepository
	ProjectNotes      domain.ProjectNoteRepository
	ItemNotes         domain.ItemNoteRepository
	ItemChecks        domain.ItemCheckRepository
	ItemEvents        domain.ItemEventRepository
	Journal           domain.JournalRepository
	Tokens            domain.TokenBox
	Pullers           map[domain.SourceKind]domain.Puller
	Now               func() time.Time
	Password          string
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) EnsureReady(ctx context.Context) error {
	count, err := s.Users.Count(ctx)
	if err != nil {
		return err
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(s.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		_, err = s.Users.Create(ctx, domain.User{
			ID:           uuid.New(),
			PasswordHash: string(hash),
			CreatedAt:    s.now(),
		})
		if err != nil {
			return err
		}
	}

	if _, err := s.Sources.Manual(ctx); err != nil {
		if err != domain.ErrNotFound {
			return err
		}
		manual, err := domain.NewSource(domain.SourceManual, "Manual", "", "", uuid.Nil, "")
		if err != nil {
			return err
		}
		if _, err := s.Sources.Create(ctx, manual); err != nil {
			return err
		}
	}

	projects, err := s.Projects.List(ctx)
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		work, err := domain.NewProject("Work", "#4C4CFF", 30.0/7)
		if err != nil {
			return err
		}
		if _, err := s.Projects.Create(ctx, work); err != nil {
			return err
		}
		life, err := domain.NewProject("Life", "#7C8CFF", 10.0/7)
		if err != nil {
			return err
		}
		if _, err := s.Projects.Create(ctx, life); err != nil {
			return err
		}
	}
	return s.Sessions.DeleteExpired(ctx, s.now())
}

func (s *Service) Login(ctx context.Context, password string) (string, error) {
	user, err := s.Users.First(ctx)
	if err != nil {
		return "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", domain.ErrUnauthorized
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("session token: %w", err)
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	session := domain.Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hex.EncodeToString(sum[:]),
		ExpiresAt: s.now().Add(30 * 24 * time.Hour),
		CreatedAt: s.now(),
	}
	if err := s.Sessions.Create(ctx, session); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	sum := sha256.Sum256([]byte(token))
	return s.Sessions.Delete(ctx, hex.EncodeToString(sum[:]))
}

func (s *Service) Authenticate(ctx context.Context, token string) error {
	if token == "" {
		return domain.ErrUnauthorized
	}
	sum := sha256.Sum256([]byte(token))
	session, err := s.Sessions.GetByTokenHash(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		return domain.ErrUnauthorized
	}
	if !session.ExpiresAt.After(s.now()) {
		return domain.ErrUnauthorized
	}
	return nil
}
