package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewPersonSiteAllowsDuplicateKinds(t *testing.T) {
	id := uuid.New()
	a, err := NewPersonSite(id, PersonSiteDraft{Kind: "github", URL: "https://github.com/a"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewPersonSite(id, PersonSiteDraft{Kind: "github", URL: "https://github.com/b"})
	if err != nil {
		t.Fatal(err)
	}
	if a.URL == b.URL {
		t.Fatal("expected distinct urls")
	}
}

func TestNewPersonSiteRejectsKind(t *testing.T) {
	_, err := NewPersonSite(uuid.New(), PersonSiteDraft{Kind: "blog", URL: "https://example.com"})
	if err == nil {
		t.Fatal("expected invalid kind")
	}
}

func TestParseSiteKindOther(t *testing.T) {
	kind, err := ParseSiteKind("other")
	if err != nil {
		t.Fatal(err)
	}
	if kind != SiteKindOther {
		t.Fatalf("got %s", kind)
	}
}
