package domain

import "time"

type LoadReport struct {
	From             time.Time       `json:"from"`
	To               time.Time       `json:"to"`
	AllocatedSeconds int64           `json:"allocatedSeconds"`
	WallSeconds      int64           `json:"wallSeconds"`
	ByProject        []ProjectLoad   `json:"byProject"`
	ByItem           []ItemLoad      `json:"byItem"`
	ByDay            []DayLoad       `json:"byDay"`
	Stress           []StressLog     `json:"stress"`
	Averages         CheckinAverages `json:"averages"`
	AverageStress    *float64        `json:"averageStress"`
}

type DayLoad struct {
	Date             string `json:"date"`
	AllocatedSeconds int64  `json:"allocatedSeconds"`
	WallSeconds      int64  `json:"wallSeconds"`
}

type CheckinAverages struct {
	Stress    *float64 `json:"stress"`
	Focus     *float64 `json:"focus"`
	Energy    *float64 `json:"energy"`
	Interest  *float64 `json:"interest"`
	Happiness *float64 `json:"happiness"`
}

type ProjectLoad struct {
	ProjectID        *string `json:"projectId"`
	Name             string  `json:"name"`
	Color            string  `json:"color"`
	TargetHoursWeek  float64 `json:"targetHoursWeek"`
	AllocatedSeconds int64   `json:"allocatedSeconds"`
}

type ItemLoad struct {
	ItemID           string   `json:"itemId"`
	Title            string   `json:"title"`
	Kind             ItemKind `json:"kind"`
	ProjectName      string   `json:"projectName"`
	AllocatedSeconds int64    `json:"allocatedSeconds"`
}

func CheckinAveragesFrom(logs []StressLog) CheckinAverages {
	sum := map[CheckinKind]int{}
	count := map[CheckinKind]int{}
	for _, log := range logs {
		kind := log.Kind
		if kind == "" {
			kind = CheckinStress
		}
		if _, err := ParseCheckinKind(string(kind)); err != nil {
			continue
		}
		sum[kind] += log.Level
		count[kind]++
	}
	return CheckinAverages{
		Stress:    mean(sum[CheckinStress], count[CheckinStress]),
		Focus:     mean(sum[CheckinFocus], count[CheckinFocus]),
		Energy:    mean(sum[CheckinEnergy], count[CheckinEnergy]),
		Interest:  mean(sum[CheckinInterest], count[CheckinInterest]),
		Happiness: mean(sum[CheckinHappiness], count[CheckinHappiness]),
	}
}

func mean(sum, n int) *float64 {
	if n == 0 {
		return nil
	}
	value := float64(sum) / float64(n)
	return &value
}

func LoadByDay(intervals []TimeInterval, from, to, now time.Time) []DayLoad {
	if !to.After(from) {
		return []DayLoad{}
	}
	loc := Moscow()
	start := from.In(loc)
	cursor := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	out := []DayLoad{}
	for cursor.Before(to) {
		dayTo := cursor.Add(24 * time.Hour)
		clipFrom := cursor
		if clipFrom.Before(from) {
			clipFrom = from
		}
		clipTo := dayTo
		if clipTo.After(to) {
			clipTo = to
		}
		if clipTo.After(clipFrom) {
			out = append(out, DayLoad{
				Date:             cursor.Format("2006-01-02"),
				AllocatedSeconds: AllocatedSeconds(intervals, clipFrom, clipTo, now),
				WallSeconds:      WallSeconds(intervals, clipFrom, clipTo, now),
			})
		}
		cursor = dayTo
	}
	return out
}
