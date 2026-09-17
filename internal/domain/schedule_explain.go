package domain

import "time"

// explain adds positional reasons that only make sense once every block is
// placed: pushed_by names the higher-ranked block that ends where this one
// starts (#28).
func (p *planner) explain(blocks []ScheduleBlock) []ScheduleBlock {
	pause := time.Duration(p.settings.SliceBreakMin) * time.Minute
	for i := range blocks {
		b := &blocks[i]
		if b.Reasons == nil {
			b.Reasons = []string{}
		}
		if b.Kind == BlockKindPing || hasReason(b.Reasons, reasonActive, reasonPinnedAt, reasonSticky) {
			continue
		}
		for _, a := range blocks {
			if a.ItemID == b.ItemID || a.Kind == BlockKindPing {
				continue
			}
			if !a.EndsAt.Equal(b.StartsAt) && !a.EndsAt.Add(pause).Equal(b.StartsAt) {
				continue
			}
			if p.rankOf[a.ItemID] >= p.rankOf[b.ItemID] {
				continue
			}
			label := a.ExternalKey
			if label == "" {
				label = a.Title
			}
			b.Reasons = appendUnique(b.Reasons, reasonPushedBy+":"+label)
			break
		}
	}
	return blocks
}

func hasReason(reasons []string, wanted ...string) bool {
	for _, reason := range reasons {
		for _, w := range wanted {
			if reason == w {
				return true
			}
		}
	}
	return false
}
