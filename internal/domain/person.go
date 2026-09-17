package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type PersonRel struct {
	ID      uuid.UUID `json:"id"`
	Comment string    `json:"comment"`
}

type Person struct {
	ID          uuid.UUID          `json:"id"`
	Name        string             `json:"name"`
	BornOn      *time.Time         `json:"bornOn"`
	Age         *int               `json:"age"`
	Projects    []PersonRel        `json:"projects"`
	Events      []PersonRel        `json:"events"`
	Companies   []PersonRel        `json:"companies"`
	ItemIDs     []uuid.UUID        `json:"itemIds"`
	Contacts    []PersonContact    `json:"contacts"`
	Sites       []PersonSite       `json:"sites"`
	Bonds       []PersonBond       `json:"bonds"`
	Professions []PersonProfession `json:"professions"`
	Absences    []PersonAbsence    `json:"absences"`
	MeBond      *MeBond            `json:"meBond"`
	LastNoteAt  *time.Time         `json:"lastNoteAt"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
}

type PersonDraft struct {
	Name      string
	BornOn    *time.Time
	AgeYears  *int
	Projects  []PersonRel
	Events    []PersonRel
	Companies []PersonRel
	ItemIDs   []uuid.UUID
}

func NewPerson(draft PersonDraft) (Person, error) {
	return newPersonAt(draft, time.Now().UTC())
}

func newPersonAt(draft PersonDraft, now time.Time) (Person, error) {
	person := Person{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := person.apply(draft, now); err != nil {
		return Person{}, err
	}
	return person.WithAge(now), nil
}

func (p *Person) Apply(draft PersonDraft) error {
	now := time.Now().UTC()
	if err := p.apply(draft, now); err != nil {
		return err
	}
	p.UpdatedAt = now
	return nil
}

func (p *Person) apply(draft PersonDraft, now time.Time) error {
	name := strings.TrimSpace(draft.Name)
	if name == "" {
		return fmt.Errorf("%w: person name", ErrInvalid)
	}
	if draft.AgeYears != nil && (*draft.AgeYears < 0 || *draft.AgeYears > 150) {
		return fmt.Errorf("%w: ageYears", ErrInvalid)
	}
	var bornOn *time.Time
	if draft.BornOn != nil {
		day := dateUTC(*draft.BornOn)
		bornOn = &day
	} else if draft.AgeYears != nil {
		day := PresumeBornOn(*draft.AgeYears, now)
		bornOn = &day
	}
	p.Name = name
	p.BornOn = bornOn
	projects, err := NormalizePersonRels(draft.Projects)
	if err != nil {
		return err
	}
	events, err := NormalizePersonRels(draft.Events)
	if err != nil {
		return err
	}
	p.Projects = projects
	p.Events = events
	p.Companies = NormalizeOptionalRels(draft.Companies)
	p.ItemIDs = NormalizeIDs(draft.ItemIDs)
	return nil
}

func PresumeBornOn(age int, now time.Time) time.Time {
	return MoscowDate(now).AddDate(-age, 0, 0)
}

func MoscowDate(now time.Time) time.Time {
	y, m, d := now.In(Moscow()).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func dateUTC(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
}

func (p Person) AgeAt(now time.Time) *int {
	if p.BornOn == nil {
		return nil
	}
	years := yearsSince(*p.BornOn, now)
	return &years
}

func (p Person) WithAge(now time.Time) Person {
	p.Age = p.AgeAt(now)
	if p.Projects == nil {
		p.Projects = []PersonRel{}
	}
	if p.Events == nil {
		p.Events = []PersonRel{}
	}
	if p.Companies == nil {
		p.Companies = []PersonRel{}
	}
	if p.ItemIDs == nil {
		p.ItemIDs = []uuid.UUID{}
	}
	if p.Contacts == nil {
		p.Contacts = []PersonContact{}
	}
	if p.Sites == nil {
		p.Sites = []PersonSite{}
	}
	if p.Bonds == nil {
		p.Bonds = []PersonBond{}
	}
	if p.Professions == nil {
		p.Professions = []PersonProfession{}
	}
	if p.Absences == nil {
		p.Absences = []PersonAbsence{}
	}
	return p
}

func yearsSince(born, now time.Time) int {
	today := MoscowDate(now)
	bornDay := dateUTC(born)
	years := today.Year() - bornDay.Year()
	anniversary := time.Date(today.Year(), bornDay.Month(), bornDay.Day(), 0, 0, 0, 0, time.UTC)
	if today.Before(anniversary) {
		years--
	}
	if years < 0 {
		return 0
	}
	return years
}

func NormalizePersonRels(rels []PersonRel) ([]PersonRel, error) {
	out := make([]PersonRel, 0, len(rels))
	seen := make(map[uuid.UUID]struct{}, len(rels))
	for _, rel := range rels {
		if rel.ID == uuid.Nil {
			continue
		}
		if _, ok := seen[rel.ID]; ok {
			continue
		}
		comment := strings.TrimSpace(rel.Comment)
		if comment == "" {
			return nil, fmt.Errorf("%w: link comment", ErrInvalid)
		}
		seen[rel.ID] = struct{}{}
		out = append(out, PersonRel{ID: rel.ID, Comment: comment})
	}
	return out, nil
}

func NormalizeIDs(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
