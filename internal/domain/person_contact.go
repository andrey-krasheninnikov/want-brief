package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ContactKind string

const (
	ContactKindPhone    ContactKind = "phone"
	ContactKindTelegram ContactKind = "telegram"
	ContactKindURL      ContactKind = "url"
)

type PersonContact struct {
	ID        uuid.UUID   `json:"id"`
	PersonID  uuid.UUID   `json:"personId"`
	Kind      ContactKind `json:"kind"`
	Label     string      `json:"label"`
	Value     string      `json:"value"`
	Note      string      `json:"note"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

type PersonContactDraft struct {
	Kind  string
	Label string
	Value string
	Note  string
}

func ParseContactKind(raw string) (ContactKind, error) {
	kind := ContactKind(strings.TrimSpace(raw))
	switch kind {
	case ContactKindPhone, ContactKindTelegram, ContactKindURL:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: contact kind", ErrInvalid)
	}
}

func NewPersonContact(personID uuid.UUID, draft PersonContactDraft) (PersonContact, error) {
	now := time.Now().UTC()
	contact := PersonContact{
		ID:        uuid.New(),
		PersonID:  personID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := contact.apply(draft); err != nil {
		return PersonContact{}, err
	}
	return contact, nil
}

func (c *PersonContact) Apply(draft PersonContactDraft) error {
	if err := c.apply(draft); err != nil {
		return err
	}
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *PersonContact) apply(draft PersonContactDraft) error {
	if c.PersonID == uuid.Nil {
		return fmt.Errorf("%w: person", ErrInvalid)
	}
	kind, err := ParseContactKind(draft.Kind)
	if err != nil {
		return err
	}
	label := strings.TrimSpace(draft.Label)
	if label == "" {
		return fmt.Errorf("%w: contact label", ErrInvalid)
	}
	value, err := normalizeContactValue(kind, draft.Value)
	if err != nil {
		return err
	}
	c.Kind = kind
	c.Label = label
	c.Value = value
	c.Note = strings.TrimSpace(draft.Note)
	return nil
}

func normalizeContactValue(kind ContactKind, raw string) (string, error) {
	if kind == ContactKindURL {
		return parseProjectURL(raw)
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("%w: contact value", ErrInvalid)
	}
	return value, nil
}
