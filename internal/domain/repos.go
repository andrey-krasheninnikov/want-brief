package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ItemFilter struct {
	SourceID        *uuid.UUID
	ProjectID       *uuid.UUID
	Kind            *ItemKind
	Status          *ItemStatus
	OpenOnly        bool
	IncludeArchived bool
	ArchivedOnly    bool
}

type ItemRepository interface {
	Get(ctx context.Context, id uuid.UUID) (Item, error)
	List(ctx context.Context, filter ItemFilter) ([]Item, error)
	Create(ctx context.Context, item Item) (Item, error)
	Update(ctx context.Context, item Item) (Item, error)
	Delete(ctx context.Context, id uuid.UUID) error
	UpsertSynced(ctx context.Context, item Item) (Item, error)
}

type ProjectRepository interface {
	Get(ctx context.Context, id uuid.UUID) (Project, error)
	List(ctx context.Context) ([]Project, error)
	Create(ctx context.Context, project Project) (Project, error)
	Update(ctx context.Context, project Project) (Project, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProjectNoteRepository interface {
	Get(ctx context.Context, id uuid.UUID) (ProjectNote, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]ProjectNote, error)
	Create(ctx context.Context, note ProjectNote) (ProjectNote, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ItemNoteRepository interface {
	Get(ctx context.Context, id uuid.UUID) (ItemNote, error)
	ListByItem(ctx context.Context, itemID uuid.UUID) ([]ItemNote, error)
	Create(ctx context.Context, note ItemNote) (ItemNote, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ItemCheckRepository interface {
	Get(ctx context.Context, id uuid.UUID) (ItemCheck, error)
	ListByItem(ctx context.Context, itemID uuid.UUID) ([]ItemCheck, error)
	Create(ctx context.Context, check ItemCheck) (ItemCheck, error)
	Update(ctx context.Context, check ItemCheck) (ItemCheck, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ItemEventRepository interface {
	Create(ctx context.Context, event ItemEvent) (ItemEvent, error)
	Update(ctx context.Context, event ItemEvent) (ItemEvent, error)
	ListByItem(ctx context.Context, itemID uuid.UUID) ([]ItemEvent, error)
	LatestByItemAndKinds(ctx context.Context, itemID uuid.UUID, kinds []ItemEventKind) (ItemEvent, error)
}

type SourceRepository interface {
	Get(ctx context.Context, id uuid.UUID) (Source, error)
	List(ctx context.Context) ([]Source, error)
	Create(ctx context.Context, source Source) (Source, error)
	Update(ctx context.Context, source Source) (Source, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Manual(ctx context.Context) (Source, error)
}

type NoteRepository interface {
	Get(ctx context.Context, id uuid.UUID) (Note, error)
	List(ctx context.Context, itemID *uuid.UUID) ([]Note, error)
	Create(ctx context.Context, note Note) (Note, error)
	Update(ctx context.Context, note Note) (Note, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type IntervalRepository interface {
	Get(ctx context.Context, id uuid.UUID) (TimeInterval, error)
	ListOpen(ctx context.Context) ([]TimeInterval, error)
	ListRange(ctx context.Context, from, to time.Time) ([]TimeInterval, error)
	Create(ctx context.Context, interval TimeInterval) (TimeInterval, error)
	Update(ctx context.Context, interval TimeInterval) (TimeInterval, error)
}

type StressRepository interface {
	ListRange(ctx context.Context, from, to time.Time) ([]StressLog, error)
	Latest(ctx context.Context) ([]StressLog, error)
	Create(ctx context.Context, log StressLog) (StressLog, error)
}

type PersonRepository interface {
	Get(ctx context.Context, id uuid.UUID) (Person, error)
	List(ctx context.Context) ([]Person, error)
	Create(ctx context.Context, person Person) (Person, error)
	Update(ctx context.Context, person Person) (Person, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type JournalRepository interface {
	List(ctx context.Context) ([]JournalEntry, error)
}

type PersonNoteRepository interface {
	Get(ctx context.Context, id uuid.UUID) (PersonNote, error)
	ListByPerson(ctx context.Context, personID uuid.UUID) ([]PersonNote, error)
	Create(ctx context.Context, note PersonNote) (PersonNote, error)
	Update(ctx context.Context, note PersonNote) (PersonNote, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type PersonContactRepository interface {
	Get(ctx context.Context, id uuid.UUID) (PersonContact, error)
	ListByPerson(ctx context.Context, personID uuid.UUID) ([]PersonContact, error)
	Create(ctx context.Context, contact PersonContact) (PersonContact, error)
	Update(ctx context.Context, contact PersonContact) (PersonContact, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type PersonSiteRepository interface {
	Get(ctx context.Context, id uuid.UUID) (PersonSite, error)
	ListByPerson(ctx context.Context, personID uuid.UUID) ([]PersonSite, error)
	Create(ctx context.Context, site PersonSite) (PersonSite, error)
	Update(ctx context.Context, site PersonSite) (PersonSite, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type PersonBondRepository interface {
	Get(ctx context.Context, id uuid.UUID) (PersonBond, error)
	ListByPerson(ctx context.Context, personID uuid.UUID) ([]PersonBond, error)
	Create(ctx context.Context, bond PersonBond, event PersonBondEvent) (PersonBond, error)
	Update(ctx context.Context, bond PersonBond, event PersonBondEvent) (PersonBond, error)
}

type PersonAbsenceRepository interface {
	Get(ctx context.Context, id uuid.UUID) (PersonAbsence, error)
	ListByPerson(ctx context.Context, personID uuid.UUID) ([]PersonAbsence, error)
	ListRange(ctx context.Context, from, to time.Time) ([]PersonAbsence, error)
	Create(ctx context.Context, absence PersonAbsence) (PersonAbsence, error)
	Update(ctx context.Context, absence PersonAbsence) (PersonAbsence, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ScheduleSettingsRepository interface {
	Get(ctx context.Context) (ScheduleSettings, error)
	Save(ctx context.Context, settings ScheduleSettings) (ScheduleSettings, error)
}

type DayOverrideRepository interface {
	ListRange(ctx context.Context, from, to Ymd) ([]DayOverride, error)
	Upsert(ctx context.Context, override DayOverride) (DayOverride, error)
	Delete(ctx context.Context, day Ymd) error
}

type ScheduleSnapshotRepository interface {
	Get(ctx context.Context, kind ScheduleKind) (ScheduleSnapshot, error)
	Save(ctx context.Context, snapshot ScheduleSnapshot) error
}

type PersonProfessionRepository interface {
	Get(ctx context.Context, id uuid.UUID) (PersonProfession, error)
	ListByPerson(ctx context.Context, personID uuid.UUID) ([]PersonProfession, error)
	Create(ctx context.Context, profession PersonProfession) (PersonProfession, error)
	Update(ctx context.Context, profession PersonProfession) (PersonProfession, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type EventRepository interface {
	Get(ctx context.Context, id uuid.UUID) (Event, error)
	List(ctx context.Context) ([]Event, error)
	Create(ctx context.Context, event Event) (Event, error)
	Update(ctx context.Context, event Event) (Event, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type EventOverrideRepository interface {
	Get(ctx context.Context, seriesID uuid.UUID, originalOn Ymd) (EventOverride, error)
	ListBySeries(ctx context.Context, seriesIDs []uuid.UUID) ([]EventOverride, error)
	Upsert(ctx context.Context, override EventOverride) (EventOverride, error)
	Delete(ctx context.Context, seriesID uuid.UUID, originalOn Ymd) error
}

type EventNoteRepository interface {
	Get(ctx context.Context, id uuid.UUID) (EventNote, error)
	ListBySeries(ctx context.Context, seriesID uuid.UUID, originalOn *Ymd) ([]EventNote, error)
	Create(ctx context.Context, note EventNote) (EventNote, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type CompanyRepository interface {
	Get(ctx context.Context, id uuid.UUID) (Company, error)
	List(ctx context.Context) ([]Company, error)
	Create(ctx context.Context, company Company) (Company, error)
	Update(ctx context.Context, company Company) (Company, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type CompanyNoteRepository interface {
	Get(ctx context.Context, id uuid.UUID) (CompanyNote, error)
	ListByCompany(ctx context.Context, companyID uuid.UUID) ([]CompanyNote, error)
	Create(ctx context.Context, note CompanyNote) (CompanyNote, error)
	Update(ctx context.Context, note CompanyNote) (CompanyNote, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type UserRepository interface {
	Count(ctx context.Context) (int, error)
	Create(ctx context.Context, user User) (User, error)
	First(ctx context.Context) (User, error)
}

type SessionRepository interface {
	Create(ctx context.Context, session Session) error
	GetByTokenHash(ctx context.Context, tokenHash string) (Session, error)
	Delete(ctx context.Context, tokenHash string) error
	DeleteExpired(ctx context.Context, now time.Time) error
}

type Puller interface {
	Probe(ctx context.Context, source Source, token string) error
	Pull(ctx context.Context, source Source, token string) ([]RemoteItem, error)
	Fetch(ctx context.Context, source Source, token, externalKey string) (RemoteItem, error)
}

type TokenBox interface {
	Seal(plaintext string) (string, error)
	Open(sealed string) (string, error)
}
