package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewProjectDerivesWeekFromDay(t *testing.T) {
	project, err := NewProject("Work", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if project.TargetHoursDay != 2 || project.TargetHoursWeek != 14 {
		t.Fatalf("day %v week %v", project.TargetHoursDay, project.TargetHoursWeek)
	}
}

func TestNewProjectRejectsNegativeDay(t *testing.T) {
	if _, err := NewProject("Work", "", -1); err == nil {
		t.Fatal("expected invalid hours")
	}
}

func TestSetMonthlyIncomeRejectsNegative(t *testing.T) {
	project, err := NewProject("Work", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := project.SetMonthlyIncomeUSD(-10); err == nil {
		t.Fatal("expected invalid usd")
	}
	if err := project.SetMonthlyIncomeRUB(-1); err == nil {
		t.Fatal("expected invalid rub")
	}
}

func TestNormalizeLinksRejectsBadURL(t *testing.T) {
	if _, err := NormalizeLinks([]ProjectLink{{URL: "not-a-url"}}); err == nil {
		t.Fatal("expected invalid url")
	}
}

func TestNormalizeLinksAcceptsHTTPS(t *testing.T) {
	links, err := NormalizeLinks([]ProjectLink{{Label: " docs ", URL: "https://example.com/x"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Label != "docs" || links[0].URL != "https://example.com/x" {
		t.Fatalf("got %+v", links)
	}
}

func TestArchiveSetsTimeOnce(t *testing.T) {
	project, err := NewProject("Work", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	project.Archive(first)
	if project.ArchivedAt == nil || !project.ArchivedAt.Equal(first) {
		t.Fatalf("archived at %v", project.ArchivedAt)
	}
	if !project.UpdatedAt.Equal(first) {
		t.Fatalf("updated at %v", project.UpdatedAt)
	}
	project.Archive(first.Add(time.Hour))
	if !project.ArchivedAt.Equal(first) {
		t.Fatalf("second archive moved time to %v", project.ArchivedAt)
	}
}

func TestRestoreClearsArchive(t *testing.T) {
	project, err := NewProject("Work", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	archived := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	project.Archive(archived)
	restored := archived.Add(time.Hour)
	project.Restore(restored)
	if project.ArchivedAt != nil {
		t.Fatalf("still archived %v", project.ArchivedAt)
	}
	if !project.UpdatedAt.Equal(restored) {
		t.Fatalf("updated at %v", project.UpdatedAt)
	}
	project.Restore(restored.Add(time.Hour))
	if !project.UpdatedAt.Equal(restored) {
		t.Fatalf("second restore moved updated at %v", project.UpdatedAt)
	}
}

func TestNewProjectNoteRejectsEmptyBody(t *testing.T) {
	if _, err := NewProjectNote(uuid.New(), "  "); err == nil {
		t.Fatal("expected invalid body")
	}
}

func TestNewProjectNoteKeepsBody(t *testing.T) {
	id := uuid.New()
	note, err := NewProjectNote(id, " shipped ")
	if err != nil {
		t.Fatal(err)
	}
	if note.ProjectID != id || note.Body != "shipped" {
		t.Fatalf("got %+v", note)
	}
}
