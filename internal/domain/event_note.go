package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type EventNote struct {
	ID         uuid.UUID `json:"id"`
	SeriesID   uuid.UUID `json:"seriesId"`
	OriginalOn Ymd       `json:"originalOn"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
}

func NewEventNote(seriesID uuid.UUID, originalOn Ymd, body string) (EventNote, error) {
	body = strings.TrimSpace(body)
	if seriesID == uuid.Nil {
		return EventNote{}, fmt.Errorf("%w: series", ErrInvalid)
	}
	on, err := ParseYmd(string(originalOn))
	if err != nil {
		return EventNote{}, err
	}
	if body == "" {
		return EventNote{}, fmt.Errorf("%w: note body", ErrInvalid)
	}
	return EventNote{
		ID:         uuid.New(),
		SeriesID:   seriesID,
		OriginalOn: on,
		Body:       body,
		CreatedAt:  time.Now().UTC(),
	}, nil
}
