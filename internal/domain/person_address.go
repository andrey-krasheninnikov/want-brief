package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AddressKind string

const (
	AddressKindHome  AddressKind = "home"
	AddressKindWork  AddressKind = "work"
	AddressKindOther AddressKind = "other"
)

type PersonAddress struct {
	ID        uuid.UUID   `json:"id"`
	PersonID  uuid.UUID   `json:"personId"`
	Kind      AddressKind `json:"kind"`
	Label     string      `json:"label"`
	Value     string      `json:"value"`
	Note      string      `json:"note"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

type PersonAddressDraft struct {
	Kind  string
	Label string
	Value string
	Note  string
}

func ParseAddressKind(raw string) (AddressKind, error) {
	kind := AddressKind(strings.TrimSpace(raw))
	switch kind {
	case AddressKindHome, AddressKindWork, AddressKindOther:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: address kind", ErrInvalid)
	}
}

func NewPersonAddress(personID uuid.UUID, draft PersonAddressDraft) (PersonAddress, error) {
	now := time.Now().UTC()
	address := PersonAddress{
		ID:        uuid.New(),
		PersonID:  personID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := address.apply(draft); err != nil {
		return PersonAddress{}, err
	}
	return address, nil
}

func (a *PersonAddress) Apply(draft PersonAddressDraft) error {
	if err := a.apply(draft); err != nil {
		return err
	}
	a.UpdatedAt = time.Now().UTC()
	return nil
}

func (a *PersonAddress) apply(draft PersonAddressDraft) error {
	if a.PersonID == uuid.Nil {
		return fmt.Errorf("%w: person", ErrInvalid)
	}
	kind, err := ParseAddressKind(draft.Kind)
	if err != nil {
		return err
	}
	label := strings.TrimSpace(draft.Label)
	if label == "" {
		return fmt.Errorf("%w: address label", ErrInvalid)
	}
	value := strings.TrimSpace(draft.Value)
	if value == "" {
		return fmt.Errorf("%w: address value", ErrInvalid)
	}
	a.Kind = kind
	a.Label = label
	a.Value = value
	a.Note = strings.TrimSpace(draft.Note)
	return nil
}
