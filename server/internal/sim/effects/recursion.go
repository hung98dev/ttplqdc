package effects

// recursion.go — the recursion safety cap of status_effects.md /
// combat.md: effects triggered by effects are bounded to depth 3 and
// each (effect_id, target) pair may apply at most once per root
// application.

// MaxRecursionDepth is the hard depth cap for effect-triggered
// applications.
const MaxRecursionDepth = 3

// RecursionGuard tracks one root application's cascade.
type RecursionGuard struct {
	depth int
	seen  map[[2]interface{}]bool
}

// NewRecursionGuard starts a root application chain.
func NewRecursionGuard() *RecursionGuard {
	return &RecursionGuard{seen: make(map[[2]interface{}]bool)}
}

// Enter admits one nested application: false when the depth cap is
// hit or the (effect_id, target) pair already applied under this
// root.
func (g *RecursionGuard) Enter(effectID string, target uint64) bool {
	if g.depth >= MaxRecursionDepth {
		return false
	}
	k := [2]interface{}{effectID, target}
	if g.seen[k] {
		return false
	}
	g.seen[k] = true
	return true
}

// Nest opens the child guard for the next cascade level under the
// same root (seen set shared).
func (g *RecursionGuard) Nest() *RecursionGuard {
	return &RecursionGuard{depth: g.depth + 1, seen: g.seen}
}

// Depth reports the current cascade depth.
func (g *RecursionGuard) Depth() int { return g.depth }
