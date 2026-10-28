package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Note struct {
	ID        uuid.UUID  `json:"id"`
	ItemID    *uuid.UUID `json:"itemId"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

func NewNote(body string, itemID *uuid.UUID) (Note, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return Note{}, fmt.Errorf("%w: note body", ErrInvalid)
	}
	now := time.Now().UTC()
	return Note{
		ID:        uuid.New(),
		ItemID:    itemID,
		Body:      body,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
