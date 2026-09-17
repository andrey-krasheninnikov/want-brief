package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SalaryCurrency string

const (
	SalaryUSD SalaryCurrency = "usd"
	SalaryRUB SalaryCurrency = "rub"
)

type ContractKind string

const (
	ContractInformal ContractKind = "informal"
	ContractGPH      ContractKind = "gph"
	ContractIP       ContractKind = "ip"
	ContractLabor    ContractKind = "labor"
	ContractContract ContractKind = "contract"
)

type Tenure struct {
	Years  int `json:"years"`
	Months int `json:"months"`
}

type CompanyTitle struct {
	ID        uuid.UUID  `json:"id"`
	CompanyID uuid.UUID  `json:"companyId"`
	Title     string     `json:"title"`
	StartedOn time.Time  `json:"startedOn"`
	EndedOn   *time.Time `json:"endedOn"`
}

type CompanySalary struct {
	ID        uuid.UUID      `json:"id"`
	CompanyID uuid.UUID      `json:"companyId"`
	Currency  SalaryCurrency `json:"currency"`
	Amount    float64        `json:"amount"`
	Comment   string         `json:"comment"`
	StartedOn time.Time      `json:"startedOn"`
	EndedOn   *time.Time     `json:"endedOn"`
}

type CompanyManager struct {
	ID        uuid.UUID  `json:"id"`
	CompanyID uuid.UUID  `json:"companyId"`
	PersonID  uuid.UUID  `json:"personId"`
	StartedOn time.Time  `json:"startedOn"`
	EndedOn   *time.Time `json:"endedOn"`
}

type CompanyReport struct {
	ID        uuid.UUID  `json:"id"`
	CompanyID uuid.UUID  `json:"companyId"`
	PersonID  uuid.UUID  `json:"personId"`
	StartedOn time.Time  `json:"startedOn"`
	EndedOn   *time.Time `json:"endedOn"`
}

type CompanyContract struct {
	ID        uuid.UUID    `json:"id"`
	CompanyID uuid.UUID    `json:"companyId"`
	PersonID  *uuid.UUID   `json:"personId"`
	Kind      ContractKind `json:"kind"`
	StartedOn time.Time    `json:"startedOn"`
	EndedOn   *time.Time   `json:"endedOn"`
}

type Company struct {
	ID          uuid.UUID         `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Links       []ProjectLink     `json:"links"`
	Projects    []PersonRel       `json:"projects"`
	Events      []PersonRel       `json:"events"`
	People      []PersonRel       `json:"people"`
	StartedOn   *time.Time        `json:"startedOn"`
	EndedOn     *time.Time        `json:"endedOn"`
	Tenure      *Tenure           `json:"tenure"`
	Titles      []CompanyTitle    `json:"titles"`
	Salaries    []CompanySalary   `json:"salaries"`
	Managers    []CompanyManager  `json:"managers"`
	Reports     []CompanyReport   `json:"reports"`
	Contracts   []CompanyContract `json:"contracts"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

type CompanyDraft struct {
	Name        string
	Description string
	Links       []ProjectLink
	Projects    []PersonRel
	Events      []PersonRel
	People      []PersonRel
	StartedOn   *time.Time
	EndedOn     *time.Time
}

func ParseSalaryCurrency(raw string) (SalaryCurrency, error) {
	kind := SalaryCurrency(strings.TrimSpace(strings.ToLower(raw)))
	switch kind {
	case SalaryUSD, SalaryRUB:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: salary currency", ErrInvalid)
	}
}

func ParseContractKind(raw string) (ContractKind, error) {
	kind := ContractKind(strings.TrimSpace(strings.ToLower(raw)))
	switch kind {
	case ContractInformal, ContractGPH, ContractIP, ContractLabor, ContractContract:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: contract kind", ErrInvalid)
	}
}

func NewCompany(draft CompanyDraft) (Company, error) {
	return newCompanyAt(draft, time.Now().UTC())
}

func newCompanyAt(draft CompanyDraft, now time.Time) (Company, error) {
	company := Company{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := company.apply(draft, now); err != nil {
		return Company{}, err
	}
	return company.WithTenure(now), nil
}

func (c *Company) Apply(draft CompanyDraft) error {
	now := time.Now().UTC()
	if err := c.apply(draft, now); err != nil {
		return err
	}
	c.UpdatedAt = now
	return nil
}

func (c *Company) apply(draft CompanyDraft, now time.Time) error {
	name := strings.TrimSpace(draft.Name)
	if name == "" {
		return fmt.Errorf("%w: company name", ErrInvalid)
	}
	links, err := NormalizeLinks(draft.Links)
	if err != nil {
		return err
	}
	projects, err := NormalizePersonRels(draft.Projects)
	if err != nil {
		return err
	}
	events, err := NormalizePersonRels(draft.Events)
	if err != nil {
		return err
	}
	people := NormalizeOptionalRels(draft.People)
	started, ended, err := normalizeTenureDates(draft.StartedOn, draft.EndedOn)
	if err != nil {
		return err
	}
	c.Name = name
	c.Description = strings.TrimSpace(draft.Description)
	c.Links = links
	c.Projects = projects
	c.Events = events
	c.People = people
	c.StartedOn = started
	c.EndedOn = ended
	c.closeOrphanContracts(MoscowDate(now))
	return c.validate()
}

func normalizeTenureDates(startedOn, endedOn *time.Time) (*time.Time, *time.Time, error) {
	var started *time.Time
	if startedOn != nil {
		day := dateUTC(*startedOn)
		started = &day
	}
	var ended *time.Time
	if endedOn != nil {
		day := dateUTC(*endedOn)
		ended = &day
	}
	if ended != nil && started == nil {
		return nil, nil, fmt.Errorf("%w: company dates", ErrInvalid)
	}
	if started != nil && ended != nil && ended.Before(*started) {
		return nil, nil, fmt.Errorf("%w: company dates", ErrInvalid)
	}
	return started, ended, nil
}

func NormalizeOptionalRels(rels []PersonRel) []PersonRel {
	out := make([]PersonRel, 0, len(rels))
	seen := make(map[uuid.UUID]struct{}, len(rels))
	for _, rel := range rels {
		if rel.ID == uuid.Nil {
			continue
		}
		if _, ok := seen[rel.ID]; ok {
			continue
		}
		seen[rel.ID] = struct{}{}
		out = append(out, PersonRel{ID: rel.ID, Comment: strings.TrimSpace(rel.Comment)})
	}
	return out
}

func (c *Company) closeOrphanContracts(today time.Time) {
	keep := make(map[uuid.UUID]struct{}, len(c.People))
	for _, rel := range c.People {
		keep[rel.ID] = struct{}{}
	}
	for i := range c.Contracts {
		if c.Contracts[i].PersonID == nil || c.Contracts[i].EndedOn != nil {
			continue
		}
		if _, ok := keep[*c.Contracts[i].PersonID]; ok {
			continue
		}
		end := today
		if end.Before(c.Contracts[i].StartedOn) {
			end = c.Contracts[i].StartedOn
		}
		c.Contracts[i].EndedOn = &end
	}
}

func (c Company) TenureAt(now time.Time) *Tenure {
	if c.StartedOn == nil {
		return nil
	}
	end := MoscowDate(now)
	if c.EndedOn != nil {
		end = dateUTC(*c.EndedOn)
	}
	years, months := spanYearsMonths(dateUTC(*c.StartedOn), end)
	return &Tenure{Years: years, Months: months}
}

func spanYearsMonths(start, end time.Time) (int, int) {
	years := end.Year() - start.Year()
	months := int(end.Month()) - int(start.Month())
	if end.Day() < start.Day() {
		months--
	}
	if months < 0 {
		years--
		months += 12
	}
	if years < 0 {
		return 0, 0
	}
	return years, months
}

func (c Company) WithTenure(now time.Time) Company {
	c.Tenure = c.TenureAt(now)
	if c.Links == nil {
		c.Links = []ProjectLink{}
	}
	if c.Projects == nil {
		c.Projects = []PersonRel{}
	}
	if c.Events == nil {
		c.Events = []PersonRel{}
	}
	if c.People == nil {
		c.People = []PersonRel{}
	}
	if c.Titles == nil {
		c.Titles = []CompanyTitle{}
	}
	if c.Salaries == nil {
		c.Salaries = []CompanySalary{}
	}
	if c.Managers == nil {
		c.Managers = []CompanyManager{}
	}
	if c.Reports == nil {
		c.Reports = []CompanyReport{}
	}
	if c.Contracts == nil {
		c.Contracts = []CompanyContract{}
	}
	return c
}

func (c *Company) SetTitle(title string, startedOn time.Time) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("%w: title", ErrInvalid)
	}
	start := dateUTC(startedOn)
	for i := range c.Titles {
		if c.Titles[i].EndedOn != nil {
			continue
		}
		if c.Titles[i].Title == title {
			return nil
		}
		if start.Before(c.Titles[i].StartedOn) {
			return fmt.Errorf("%w: title dates", ErrInvalid)
		}
		end := start
		c.Titles[i].EndedOn = &end
		break
	}
	c.Titles = append(c.Titles, CompanyTitle{
		ID:        uuid.New(),
		CompanyID: c.ID,
		Title:     title,
		StartedOn: start,
	})
	c.UpdatedAt = time.Now().UTC()
	return c.validate()
}

func (c *Company) SetSalary(currency SalaryCurrency, amount float64, startedOn time.Time, comment string) error {
	if _, err := ParseSalaryCurrency(string(currency)); err != nil {
		return err
	}
	if amount < 0 {
		return fmt.Errorf("%w: salary amount", ErrInvalid)
	}
	start := dateUTC(startedOn)
	note := strings.TrimSpace(comment)
	for i := range c.Salaries {
		if c.Salaries[i].EndedOn != nil {
			continue
		}
		if c.Salaries[i].Currency == currency && c.Salaries[i].Amount == amount {
			return nil
		}
		if note == "" {
			return fmt.Errorf("%w: salary comment", ErrInvalid)
		}
		if start.Before(c.Salaries[i].StartedOn) {
			return fmt.Errorf("%w: salary dates", ErrInvalid)
		}
		end := start
		c.Salaries[i].EndedOn = &end
		break
	}
	c.Salaries = append(c.Salaries, CompanySalary{
		ID:        uuid.New(),
		CompanyID: c.ID,
		Currency:  currency,
		Amount:    amount,
		Comment:   note,
		StartedOn: start,
	})
	c.UpdatedAt = time.Now().UTC()
	return c.validate()
}

func (c *Company) SetManager(personID *uuid.UUID, startedOn time.Time) error {
	start := dateUTC(startedOn)
	if personID != nil && *personID == uuid.Nil {
		personID = nil
	}
	if personID != nil && c.hasOpenReport(*personID) {
		return fmt.Errorf("%w: manager is report", ErrInvalid)
	}
	for i := range c.Managers {
		if c.Managers[i].EndedOn != nil {
			continue
		}
		if personID != nil && c.Managers[i].PersonID == *personID {
			return nil
		}
		if start.Before(c.Managers[i].StartedOn) {
			return fmt.Errorf("%w: manager dates", ErrInvalid)
		}
		end := start
		c.Managers[i].EndedOn = &end
		break
	}
	if personID == nil {
		c.UpdatedAt = time.Now().UTC()
		return c.validate()
	}
	c.Managers = append(c.Managers, CompanyManager{
		ID:        uuid.New(),
		CompanyID: c.ID,
		PersonID:  *personID,
		StartedOn: start,
	})
	c.UpdatedAt = time.Now().UTC()
	return c.validate()
}

func (c *Company) AddReport(personID uuid.UUID, startedOn time.Time) error {
	if personID == uuid.Nil {
		return fmt.Errorf("%w: report person", ErrInvalid)
	}
	if mgr := c.openManager(); mgr != nil && mgr.PersonID == personID {
		return fmt.Errorf("%w: report is manager", ErrInvalid)
	}
	start := dateUTC(startedOn)
	for i := range c.Reports {
		if c.Reports[i].EndedOn != nil {
			continue
		}
		if c.Reports[i].PersonID == personID {
			return nil
		}
	}
	c.Reports = append(c.Reports, CompanyReport{
		ID:        uuid.New(),
		CompanyID: c.ID,
		PersonID:  personID,
		StartedOn: start,
	})
	c.UpdatedAt = time.Now().UTC()
	return c.validate()
}

func (c *Company) EndReport(reportID uuid.UUID, endedOn time.Time) error {
	end := dateUTC(endedOn)
	for i := range c.Reports {
		if c.Reports[i].ID != reportID {
			continue
		}
		if c.Reports[i].EndedOn != nil {
			return nil
		}
		if end.Before(c.Reports[i].StartedOn) {
			return fmt.Errorf("%w: report dates", ErrInvalid)
		}
		c.Reports[i].EndedOn = &end
		c.UpdatedAt = time.Now().UTC()
		return c.validate()
	}
	return fmt.Errorf("%w: report", ErrNotFound)
}

func (c *Company) SetContract(personID *uuid.UUID, kind ContractKind, startedOn time.Time) error {
	parsed, err := ParseContractKind(string(kind))
	if err != nil {
		return err
	}
	if personID != nil && *personID == uuid.Nil {
		personID = nil
	}
	if personID != nil && !c.hasPerson(*personID) {
		return fmt.Errorf("%w: contract person", ErrInvalid)
	}
	start := dateUTC(startedOn)
	for i := range c.Contracts {
		if !sameContractSubject(c.Contracts[i].PersonID, personID) || c.Contracts[i].EndedOn != nil {
			continue
		}
		if c.Contracts[i].Kind == parsed {
			return nil
		}
		if start.Before(c.Contracts[i].StartedOn) {
			return fmt.Errorf("%w: contract dates", ErrInvalid)
		}
		end := start
		c.Contracts[i].EndedOn = &end
		break
	}
	c.Contracts = append(c.Contracts, CompanyContract{
		ID:        uuid.New(),
		CompanyID: c.ID,
		PersonID:  copyUUID(personID),
		Kind:      parsed,
		StartedOn: start,
	})
	c.UpdatedAt = time.Now().UTC()
	return c.validate()
}

func copyUUID(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	value := *id
	return &value
}

func sameContractSubject(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func (c Company) hasPerson(id uuid.UUID) bool {
	for _, rel := range c.People {
		if rel.ID == id {
			return true
		}
	}
	return false
}

func (c Company) openManager() *CompanyManager {
	for i := range c.Managers {
		if c.Managers[i].EndedOn == nil {
			return &c.Managers[i]
		}
	}
	return nil
}

func (c Company) hasOpenReport(personID uuid.UUID) bool {
	for _, row := range c.Reports {
		if row.EndedOn == nil && row.PersonID == personID {
			return true
		}
	}
	return false
}

func (c Company) OpenTitle() *CompanyTitle {
	for i := range c.Titles {
		if c.Titles[i].EndedOn == nil {
			return &c.Titles[i]
		}
	}
	return nil
}

func (c Company) OpenSalary() *CompanySalary {
	for i := range c.Salaries {
		if c.Salaries[i].EndedOn == nil {
			return &c.Salaries[i]
		}
	}
	return nil
}

func (c Company) validate() error {
	if err := validateDated("title", openCount(c.Titles, func(row CompanyTitle) bool { return row.EndedOn == nil }), c.Titles, func(row CompanyTitle) (time.Time, *time.Time) {
		return row.StartedOn, row.EndedOn
	}); err != nil {
		return err
	}
	openSalary := 0
	for _, row := range c.Salaries {
		if _, err := ParseSalaryCurrency(string(row.Currency)); err != nil {
			return err
		}
		if row.Amount < 0 {
			return fmt.Errorf("%w: salary amount", ErrInvalid)
		}
		if row.EndedOn != nil && row.EndedOn.Before(row.StartedOn) {
			return fmt.Errorf("%w: salary dates", ErrInvalid)
		}
		if row.EndedOn == nil {
			openSalary++
		}
	}
	if openSalary > 1 {
		return fmt.Errorf("%w: open salary", ErrInvalid)
	}
	if err := validateDated("manager", openCount(c.Managers, func(row CompanyManager) bool { return row.EndedOn == nil }), c.Managers, func(row CompanyManager) (time.Time, *time.Time) {
		return row.StartedOn, row.EndedOn
	}); err != nil {
		return err
	}
	openReports := map[uuid.UUID]struct{}{}
	for _, row := range c.Reports {
		if row.PersonID == uuid.Nil {
			return fmt.Errorf("%w: report person", ErrInvalid)
		}
		if row.EndedOn != nil && row.EndedOn.Before(row.StartedOn) {
			return fmt.Errorf("%w: report dates", ErrInvalid)
		}
		if row.EndedOn == nil {
			if _, ok := openReports[row.PersonID]; ok {
				return fmt.Errorf("%w: open report", ErrInvalid)
			}
			openReports[row.PersonID] = struct{}{}
		}
	}
	if mgr := c.openManager(); mgr != nil {
		if _, ok := openReports[mgr.PersonID]; ok {
			return fmt.Errorf("%w: manager is report", ErrInvalid)
		}
	}
	openContracts := map[string]struct{}{}
	for _, row := range c.Contracts {
		if _, err := ParseContractKind(string(row.Kind)); err != nil {
			return err
		}
		if row.EndedOn != nil && row.EndedOn.Before(row.StartedOn) {
			return fmt.Errorf("%w: contract dates", ErrInvalid)
		}
		if row.EndedOn == nil {
			key := "me"
			if row.PersonID != nil {
				key = row.PersonID.String()
			}
			if _, ok := openContracts[key]; ok {
				return fmt.Errorf("%w: open contract", ErrInvalid)
			}
			openContracts[key] = struct{}{}
		}
	}
	return nil
}

func openCount[T any](rows []T, open func(T) bool) int {
	n := 0
	for _, row := range rows {
		if open(row) {
			n++
		}
	}
	return n
}

func validateDated[T any](label string, open int, rows []T, span func(T) (time.Time, *time.Time)) error {
	if open > 1 {
		return fmt.Errorf("%w: open %s", ErrInvalid, label)
	}
	for _, row := range rows {
		start, ended := span(row)
		if ended != nil && ended.Before(start) {
			return fmt.Errorf("%w: %s dates", ErrInvalid, label)
		}
	}
	return nil
}
