package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewSourceRequiresProjectForJira(t *testing.T) {
	_, err := NewSource(SourceJira, "Work Jira", "https://ex.atlassian.net", "", uuid.Nil, "a@b.com")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err %v", err)
	}
}

func TestNewSourceRequiresEmailForJira(t *testing.T) {
	_, err := NewSource(SourceJira, "Work Jira", "https://ex.atlassian.net", "", uuid.New(), "not-an-email")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err %v", err)
	}
}

func TestNewSourceJiraIgnoresCustomQuery(t *testing.T) {
	projectID := uuid.New()
	source, err := NewSource(SourceJira, "Work Jira", "https://ex.atlassian.net/", "project = X", projectID, "dev@ex.com")
	if err != nil {
		t.Fatal(err)
	}
	if source.QueryFilter != DefaultQuery(SourceJira) {
		t.Fatalf("query %q", source.QueryFilter)
	}
	if source.ProjectID == nil || *source.ProjectID != projectID {
		t.Fatalf("project %+v", source.ProjectID)
	}
	if source.BaseURL != "https://ex.atlassian.net" {
		t.Fatalf("base %q", source.BaseURL)
	}
	if source.Email != "dev@ex.com" {
		t.Fatalf("email %q", source.Email)
	}
}

func TestNewSourceManualAllowsNoProject(t *testing.T) {
	source, err := NewSource(SourceManual, "Manual", "", "", uuid.Nil, "ignored@ex.com")
	if err != nil {
		t.Fatal(err)
	}
	if source.ProjectID != nil {
		t.Fatalf("project %+v", source.ProjectID)
	}
	if source.Email != "" {
		t.Fatalf("email %q", source.Email)
	}
}
