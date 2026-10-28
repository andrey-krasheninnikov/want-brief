package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewItemCheckRejectsEmptyBody(t *testing.T) {
	if _, err := NewItemCheck(uuid.New(), "  ", 0); err == nil {
		t.Fatal("expected invalid body")
	}
}

func TestNewItemCheckStoresBody(t *testing.T) {
	check, err := NewItemCheck(uuid.New(), " Ship ", 2)
	if err != nil {
		t.Fatal(err)
	}
	if check.Body != "Ship" || check.Done || check.Position != 2 {
		t.Fatalf("%+v", check)
	}
}

func TestItemCheckSetDone(t *testing.T) {
	check, err := NewItemCheck(uuid.New(), "Ship", 0)
	if err != nil {
		t.Fatal(err)
	}
	check.SetDone(true)
	if !check.Done {
		t.Fatal("expected done")
	}
}

func TestAssignCheckPositionsSwaps(t *testing.T) {
	itemID := uuid.New()
	a, err := NewItemCheck(itemID, "A", 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewItemCheck(itemID, "B", 1)
	if err != nil {
		t.Fatal(err)
	}
	checks := []ItemCheck{a, b}
	if err := AssignCheckPositions(checks, []uuid.UUID{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	if checks[0].Position != 1 || checks[1].Position != 0 {
		t.Fatalf("positions %d %d", checks[0].Position, checks[1].Position)
	}
}

func TestAssignCheckPositionsRejectsAlien(t *testing.T) {
	itemID := uuid.New()
	a, err := NewItemCheck(itemID, "A", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := AssignCheckPositions([]ItemCheck{a}, []uuid.UUID{uuid.New()}); err == nil {
		t.Fatal("expected invalid order")
	}
}

func TestAssignCheckPositionsRejectsDup(t *testing.T) {
	itemID := uuid.New()
	a, err := NewItemCheck(itemID, "A", 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewItemCheck(itemID, "B", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := AssignCheckPositions([]ItemCheck{a, b}, []uuid.UUID{a.ID, a.ID}); err == nil {
		t.Fatal("expected invalid order")
	}
}
