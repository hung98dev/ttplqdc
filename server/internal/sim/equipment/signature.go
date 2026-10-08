package equipment

// Support signature selection (equipment.md § SUPPORT Signature):
// each SUPPORT loadout contributes at most one support_signature,
// selected deterministically from content definitions matched by coarse
// build facts — never scaling with enhancement, rarity, roll quality,
// or gear score. A character has at most two simultaneous signatures.

// MaxSupportSignatures bounds signatures per character (2 SUPPORT
// loadouts x 1 each).
const MaxSupportSignatures = 2

// SetSignature is one compiled `set_support` record: a signature a set
// contributes at its 2-piece threshold (equipment_catalog.md §
// Support Signature Guardrail — support = 2pc-derived only).
type SetSignature struct {
	SetKey    string
	SupportID string // support.set.<key> id; lexical tiebreaker
	Priority  int    // support_priority param; higher wins
	Threshold int    // pieces required; catalog emits 2
	EffectOps []string
}

// BuildFacts is the coarse-fact projection a signature matcher may
// inspect (Anti-Grind Constraint): nothing finer than these fields is
// legal input.
type BuildFacts struct {
	// SetPieces counts equipped pieces per set_key in THIS loadout.
	SetPieces map[string]int
	// SoulElementFlag / SoulRarityFlag: one contracted-Soul
	// element/rarity presence flag each (matched by the signature def).
	SoulElementFlag string
	SoulRarityFlag  string
}

// SelectSignature picks the at-most-one signature for one SUPPORT
// loadout: eligible = set reaches its threshold (<=4 pieces, catalog is
// 2pc); winner = higher support_priority, then lexical support_id.
// Returns ok=false when no signature matches.
func SelectSignature(facts BuildFacts, defs []SetSignature) (SetSignature, bool) {
	var best SetSignature
	found := false
	for _, d := range defs {
		if d.Threshold > 4 {
			continue // spec guardrail: never a >4pc requirement
		}
		if facts.SetPieces[d.SetKey] < d.Threshold {
			continue
		}
		if !found || d.Priority > best.Priority ||
			(d.Priority == best.Priority && d.SupportID < best.SupportID) {
			best, found = d, true
		}
	}
	return best, found
}

// Signatures resolves the <=2 signatures of a character from its two
// SUPPORT loadouts' facts — one per loadout, in loadout order.
func Signatures(support []BuildFacts, defs []SetSignature) []SetSignature {
	var out []SetSignature
	for _, f := range support {
		if sig, ok := SelectSignature(f, defs); ok {
			out = append(out, sig)
			if len(out) == MaxSupportSignatures {
				break
			}
		}
	}
	return out
}
