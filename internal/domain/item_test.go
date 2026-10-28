package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestQuadrantOf(t *testing.T) {
	cases := []struct {
		urgent    bool
		important bool
		want      Quadrant
	}{
		{true, true, QuadrantDo},
		{false, true, QuadrantSchedule},
		{true, false, QuadrantDelegate},
		{false, false, QuadrantDrop},
	}
	for _, tc := range cases {
		got := QuadrantOf(tc.urgent, tc.important)
		if got != tc.want {
			t.Fatalf("QuadrantOf(%v,%v)=%s want %s", tc.urgent, tc.important, got, tc.want)
		}
	}
}

func TestNewLocalItemRejectsEmptyTitle(t *testing.T) {
	_, err := NewLocalItem(uuid.Nil, "  ", KindTask)
	if err == nil {
		t.Fatal("expected invalid title")
	}
}

func TestNewLocalItemStartsBacklog(t *testing.T) {
	item, err := NewLocalItem(uuid.New(), "Ship", KindTask)
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != StatusBacklog {
		t.Fatalf("status %s", item.Status)
	}
}

func TestParseItemStatusForKindRejectsReviewOnAgreement(t *testing.T) {
	if _, err := ParseItemStatusForKind(string(StatusReview), KindAgreement); err == nil {
		t.Fatal("expected invalid status")
	}
}

func TestParseItemStatusForKindAllowsToDoOnAgreement(t *testing.T) {
	status, err := ParseItemStatusForKind(string(StatusToDo), KindAgreement)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusToDo {
		t.Fatalf("status %s", status)
	}
}

func TestParseItemStatusForKindAllowsNeedsGroomingOnAgreement(t *testing.T) {
	status, err := ParseItemStatusForKind(string(StatusNeedsGrooming), KindAgreement)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusNeedsGrooming {
		t.Fatalf("status %s", status)
	}
}

func TestParseItemStatusRejectsClarification(t *testing.T) {
	if _, err := ParseItemStatus("clarification"); err == nil {
		t.Fatal("expected invalid status")
	}
}

func TestParseItemKindAllowsRequest(t *testing.T) {
	kind, err := ParseItemKind("request")
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindRequest {
		t.Fatalf("kind %s", kind)
	}
}

func TestParseItemStatusForKindRejectsReviewOnRequest(t *testing.T) {
	if _, err := ParseItemStatusForKind(string(StatusReview), KindRequest); err == nil {
		t.Fatal("expected invalid status")
	}
}

func TestNewLocalItemRequestStartsWaiting(t *testing.T) {
	item, err := NewLocalItem(uuid.New(), "Access", KindRequest)
	if err != nil {
		t.Fatal(err)
	}
	if item.Occupancy != OccupancyWaiting {
		t.Fatalf("occupancy %s", item.Occupancy)
	}
	if item.Kind != KindRequest {
		t.Fatalf("kind %s", item.Kind)
	}
}

func TestNewLocalItemStartsSolo(t *testing.T) {
	item, err := NewLocalItem(uuid.New(), "Ship", KindTask)
	if err != nil {
		t.Fatal(err)
	}
	if item.Occupancy != OccupancySolo {
		t.Fatalf("occupancy %s", item.Occupancy)
	}
}

func TestParseOccupancyDefaultsSolo(t *testing.T) {
	got, err := ParseOccupancy("")
	if err != nil {
		t.Fatal(err)
	}
	if got != OccupancySolo {
		t.Fatalf("occupancy %s", got)
	}
}

func TestParseOccupancyRejectsUnknown(t *testing.T) {
	if _, err := ParseOccupancy("busy"); err == nil {
		t.Fatal("expected invalid occupancy")
	}
}

func TestParseItemStatusForKindAllowsCancelledOnAgreement(t *testing.T) {
	status, err := ParseItemStatusForKind(string(StatusCancelled), KindAgreement)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusCancelled {
		t.Fatalf("status %s", status)
	}
}

func TestParseItemStatusForKindAllowsReviewOnTask(t *testing.T) {
	status, err := ParseItemStatusForKind(string(StatusReview), KindTask)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusReview {
		t.Fatalf("status %s", status)
	}
}

func TestParseItemStatusForKindAllowsAwaitingDecisionOnTask(t *testing.T) {
	status, err := ParseItemStatusForKind(string(StatusAwaitingDecision), KindTask)
	if err != nil {
		t.Fatal(err)
	}
	if status != StatusAwaitingDecision {
		t.Fatalf("status %s", status)
	}
}

func TestParseItemStatusForKindRejectsAwaitingDecisionOnAgreement(t *testing.T) {
	if _, err := ParseItemStatusForKind(string(StatusAwaitingDecision), KindAgreement); err == nil {
		t.Fatal("expected invalid status")
	}
}

func TestSetPlannedSecondsRejectsNegative(t *testing.T) {
	item, err := NewLocalItem(uuid.New(), "Ship", KindTask)
	if err != nil {
		t.Fatal(err)
	}
	if err := item.SetPlannedSeconds(-1); err == nil {
		t.Fatal("expected invalid planned seconds")
	}
}

func TestItemArchiveSetsTimeOnce(t *testing.T) {
	item, err := NewLocalItem(uuid.New(), "Ship", KindTask)
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	item.Archive(first)
	if item.ArchivedAt == nil || !item.ArchivedAt.Equal(first) {
		t.Fatalf("archived at %v", item.ArchivedAt)
	}
	item.Archive(first.Add(time.Hour))
	if !item.ArchivedAt.Equal(first) {
		t.Fatalf("second archive moved time to %v", item.ArchivedAt)
	}
}

func TestItemDeleteSetsTimeOnce(t *testing.T) {
	item, err := NewLocalItem(uuid.New(), "Ship", KindTask)
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	item.Delete(first)
	if item.DeletedAt == nil || !item.DeletedAt.Equal(first) {
		t.Fatalf("deleted at %v", item.DeletedAt)
	}
	item.Delete(first.Add(time.Hour))
	if !item.DeletedAt.Equal(first) {
		t.Fatalf("second delete moved time to %v", item.DeletedAt)
	}
}

func TestItemUndeleteClearsDeleted(t *testing.T) {
	item, err := NewLocalItem(uuid.New(), "Ship", KindTask)
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	item.Delete(first)
	item.Undelete(first.Add(time.Hour))
	if item.DeletedAt != nil {
		t.Fatalf("still deleted %v", item.DeletedAt)
	}
	if !item.UpdatedAt.Equal(first.Add(time.Hour)) {
		t.Fatalf("updated at %v", item.UpdatedAt)
	}
}

func TestItemRestoreClearsArchive(t *testing.T) {
	item, err := NewLocalItem(uuid.New(), "Ship", KindTask)
	if err != nil {
		t.Fatal(err)
	}
	archived := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	item.Archive(archived)
	restored := archived.Add(time.Hour)
	item.Restore(restored)
	if item.ArchivedAt != nil {
		t.Fatalf("still archived %v", item.ArchivedAt)
	}
	if !item.UpdatedAt.Equal(restored) {
		t.Fatalf("updated at %v", item.UpdatedAt)
	}
}

func TestNewItemNoteRejectsEmptyBody(t *testing.T) {
	if _, err := NewItemNote(uuid.New(), "  ", SourceManual); err == nil {
		t.Fatal("expected invalid body")
	}
}

func TestNewItemNoteRejectsUnknownSourceKind(t *testing.T) {
	if _, err := NewItemNote(uuid.New(), "shipped", SourceKind("slack")); err == nil {
		t.Fatal("expected invalid source kind")
	}
}

func TestNewItemNoteKeepsSourceKind(t *testing.T) {
	note, err := NewItemNote(uuid.New(), "shipped", SourceJira)
	if err != nil {
		t.Fatal(err)
	}
	if note.SourceKind != SourceJira || note.Body != "shipped" {
		t.Fatalf("got %+v", note)
	}
}
