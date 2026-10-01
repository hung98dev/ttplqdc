package rng

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// ErrInvalidSelection is returned by SelectWeighted for malformed inputs:
// empty or length-mismatched candidate/weight slices, non-positive total
// weight, or any weight that is negative, NaN or +Inf.
var ErrInvalidSelection = errors.New("rng: invalid weighted selection")

// SelectWeighted draws one candidate from the ordered items using the owning
// stream. Determinism: identical stream state + identical ordered inputs
// produce the identical index — candidates must arrive in canonical order.
// Bounds: one Float64Range(0, total) draw + binary search — O(log n).
func SelectWeighted[T any](s *Stream, items []T, weights []float64) (T, error) {
	var zero T
	if len(items) == 0 {
		return zero, fmt.Errorf("%w: empty candidates", ErrInvalidSelection)
	}
	if len(items) != len(weights) {
		return zero, fmt.Errorf("%w: %d candidates vs %d weights", ErrInvalidSelection, len(items), len(weights))
	}
	cumulative := make([]float64, len(weights))
	var total float64
	for i, w := range weights {
		if w < 0 || math.IsNaN(w) || math.IsInf(w, 0) {
			return zero, fmt.Errorf("%w: weight[%d]=%v", ErrInvalidSelection, i, w)
		}
		total += w
		cumulative[i] = total
	}
	if total <= 0 {
		return zero, fmt.Errorf("%w: total weight must be positive", ErrInvalidSelection)
	}
	draw := s.Float64Range(0, total)
	idx := sort.Search(len(cumulative), func(i int) bool { return cumulative[i] > draw })
	return items[idx], nil
}
