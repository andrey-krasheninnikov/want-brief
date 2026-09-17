package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

type EventWrite struct {
	Title             string
	Description       string
	Agenda            string
	Kind              string
	Type              string
	ProjectID         *uuid.UUID
	StartsAt          time.Time
	DurationSeconds   int
	Recurrence        string
	RepeatUntil       *domain.Ymd
	Weekdays          []int
	Links             []domain.ProjectLink
	MeetURL           string
	Involvement       *int
	ActiveStartOffset *int
	ActiveEndOffset   *int
	CanSkip           *bool
	People            *[]domain.PersonRel
}

type EventOccurrenceWrite struct {
	OriginalOn      domain.Ymd
	StartsAt        *time.Time
	DurationSeconds *int
	Skipped         bool
}

func (s *Service) ListEventOccurrences(ctx context.Context, from, to time.Time) ([]domain.EventOccurrence, error) {
	series, err := s.Events.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(series))
	for _, event := range series {
		ids = append(ids, event.ID)
	}
	overrides, err := s.EventOverrides.ListBySeries(ctx, ids)
	if err != nil {
		return nil, err
	}
	occs := domain.ExpandEvents(series, from, to)
	out := domain.MergeOccurrences(series, occs, overrides, from, to)
	if out == nil {
		return []domain.EventOccurrence{}, nil
	}
	return out, nil
}

func (s *Service) ListEvents(ctx context.Context) ([]domain.Event, error) {
	return s.Events.List(ctx)
}

func (s *Service) GetEvent(ctx context.Context, id uuid.UUID) (domain.Event, error) {
	return s.Events.Get(ctx, id)
}

func (s *Service) GetEventOccurrence(ctx context.Context, id uuid.UUID, originalOn domain.Ymd) (domain.EventOccurrence, error) {
	event, err := s.Events.Get(ctx, id)
	if err != nil {
		return domain.EventOccurrence{}, err
	}
	override, err := s.EventOverrides.Get(ctx, id, originalOn)
	if err != nil && err != domain.ErrNotFound {
		return domain.EventOccurrence{}, err
	}
	var ov *domain.EventOverride
	if err == nil {
		ov = &override
	}
	return event.SlotOccurrence(originalOn, ov)
}

func (s *Service) PutEventOccurrence(ctx context.Context, id uuid.UUID, write EventOccurrenceWrite) (domain.EventOccurrence, error) {
	event, err := s.Events.Get(ctx, id)
	if err != nil {
		return domain.EventOccurrence{}, err
	}
	if !event.HasSlot(write.OriginalOn) {
		return domain.EventOccurrence{}, domain.ErrNotFound
	}
	override, err := domain.NewEventOverride(id, write.OriginalOn, write.StartsAt, write.DurationSeconds, write.Skipped)
	if err != nil {
		return domain.EventOccurrence{}, err
	}
	saved, err := s.EventOverrides.Upsert(ctx, override)
	if err != nil {
		return domain.EventOccurrence{}, err
	}
	return event.SlotOccurrence(write.OriginalOn, &saved)
}

func (s *Service) DeleteEventOccurrence(ctx context.Context, id uuid.UUID, originalOn domain.Ymd) error {
	if _, err := s.Events.Get(ctx, id); err != nil {
		return err
	}
	err := s.EventOverrides.Delete(ctx, id, originalOn)
	if err == domain.ErrNotFound {
		return nil
	}
	return err
}

func (s *Service) ListEventNotes(ctx context.Context, id uuid.UUID, originalOn *domain.Ymd) ([]domain.EventNote, error) {
	if _, err := s.Events.Get(ctx, id); err != nil {
		return nil, err
	}
	return s.EventNotes.ListBySeries(ctx, id, originalOn)
}

func (s *Service) CreateEventNote(ctx context.Context, id uuid.UUID, originalOn domain.Ymd, body string) (domain.EventNote, error) {
	event, err := s.Events.Get(ctx, id)
	if err != nil {
		return domain.EventNote{}, err
	}
	if !event.HasSlot(originalOn) {
		return domain.EventNote{}, domain.ErrInvalid
	}
	note, err := domain.NewEventNote(id, originalOn, body)
	if err != nil {
		return domain.EventNote{}, err
	}
	return s.EventNotes.Create(ctx, note)
}

func (s *Service) DeleteEventNote(ctx context.Context, id, noteID uuid.UUID) error {
	if _, err := s.Events.Get(ctx, id); err != nil {
		return err
	}
	note, err := s.EventNotes.Get(ctx, noteID)
	if err != nil {
		return err
	}
	if note.SeriesID != id {
		return domain.ErrNotFound
	}
	return s.EventNotes.Delete(ctx, noteID)
}

func (s *Service) CreateEvent(ctx context.Context, write EventWrite) (domain.Event, error) {
	draft, err := draftFromWrite(write)
	if err != nil {
		return domain.Event{}, err
	}
	event, err := domain.NewEventFrom(draft)
	if err != nil {
		return domain.Event{}, err
	}
	if write.People != nil {
		people, nerr := domain.NormalizePersonRels(*write.People)
		if nerr != nil {
			return domain.Event{}, nerr
		}
		event.People = people
	} else {
		event.People = []domain.PersonRel{}
	}
	return s.Events.Create(ctx, event)
}

func (s *Service) ReplaceEvent(ctx context.Context, id uuid.UUID, write EventWrite) (domain.Event, error) {
	event, err := s.Events.Get(ctx, id)
	if err != nil {
		return domain.Event{}, err
	}
	draft, err := draftFromWrite(write)
	if err != nil {
		return domain.Event{}, err
	}
	if err := event.Apply(draft); err != nil {
		return domain.Event{}, err
	}
	if write.People != nil {
		people, nerr := domain.NormalizePersonRels(*write.People)
		if nerr != nil {
			return domain.Event{}, nerr
		}
		event.People = people
	}
	return s.Events.Update(ctx, event)
}

func (s *Service) DeleteEvent(ctx context.Context, id uuid.UUID) error {
	return s.Events.Delete(ctx, id)
}

func draftFromWrite(write EventWrite) (domain.EventDraft, error) {
	kind := domain.EventKindCall
	if write.Kind != "" {
		parsed, err := domain.ParseEventKind(write.Kind)
		if err != nil {
			return domain.EventDraft{}, err
		}
		kind = parsed
	}
	typ, err := domain.ParseEventType(write.Type)
	if err != nil {
		return domain.EventDraft{}, err
	}
	rec, err := domain.ParseEventRecurrence(write.Recurrence)
	if err != nil {
		return domain.EventDraft{}, err
	}
	duration := write.DurationSeconds
	if duration < 1 {
		duration = domain.DefaultEventDuration
	}
	involvement := 5
	if write.Involvement != nil {
		involvement = *write.Involvement
	}
	canSkip := true
	if write.CanSkip != nil {
		canSkip = *write.CanSkip
	}
	return domain.EventDraft{
		Title:             write.Title,
		Description:       write.Description,
		Agenda:            write.Agenda,
		Kind:              kind,
		Type:              typ,
		ProjectID:         write.ProjectID,
		StartsAt:          write.StartsAt,
		DurationSeconds:   duration,
		Recurrence:        rec,
		RepeatUntil:       write.RepeatUntil,
		Weekdays:          write.Weekdays,
		Links:             write.Links,
		MeetURL:           write.MeetURL,
		Involvement:       involvement,
		ActiveStartOffset: write.ActiveStartOffset,
		ActiveEndOffset:   write.ActiveEndOffset,
		CanSkip:           canSkip,
	}, nil
}
