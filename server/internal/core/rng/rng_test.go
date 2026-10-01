package rng

import (
	"errors"
	"math"
	"testing"

	"thinhthan/internal/core/id"
)

// Regression fixture (gameplay.md § Regression Seeds): one fixed authority
// context. Any change to derivation, encoding or sampling must break these
// literals.
var regressionContext = Context{
	ContentRevision: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	SourceID:        id.UUID{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00},
	OperationID:     id.UUID{0x01, 0x8c, 0xfa, 0x12, 0x34, 0x56, 0x7a, 0xbc, 0x8d, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab},
	EventID:         42,
	Seed:            0xdeadbeefcafe,
}

// PCG-64 regression vector: fixed context produces the fixed sequence.
func TestNewStream_RegressionVector(t *testing.T) {
	s, err := NewStream(regressionContext)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	wantFloats := []float64{0.6692615005980724, 0.4250501836453051, 0.6622860848752979, 0.2903193394212146}
	for i, want := range wantFloats {
		if got := s.Float64Range(0, 1); got != want {
			t.Fatalf("Float64Range[%d] = %v, want %v", i, got, want)
		}
	}
	wantInts := []int{39, 87, 25, 59}
	for i, want := range wantInts {
		if got := s.IntN(100); got != want {
			t.Fatalf("IntN[%d] = %d, want %d", i, got, want)
		}
	}
	// Weighted picks on a fresh stream of the same context.
	items := []int{10, 20, 30, 40}
	weights := []float64{1, 2, 5, 2}
	wantIdx := []int{2, 2, 2, 1}
	s2, _ := NewStream(regressionContext)
	for i, want := range wantIdx {
		got, err := SelectWeighted(s2, items, weights)
		if err != nil {
			t.Fatalf("SelectWeighted[%d]: %v", i, err)
		}
		if got != items[want] {
			t.Fatalf("SelectWeighted[%d] = %d, want items[%d]=%d", i, got, want, items[want])
		}
	}
}

// Same revision+seed+ordered inputs ⇒ identical results across fresh streams.
func TestNewStream_SameContextIdenticalResults(t *testing.T) {
	a, err := NewStream(regressionContext)
	if err != nil {
		t.Fatalf("NewStream a: %v", err)
	}
	b, err := NewStream(regressionContext)
	if err != nil {
		t.Fatalf("NewStream b: %v", err)
	}
	for i := 0; i < 64; i++ {
		if fa, fb := a.Float64Range(-1, 1), b.Float64Range(-1, 1); fa != fb {
			t.Fatalf("draw %d diverges: %v != %v", i, fa, fb)
		}
	}
	items := []string{"a", "b", "c", "d"}
	weights := []float64{3, 1, 4, 1}
	a2, _ := NewStream(regressionContext)
	b2, _ := NewStream(regressionContext)
	for i := 0; i < 32; i++ {
		pa, ea := SelectWeighted(a2, items, weights)
		pb, eb := SelectWeighted(b2, items, weights)
		if ea != nil || eb != nil || pa != pb {
			t.Fatalf("pick %d diverges: (%v,%v) vs (%v,%v)", i, pa, ea, pb, eb)
		}
	}
}

// Distinct contexts ⇒ independent streams (different source/event diverge).
func TestNewStream_IndependentContexts(t *testing.T) {
	altSource := regressionContext
	altSource.SourceID[0] ^= 0xff
	altEvent := regressionContext
	altEvent.EventID = 43
	altOp := regressionContext
	altOp.OperationID[15] ^= 0xff
	altSeed := regressionContext
	altSeed.Seed++

	base, _ := NewStream(regressionContext)
	for name, ctx := range map[string]Context{"source": altSource, "event": altEvent, "operation": altOp, "seed": altSeed} {
		other, err := NewStream(ctx)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		same := true
		for i := 0; i < 16; i++ {
			if base.Float64Range(0, 1) != other.Float64Range(0, 1) {
				same = false
				break
			}
		}
		if same {
			t.Fatalf("%s context produced identical prefix", name)
		}
		base, _ = NewStream(regressionContext)
	}
}

// Invalid context inputs return typed errors; NewStream never panics.
func TestNewStream_InvalidContext(t *testing.T) {
	badRev := regressionContext
	badRev.ContentRevision = "nothex"
	allZero := Context{ContentRevision: regressionContext.ContentRevision}
	badOp := regressionContext
	badOp.OperationID = id.NewV4() // v4 is not a well-formed v7
	for name, ctx := range map[string]Context{"bad_revision": badRev, "no_identity": allZero, "non_v7_operation": badOp} {
		if _, err := NewStream(ctx); !errors.Is(err, ErrInvalidContext) {
			t.Fatalf("%s: err = %v, want ErrInvalidContext", name, err)
		}
	}
}

// Weighted selection: deterministic picks, zero weights allowed but never
// selected, malformed inputs rejected.
func TestSelectWeighted(t *testing.T) {
	items := []string{"common", "rare", "epic"}
	weights := []float64{7, 0, 3} // "rare" can never be picked
	s, err := NewStream(regressionContext)
	if err != nil {
		t.Fatal(err)
	}
	picks := map[string]int{}
	for i := 0; i < 200; i++ {
		got, err := SelectWeighted(s, items, weights)
		if err != nil {
			t.Fatalf("SelectWeighted: %v", err)
		}
		picks[got]++
	}
	if picks["rare"] != 0 || picks["common"] == 0 || picks["epic"] == 0 {
		t.Fatalf("unexpected picks: %v", picks)
	}
	for name, args := range map[string]struct {
		items   []string
		weights []float64
	}{
		"empty":        {nil, nil},
		"len_mismatch": {items, []float64{1, 2}},
		"zero_total":   {items, []float64{0, 0, 0}},
		"negative":     {items, []float64{1, -1, 1}},
		"nan":          {items, []float64{1, math.NaN(), 1}},
		"inf":          {items, []float64{1, math.Inf(1), 1}},
	} {
		if _, err := SelectWeighted(s, args.items, args.weights); !errors.Is(err, ErrInvalidSelection) {
			t.Fatalf("%s: err = %v, want ErrInvalidSelection", name, err)
		}
	}
}

// Half-open range boundaries: every sample in [lo,hi), max < hi, and across
// enumerated boundary seeds a sample equal to lo occurs (hi never does).
func TestStream_HalfOpenBoundaries(t *testing.T) {
	for _, seed := range []uint64{0, 1, 2, 0xffffffffffffffff, 0x0123456789abcdef} {
		ctx := regressionContext
		ctx.Seed = seed
		s, err := NewStream(ctx)
		if err != nil {
			t.Fatalf("seed %x: %v", seed, err)
		}
		for i := 0; i < 512; i++ {
			v := s.Float64Range(-0.5, 3.25)
			if v < -0.5 || v >= 3.25 {
				t.Fatalf("seed %x draw %d out of range: %v", seed, i, v)
			}
		}
	}
	// Degenerate width: lo+0*(hi-lo) collapses to lo — sanity bound only.
	s, _ := NewStream(regressionContext)
	if v := s.Float64Range(2, 2+1e-12); v < 2 || v >= 2+1e-12 {
		t.Fatalf("narrow range out of bounds: %v", v)
	}
	// Rejected bounds panic (caller bug, same contract as rand.IntN).
	for _, b := range [][2]float64{{1, 1}, {2, 1}, {math.NaN(), 1}, {0, math.NaN()}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("Float64Range(%v,%v) did not panic", b[0], b[1])
				}
			}()
			s.Float64Range(b[0], b[1])
		}()
	}
	if func() (p any) { defer func() { p = recover() }(); s.IntN(0); return nil }() == nil {
		t.Fatal("IntN(0) did not panic")
	}
}
