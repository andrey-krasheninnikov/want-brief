package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CheckinKind string

const (
	CheckinStress    CheckinKind = "stress"
	CheckinFocus     CheckinKind = "focus"
	CheckinEnergy    CheckinKind = "energy"
	CheckinInterest  CheckinKind = "interest"
	CheckinHappiness CheckinKind = "happiness"
)

var CheckinKinds = []CheckinKind{CheckinStress, CheckinFocus, CheckinEnergy, CheckinInterest, CheckinHappiness}

type StressLog struct {
	ID       uuid.UUID   `json:"id"`
	ItemID   *uuid.UUID  `json:"itemId"`
	Kind     CheckinKind `json:"kind"`
	Level    int         `json:"level"`
	LoggedAt time.Time   `json:"loggedAt"`
}

type LatestCheckins struct {
	Stress    int `json:"stress"`
	Focus     int `json:"focus"`
	Energy    int `json:"energy"`
	Interest  int `json:"interest"`
	Happiness int `json:"happiness"`
}

func ParseCheckinKind(raw string) (CheckinKind, error) {
	kind := CheckinKind(raw)
	switch kind {
	case CheckinStress, CheckinFocus, CheckinEnergy, CheckinInterest, CheckinHappiness:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: checkin kind", ErrInvalid)
	}
}

func NewStressLog(level int, itemID *uuid.UUID, at time.Time) (StressLog, error) {
	return NewCheckin(CheckinStress, level, itemID, at)
}

func NewCheckin(kind CheckinKind, level int, itemID *uuid.UUID, at time.Time) (StressLog, error) {
	if _, err := ParseCheckinKind(string(kind)); err != nil {
		return StressLog{}, err
	}
	if err := ValidateStress(level); err != nil {
		return StressLog{}, err
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return StressLog{
		ID:       uuid.New(),
		ItemID:   itemID,
		Kind:     kind,
		Level:    level,
		LoggedAt: at.UTC(),
	}, nil
}

func DefaultLatestCheckins() LatestCheckins {
	return LatestCheckins{Stress: 3, Focus: 3, Energy: 3, Interest: 3, Happiness: 3}
}

func LatestFromLogs(logs []StressLog) LatestCheckins {
	out := DefaultLatestCheckins()
	seen := map[CheckinKind]bool{}
	for _, log := range logs {
		if seen[log.Kind] {
			continue
		}
		seen[log.Kind] = true
		switch log.Kind {
		case CheckinStress:
			out.Stress = log.Level
		case CheckinFocus:
			out.Focus = log.Level
		case CheckinEnergy:
			out.Energy = log.Level
		case CheckinInterest:
			out.Interest = log.Level
		case CheckinHappiness:
			out.Happiness = log.Level
		}
	}
	return out
}
