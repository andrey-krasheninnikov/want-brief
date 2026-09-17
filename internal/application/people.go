package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

type PersonWrite struct {
	Name      string
	BornOn    *time.Time
	AgeYears  *int
	Projects  []domain.PersonRel
	Events    []domain.PersonRel
	Companies []domain.PersonRel
	ItemIDs   []uuid.UUID
}

func (s *Service) ListPeople(ctx context.Context) ([]domain.Person, error) {
	return s.People.List(ctx)
}

func (s *Service) GetPerson(ctx context.Context, id uuid.UUID) (domain.Person, error) {
	return s.People.Get(ctx, id)
}

func (s *Service) CreatePerson(ctx context.Context, write PersonWrite) (domain.Person, error) {
	person, err := domain.NewPerson(draftFromPerson(write))
	if err != nil {
		return domain.Person{}, err
	}
	return s.People.Create(ctx, person)
}

func (s *Service) ReplacePerson(ctx context.Context, id uuid.UUID, write PersonWrite) (domain.Person, error) {
	person, err := s.People.Get(ctx, id)
	if err != nil {
		return domain.Person{}, err
	}
	if err := person.Apply(draftFromPerson(write)); err != nil {
		return domain.Person{}, err
	}
	return s.People.Update(ctx, person)
}

func (s *Service) DeletePerson(ctx context.Context, id uuid.UUID) error {
	return s.People.Delete(ctx, id)
}

func (s *Service) ListPersonNotes(ctx context.Context, personID uuid.UUID) ([]domain.PersonNote, error) {
	if _, err := s.People.Get(ctx, personID); err != nil {
		return nil, err
	}
	return s.PersonNotes.ListByPerson(ctx, personID)
}

func (s *Service) CreatePersonNote(ctx context.Context, personID uuid.UUID, body string) (domain.PersonNote, error) {
	if _, err := s.People.Get(ctx, personID); err != nil {
		return domain.PersonNote{}, err
	}
	note, err := domain.NewPersonNote(personID, body)
	if err != nil {
		return domain.PersonNote{}, err
	}
	return s.PersonNotes.Create(ctx, note)
}

func (s *Service) ReplacePersonNote(ctx context.Context, personID, noteID uuid.UUID, body string) (domain.PersonNote, error) {
	note, err := s.PersonNotes.Get(ctx, noteID)
	if err != nil {
		return domain.PersonNote{}, err
	}
	if note.PersonID != personID {
		return domain.PersonNote{}, domain.ErrNotFound
	}
	if err := note.Apply(body); err != nil {
		return domain.PersonNote{}, err
	}
	return s.PersonNotes.Update(ctx, note)
}

func (s *Service) DeletePersonNote(ctx context.Context, personID, noteID uuid.UUID) error {
	note, err := s.PersonNotes.Get(ctx, noteID)
	if err != nil {
		return err
	}
	if note.PersonID != personID {
		return domain.ErrNotFound
	}
	return s.PersonNotes.Delete(ctx, noteID)
}

func (s *Service) CreatePersonContact(ctx context.Context, personID uuid.UUID, draft domain.PersonContactDraft) (domain.PersonContact, error) {
	if _, err := s.People.Get(ctx, personID); err != nil {
		return domain.PersonContact{}, err
	}
	contact, err := domain.NewPersonContact(personID, draft)
	if err != nil {
		return domain.PersonContact{}, err
	}
	return s.PersonContacts.Create(ctx, contact)
}

func (s *Service) ReplacePersonContact(ctx context.Context, personID, contactID uuid.UUID, draft domain.PersonContactDraft) (domain.PersonContact, error) {
	contact, err := s.PersonContacts.Get(ctx, contactID)
	if err != nil {
		return domain.PersonContact{}, err
	}
	if contact.PersonID != personID {
		return domain.PersonContact{}, domain.ErrNotFound
	}
	if err := contact.Apply(draft); err != nil {
		return domain.PersonContact{}, err
	}
	return s.PersonContacts.Update(ctx, contact)
}

func (s *Service) DeletePersonContact(ctx context.Context, personID, contactID uuid.UUID) error {
	contact, err := s.PersonContacts.Get(ctx, contactID)
	if err != nil {
		return err
	}
	if contact.PersonID != personID {
		return domain.ErrNotFound
	}
	return s.PersonContacts.Delete(ctx, contactID)
}

func (s *Service) CreatePersonAbsence(ctx context.Context, personID uuid.UUID, draft domain.PersonAbsenceDraft) (domain.PersonAbsence, error) {
	if _, err := s.People.Get(ctx, personID); err != nil {
		return domain.PersonAbsence{}, err
	}
	absence, err := domain.NewPersonAbsence(personID, draft, s.now())
	if err != nil {
		return domain.PersonAbsence{}, err
	}
	return s.PersonAbsences.Create(ctx, absence)
}

func (s *Service) ReplacePersonAbsence(ctx context.Context, personID, absenceID uuid.UUID, draft domain.PersonAbsenceDraft) (domain.PersonAbsence, error) {
	absence, err := s.PersonAbsences.Get(ctx, absenceID)
	if err != nil {
		return domain.PersonAbsence{}, err
	}
	if absence.PersonID != personID {
		return domain.PersonAbsence{}, domain.ErrNotFound
	}
	if err := absence.Apply(draft, s.now()); err != nil {
		return domain.PersonAbsence{}, err
	}
	return s.PersonAbsences.Update(ctx, absence)
}

func (s *Service) DeletePersonAbsence(ctx context.Context, personID, absenceID uuid.UUID) error {
	absence, err := s.PersonAbsences.Get(ctx, absenceID)
	if err != nil {
		return err
	}
	if absence.PersonID != personID {
		return domain.ErrNotFound
	}
	return s.PersonAbsences.Delete(ctx, absenceID)
}

func (s *Service) CreatePersonSite(ctx context.Context, personID uuid.UUID, draft domain.PersonSiteDraft) (domain.PersonSite, error) {
	if _, err := s.People.Get(ctx, personID); err != nil {
		return domain.PersonSite{}, err
	}
	site, err := domain.NewPersonSite(personID, draft)
	if err != nil {
		return domain.PersonSite{}, err
	}
	return s.PersonSites.Create(ctx, site)
}

func (s *Service) ReplacePersonSite(ctx context.Context, personID, siteID uuid.UUID, draft domain.PersonSiteDraft) (domain.PersonSite, error) {
	site, err := s.PersonSites.Get(ctx, siteID)
	if err != nil {
		return domain.PersonSite{}, err
	}
	if site.PersonID != personID {
		return domain.PersonSite{}, domain.ErrNotFound
	}
	if err := site.Apply(draft); err != nil {
		return domain.PersonSite{}, err
	}
	return s.PersonSites.Update(ctx, site)
}

func (s *Service) DeletePersonSite(ctx context.Context, personID, siteID uuid.UUID) error {
	site, err := s.PersonSites.Get(ctx, siteID)
	if err != nil {
		return err
	}
	if site.PersonID != personID {
		return domain.ErrNotFound
	}
	return s.PersonSites.Delete(ctx, siteID)
}

type ProfessionWrite struct {
	Title            string
	Comment          string
	StartedOn        *time.Time
	EndedOn          *time.Time
	MonthlySalaryUSD float64
	MonthlySalaryRUB float64
}

func (s *Service) CreatePersonProfession(ctx context.Context, personID uuid.UUID, write ProfessionWrite) (domain.PersonProfession, error) {
	if _, err := s.People.Get(ctx, personID); err != nil {
		return domain.PersonProfession{}, err
	}
	now := s.now()
	started := domain.MoscowDate(now)
	if write.StartedOn != nil {
		started = *write.StartedOn
	}
	prof, err := domain.NewPersonProfession(personID, domain.PersonProfessionDraft{
		Title:     write.Title,
		Comment:   write.Comment,
		StartedOn: started,
		EndedOn:   write.EndedOn,
	})
	if err != nil {
		return domain.PersonProfession{}, err
	}
	if err := prof.SetSalary(write.MonthlySalaryUSD, write.MonthlySalaryRUB, started); err != nil {
		return domain.PersonProfession{}, err
	}
	return s.PersonProfessions.Create(ctx, prof)
}

func (s *Service) ReplacePersonProfession(ctx context.Context, personID, professionID uuid.UUID, write ProfessionWrite) (domain.PersonProfession, error) {
	prof, err := s.PersonProfessions.Get(ctx, professionID)
	if err != nil {
		return domain.PersonProfession{}, err
	}
	if prof.PersonID != personID {
		return domain.PersonProfession{}, domain.ErrNotFound
	}
	started := prof.StartedOn
	if write.StartedOn != nil {
		started = *write.StartedOn
	}
	if err := prof.Apply(domain.PersonProfessionDraft{
		Title:     write.Title,
		Comment:   write.Comment,
		StartedOn: started,
		EndedOn:   write.EndedOn,
	}); err != nil {
		return domain.PersonProfession{}, err
	}
	if err := prof.SetSalary(write.MonthlySalaryUSD, write.MonthlySalaryRUB, domain.MoscowDate(s.now())); err != nil {
		return domain.PersonProfession{}, err
	}
	return s.PersonProfessions.Update(ctx, prof)
}

func (s *Service) DeletePersonProfession(ctx context.Context, personID, professionID uuid.UUID) error {
	prof, err := s.PersonProfessions.Get(ctx, professionID)
	if err != nil {
		return err
	}
	if prof.PersonID != personID {
		return domain.ErrNotFound
	}
	return s.PersonProfessions.Delete(ctx, professionID)
}

type BondOpen struct {
	OtherID   *uuid.UUID
	Kind      string
	Comment   string
	StartedOn *time.Time
}

type BondPatch struct {
	domain.BondChange
	ActionComment string
}

type BondEnd struct {
	EndedOn *time.Time
	Comment string
}

func (s *Service) OpenBond(ctx context.Context, personID uuid.UUID, write BondOpen) (domain.PersonBond, error) {
	if _, err := s.People.Get(ctx, personID); err != nil {
		return domain.PersonBond{}, err
	}
	if write.OtherID != nil {
		if _, err := s.People.Get(ctx, *write.OtherID); err != nil {
			return domain.PersonBond{}, err
		}
	}
	kind, err := domain.ParseBondKind(write.Kind)
	if err != nil {
		return domain.PersonBond{}, err
	}
	a, b, err := domain.CanonicalBondPair(personID, write.OtherID)
	if err != nil {
		return domain.PersonBond{}, err
	}
	now := s.now()
	started := domain.MoscowDate(now)
	if write.StartedOn != nil {
		started = *write.StartedOn
	}
	bond, err := domain.NewPersonBond(a, b, kind, write.Comment, started, now)
	if err != nil {
		return domain.PersonBond{}, err
	}
	event := bond.Snapshot(domain.BondActionOpen, write.Comment, now)
	saved, err := s.PersonBonds.Create(ctx, bond, event)
	if err != nil {
		return domain.PersonBond{}, err
	}
	return saved.ViewedFrom(personID), nil
}

func (s *Service) ChangeBond(ctx context.Context, personID, bondID uuid.UUID, write BondPatch) (domain.PersonBond, error) {
	bond, err := s.bondForPerson(ctx, personID, bondID)
	if err != nil {
		return domain.PersonBond{}, err
	}
	now := s.now()
	if err := bond.Change(write.BondChange, now); err != nil {
		return domain.PersonBond{}, err
	}
	event := bond.Snapshot(domain.BondActionChange, write.ActionComment, now)
	saved, err := s.PersonBonds.Update(ctx, bond, event)
	if err != nil {
		return domain.PersonBond{}, err
	}
	return saved.ViewedFrom(personID), nil
}

func (s *Service) EndBond(ctx context.Context, personID, bondID uuid.UUID, write BondEnd) (domain.PersonBond, error) {
	bond, err := s.bondForPerson(ctx, personID, bondID)
	if err != nil {
		return domain.PersonBond{}, err
	}
	now := s.now()
	if err := bond.End(write.EndedOn, now); err != nil {
		return domain.PersonBond{}, err
	}
	event := bond.Snapshot(domain.BondActionEnd, write.Comment, now)
	saved, err := s.PersonBonds.Update(ctx, bond, event)
	if err != nil {
		return domain.PersonBond{}, err
	}
	return saved.ViewedFrom(personID), nil
}

func (s *Service) bondForPerson(ctx context.Context, personID, bondID uuid.UUID) (domain.PersonBond, error) {
	if _, err := s.People.Get(ctx, personID); err != nil {
		return domain.PersonBond{}, err
	}
	bond, err := s.PersonBonds.Get(ctx, bondID)
	if err != nil {
		return domain.PersonBond{}, err
	}
	if !bond.Involves(personID) {
		return domain.PersonBond{}, domain.ErrNotFound
	}
	return bond, nil
}

func draftFromPerson(write PersonWrite) domain.PersonDraft {
	return domain.PersonDraft{
		Name:      write.Name,
		BornOn:    write.BornOn,
		AgeYears:  write.AgeYears,
		Projects:  write.Projects,
		Events:    write.Events,
		Companies: write.Companies,
		ItemIDs:   write.ItemIDs,
	}
}
