package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type BondKind string

const (
	BondKindAcquaintance BondKind = "acquaintance"
	BondKindColleague    BondKind = "colleague"
	BondKindComrade      BondKind = "comrade"
	BondKindFriend       BondKind = "friend"
	BondKindRelative     BondKind = "relative"
	BondKindSpouse       BondKind = "spouse"
	BondKindAdversary    BondKind = "adversary"
	BondKindOther        BondKind = "other"
)

type MeBond struct {
	ID   uuid.UUID `json:"id"`
	Kind BondKind  `json:"kind"`
}

type BondAction string

const (
	BondActionOpen   BondAction = "open"
	BondActionChange BondAction = "change"
	BondActionEnd    BondAction = "end"
)

type PersonBondEvent struct {
	ID        uuid.UUID  `json:"id"`
	BondID    uuid.UUID  `json:"bondId"`
	Action    BondAction `json:"action"`
	Kind      BondKind   `json:"kind"`
	Comment   string     `json:"comment"`
	StartedOn time.Time  `json:"startedOn"`
	ChangedOn time.Time  `json:"changedOn"`
	EndedOn   *time.Time `json:"endedOn"`
	At        time.Time  `json:"at"`
}

type PersonBond struct {
	ID        uuid.UUID         `json:"id"`
	PersonAID *uuid.UUID        `json:"personAId"`
	PersonBID uuid.UUID         `json:"personBId"`
	OtherID   *uuid.UUID        `json:"otherId"`
	OtherName string            `json:"otherName"`
	Kind      BondKind          `json:"kind"`
	Comment   string            `json:"comment"`
	StartedOn time.Time         `json:"startedOn"`
	ChangedOn time.Time         `json:"changedOn"`
	EndedOn   *time.Time        `json:"endedOn"`
	Events    []PersonBondEvent `json:"events"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type BondChange struct {
	Kind      string
	Comment   *string
	StartedOn *time.Time
	ChangedOn *time.Time
}

func ParseBondKind(raw string) (BondKind, error) {
	kind := BondKind(strings.TrimSpace(raw))
	switch kind {
	case BondKindAcquaintance, BondKindColleague, BondKindComrade, BondKindFriend, BondKindRelative, BondKindSpouse, BondKindAdversary, BondKindOther:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: bond kind", ErrInvalid)
	}
}

func CanonicalBondPair(subject uuid.UUID, other *uuid.UUID) (*uuid.UUID, uuid.UUID, error) {
	if subject == uuid.Nil {
		return nil, uuid.Nil, fmt.Errorf("%w: bond person", ErrInvalid)
	}
	if other == nil || *other == uuid.Nil {
		return nil, subject, nil
	}
	if *other == subject {
		return nil, uuid.Nil, fmt.Errorf("%w: bond self", ErrInvalid)
	}
	if subject.String() < other.String() {
		return &subject, *other, nil
	}
	return other, subject, nil
}

func NewPersonBond(a *uuid.UUID, b uuid.UUID, kind BondKind, comment string, startedOn, now time.Time) (PersonBond, error) {
	if b == uuid.Nil {
		return PersonBond{}, fmt.Errorf("%w: bond person", ErrInvalid)
	}
	if a != nil && *a == b {
		return PersonBond{}, fmt.Errorf("%w: bond self", ErrInvalid)
	}
	start := dateUTC(startedOn)
	at := now.UTC()
	bond := PersonBond{
		ID:        uuid.New(),
		PersonAID: a,
		PersonBID: b,
		Kind:      kind,
		Comment:   strings.TrimSpace(comment),
		StartedOn: start,
		ChangedOn: start,
		Events:    []PersonBondEvent{},
		CreatedAt: at,
		UpdatedAt: at,
	}
	return bond, nil
}

func (b *PersonBond) Change(draft BondChange, now time.Time) error {
	if b.EndedOn != nil {
		return fmt.Errorf("%w: bond ended", ErrInvalid)
	}
	kind := b.Kind
	if strings.TrimSpace(draft.Kind) != "" {
		parsed, err := ParseBondKind(draft.Kind)
		if err != nil {
			return err
		}
		kind = parsed
	}
	started := b.StartedOn
	if draft.StartedOn != nil {
		started = dateUTC(*draft.StartedOn)
	}
	changed := MoscowDate(now)
	if draft.ChangedOn != nil {
		changed = dateUTC(*draft.ChangedOn)
	}
	if changed.Before(started) {
		return fmt.Errorf("%w: bond dates", ErrInvalid)
	}
	comment := b.Comment
	if draft.Comment != nil {
		comment = strings.TrimSpace(*draft.Comment)
	}
	b.Kind = kind
	b.Comment = comment
	b.StartedOn = started
	b.ChangedOn = changed
	b.UpdatedAt = now.UTC()
	return nil
}

func (b *PersonBond) End(endedOn *time.Time, now time.Time) error {
	if b.EndedOn != nil {
		return fmt.Errorf("%w: bond ended", ErrInvalid)
	}
	day := MoscowDate(now)
	if endedOn != nil {
		day = dateUTC(*endedOn)
	}
	if day.Before(b.StartedOn) {
		return fmt.Errorf("%w: bond dates", ErrInvalid)
	}
	b.EndedOn = &day
	b.UpdatedAt = now.UTC()
	return nil
}

func (b PersonBond) Involves(id uuid.UUID) bool {
	if b.PersonBID == id {
		return true
	}
	return b.PersonAID != nil && *b.PersonAID == id
}

func (b PersonBond) ViewedFrom(id uuid.UUID) PersonBond {
	if b.PersonAID == nil {
		b.OtherID = nil
		b.OtherName = "Me"
		return b
	}
	if *b.PersonAID == id {
		other := b.PersonBID
		b.OtherID = &other
		return b
	}
	other := *b.PersonAID
	b.OtherID = &other
	return b
}

func (b PersonBond) Snapshot(action BondAction, comment string, at time.Time) PersonBondEvent {
	return PersonBondEvent{
		ID:        uuid.New(),
		BondID:    b.ID,
		Action:    action,
		Kind:      b.Kind,
		Comment:   strings.TrimSpace(comment),
		StartedOn: b.StartedOn,
		ChangedOn: b.ChangedOn,
		EndedOn:   b.EndedOn,
		At:        at.UTC(),
	}
}
