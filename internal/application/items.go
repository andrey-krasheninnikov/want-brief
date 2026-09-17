package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pchkauu/want-brief/internal/domain"
)

type ItemPatch struct {
	Title          *string
	Status         *string
	Kind           *string
	ProjectID      *uuid.UUID
	ClearProj      bool
	Urgent         *bool
	Important      *bool
	Pinned         *bool
	PinnedAt       *time.Time
	ClearPinnedAt  bool
	Stress         *int
	ClearStr       bool
	DueAt          *time.Time
	ClearDue       bool
	DevDueAt       *time.Time
	ClearDevDue    bool
	ReviewDueAt    *time.Time
	ClearReviewDue bool
	TestDueAt      *time.Time
	ClearTestDue   bool
	Description    *string
	PlannedSeconds *int
	Links          *[]domain.ProjectLink
	PersonIDs      *[]uuid.UUID
	ExternalKey    *string
	Occupancy      *string
	ExternalStatus *string
	Archived       *bool
}

func (s *Service) ListItems(ctx context.Context, filter domain.ItemFilter) ([]domain.Item, error) {
	return s.Items.List(ctx, filter)
}

func (s *Service) GetItem(ctx context.Context, id uuid.UUID) (domain.Item, error) {
	item, err := s.Items.Get(ctx, id)
	if err != nil {
		return domain.Item{}, err
	}
	if item.DeletedAt != nil {
		return domain.Item{}, domain.ErrNotFound
	}
	return item, nil
}

func (s *Service) CreateItem(ctx context.Context, title, kind string, projectID *uuid.UUID, urgent, important bool) (domain.Item, error) {
	parsedKind, err := domain.ParseItemKind(kind)
	if err != nil {
		parsedKind = domain.KindTask
		if strings.TrimSpace(kind) != "" {
			return domain.Item{}, err
		}
	}
	manual, err := s.Sources.Manual(ctx)
	if err != nil {
		return domain.Item{}, err
	}
	item, err := domain.NewLocalItem(manual.ID, title, parsedKind)
	if err != nil {
		return domain.Item{}, err
	}
	item.ProjectID = projectID
	item.Urgent = urgent
	item.Important = important
	return s.Items.Create(ctx, item)
}

func (s *Service) PatchItem(ctx context.Context, id uuid.UUID, patch ItemPatch) (domain.Item, error) {
	item, err := s.GetItem(ctx, id)
	if err != nil {
		return domain.Item{}, err
	}
	before := itemSnapshots(item)
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" {
			return domain.Item{}, fmt.Errorf("%w: title", domain.ErrInvalid)
		}
		item.Title = title
	}
	if patch.Kind != nil {
		kind, err := domain.ParseItemKind(*patch.Kind)
		if err != nil {
			return domain.Item{}, err
		}
		item.Kind = kind
	}
	if patch.Status != nil {
		status, err := domain.ParseItemStatusForKind(*patch.Status, item.Kind)
		if err != nil {
			return domain.Item{}, err
		}
		item.Status = status
	} else if !domain.StatusAllowed(item.Kind, item.Status) {
		return domain.Item{}, fmt.Errorf("%w: item status", domain.ErrInvalid)
	}
	if patch.ClearProj {
		item.ProjectID = nil
	} else if patch.ProjectID != nil {
		item.ProjectID = patch.ProjectID
	}
	if patch.Urgent != nil {
		item.Urgent = *patch.Urgent
	}
	if patch.Important != nil {
		item.Important = *patch.Important
	}
	if patch.Pinned != nil {
		item.Pinned = *patch.Pinned
	}
	if patch.ClearPinnedAt {
		item.PinnedAt = nil
	} else if patch.PinnedAt != nil {
		at := patch.PinnedAt.UTC()
		item.PinnedAt = &at
	}
	if patch.ClearStr {
		item.Stress = nil
	} else if patch.Stress != nil {
		if err := domain.ValidateStress(*patch.Stress); err != nil {
			return domain.Item{}, err
		}
		item.Stress = patch.Stress
	}
	if patch.ClearDue {
		item.DueAt = nil
	} else if patch.DueAt != nil {
		item.DueAt = patch.DueAt
	}
	if patch.ClearDevDue {
		item.DevDueAt = nil
	} else if patch.DevDueAt != nil {
		item.DevDueAt = patch.DevDueAt
	}
	if patch.ClearReviewDue {
		item.ReviewDueAt = nil
	} else if patch.ReviewDueAt != nil {
		item.ReviewDueAt = patch.ReviewDueAt
	}
	if patch.ClearTestDue {
		item.TestDueAt = nil
	} else if patch.TestDueAt != nil {
		item.TestDueAt = patch.TestDueAt
	}
	if patch.Description != nil {
		item.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.PlannedSeconds != nil {
		if err := item.SetPlannedSeconds(*patch.PlannedSeconds); err != nil {
			return domain.Item{}, err
		}
	}
	if patch.Links != nil {
		item.Links, err = domain.NormalizeLinks(*patch.Links)
		if err != nil {
			return domain.Item{}, err
		}
	}
	if patch.PersonIDs != nil {
		item.PersonIDs = domain.NormalizeIDs(*patch.PersonIDs)
	}
	if patch.ExternalKey != nil {
		if item.SourceKind != domain.SourceManual {
			return domain.Item{}, fmt.Errorf("%w: external key", domain.ErrForbidden)
		}
		item.ExternalKey = strings.TrimSpace(*patch.ExternalKey)
	}
	if patch.Occupancy != nil {
		if err := item.SetOccupancy(*patch.Occupancy); err != nil {
			return domain.Item{}, err
		}
	}
	if patch.ExternalStatus != nil {
		item.ExternalStatus = strings.TrimSpace(*patch.ExternalStatus)
	}
	now := s.now()
	if patch.Archived != nil {
		if *patch.Archived {
			item.Archive(now)
		} else {
			item.Restore(now)
		}
	}
	item.UpdatedAt = now
	updated, err := s.Items.Update(ctx, item)
	if err != nil {
		return domain.Item{}, err
	}
	if err := s.recordItemChanges(ctx, updated.ID, before, itemSnapshots(updated)); err != nil {
		return domain.Item{}, err
	}
	return updated, nil
}

func (s *Service) ListItemNotes(ctx context.Context, itemID uuid.UUID) ([]domain.ItemNote, error) {
	if _, err := s.GetItem(ctx, itemID); err != nil {
		return nil, err
	}
	return s.ItemNotes.ListByItem(ctx, itemID)
}

func (s *Service) CreateItemNote(ctx context.Context, itemID uuid.UUID, body string) (domain.ItemNote, error) {
	if _, err := s.GetItem(ctx, itemID); err != nil {
		return domain.ItemNote{}, err
	}
	note, err := domain.NewItemNote(itemID, body, domain.SourceManual)
	if err != nil {
		return domain.ItemNote{}, err
	}
	return s.ItemNotes.Create(ctx, note)
}

func (s *Service) DeleteItemNote(ctx context.Context, itemID, noteID uuid.UUID) error {
	if _, err := s.GetItem(ctx, itemID); err != nil {
		return err
	}
	note, err := s.ItemNotes.Get(ctx, noteID)
	if err != nil {
		return err
	}
	if note.ItemID != itemID {
		return domain.ErrNotFound
	}
	return s.ItemNotes.Delete(ctx, noteID)
}

func (s *Service) DeleteItem(ctx context.Context, id uuid.UUID) error {
	item, err := s.Items.Get(ctx, id)
	if err != nil {
		return err
	}
	if item.DeletedAt != nil {
		return domain.ErrNotFound
	}
	item.Delete(s.now())
	_, err = s.Items.Update(ctx, item)
	return err
}

func (s *Service) UndeleteItem(ctx context.Context, id uuid.UUID) (domain.Item, error) {
	item, err := s.Items.Get(ctx, id)
	if err != nil {
		return domain.Item{}, err
	}
	if item.DeletedAt == nil {
		return domain.Item{}, domain.ErrNotFound
	}
	item.Undelete(s.now())
	return s.Items.Update(ctx, item)
}

func (s *Service) ListItemChecks(ctx context.Context, itemID uuid.UUID) ([]domain.ItemCheck, error) {
	if _, err := s.GetItem(ctx, itemID); err != nil {
		return nil, err
	}
	return s.ItemChecks.ListByItem(ctx, itemID)
}

func (s *Service) CreateItemCheck(ctx context.Context, itemID uuid.UUID, body string) (domain.ItemCheck, error) {
	if _, err := s.GetItem(ctx, itemID); err != nil {
		return domain.ItemCheck{}, err
	}
	existing, err := s.ItemChecks.ListByItem(ctx, itemID)
	if err != nil {
		return domain.ItemCheck{}, err
	}
	position := 0
	for _, check := range existing {
		if check.Position >= position {
			position = check.Position + 1
		}
	}
	check, err := domain.NewItemCheck(itemID, body, position)
	if err != nil {
		return domain.ItemCheck{}, err
	}
	created, err := s.ItemChecks.Create(ctx, check)
	if err != nil {
		return domain.ItemCheck{}, err
	}
	if err := s.recordItemEvent(ctx, itemID, domain.ItemEventCheck, "create", "", created.Body); err != nil {
		return domain.ItemCheck{}, err
	}
	return created, nil
}

func (s *Service) PatchItemCheck(ctx context.Context, itemID, checkID uuid.UUID, body *string, done *bool) (domain.ItemCheck, error) {
	if _, err := s.GetItem(ctx, itemID); err != nil {
		return domain.ItemCheck{}, err
	}
	check, err := s.ItemChecks.Get(ctx, checkID)
	if err != nil {
		return domain.ItemCheck{}, err
	}
	if check.ItemID != itemID {
		return domain.ItemCheck{}, domain.ErrNotFound
	}
	fromBody, fromDone := check.Body, snapshotBool(check.Done)
	if body != nil {
		if err := check.SetBody(*body); err != nil {
			return domain.ItemCheck{}, err
		}
	}
	if done != nil {
		check.SetDone(*done)
	}
	updated, err := s.ItemChecks.Update(ctx, check)
	if err != nil {
		return domain.ItemCheck{}, err
	}
	if fromBody != updated.Body {
		if err := s.recordItemEvent(ctx, itemID, domain.ItemEventCheck, "body", fromBody, updated.Body); err != nil {
			return domain.ItemCheck{}, err
		}
	}
	if done != nil && fromDone != snapshotBool(updated.Done) {
		if err := s.recordItemEvent(ctx, itemID, domain.ItemEventCheck, "done", fromDone, snapshotBool(updated.Done)); err != nil {
			return domain.ItemCheck{}, err
		}
	}
	return updated, nil
}

func (s *Service) DeleteItemCheck(ctx context.Context, itemID, checkID uuid.UUID) error {
	if _, err := s.GetItem(ctx, itemID); err != nil {
		return err
	}
	check, err := s.ItemChecks.Get(ctx, checkID)
	if err != nil {
		return err
	}
	if check.ItemID != itemID {
		return domain.ErrNotFound
	}
	if err := s.ItemChecks.Delete(ctx, checkID); err != nil {
		return err
	}
	return s.recordItemEvent(ctx, itemID, domain.ItemEventCheck, "delete", check.Body, "")
}

func (s *Service) SyncItem(ctx context.Context, id uuid.UUID) (domain.Item, error) {
	item, err := s.GetItem(ctx, id)
	if err != nil {
		return domain.Item{}, err
	}
	if item.SourceKind != domain.SourceJira && item.SourceKind != domain.SourceTodoist {
		return domain.Item{}, fmt.Errorf("%w: sync", domain.ErrForbidden)
	}
	if strings.TrimSpace(item.ExternalKey) == "" {
		return domain.Item{}, fmt.Errorf("%w: external key", domain.ErrInvalid)
	}
	source, err := s.Sources.Get(ctx, item.SourceID)
	if err != nil {
		return domain.Item{}, err
	}
	puller, ok := s.Pullers[source.Kind]
	if !ok {
		return domain.Item{}, domain.ErrUnsupported
	}
	if source.TokenSealed == "" {
		return domain.Item{}, domain.ErrNoToken
	}
	token, err := s.Tokens.Open(source.TokenSealed)
	if err != nil {
		return domain.Item{}, fmt.Errorf("open token: %w", err)
	}
	remote, err := puller.Fetch(ctx, source, token, item.ExternalKey)
	if err != nil {
		return domain.Item{}, err
	}
	item.ExternalStatus = strings.TrimSpace(remote.ExternalStatus)
	if title := strings.TrimSpace(remote.Title); title != "" {
		item.Title = title
	}
	if desc := strings.TrimSpace(remote.Description); desc != "" {
		item.Description = desc
	}
	item.UpdatedAt = s.now()
	updated, err := s.Items.Update(ctx, item)
	if err != nil {
		return domain.Item{}, err
	}
	notes, err := s.ItemNotes.ListByItem(ctx, item.ID)
	if err != nil {
		return domain.Item{}, err
	}
	seenBody := map[string]bool{}
	seenExternal := map[string]bool{}
	for _, note := range notes {
		seenBody[strings.TrimSpace(note.Body)] = true
		if note.ExternalID != "" {
			seenExternal[note.ExternalID] = true
		}
	}
	for _, comment := range remote.Comments {
		comment.Body = strings.TrimSpace(comment.Body)
		if comment.Body == "" {
			continue
		}
		// Notes synced before external ids existed carry the body only, so fall back to it.
		if seenExternal[comment.ExternalID] || seenBody[comment.Body] {
			continue
		}
		note, err := domain.NewSourceItemNote(item.ID, item.SourceKind, comment)
		if err != nil {
			return domain.Item{}, err
		}
		if _, err := s.ItemNotes.Create(ctx, note); err != nil {
			return domain.Item{}, err
		}
		seenBody[comment.Body] = true
		if note.ExternalID != "" {
			seenExternal[note.ExternalID] = true
		}
	}
	return s.GetItem(ctx, updated.ID)
}

func (s *Service) SyncActiveItems(ctx context.Context) (int, int, error) {
	items, err := s.ListItems(ctx, domain.ItemFilter{OpenOnly: true})
	if err != nil {
		return 0, 0, err
	}
	var synced, failed int
	for _, item := range items {
		if item.SourceKind != domain.SourceJira && item.SourceKind != domain.SourceTodoist {
			continue
		}
		if strings.TrimSpace(item.ExternalKey) == "" {
			continue
		}
		if _, err := s.SyncItem(ctx, item.ID); err != nil {
			failed++
			continue
		}
		synced++
	}
	return synced, failed, nil
}
