package domain

import (
	"math"

	"github.com/google/uuid"
)

// ScheduleScore is a single number to compare layouts (#30). Lower is better.
// Weights: an hour late costs 10, each extra fragment 1, each project switch
// 0.5, plus the variance of daily load in hours².
type ScheduleScore struct {
	LateSeconds  int64   `json:"lateSeconds"`
	Fragments    int     `json:"fragments"`
	Switches     int     `json:"switches"`
	LoadVariance float64 `json:"loadVariance"`
	Total        float64 `json:"total"`
}

const (
	scoreLatePerHour = 10.0
	scoreFragment    = 1.0
	scoreSwitch      = 0.5
)

func (p *planner) score(blocks []ScheduleBlock, overflow ScheduleOverflow) ScheduleScore {
	out := ScheduleScore{LateSeconds: overflow.Seconds}
	items := map[uuid.UUID]bool{}
	work := 0
	lastProject := ""
	perDay := map[Ymd]float64{}
	for _, block := range blocks {
		if block.Kind == BlockKindPing {
			continue
		}
		work++
		items[block.ItemID] = true
		project := p.itemLane[block.ItemID]
		if lastProject != "" && project != lastProject {
			out.Switches++
		}
		lastProject = project
		if block.Occupancy != OccupancyWaiting {
			perDay[p.cal.ymd(block.StartsAt)] += block.EndsAt.Sub(block.StartsAt).Hours()
		}
	}
	out.Fragments = work - len(items)
	if out.Fragments < 0 {
		out.Fragments = 0
	}
	out.LoadVariance = variance(perDay)
	out.Total = float64(out.LateSeconds)/3600*scoreLatePerHour +
		float64(out.Fragments)*scoreFragment +
		float64(out.Switches)*scoreSwitch +
		out.LoadVariance
	out.Total = math.Round(out.Total*100) / 100
	return out
}

func variance(values map[Ymd]float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	var acc float64
	for _, v := range values {
		acc += (v - mean) * (v - mean)
	}
	return math.Round(acc/float64(len(values))*100) / 100
}
