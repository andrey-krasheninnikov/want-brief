package domain

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ProjectLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Project struct {
	ID               uuid.UUID     `json:"id"`
	Name             string        `json:"name"`
	Color            string        `json:"color"`
	Description      string        `json:"description"`
	MonthlyIncomeUSD float64       `json:"monthlyIncomeUsd"`
	MonthlyIncomeRUB float64       `json:"monthlyIncomeRub"`
	TargetHoursDay   float64       `json:"targetHoursDay"`
	TargetHoursWeek  float64       `json:"targetHoursWeek"`
	Links            []ProjectLink `json:"links"`
	People           []PersonRel   `json:"people"`
	Companies        []PersonRel   `json:"companies"`
	ArchivedAt       *time.Time    `json:"archivedAt"`
	CreatedAt        time.Time     `json:"createdAt"`
	UpdatedAt        time.Time     `json:"updatedAt"`
}

func NewProject(name, color string, targetHoursDay float64) (Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, fmt.Errorf("%w: project name", ErrInvalid)
	}
	if color == "" {
		color = "#6152ED"
	}
	now := time.Now().UTC()
	project := Project{
		ID:        uuid.New(),
		Name:      name,
		Color:     color,
		Links:     []ProjectLink{},
		People:    []PersonRel{},
		Companies: []PersonRel{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := project.SetTargetHoursDay(targetHoursDay); err != nil {
		return Project{}, err
	}
	return project, nil
}

func (p *Project) SetTargetHoursDay(day float64) error {
	if day < 0 {
		return fmt.Errorf("%w: target hours", ErrInvalid)
	}
	p.TargetHoursDay = day
	p.TargetHoursWeek = day * 7
	return nil
}

func (p *Project) SetMonthlyIncomeUSD(income float64) error {
	if income < 0 {
		return fmt.Errorf("%w: monthly income usd", ErrInvalid)
	}
	p.MonthlyIncomeUSD = income
	return nil
}

func (p *Project) SetMonthlyIncomeRUB(income float64) error {
	if income < 0 {
		return fmt.Errorf("%w: monthly income rub", ErrInvalid)
	}
	p.MonthlyIncomeRUB = income
	return nil
}

func (p *Project) Archive(now time.Time) {
	if p.ArchivedAt != nil {
		return
	}
	at := now.UTC()
	p.ArchivedAt = &at
	p.UpdatedAt = at
}

func (p *Project) Restore(now time.Time) {
	if p.ArchivedAt == nil {
		return
	}
	p.ArchivedAt = nil
	p.UpdatedAt = now.UTC()
}

func NormalizeLinks(links []ProjectLink) ([]ProjectLink, error) {
	out := make([]ProjectLink, 0, len(links))
	for _, link := range links {
		parsed, err := parseProjectURL(link.URL)
		if err != nil {
			return nil, err
		}
		out = append(out, ProjectLink{
			Label: strings.TrimSpace(link.Label),
			URL:   parsed,
		})
	}
	return out, nil
}

func parseProjectURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("%w: link url", ErrInvalid)
	}
	return parsed.String(), nil
}
