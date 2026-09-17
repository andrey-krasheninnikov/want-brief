package domain

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type EventKind string

const (
	EventKindEvent EventKind = "event"
	EventKindCall  EventKind = "call"
)

type EventType string

const (
	EventTypeSync       EventType = "sync"
	EventTypeGrooming   EventType = "grooming"
	EventTypeLesson     EventType = "lesson"
	EventTypeMentorship EventType = "mentorship"
	EventTypePlanning   EventType = "planning"
	EventTypeDaily      EventType = "daily"
	EventTypeRetro      EventType = "retro"
	EventTypeOneOnOne   EventType = "one_on_one"
	EventTypeGlobal     EventType = "global"
	EventTypeTeamBuild  EventType = "team_building"
	EventTypeExternal   EventType = "external"
)

type EventRecurrence string

const (
	RecurrenceOnce    EventRecurrence = "once"
	RecurrenceWeekly  EventRecurrence = "weekly"
	RecurrenceMonthly EventRecurrence = "monthly"
)

const (
	DefaultEventDuration = 30 * 60
	maxEventExpand       = 8 * 7 * 24 * time.Hour
	maxEventInvolvement  = 10
	weekdayCycleDays     = 14
)

type Event struct {
	ID                uuid.UUID       `json:"id"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	Agenda            string          `json:"agenda"`
	Kind              EventKind       `json:"kind"`
	Type              EventType       `json:"type"`
	ProjectID         *uuid.UUID      `json:"projectId"`
	StartsAt          time.Time       `json:"startsAt"`
	DurationSeconds   int             `json:"durationSeconds"`
	Recurrence        EventRecurrence `json:"recurrence"`
	RepeatUntil       *Ymd            `json:"repeatUntil"`
	Weekdays          []int           `json:"weekdays"`
	Links             []ProjectLink   `json:"links"`
	MeetURL           string          `json:"meetUrl"`
	Involvement       int             `json:"involvement"`
	ActiveStartOffset *int            `json:"activeStartOffset"`
	ActiveEndOffset   *int            `json:"activeEndOffset"`
	CanSkip           bool            `json:"canSkip"`
	People            []PersonRel     `json:"people"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

type EventOccurrence struct {
	ID                uuid.UUID       `json:"id"`
	SeriesID          uuid.UUID       `json:"seriesId"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	Agenda            string          `json:"agenda"`
	Kind              EventKind       `json:"kind"`
	Type              EventType       `json:"type"`
	ProjectID         *uuid.UUID      `json:"projectId"`
	StartsAt          time.Time       `json:"startsAt"`
	EndsAt            time.Time       `json:"endsAt"`
	DurationSeconds   int             `json:"durationSeconds"`
	Recurrence        EventRecurrence `json:"recurrence"`
	OriginalOn        Ymd             `json:"originalOn"`
	Overridden        bool            `json:"overridden"`
	Skipped           bool            `json:"skipped,omitempty"`
	Links             []ProjectLink   `json:"links"`
	MeetURL           string          `json:"meetUrl"`
	Involvement       int             `json:"involvement"`
	ActiveStartOffset *int            `json:"activeStartOffset"`
	ActiveEndOffset   *int            `json:"activeEndOffset"`
	CanSkip           bool            `json:"canSkip"`
	People            []PersonRel     `json:"people"`
	CreatedAt         time.Time       `json:"createdAt"`
}

type EventDraft struct {
	Title             string
	Description       string
	Agenda            string
	Kind              EventKind
	Type              EventType
	ProjectID         *uuid.UUID
	StartsAt          time.Time
	DurationSeconds   int
	Recurrence        EventRecurrence
	RepeatUntil       *Ymd
	Weekdays          []int
	Links             []ProjectLink
	MeetURL           string
	Involvement       int
	ActiveStartOffset *int
	ActiveEndOffset   *int
	CanSkip           bool
}

func ParseEventKind(raw string) (EventKind, error) {
	kind := EventKind(raw)
	switch kind {
	case EventKindEvent, EventKindCall:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: event kind", ErrInvalid)
	}
}

func ParseEventType(raw string) (EventType, error) {
	if raw == "" {
		return EventTypeSync, nil
	}
	typ := EventType(raw)
	switch typ {
	case EventTypeSync, EventTypeGrooming, EventTypeLesson, EventTypeMentorship, EventTypePlanning,
		EventTypeDaily, EventTypeRetro, EventTypeOneOnOne, EventTypeGlobal, EventTypeTeamBuild, EventTypeExternal:
		return typ, nil
	default:
		return "", fmt.Errorf("%w: event type", ErrInvalid)
	}
}

func ParseEventRecurrence(raw string) (EventRecurrence, error) {
	if raw == "" {
		return RecurrenceOnce, nil
	}
	rec := EventRecurrence(raw)
	switch rec {
	case RecurrenceOnce, RecurrenceWeekly, RecurrenceMonthly:
		return rec, nil
	default:
		return "", fmt.Errorf("%w: event recurrence", ErrInvalid)
	}
}

func NormalizeWeekdays(raw []int) ([]int, error) {
	seen := map[int]bool{}
	out := make([]int, 0, len(raw))
	for _, slot := range raw {
		if slot < 0 || slot >= weekdayCycleDays {
			return nil, fmt.Errorf("%w: weekday", ErrInvalid)
		}
		if seen[slot] {
			continue
		}
		seen[slot] = true
		out = append(out, slot)
	}
	sort.Ints(out)
	return out, nil
}

func NewEvent(title string, kind EventKind, startsAt time.Time, endsAt *time.Time) (Event, error) {
	duration := DefaultEventDuration
	if endsAt != nil && !endsAt.Before(startsAt) {
		duration = int(endsAt.Sub(startsAt).Seconds())
		if duration < 1 {
			duration = DefaultEventDuration
		}
	}
	return NewEventFrom(EventDraft{
		Title:           title,
		Kind:            kind,
		Type:            EventTypeSync,
		StartsAt:        startsAt,
		DurationSeconds: duration,
		Recurrence:      RecurrenceOnce,
		Involvement:     5,
		CanSkip:         true,
	})
}

func NewEventFrom(draft EventDraft) (Event, error) {
	now := time.Now().UTC()
	event := Event{
		ID:        uuid.New(),
		CanSkip:   draft.CanSkip,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := event.apply(draft); err != nil {
		return Event{}, err
	}
	return event, nil
}

func (e *Event) Apply(draft EventDraft) error {
	if err := e.apply(draft); err != nil {
		return err
	}
	e.UpdatedAt = time.Now().UTC()
	return nil
}

func (e *Event) apply(draft EventDraft) error {
	title := strings.TrimSpace(draft.Title)
	if title == "" {
		return fmt.Errorf("%w: title", ErrInvalid)
	}
	if _, err := ParseEventKind(string(draft.Kind)); err != nil {
		return err
	}
	typ, err := ParseEventType(string(draft.Type))
	if err != nil {
		return err
	}
	if draft.StartsAt.IsZero() {
		return fmt.Errorf("%w: startsAt", ErrInvalid)
	}
	duration := draft.DurationSeconds
	if duration < 1 {
		return fmt.Errorf("%w: duration", ErrInvalid)
	}
	rec, err := ParseEventRecurrence(string(draft.Recurrence))
	if err != nil {
		return err
	}
	weekdays, err := NormalizeWeekdays(draft.Weekdays)
	if err != nil {
		return err
	}
	if rec == RecurrenceWeekly {
		if len(weekdays) == 0 {
			return fmt.Errorf("%w: weekdays", ErrInvalid)
		}
	} else {
		weekdays = []int{}
	}
	var until *Ymd
	if draft.RepeatUntil != nil && strings.TrimSpace(string(*draft.RepeatUntil)) != "" {
		parsed, err := ParseYmd(string(*draft.RepeatUntil))
		if err != nil {
			return err
		}
		startDay := YmdOf(draft.StartsAt, Moscow())
		if parsed < startDay {
			return fmt.Errorf("%w: repeatUntil", ErrInvalid)
		}
		until = &parsed
	}
	if draft.Involvement < 0 || draft.Involvement > maxEventInvolvement {
		return fmt.Errorf("%w: involvement", ErrInvalid)
	}
	if err := validateActiveWindow(draft.ActiveStartOffset, draft.ActiveEndOffset, duration); err != nil {
		return err
	}
	meet, err := parseOptionalURL(draft.MeetURL)
	if err != nil {
		return err
	}
	links, err := NormalizeLinks(draft.Links)
	if err != nil {
		return err
	}
	e.Title = title
	e.Description = strings.TrimSpace(draft.Description)
	e.Agenda = strings.TrimSpace(draft.Agenda)
	e.Kind = draft.Kind
	e.Type = typ
	e.ProjectID = draft.ProjectID
	e.StartsAt = draft.StartsAt.UTC()
	e.DurationSeconds = duration
	e.Recurrence = rec
	e.RepeatUntil = until
	e.Weekdays = weekdays
	e.Links = links
	e.MeetURL = meet
	e.Involvement = draft.Involvement
	e.ActiveStartOffset = draft.ActiveStartOffset
	e.ActiveEndOffset = draft.ActiveEndOffset
	e.CanSkip = draft.CanSkip
	return nil
}

func validateActiveWindow(start, end *int, duration int) error {
	if start == nil && end == nil {
		return nil
	}
	if start == nil || end == nil {
		return fmt.Errorf("%w: active interval", ErrInvalid)
	}
	if *start < 0 || *end > duration || *end <= *start {
		return fmt.Errorf("%w: active interval", ErrInvalid)
	}
	return nil
}

func parseOptionalURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("%w: meet url", ErrInvalid)
	}
	return parsed.String(), nil
}

func (e Event) EndsAt() time.Time {
	return e.StartsAt.Add(time.Duration(e.DurationSeconds) * time.Second)
}

// ActiveSpan is the part of the occurrence that actually occupies people.
// Offsets are seconds from StartsAt; missing offsets mean the full interval.
func (e EventOccurrence) ActiveSpan() (time.Time, time.Time) {
	if e.ActiveStartOffset == nil || e.ActiveEndOffset == nil {
		return e.StartsAt, e.EndsAt
	}
	start := e.StartsAt.Add(time.Duration(*e.ActiveStartOffset) * time.Second)
	end := e.StartsAt.Add(time.Duration(*e.ActiveEndOffset) * time.Second)
	if !end.After(start) {
		return e.StartsAt, e.EndsAt
	}
	return start, end
}

func (e Event) Occurrence(at time.Time) EventOccurrence {
	start := at.UTC()
	return EventOccurrence{
		ID:                e.ID,
		SeriesID:          e.ID,
		Title:             e.Title,
		Description:       e.Description,
		Agenda:            e.Agenda,
		Kind:              e.Kind,
		Type:              e.Type,
		ProjectID:         e.ProjectID,
		StartsAt:          start,
		EndsAt:            start.Add(time.Duration(e.DurationSeconds) * time.Second),
		DurationSeconds:   e.DurationSeconds,
		Recurrence:        e.Recurrence,
		OriginalOn:        YmdOf(start, Moscow()),
		Links:             e.Links,
		MeetURL:           e.MeetURL,
		Involvement:       e.Involvement,
		ActiveStartOffset: e.ActiveStartOffset,
		ActiveEndOffset:   e.ActiveEndOffset,
		CanSkip:           e.CanSkip,
		People:            e.People,
		CreatedAt:         e.CreatedAt,
	}
}

func (e Event) OccurrenceOn(day Ymd) EventOccurrence {
	loc := Moscow()
	h, m, s := e.StartsAt.In(loc).Clock()
	midnight := day.Midnight(loc)
	start := time.Date(midnight.Year(), midnight.Month(), midnight.Day(), h, m, s, 0, loc)
	occ := e.Occurrence(start)
	occ.OriginalOn = day
	return occ
}

func (e Event) HasSlot(day Ymd) bool {
	if _, err := ParseYmd(string(day)); err != nil {
		return false
	}
	loc := Moscow()
	startDay := YmdOf(e.StartsAt, loc)
	if day < startDay {
		return false
	}
	if e.RepeatUntil != nil && day > *e.RepeatUntil {
		return false
	}
	switch e.Recurrence {
	case RecurrenceOnce:
		return day == startDay
	case RecurrenceMonthly:
		return e.monthlyHits(day, loc)
	default:
		return containsInt(e.Weekdays, weekdaySlot(startDay, day, loc))
	}
}

func (e Event) SlotOccurrence(day Ymd, override *EventOverride) (EventOccurrence, error) {
	if !e.HasSlot(day) {
		return EventOccurrence{}, ErrNotFound
	}
	occ := e.OccurrenceOn(day)
	if override == nil {
		return occ, nil
	}
	if override.Skipped {
		occ.Skipped = true
		occ.Overridden = true
		return occ, nil
	}
	return override.Apply(occ), nil
}

func (e Event) Expand(from, to time.Time) []EventOccurrence {
	from, to = e.clampRange(from, to)
	switch e.Recurrence {
	case RecurrenceWeekly:
		return e.expandWeekly(from, to)
	case RecurrenceMonthly:
		return e.expandMonthly(from, to)
	default:
		if !e.StartsAt.Before(from) && e.StartsAt.Before(to) {
			return []EventOccurrence{e.OccurrenceOn(YmdOf(e.StartsAt, Moscow()))}
		}
		return nil
	}
}

func (e Event) expandWeekly(from, to time.Time) []EventOccurrence {
	loc := Moscow()
	startDay := YmdOf(e.StartsAt, loc)
	cursor := civilDay(from.In(loc), loc)
	startMidnight := startDay.Midnight(loc)
	if cursor.Before(startMidnight) {
		cursor = startMidnight
	}
	var out []EventOccurrence
	for cursor.Before(to) {
		day := YmdOf(cursor, loc)
		if e.HasSlot(day) {
			occ := e.OccurrenceOn(day)
			if !occ.StartsAt.Before(from) && occ.StartsAt.Before(to) {
				out = append(out, occ)
			}
		}
		cursor = cursor.AddDate(0, 0, 1)
	}
	return out
}

func (e Event) expandMonthly(from, to time.Time) []EventOccurrence {
	start := e.StartsAt
	for start.Before(from) {
		next := start.AddDate(0, 1, 0)
		if !next.After(start) {
			return nil
		}
		start = next
	}
	var out []EventOccurrence
	for start.Before(to) {
		day := YmdOf(start, Moscow())
		if e.HasSlot(day) {
			out = append(out, e.OccurrenceOn(day))
		}
		next := start.AddDate(0, 1, 0)
		if !next.After(start) {
			break
		}
		start = next
	}
	return out
}

func (e Event) monthlyHits(day Ymd, loc *time.Location) bool {
	target := day.Midnight(loc)
	cursor := civilDay(e.StartsAt.In(loc), loc)
	for !cursor.After(target) {
		if cursor.Equal(target) {
			return true
		}
		next := cursor.AddDate(0, 1, 0)
		if !next.After(cursor) {
			return false
		}
		cursor = next
	}
	return false
}

func (e Event) clampRange(from, to time.Time) (time.Time, time.Time) {
	from, to = clampEventRange(from, to)
	if e.RepeatUntil == nil {
		return from, to
	}
	untilEnd := e.RepeatUntil.Midnight(Moscow()).Add(24 * time.Hour)
	if untilEnd.Before(to) {
		to = untilEnd
	}
	return from, to
}

func clampEventRange(from, to time.Time) (time.Time, time.Time) {
	if from.IsZero() {
		from = time.Now().UTC()
	}
	if to.IsZero() || to.Before(from) {
		to = from.Add(maxEventExpand)
	}
	if to.Sub(from) > maxEventExpand {
		to = from.Add(maxEventExpand)
	}
	return from.UTC(), to.UTC()
}

func ExpandEvents(events []Event, from, to time.Time) []EventOccurrence {
	from, to = clampEventRange(from, to)
	out := make([]EventOccurrence, 0)
	for _, event := range events {
		out = append(out, event.Expand(from, to)...)
	}
	return out
}

func weekdaySlot(startDay, day Ymd, loc *time.Location) int {
	monday := mondayOf(startDay.Midnight(loc))
	days := civilDays(monday, day.Midnight(loc))
	if days < 0 {
		days = ((days % weekdayCycleDays) + weekdayCycleDays) % weekdayCycleDays
	}
	return days % weekdayCycleDays
}

func mondayOf(day time.Time) time.Time {
	offset := (int(day.Weekday()) + 6) % 7
	return civilDay(day, day.Location()).AddDate(0, 0, -offset)
}

func civilDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func civilDays(from, to time.Time) int {
	a := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a) / (24 * time.Hour))
}

func containsInt(slots []int, slot int) bool {
	for _, value := range slots {
		if value == slot {
			return true
		}
	}
	return false
}
