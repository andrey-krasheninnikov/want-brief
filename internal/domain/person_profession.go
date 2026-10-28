package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SalaryPeriod struct {
	ID               uuid.UUID  `json:"id"`
	ProfessionID     uuid.UUID  `json:"professionId"`
	StartedOn        time.Time  `json:"startedOn"`
	EndedOn          *time.Time `json:"endedOn"`
	MonthlySalaryUSD float64    `json:"monthlySalaryUsd"`
	MonthlySalaryRUB float64    `json:"monthlySalaryRub"`
}

type PersonProfession struct {
	ID        uuid.UUID      `json:"id"`
	PersonID  uuid.UUID      `json:"personId"`
	Title     string         `json:"title"`
	Comment   string         `json:"comment"`
	StartedOn time.Time      `json:"startedOn"`
	EndedOn   *time.Time     `json:"endedOn"`
	Salaries  []SalaryPeriod `json:"salaries"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type PersonProfessionDraft struct {
	Title     string
	Comment   string
	StartedOn time.Time
	EndedOn   *time.Time
}

func NewPersonProfession(personID uuid.UUID, draft PersonProfessionDraft) (PersonProfession, error) {
	now := time.Now().UTC()
	prof := PersonProfession{
		ID:        uuid.New(),
		PersonID:  personID,
		Salaries:  []SalaryPeriod{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := prof.apply(draft); err != nil {
		return PersonProfession{}, err
	}
	return prof, nil
}

func (p *PersonProfession) Apply(draft PersonProfessionDraft) error {
	if err := p.apply(draft); err != nil {
		return err
	}
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *PersonProfession) apply(draft PersonProfessionDraft) error {
	if p.PersonID == uuid.Nil {
		return fmt.Errorf("%w: person", ErrInvalid)
	}
	title := strings.TrimSpace(draft.Title)
	if title == "" {
		return fmt.Errorf("%w: profession title", ErrInvalid)
	}
	start := dateUTC(draft.StartedOn)
	var ended *time.Time
	if draft.EndedOn != nil {
		day := dateUTC(*draft.EndedOn)
		if day.Before(start) {
			return fmt.Errorf("%w: profession dates", ErrInvalid)
		}
		ended = &day
	}
	if err := validateSalaries(p.Salaries); err != nil {
		return err
	}
	p.Title = title
	p.Comment = strings.TrimSpace(draft.Comment)
	p.StartedOn = start
	p.EndedOn = ended
	if p.Salaries == nil {
		p.Salaries = []SalaryPeriod{}
	}
	return nil
}

func (p *PersonProfession) SetSalary(usd, rub float64, startedOn time.Time) error {
	if usd < 0 || rub < 0 {
		return fmt.Errorf("%w: monthly salary", ErrInvalid)
	}
	start := dateUTC(startedOn)
	for i := range p.Salaries {
		if p.Salaries[i].EndedOn != nil {
			continue
		}
		if p.Salaries[i].MonthlySalaryUSD == usd && p.Salaries[i].MonthlySalaryRUB == rub {
			return nil
		}
		if start.Before(p.Salaries[i].StartedOn) {
			return fmt.Errorf("%w: salary dates", ErrInvalid)
		}
		end := start
		p.Salaries[i].EndedOn = &end
		break
	}
	p.Salaries = append(p.Salaries, SalaryPeriod{
		ID:               uuid.New(),
		ProfessionID:     p.ID,
		StartedOn:        start,
		EndedOn:          nil,
		MonthlySalaryUSD: usd,
		MonthlySalaryRUB: rub,
	})
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p PersonProfession) Current() PersonProfession {
	open := []SalaryPeriod{}
	for _, period := range p.Salaries {
		if period.EndedOn == nil {
			open = append(open, period)
		}
	}
	p.Salaries = open
	return p
}

func validateSalaries(salaries []SalaryPeriod) error {
	open := 0
	for _, period := range salaries {
		if period.MonthlySalaryUSD < 0 || period.MonthlySalaryRUB < 0 {
			return fmt.Errorf("%w: monthly salary", ErrInvalid)
		}
		if period.EndedOn != nil && period.EndedOn.Before(period.StartedOn) {
			return fmt.Errorf("%w: salary dates", ErrInvalid)
		}
		if period.EndedOn == nil {
			open++
		}
	}
	if open > 1 {
		return fmt.Errorf("%w: open salary", ErrInvalid)
	}
	return nil
}
