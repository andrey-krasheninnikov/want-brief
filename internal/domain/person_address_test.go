package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewPersonAddressRejectsEmptyValue(t *testing.T) {
	_, err := NewPersonAddress(uuid.New(), PersonAddressDraft{Kind: "home", Label: "Home", Value: "  "})
	if err == nil {
		t.Fatal("expected invalid value")
	}
}

func TestNewPersonAddressRejectsEmptyLabel(t *testing.T) {
	_, err := NewPersonAddress(uuid.New(), PersonAddressDraft{Kind: "home", Label: "  ", Value: "Main st"})
	if err == nil {
		t.Fatal("expected invalid label")
	}
}

func TestNewPersonAddressRejectsBadKind(t *testing.T) {
	_, err := NewPersonAddress(uuid.New(), PersonAddressDraft{Kind: "cabin", Label: "Home", Value: "Main st"})
	if err == nil {
		t.Fatal("expected invalid kind")
	}
}

func TestNewPersonAddressTrimsNote(t *testing.T) {
	address, err := NewPersonAddress(uuid.New(), PersonAddressDraft{
		Kind:  "work",
		Label: " Office ",
		Value: " 1 Main st ",
		Note:  " buzzer 4 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if address.Kind != AddressKindWork || address.Label != "Office" || address.Value != "1 Main st" || address.Note != "buzzer 4" {
		t.Fatalf("got %+v", address)
	}
}
