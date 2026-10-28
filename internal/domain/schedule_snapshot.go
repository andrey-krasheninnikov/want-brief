package domain

import "time"

// ScheduleSnapshot remembers the last layout per kind so the next packing
// run can keep blocks in place unless moving them is clearly worth it.
type ScheduleSnapshot struct {
	Kind    ScheduleKind    `json:"kind"`
	TakenAt time.Time       `json:"takenAt"`
	Blocks  []ScheduleBlock `json:"blocks"`
}
