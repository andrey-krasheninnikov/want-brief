package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ItemEventKind string

const (
	ItemEventStatus     ItemEventKind = "status"
	ItemEventField      ItemEventKind = "field"
	ItemEventTimerStart ItemEventKind = "timer_start"
	ItemEventTimerStop  ItemEventKind = "timer_stop"
	ItemEventTimerLog   ItemEventKind = "timer_log"
	ItemEventCheck      ItemEventKind = "check"
)

type ItemEvent struct {
	ID        uuid.UUID     `json:"id"`
	ItemID    uuid.UUID     `json:"itemId"`
	Kind      ItemEventKind `json:"kind"`
	Field     string        `json:"field"`
	From      string        `json:"from"`
	To        string        `json:"to"`
	Note      string        `json:"note"`
	Stress    *int          `json:"stress"`
	CreatedAt time.Time     `json:"createdAt"`
}

func ParseItemEventKind(raw string) (ItemEventKind, error) {
	kind := ItemEventKind(raw)
	switch kind {
	case ItemEventStatus, ItemEventField, ItemEventTimerStart, ItemEventTimerStop, ItemEventTimerLog, ItemEventCheck:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: item event kind", ErrInvalid)
	}
}

func NewItemEvent(itemID uuid.UUID, kind ItemEventKind, field, from, to string, at time.Time) (ItemEvent, error) {
	parsed, err := ParseItemEventKind(string(kind))
	if err != nil {
		return ItemEvent{}, err
	}
	if itemID == uuid.Nil {
		return ItemEvent{}, fmt.Errorf("%w: item", ErrInvalid)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return ItemEvent{
		ID:        uuid.New(),
		ItemID:    itemID,
		Kind:      parsed,
		Field:     strings.TrimSpace(field),
		From:      from,
		To:        to,
		CreatedAt: at.UTC(),
	}, nil
}

func (e ItemEvent) Annotate(note string, stress *int) (ItemEvent, error) {
	note = strings.TrimSpace(note)
	if stress != nil {
		if err := ValidateStress(*stress); err != nil {
			return ItemEvent{}, err
		}
		e.Stress = stress
	}
	if note != "" {
		e.Note = note
	}
	return e, nil
}
