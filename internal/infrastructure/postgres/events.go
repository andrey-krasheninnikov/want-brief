package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pchkauu/want-brief/internal/domain"
)

const eventCols = `id, title, description, agenda, kind, type, project_id, starts_at, duration_seconds,
	recurrence, repeat_until, weekdays, links, meet_url, involvement, active_start_offset, active_end_offset, can_skip, created_at, updated_at`

func scanEvent(scan func(dest ...any) error) (domain.Event, error) {
	var event domain.Event
	var links []byte
	var until *time.Time
	var days []int16
	err := scan(
		&event.ID, &event.Title, &event.Description, &event.Agenda, &event.Kind, &event.Type, &event.ProjectID,
		&event.StartsAt, &event.DurationSeconds, &event.Recurrence, &until, &days, &links, &event.MeetURL, &event.Involvement,
		&event.ActiveStartOffset, &event.ActiveEndOffset, &event.CanSkip, &event.CreatedAt, &event.UpdatedAt,
	)
	if err != nil {
		return domain.Event{}, err
	}
	if until != nil {
		day := domain.Ymd(until.Format("2006-01-02"))
		event.RepeatUntil = &day
	}
	event.Weekdays = int16sToInts(days)
	event.Links, err = decodeProjectLinks(links)
	if err != nil {
		return domain.Event{}, err
	}
	return event, nil
}

func int16sToInts(days []int16) []int {
	out := make([]int, 0, len(days))
	for _, day := range days {
		out = append(out, int(day))
	}
	return out
}

func intsToInt16s(days []int) []int16 {
	out := make([]int16, 0, len(days))
	for _, day := range days {
		out = append(out, int16(day))
	}
	return out
}

func repeatUntilArg(until *domain.Ymd) any {
	if until == nil {
		return nil
	}
	return string(*until)
}

func (s *Store) attachEventPeople(ctx context.Context, rows []domain.Event) error {
	ids := make([]uuid.UUID, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, event := range rows {
		ids[i] = event.ID
		index[event.ID] = i
		rows[i].People = []domain.PersonRel{}
	}
	return s.attachRels(ctx, "person_events", "event_id", "person_id", ids, func(id uuid.UUID, people []domain.PersonRel) {
		rows[index[id]].People = people
	})
}

func (s *Store) GetEvent(ctx context.Context, id uuid.UUID) (domain.Event, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+eventCols+` FROM events WHERE id=$1`, id)
	event, err := scanEvent(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Event{}, err
	}
	out := []domain.Event{event}
	if err := s.attachEventPeople(ctx, out); err != nil {
		return domain.Event{}, err
	}
	return out[0], nil
}

func (s *Store) ListEvents(ctx context.Context) ([]domain.Event, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+eventCols+` FROM events ORDER BY starts_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Event
	for rows.Next() {
		event, err := scanEvent(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachEventPeople(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) CreateEvent(ctx context.Context, event domain.Event) (domain.Event, error) {
	raw, err := encodeProjectLinks(event.Links)
	if err != nil {
		return domain.Event{}, err
	}
	ends := event.EndsAt()
	row := s.pool.QueryRow(ctx, `
		INSERT INTO events (
			id, title, description, agenda, kind, type, project_id, starts_at, ends_at, duration_seconds,
			recurrence, repeat_until, weekdays, links, meet_url, involvement, active_start_offset, active_end_offset, can_skip, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		RETURNING `+eventCols, event.ID, event.Title, event.Description, event.Agenda, string(event.Kind), string(event.Type),
		event.ProjectID, event.StartsAt, ends, event.DurationSeconds, string(event.Recurrence), repeatUntilArg(event.RepeatUntil),
		intsToInt16s(event.Weekdays), raw, event.MeetURL, event.Involvement, event.ActiveStartOffset, event.ActiveEndOffset,
		event.CanSkip, event.CreatedAt, event.UpdatedAt)
	people := event.People
	created, err := scanEvent(row.Scan)
	if err != nil {
		return domain.Event{}, err
	}
	if err := s.replaceRels(ctx, "person_events", "event_id", "person_id", created.ID, people); err != nil {
		return domain.Event{}, err
	}
	return s.GetEvent(ctx, created.ID)
}

func (s *Store) UpdateEvent(ctx context.Context, event domain.Event) (domain.Event, error) {
	raw, err := encodeProjectLinks(event.Links)
	if err != nil {
		return domain.Event{}, err
	}
	ends := event.EndsAt()
	tag, err := s.pool.Exec(ctx, `
		UPDATE events SET
			title=$2, description=$3, agenda=$4, kind=$5, type=$6, project_id=$7, starts_at=$8, ends_at=$9,
			duration_seconds=$10, recurrence=$11, repeat_until=$12, weekdays=$13, links=$14, meet_url=$15, involvement=$16,
			active_start_offset=$17, active_end_offset=$18, can_skip=$19, updated_at=$20
		WHERE id=$1
	`, event.ID, event.Title, event.Description, event.Agenda, string(event.Kind), string(event.Type), event.ProjectID,
		event.StartsAt, ends, event.DurationSeconds, string(event.Recurrence), repeatUntilArg(event.RepeatUntil),
		intsToInt16s(event.Weekdays), raw, event.MeetURL, event.Involvement, event.ActiveStartOffset, event.ActiveEndOffset,
		event.CanSkip, event.UpdatedAt)
	if err != nil {
		return domain.Event{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Event{}, domain.ErrNotFound
	}
	if err := s.replaceRels(ctx, "person_events", "event_id", "person_id", event.ID, event.People); err != nil {
		return domain.Event{}, err
	}
	return s.GetEvent(ctx, event.ID)
}

func (s *Store) DeleteEvent(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM events WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type EventRepo struct{ *Store }

func (r EventRepo) Get(ctx context.Context, id uuid.UUID) (domain.Event, error) {
	return r.Store.GetEvent(ctx, id)
}
func (r EventRepo) List(ctx context.Context) ([]domain.Event, error) {
	return r.Store.ListEvents(ctx)
}
func (r EventRepo) Create(ctx context.Context, event domain.Event) (domain.Event, error) {
	return r.Store.CreateEvent(ctx, event)
}
func (r EventRepo) Update(ctx context.Context, event domain.Event) (domain.Event, error) {
	return r.Store.UpdateEvent(ctx, event)
}
func (r EventRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.Store.DeleteEvent(ctx, id)
}
