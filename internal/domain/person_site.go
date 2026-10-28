package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SiteKind string

const (
	SiteKindPersonal        SiteKind = "personal_site"
	SiteKindCompany         SiteKind = "company_site"
	SiteKindGitHub          SiteKind = "github"
	SiteKindLinkedIn        SiteKind = "linkedin"
	SiteKindYouTube         SiteKind = "youtube"
	SiteKindTelegramChannel SiteKind = "telegram_channel"
	SiteKindOther           SiteKind = "other"
)

type PersonSite struct {
	ID        uuid.UUID `json:"id"`
	PersonID  uuid.UUID `json:"personId"`
	Kind      SiteKind  `json:"kind"`
	URL       string    `json:"url"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PersonSiteDraft struct {
	Kind    string
	URL     string
	Comment string
}

func ParseSiteKind(raw string) (SiteKind, error) {
	kind := SiteKind(strings.TrimSpace(raw))
	switch kind {
	case SiteKindPersonal, SiteKindCompany, SiteKindGitHub, SiteKindLinkedIn, SiteKindYouTube, SiteKindTelegramChannel, SiteKindOther:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: site kind", ErrInvalid)
	}
}

func NewPersonSite(personID uuid.UUID, draft PersonSiteDraft) (PersonSite, error) {
	now := time.Now().UTC()
	site := PersonSite{
		ID:        uuid.New(),
		PersonID:  personID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := site.apply(draft); err != nil {
		return PersonSite{}, err
	}
	return site, nil
}

func (s *PersonSite) Apply(draft PersonSiteDraft) error {
	if err := s.apply(draft); err != nil {
		return err
	}
	s.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *PersonSite) apply(draft PersonSiteDraft) error {
	if s.PersonID == uuid.Nil {
		return fmt.Errorf("%w: person", ErrInvalid)
	}
	kind, err := ParseSiteKind(draft.Kind)
	if err != nil {
		return err
	}
	parsed, err := parseProjectURL(draft.URL)
	if err != nil {
		return err
	}
	s.Kind = kind
	s.URL = parsed
	s.Comment = strings.TrimSpace(draft.Comment)
	return nil
}
