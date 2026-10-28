package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PersonNote struct {
	ID        uuid.UUID `json:"id"`
	PersonID  uuid.UUID `json:"personId"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewPersonNote(personID uuid.UUID, body string) (PersonNote, error) {
	body = strings.TrimSpace(body)
	if personID == uuid.Nil {
		return PersonNote{}, fmt.Errorf("%w: person", ErrInvalid)
	}
	if body == "" {
		return PersonNote{}, fmt.Errorf("%w: note body", ErrInvalid)
	}
	return PersonNote{
		ID:        uuid.New(),
		PersonID:  personID,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (n *PersonNote) Apply(body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return fmt.Errorf("%w: note body", ErrInvalid)
	}
	n.Body = body
	return nil
}
