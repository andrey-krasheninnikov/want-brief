package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ItemNote struct {
	ID         uuid.UUID  `json:"id"`
	ItemID     uuid.UUID  `json:"itemId"`
	Body       string     `json:"body"`
	SourceKind SourceKind `json:"sourceKind"`
	ExternalID string     `json:"externalId"`
	AuthorName string     `json:"authorName"`
	URL        string     `json:"url"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func NewItemNote(itemID uuid.UUID, body string, sourceKind SourceKind) (ItemNote, error) {
	body = strings.TrimSpace(body)
	kind, err := ParseSourceKind(strings.TrimSpace(string(sourceKind)))
	if err != nil {
		return ItemNote{}, err
	}
	if itemID == uuid.Nil {
		return ItemNote{}, fmt.Errorf("%w: item", ErrInvalid)
	}
	if body == "" {
		return ItemNote{}, fmt.Errorf("%w: note body", ErrInvalid)
	}
	return ItemNote{
		ID:         uuid.New(),
		ItemID:     itemID,
		Body:       body,
		SourceKind: kind,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func NewSourceItemNote(itemID uuid.UUID, sourceKind SourceKind, comment RemoteComment) (ItemNote, error) {
	note, err := NewItemNote(itemID, comment.Body, sourceKind)
	if err != nil {
		return ItemNote{}, err
	}
	note.ExternalID = strings.TrimSpace(comment.ExternalID)
	note.AuthorName = strings.TrimSpace(comment.Author)
	note.URL = strings.TrimSpace(comment.URL)
	if comment.CreatedAt != nil {
		note.CreatedAt = comment.CreatedAt.UTC()
	}
	return note, nil
}
