package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

var itemEventFields = []string{
	"title", "kind", "status", "projectId", "urgent", "important", "pinned", "pinnedAt",
	"stress", "dueAt", "devDueAt", "reviewDueAt", "testDueAt", "description",
	"plannedSeconds", "links", "personIds", "occupancy", "archived", "externalKey", "externalStatus",
}

func (s *Service) ListItemEvents(ctx context.Context, itemID uuid.UUID) ([]domain.ItemEvent, error) {
	if _, err := s.GetItem(ctx, itemID); err != nil {
		return nil, err
	}
	return s.ItemEvents.ListByItem(ctx, itemID)
}

func (s *Service) AnnotateLatestItemEvent(ctx context.Context, itemID uuid.UUID, kinds []string, note string, stress *int) (domain.ItemEvent, error) {
	if _, err := s.GetItem(ctx, itemID); err != nil {
		return domain.ItemEvent{}, err
	}
	parsed := make([]domain.ItemEventKind, 0, len(kinds))
	for _, raw := range kinds {
		kind, err := domain.ParseItemEventKind(raw)
		if err != nil {
			return domain.ItemEvent{}, err
		}
		parsed = append(parsed, kind)
	}
	if len(parsed) == 0 {
		return domain.ItemEvent{}, fmt.Errorf("%w: item event kind", domain.ErrInvalid)
	}
	event, err := s.ItemEvents.LatestByItemAndKinds(ctx, itemID, parsed)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.ItemEvent{}, err
	}
	if err == nil {
		event, err = event.Annotate(note, stress)
		if err != nil {
			return domain.ItemEvent{}, err
		}
		event, err = s.ItemEvents.Update(ctx, event)
		if err != nil {
			return domain.ItemEvent{}, err
		}
	} else {
		event = domain.ItemEvent{}
	}
	if stress != nil {
		if _, err := s.CreateStress(ctx, *stress, &itemID, nil); err != nil {
			return domain.ItemEvent{}, err
		}
	}
	if strings.TrimSpace(note) != "" {
		if _, err := s.CreateItemNote(ctx, itemID, note); err != nil {
			return domain.ItemEvent{}, err
		}
	}
	return event, nil
}

func (s *Service) recordItemEvent(ctx context.Context, itemID uuid.UUID, kind domain.ItemEventKind, field, from, to string) error {
	event, err := domain.NewItemEvent(itemID, kind, field, from, to, s.now())
	if err != nil {
		return err
	}
	_, err = s.ItemEvents.Create(ctx, event)
	return err
}

func (s *Service) recordItemChanges(ctx context.Context, itemID uuid.UUID, before, after map[string]string) error {
	for _, field := range itemEventFields {
		if before[field] == after[field] {
			continue
		}
		kind := domain.ItemEventField
		if field == "status" {
			kind = domain.ItemEventStatus
		}
		if err := s.recordItemEvent(ctx, itemID, kind, field, before[field], after[field]); err != nil {
			return err
		}
	}
	return nil
}

func itemSnapshots(item domain.Item) map[string]string {
	return map[string]string{
		"title":          item.Title,
		"kind":           string(item.Kind),
		"status":         string(item.Status),
		"projectId":      snapshotUUID(item.ProjectID),
		"urgent":         snapshotBool(item.Urgent),
		"important":      snapshotBool(item.Important),
		"pinned":         snapshotBool(item.Pinned),
		"pinnedAt":       snapshotTime(item.PinnedAt),
		"stress":         snapshotIntPtr(item.Stress),
		"dueAt":          snapshotTime(item.DueAt),
		"devDueAt":       snapshotTime(item.DevDueAt),
		"reviewDueAt":    snapshotTime(item.ReviewDueAt),
		"testDueAt":      snapshotTime(item.TestDueAt),
		"description":    item.Description,
		"plannedSeconds": strconv.Itoa(item.PlannedSeconds),
		"links":          snapshotLinks(item.Links),
		"personIds":      snapshotIDs(item.PersonIDs),
		"occupancy":      string(item.Occupancy),
		"archived":       snapshotTime(item.ArchivedAt),
		"externalKey":    item.ExternalKey,
		"externalStatus": item.ExternalStatus,
	}
}

func snapshotBool(v bool) string {
	return strconv.FormatBool(v)
}

func snapshotIntPtr(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func snapshotTime(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}

func snapshotUUID(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func snapshotIDs(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return strings.Join(parts, ",")
}

func snapshotLinks(links []domain.ProjectLink) string {
	raw, err := json.Marshal(links)
	if err != nil {
		return ""
	}
	return string(raw)
}

func snapshotSeconds(d time.Duration) string {
	return strconv.FormatInt(int64(d/time.Second), 10)
}
