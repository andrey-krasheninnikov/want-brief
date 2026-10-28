package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SourceKind string

const (
	SourceJira    SourceKind = "jira"
	SourceTodoist SourceKind = "todoist"
	SourceManual  SourceKind = "manual"
)

type Source struct {
	ID          uuid.UUID  `json:"id"`
	Kind        SourceKind `json:"kind"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	BaseURL     string     `json:"baseUrl"`
	TokenSealed string     `json:"-"`
	HasToken    bool       `json:"hasToken"`
	QueryFilter string     `json:"query"`
	ProjectID   *uuid.UUID `json:"projectId"`
	Connected   bool       `json:"connected"`
	InsecureTLS bool       `json:"insecureTls"`
	LastError   string     `json:"lastError,omitempty"`
	LastSyncAt  *time.Time `json:"lastSyncAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type RemoteItem struct {
	ExternalKey    string
	Title          string
	Status         ItemStatus
	ExternalStatus string
	Description    string
	Comments       []RemoteComment
	DueAt          *time.Time
	CreatedAt      *time.Time
	URL            string
	HintUrgent     bool
	HintImportant  bool
}

type RemoteComment struct {
	ExternalID string
	Author     string
	Body       string
	URL        string
	CreatedAt  *time.Time
}

func ParseSourceKind(raw string) (SourceKind, error) {
	kind := SourceKind(raw)
	switch kind {
	case SourceJira, SourceTodoist, SourceManual:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: source kind", ErrInvalid)
	}
}

func DefaultQuery(kind SourceKind) string {
	switch kind {
	case SourceJira:
		return "assignee = currentUser() AND statusCategory != Done"
	default:
		return ""
	}
}

func NormalizeJiraEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	if email == "" || !strings.Contains(email, "@") {
		return "", fmt.Errorf("%w: jira email", ErrInvalid)
	}
	return email, nil
}

func NewSource(kind SourceKind, name, baseURL, query string, projectID uuid.UUID, email string) (Source, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Source{}, fmt.Errorf("%w: source name", ErrInvalid)
	}
	if kind == SourceJira && strings.TrimSpace(baseURL) == "" {
		return Source{}, fmt.Errorf("%w: jira base url", ErrInvalid)
	}
	if kind == SourceJira {
		normalized, err := NormalizeJiraEmail(email)
		if err != nil {
			return Source{}, err
		}
		email = normalized
	} else {
		email = ""
	}
	if kind == SourceJira || kind == SourceTodoist {
		if projectID == uuid.Nil {
			return Source{}, fmt.Errorf("%w: source project", ErrInvalid)
		}
	}
	if kind == SourceJira {
		query = DefaultQuery(kind)
	} else if query == "" {
		query = DefaultQuery(kind)
	}
	if kind == SourceTodoist && strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.todoist.com"
	}
	now := time.Now().UTC()
	source := Source{
		ID:          uuid.New(),
		Kind:        kind,
		Name:        name,
		Email:       email,
		BaseURL:     strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		QueryFilter: query,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if projectID != uuid.Nil {
		id := projectID
		source.ProjectID = &id
	}
	return source, nil
}

func (s Source) Public() Source {
	s.TokenSealed = ""
	return s
}
