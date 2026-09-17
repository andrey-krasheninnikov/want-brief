package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewCompanyRequiresName(t *testing.T) {
	_, err := NewCompany(CompanyDraft{})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestCompanyTenureYearsMonths(t *testing.T) {
	start := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	company, err := NewCompany(CompanyDraft{Name: "Acme", StartedOn: &start, EndedOn: &end})
	if err != nil {
		t.Fatal(err)
	}
	if company.Tenure == nil || company.Tenure.Years != 2 || company.Tenure.Months != 3 {
		t.Fatalf("got %+v", company.Tenure)
	}
}

func TestCompanyRejectsEndedBeforeStart(t *testing.T) {
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := NewCompany(CompanyDraft{Name: "Acme", StartedOn: &start, EndedOn: &end})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestSetTitleClosesOpen(t *testing.T) {
	company, err := NewCompany(CompanyDraft{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	next := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := company.SetTitle("Engineer", start); err != nil {
		t.Fatal(err)
	}
	if err := company.SetTitle("Lead", next); err != nil {
		t.Fatal(err)
	}
	if len(company.Titles) != 2 {
		t.Fatalf("got %d", len(company.Titles))
	}
	if company.Titles[0].EndedOn == nil || !company.Titles[0].EndedOn.Equal(next) {
		t.Fatalf("closed %+v", company.Titles[0])
	}
	if company.OpenTitle() == nil || company.OpenTitle().Title != "Lead" {
		t.Fatalf("open %+v", company.OpenTitle())
	}
}

func TestSetSalaryRequiresCommentOnChange(t *testing.T) {
	company, err := NewCompany(CompanyDraft{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := company.SetSalary(SalaryUSD, 100, start, ""); err != nil {
		t.Fatal(err)
	}
	next := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if err := company.SetSalary(SalaryUSD, 200, next, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
	if err := company.SetSalary(SalaryUSD, 200, next, "raise"); err != nil {
		t.Fatal(err)
	}
	if len(company.Salaries) != 2 || company.Salaries[1].Comment != "raise" {
		t.Fatalf("got %+v", company.Salaries)
	}
}

func TestSetSalarySameAmountNoop(t *testing.T) {
	company, err := NewCompany(CompanyDraft{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := company.SetSalary(SalaryRUB, 150000, start, ""); err != nil {
		t.Fatal(err)
	}
	id := company.Salaries[0].ID
	if err := company.SetSalary(SalaryRUB, 150000, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), ""); err != nil {
		t.Fatal(err)
	}
	if len(company.Salaries) != 1 || company.Salaries[0].ID != id || company.Salaries[0].EndedOn != nil {
		t.Fatalf("got %+v", company.Salaries)
	}
}

func TestSetSalaryRejectsUnknownCurrency(t *testing.T) {
	company, err := NewCompany(CompanyDraft{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	err = company.SetSalary("eur", 10, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestManagerCannotBeReport(t *testing.T) {
	company, err := NewCompany(CompanyDraft{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	person := uuid.New()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := company.AddReport(person, start); err != nil {
		t.Fatal(err)
	}
	if err := company.SetManager(&person, start); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestReportCannotBeManager(t *testing.T) {
	company, err := NewCompany(CompanyDraft{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	person := uuid.New()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := company.SetManager(&person, start); err != nil {
		t.Fatal(err)
	}
	if err := company.AddReport(person, start); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestEndReport(t *testing.T) {
	company, err := NewCompany(CompanyDraft{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	person := uuid.New()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := company.AddReport(person, start); err != nil {
		t.Fatal(err)
	}
	end := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := company.EndReport(company.Reports[0].ID, end); err != nil {
		t.Fatal(err)
	}
	if company.Reports[0].EndedOn == nil || !company.Reports[0].EndedOn.Equal(end) {
		t.Fatalf("got %+v", company.Reports[0])
	}
}

func TestClearManager(t *testing.T) {
	company, err := NewCompany(CompanyDraft{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	person := uuid.New()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := company.SetManager(&person, start); err != nil {
		t.Fatal(err)
	}
	next := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if err := company.SetManager(nil, next); err != nil {
		t.Fatal(err)
	}
	if company.openManager() != nil || company.Managers[0].EndedOn == nil {
		t.Fatalf("got %+v", company.Managers)
	}
}

func TestSetContractMeAndPerson(t *testing.T) {
	person := uuid.New()
	company, err := NewCompany(CompanyDraft{Name: "Acme", People: []PersonRel{{ID: person, Comment: "lead"}}})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := company.SetContract(nil, ContractLabor, start); err != nil {
		t.Fatal(err)
	}
	if err := company.SetContract(&person, ContractGPH, start); err != nil {
		t.Fatal(err)
	}
	if len(company.Contracts) != 2 {
		t.Fatalf("got %d", len(company.Contracts))
	}
}

func TestSetContractUnknownPerson(t *testing.T) {
	company, err := NewCompany(CompanyDraft{Name: "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	person := uuid.New()
	err = company.SetContract(&person, ContractIP, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestApplyClosesRemovedPersonContract(t *testing.T) {
	person := uuid.New()
	company, err := NewCompany(CompanyDraft{Name: "Acme", People: []PersonRel{{ID: person}}})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := company.SetContract(&person, ContractInformal, start); err != nil {
		t.Fatal(err)
	}
	if err := company.Apply(CompanyDraft{Name: "Acme"}); err != nil {
		t.Fatal(err)
	}
	if company.Contracts[0].EndedOn == nil {
		t.Fatal("expected closed contract")
	}
}

func TestNewCompanyNoteEmptyBody(t *testing.T) {
	_, err := NewCompanyNote(uuid.New(), "  ")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestWeeklyProjectCommentRequired(t *testing.T) {
	_, err := NewCompany(CompanyDraft{Name: "Acme", Projects: []PersonRel{{ID: uuid.New()}}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}
