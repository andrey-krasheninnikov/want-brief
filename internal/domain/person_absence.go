package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PersonAbsence is a closed date range during which a person is unavailable
// (vacation, sick leave, trip). Dates are calendar days, inclusive.
type PersonAbsence struct {
	ID        uuid.UUID `json:"id"`
	PersonID  uuid.UUID `json:"personId"`
	StartsOn  time.Time `json:"startsOn"`
	EndsOn    time.Time `json:"endsOn"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PersonAbsenceDraft struct {
	StartsOn time.Time
	EndsOn   time.Time
	Note     string
}

func NewPersonAbsence(personID uuid.UUID, draft PersonAbsenceDraft, now time.Time) (PersonAbsence, error) {
	if personID == uuid.Nil {
		return PersonAbsence{}, fmt.Errorf("%w: person", ErrInvalid)
	}
	at := now.UTC()
	absence := PersonAbsence{
		ID:        uuid.New(),
		PersonID:  personID,
		CreatedAt: at,
		UpdatedAt: at,
	}
	if err := absence.apply(draft); err != nil {
		return PersonAbsence{}, err
	}
	return absence, nil
}

func (a *PersonAbsence) Apply(draft PersonAbsenceDraft, now time.Time) error {
	if err := a.apply(draft); err != nil {
		return err
	}
	a.UpdatedAt = now.UTC()
	return nil
}

func (a *PersonAbsence) apply(draft PersonAbsenceDraft) error {
	if draft.StartsOn.IsZero() || draft.EndsOn.IsZero() {
		return fmt.Errorf("%w: absence dates", ErrInvalid)
	}
	start := dateUTC(draft.StartsOn)
	end := dateUTC(draft.EndsOn)
	if end.Before(start) {
		return fmt.Errorf("%w: absence ends before start", ErrInvalid)
	}
	a.StartsOn = start
	a.EndsOn = end
	a.Note = strings.TrimSpace(draft.Note)
	return nil
}

// Covers reports whether the absence includes the given instant's calendar
// day in loc.
func (a PersonAbsence) Covers(at time.Time, loc *time.Location) bool {
	day := dateUTC(at.In(loc))
	return !day.Before(a.StartsOn) && !day.After(a.EndsOn)
}
