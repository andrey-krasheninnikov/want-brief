package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewPersonContactRejectsEmptyValue(t *testing.T) {
	_, err := NewPersonContact(uuid.New(), PersonContactDraft{Kind: "phone", Label: "Work", Value: "  "})
	if err == nil {
		t.Fatal("expected invalid value")
	}
}

func TestNewPersonContactNormalizesURL(t *testing.T) {
	contact, err := NewPersonContact(uuid.New(), PersonContactDraft{
		Kind:  "url",
		Label: "Site",
		Value: "https://example.com/a",
		Note:  " work ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if contact.Value != "https://example.com/a" || contact.Note != "work" {
		t.Fatalf("got %+v", contact)
	}
}

func TestNewPersonContactRejectsBadURL(t *testing.T) {
	_, err := NewPersonContact(uuid.New(), PersonContactDraft{Kind: "url", Label: "Site", Value: "ftp://x"})
	if err == nil {
		t.Fatal("expected invalid url")
	}
}
