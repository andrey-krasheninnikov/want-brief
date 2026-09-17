package domain

import (
	"fmt"
	"math"
	"time"
)

type EarnDay struct {
	Day       Ymd       `json:"day"`
	USD       float64   `json:"usd"`
	RUB       float64   `json:"rub"`
	Rate      float64   `json:"rate"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewEarnDay(day Ymd, usd, rub, rate float64, now time.Time) (EarnDay, error) {
	if _, err := ParseYmd(string(day)); err != nil {
		return EarnDay{}, err
	}
	if usd < 0 || rub < 0 || rate <= 0 ||
		math.IsNaN(usd) || math.IsNaN(rub) || math.IsNaN(rate) ||
		math.IsInf(usd, 0) || math.IsInf(rub, 0) || math.IsInf(rate, 0) {
		return EarnDay{}, fmt.Errorf("%w: earn", ErrInvalid)
	}
	return EarnDay{Day: day, USD: usd, RUB: rub, Rate: rate, UpdatedAt: now.UTC()}, nil
}
