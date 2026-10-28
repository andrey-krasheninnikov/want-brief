package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewPersonProfessionRequiresTitle(t *testing.T) {
	_, err := NewPersonProfession(uuid.New(), PersonProfessionDraft{StartedOn: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err == nil {
		t.Fatal("expected invalid title")
	}
}

func TestPersonProfessionRejectsEndedBeforeStart(t *testing.T) {
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := NewPersonProfession(uuid.New(), PersonProfessionDraft{
		Title:     "Dev",
		StartedOn: start,
		EndedOn:   &end,
	})
	if err == nil {
		t.Fatal("expected invalid dates")
	}
}

func TestSetSalaryClosesOpenPeriod(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	next := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	prof, err := NewPersonProfession(uuid.New(), PersonProfessionDraft{Title: "Dev", StartedOn: start})
	if err != nil {
		t.Fatal(err)
	}
	if err := prof.SetSalary(100, 0, start); err != nil {
		t.Fatal(err)
	}
	if err := prof.SetSalary(200, 10, next); err != nil {
		t.Fatal(err)
	}
	if len(prof.Salaries) != 2 {
		t.Fatalf("got %d periods", len(prof.Salaries))
	}
	if prof.Salaries[0].EndedOn == nil || !prof.Salaries[0].EndedOn.Equal(next) {
		t.Fatalf("closed endedOn %v", prof.Salaries[0].EndedOn)
	}
	if prof.Salaries[1].EndedOn != nil || prof.Salaries[1].MonthlySalaryUSD != 200 {
		t.Fatalf("open %+v", prof.Salaries[1])
	}
}

func TestSetSalarySameAmountsNoop(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	prof, err := NewPersonProfession(uuid.New(), PersonProfessionDraft{Title: "Dev", StartedOn: start})
	if err != nil {
		t.Fatal(err)
	}
	if err := prof.SetSalary(100, 50, start); err != nil {
		t.Fatal(err)
	}
	id := prof.Salaries[0].ID
	if err := prof.SetSalary(100, 50, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(prof.Salaries) != 1 || prof.Salaries[0].ID != id || prof.Salaries[0].EndedOn != nil {
		t.Fatalf("got %+v", prof.Salaries)
	}
}

func TestSetSalaryRejectsBeforeOpenStart(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	prof, err := NewPersonProfession(uuid.New(), PersonProfessionDraft{Title: "Dev", StartedOn: start})
	if err != nil {
		t.Fatal(err)
	}
	if err := prof.SetSalary(100, 0, start); err != nil {
		t.Fatal(err)
	}
	if err := prof.SetSalary(200, 0, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("expected invalid salary dates")
	}
	if len(prof.Salaries) != 1 || prof.Salaries[0].EndedOn != nil {
		t.Fatalf("state changed %+v", prof.Salaries)
	}
}

func TestPersonProfessionCurrentDropsClosed(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	prof, err := NewPersonProfession(uuid.New(), PersonProfessionDraft{Title: "Dev", StartedOn: start})
	if err != nil {
		t.Fatal(err)
	}
	if err := prof.SetSalary(100, 0, start); err != nil {
		t.Fatal(err)
	}
	if err := prof.SetSalary(200, 0, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	cur := prof.Current()
	if len(cur.Salaries) != 1 || cur.Salaries[0].MonthlySalaryUSD != 200 {
		t.Fatalf("got %+v", cur.Salaries)
	}
	if len(prof.Salaries) != 2 {
		t.Fatalf("source mutated %d", len(prof.Salaries))
	}
}
