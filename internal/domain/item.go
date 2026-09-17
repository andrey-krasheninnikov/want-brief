package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ItemKind string

const (
	KindTask       ItemKind = "task"
	KindNote       ItemKind = "note"
	KindAgreement  ItemKind = "agreement"
	KindObligation ItemKind = "obligation"
	KindInitiative ItemKind = "initiative"
	KindLife       ItemKind = "life"
)

type ItemStatus string

const (
	StatusBacklog          ItemStatus = "backlog"
	StatusNeedsGrooming    ItemStatus = "needs_grooming"
	StatusToDo             ItemStatus = "to_do"
	StatusInProgress       ItemStatus = "in_progress"
	StatusBlocked          ItemStatus = "blocked"
	StatusReview           ItemStatus = "review"
	StatusQA               ItemStatus = "qa"
	StatusAwaitingDecision ItemStatus = "awaiting_decision"
	StatusReleaseCandidate ItemStatus = "release_candidate"
	StatusDone             ItemStatus = "done"
	StatusCancelled        ItemStatus = "cancelled"
)

type Occupancy string

const (
	// OccupancySolo takes the whole day: nothing else is packed alongside.
	OccupancySolo Occupancy = "solo"
	// OccupancyParallel takes the single active track; waiting tracks stay free.
	OccupancyParallel Occupancy = "parallel"
	// OccupancyWaiting is mostly idle time (waiting on a build, a reply); it
	// takes one of the waiting tracks and never the active one.
	OccupancyWaiting Occupancy = "waiting"
)

type Quadrant string

const (
	QuadrantDo       Quadrant = "do"
	QuadrantSchedule Quadrant = "schedule"
	QuadrantDelegate Quadrant = "delegate"
	QuadrantDrop     Quadrant = "drop"
)

type Item struct {
	ID             uuid.UUID     `json:"id"`
	SourceID       uuid.UUID     `json:"sourceId"`
	ExternalKey    string        `json:"externalKey"`
	Title          string        `json:"title"`
	Status         ItemStatus    `json:"status"`
	Occupancy      Occupancy     `json:"occupancy"`
	ExternalStatus string        `json:"externalStatus"`
	Kind           ItemKind      `json:"kind"`
	ProjectID      *uuid.UUID    `json:"projectId"`
	Urgent         bool          `json:"urgent"`
	Important      bool          `json:"important"`
	Pinned         bool          `json:"pinned"`
	PinnedAt       *time.Time    `json:"pinnedAt"`
	Stress         *int          `json:"stress"`
	DueAt          *time.Time    `json:"dueAt"`
	DevDueAt       *time.Time    `json:"devDueAt"`
	ReviewDueAt    *time.Time    `json:"reviewDueAt"`
	TestDueAt      *time.Time    `json:"testDueAt"`
	Description    string        `json:"description"`
	PlannedSeconds int           `json:"plannedSeconds"`
	Links          []ProjectLink `json:"links"`
	PersonIDs      []uuid.UUID   `json:"personIds"`
	TrackedSeconds int64         `json:"trackedSeconds"`
	ArchivedAt     *time.Time    `json:"archivedAt"`
	DeletedAt      *time.Time    `json:"deletedAt"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	SourceName     string        `json:"sourceName"`
	SourceKind     SourceKind    `json:"sourceKind"`
	ProjectName    string        `json:"projectName"`
	ProjectColor   string        `json:"projectColor"`
	QuadrantName   Quadrant      `json:"quadrant"`
	CheckTotal     int           `json:"checkTotal"`
	CheckDone      int           `json:"checkDone"`
}

func ParseItemKind(raw string) (ItemKind, error) {
	kind := ItemKind(raw)
	switch kind {
	case KindTask, KindNote, KindAgreement, KindObligation, KindInitiative, KindLife:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: item kind", ErrInvalid)
	}
}

func ParseItemStatus(raw string) (ItemStatus, error) {
	status := ItemStatus(raw)
	switch status {
	case StatusBacklog, StatusNeedsGrooming, StatusToDo, StatusInProgress, StatusBlocked, StatusReview, StatusQA, StatusAwaitingDecision, StatusReleaseCandidate, StatusDone, StatusCancelled:
		return status, nil
	default:
		return "", fmt.Errorf("%w: item status", ErrInvalid)
	}
}

func StatusAllowed(kind ItemKind, status ItemStatus) bool {
	switch status {
	case StatusBacklog, StatusNeedsGrooming, StatusToDo, StatusInProgress, StatusBlocked, StatusDone, StatusCancelled:
		return true
	case StatusReview, StatusQA, StatusAwaitingDecision, StatusReleaseCandidate:
		return kind == KindTask
	default:
		return false
	}
}

func ParseItemStatusForKind(raw string, kind ItemKind) (ItemStatus, error) {
	status, err := ParseItemStatus(raw)
	if err != nil {
		return "", err
	}
	if !StatusAllowed(kind, status) {
		return "", fmt.Errorf("%w: item status", ErrInvalid)
	}
	return status, nil
}

func ParseOccupancy(raw string) (Occupancy, error) {
	occupancy := Occupancy(strings.TrimSpace(raw))
	if occupancy == "" {
		return OccupancySolo, nil
	}
	switch occupancy {
	case OccupancySolo, OccupancyParallel, OccupancyWaiting:
		return occupancy, nil
	default:
		return "", fmt.Errorf("%w: occupancy", ErrInvalid)
	}
}

func (i Item) EffectiveOccupancy() Occupancy {
	switch i.Occupancy {
	case OccupancyParallel, OccupancyWaiting:
		return i.Occupancy
	default:
		return OccupancySolo
	}
}

func (i *Item) SetOccupancy(raw string) error {
	occupancy, err := ParseOccupancy(raw)
	if err != nil {
		return err
	}
	i.Occupancy = occupancy
	return nil
}

func (i *Item) SetPlannedSeconds(seconds int) error {
	if seconds < 0 {
		return fmt.Errorf("%w: planned seconds", ErrInvalid)
	}
	i.PlannedSeconds = seconds
	return nil
}

func (i *Item) Archive(now time.Time) {
	if i.ArchivedAt != nil {
		return
	}
	at := now.UTC()
	i.ArchivedAt = &at
	i.UpdatedAt = at
}

func (i *Item) Restore(now time.Time) {
	if i.ArchivedAt == nil {
		return
	}
	i.ArchivedAt = nil
	i.UpdatedAt = now.UTC()
}

func (i *Item) Delete(now time.Time) {
	if i.DeletedAt != nil {
		return
	}
	at := now.UTC()
	i.DeletedAt = &at
	i.UpdatedAt = at
}

func (i *Item) Undelete(now time.Time) {
	if i.DeletedAt == nil {
		return
	}
	i.DeletedAt = nil
	i.UpdatedAt = now.UTC()
}

func (i Item) Quadrant() Quadrant {
	return QuadrantOf(i.Urgent, i.Important)
}

func (i Item) WithQuadrant() Item {
	i.QuadrantName = i.Quadrant()
	return i
}

func QuadrantOf(urgent, important bool) Quadrant {
	switch {
	case urgent && important:
		return QuadrantDo
	case important:
		return QuadrantSchedule
	case urgent:
		return QuadrantDelegate
	default:
		return QuadrantDrop
	}
}

func ValidateStress(level int) error {
	if level < 1 || level > 5 {
		return fmt.Errorf("%w: stress must be 1-5", ErrInvalid)
	}
	return nil
}

func NewLocalItem(sourceID uuid.UUID, title string, kind ItemKind) (Item, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Item{}, fmt.Errorf("%w: title", ErrInvalid)
	}
	if kind == "" {
		kind = KindTask
	}
	if _, err := ParseItemKind(string(kind)); err != nil {
		return Item{}, err
	}
	now := time.Now().UTC()
	return Item{
		ID:        uuid.New(),
		SourceID:  sourceID,
		Title:     title,
		Status:    StatusBacklog,
		Occupancy: OccupancySolo,
		Kind:      kind,
		Links:     []ProjectLink{},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
