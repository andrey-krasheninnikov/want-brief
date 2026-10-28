package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ItemCheck struct {
	ID        uuid.UUID `json:"id"`
	ItemID    uuid.UUID `json:"itemId"`
	Body      string    `json:"body"`
	Done      bool      `json:"done"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewItemCheck(itemID uuid.UUID, body string, position int) (ItemCheck, error) {
	now := time.Now().UTC()
	check := ItemCheck{
		ID:        uuid.New(),
		ItemID:    itemID,
		Done:      false,
		Position:  position,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := check.SetBody(body); err != nil {
		return ItemCheck{}, err
	}
	return check, nil
}

func (c *ItemCheck) SetBody(raw string) error {
	body := strings.TrimSpace(raw)
	if c.ItemID == uuid.Nil {
		return fmt.Errorf("%w: item", ErrInvalid)
	}
	if body == "" {
		return fmt.Errorf("%w: check body", ErrInvalid)
	}
	c.Body = body
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *ItemCheck) SetDone(done bool) {
	c.Done = done
	c.UpdatedAt = time.Now().UTC()
}

func AssignCheckPositions(checks []ItemCheck, ids []uuid.UUID) error {
	if len(checks) != len(ids) {
		return fmt.Errorf("%w: check order", ErrInvalid)
	}
	index := make(map[uuid.UUID]int, len(checks))
	for i, check := range checks {
		if _, ok := index[check.ID]; ok {
			return fmt.Errorf("%w: check order", ErrInvalid)
		}
		index[check.ID] = i
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	now := time.Now().UTC()
	for pos, id := range ids {
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: check order", ErrInvalid)
		}
		i, ok := index[id]
		if !ok {
			return fmt.Errorf("%w: check order", ErrInvalid)
		}
		seen[id] = struct{}{}
		checks[i].Position = pos
		checks[i].UpdatedAt = now
	}
	return nil
}
