package domain

import (
	"time"

	"github.com/google/uuid"
)

type WhatIfMove struct {
	ItemID uuid.UUID `json:"itemId"`
	DueAt  time.Time `json:"dueAt"`
}

// WhatIfScenario describes hypothetical edits: move due dates, drop tasks,
// skip meetings (by series).
type WhatIfScenario struct {
	MoveDue    []WhatIfMove `json:"moveDue"`
	DropItems  []uuid.UUID  `json:"dropItems"`
	SkipEvents []uuid.UUID  `json:"skipEvents"`
}

func (s WhatIfScenario) Empty() bool {
	return len(s.MoveDue) == 0 && len(s.DropItems) == 0 && len(s.SkipEvents) == 0
}

type WhatIfSummary struct {
	LateSeconds     int64   `json:"lateSeconds"`
	OverflowItems   int     `json:"overflowItems"`
	OverflowSeconds int64   `json:"overflowSeconds"`
	AtRisk          int     `json:"atRisk"`
	Fragments       int     `json:"fragments"`
	Switches        int     `json:"switches"`
	Score           float64 `json:"score"`
}

type WhatIfResult struct {
	Base    WhatIfSummary `json:"base"`
	Variant WhatIfSummary `json:"variant"`
	Delta   WhatIfSummary `json:"delta"`
}

// WhatIf packs the input as-is and with the scenario applied, without the
// stability snapshot so both runs are comparable (#29).
func WhatIf(in ScheduleInput, scenario WhatIfScenario) WhatIfResult {
	in.Previous = nil
	base := summarize(Plan(in))
	variant := summarize(Plan(applyScenario(in, scenario)))
	return WhatIfResult{
		Base:    base,
		Variant: variant,
		Delta: WhatIfSummary{
			LateSeconds:     variant.LateSeconds - base.LateSeconds,
			OverflowItems:   variant.OverflowItems - base.OverflowItems,
			OverflowSeconds: variant.OverflowSeconds - base.OverflowSeconds,
			AtRisk:          variant.AtRisk - base.AtRisk,
			Fragments:       variant.Fragments - base.Fragments,
			Switches:        variant.Switches - base.Switches,
			Score:           variant.Score - base.Score,
		},
	}
}

func summarize(s Schedule) WhatIfSummary {
	return WhatIfSummary{
		LateSeconds:     s.Score.LateSeconds,
		OverflowItems:   s.Overflow.ItemCount,
		OverflowSeconds: s.Overflow.Seconds,
		AtRisk:          len(s.AtRisk),
		Fragments:       s.Score.Fragments,
		Switches:        s.Score.Switches,
		Score:           s.Score.Total,
	}
}

func applyScenario(in ScheduleInput, scenario WhatIfScenario) ScheduleInput {
	kind := in.Kind
	if kind == "" {
		kind = ScheduleWork
	}
	drop := map[uuid.UUID]bool{}
	for _, id := range scenario.DropItems {
		drop[id] = true
	}
	moves := map[uuid.UUID]time.Time{}
	for _, move := range scenario.MoveDue {
		moves[move.ItemID] = move.DueAt
	}
	items := make([]Item, 0, len(in.Items))
	for _, item := range in.Items {
		if drop[item.ID] {
			continue
		}
		if due, ok := moves[item.ID]; ok {
			item = withPackingDue(item, kind, due)
		}
		items = append(items, item)
	}
	skip := map[uuid.UUID]bool{}
	for _, id := range scenario.SkipEvents {
		skip[id] = true
	}
	events := make([]EventOccurrence, 0, len(in.Events))
	for _, event := range in.Events {
		if skip[event.SeriesID] || skip[event.ID] {
			continue
		}
		events = append(events, event)
	}
	in.Items = items
	in.Events = events
	return in
}

// withPackingDue sets the due field the packer reads for this kind/status.
func withPackingDue(item Item, kind ScheduleKind, due time.Time) Item {
	at := due.UTC()
	if kind == ScheduleFollowup {
		switch item.Status {
		case StatusReview:
			item.ReviewDueAt = &at
		case StatusQA:
			item.TestDueAt = &at
		default:
			item.DueAt = &at
		}
		return item
	}
	item.DevDueAt = &at
	return item
}
