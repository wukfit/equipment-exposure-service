package domain

import "math"

// PartialExposure holds the two HAVS partial-exposure metrics for an activity.
type PartialExposure struct {
	a8     float64
	points float64
}

// NewPartialExposure computes the HSE partial exposure from a vibration
// magnitude (m/s^2) and a duration in minutes.
//
// NOTE: the brief's reference code used Go integer division on (minutes/60)/8,
// which truncates to 0 for any exposure under 8 hours. This uses float division
// (the mathematically intended form). See docs/design.md §3.
func NewPartialExposure(magnitude float64, minutes int) PartialExposure {
	hours := float64(minutes) / 60.0
	return PartialExposure{
		a8:     magnitude * math.Sqrt(hours/8.0),
		points: math.Round(math.Pow(magnitude/2.5, 2) * (hours / 8.0) * 100),
	}
}

func (p PartialExposure) A8() float64     { return p.a8 }
func (p PartialExposure) Points() float64 { return p.points }

// Aggregate combines partial exposures: A(8) by root-sum-of-squares (energy
// combination, per HSE), points by linear sum. See docs/design.md §5.4.
func Aggregate(parts []PartialExposure) PartialExposure {
	var sumSquares, sumPoints float64
	for _, p := range parts {
		sumSquares += p.a8 * p.a8
		sumPoints += p.points
	}
	return PartialExposure{a8: math.Sqrt(sumSquares), points: sumPoints}
}
