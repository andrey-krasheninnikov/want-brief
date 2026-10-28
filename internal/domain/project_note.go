package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ProjectNote struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"projectId"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewProjectNote(projectID uuid.UUID, body string) (ProjectNote, error) {
	body = strings.TrimSpace(body)
	if projectID == uuid.Nil {
		return ProjectNote{}, fmt.Errorf("%w: project", ErrInvalid)
	}
	if body == "" {
		return ProjectNote{}, fmt.Errorf("%w: note body", ErrInvalid)
	}
	return ProjectNote{
		ID:        uuid.New(),
		ProjectID: projectID,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}, nil
}
