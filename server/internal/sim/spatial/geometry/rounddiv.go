package geometry

import "math"

// RoundDiv divides signed n by positive d and rounds the absolute quotient to
// the nearest integer, halves away from zero (contract §2.2):
//
//	q = |n| / d;  r = |n| % d;  if r >= d - r { q++ }
//
// No overflow wrap and no saturating fallback: n == math.MinInt64 and d <= 0
// are rejected. A violation is an impossible-state assertion — parsed geometry
// bounds every coordinate, and denominators are strictly positive segment
// spans or tick constants — so the failure is a panic, not a silent value.
func RoundDiv(n, d int64) int64 {
	if d <= 0 {
		panic("geometry: RoundDiv non-positive denominator")
	}
	if n == math.MinInt64 {
		panic("geometry: RoundDiv INT64_MIN numerator")
	}
	a := n
	neg := a < 0
	if neg {
		a = -a
	}
	q, r := a/d, a%d
	if r >= d-r {
		q++
	}
	if neg {
		return -q
	}
	return q
}
