package reward

// Claim-cap policy (reward_claims.md § Capacity / Abuse, ADR-0062).
//
//	pending_count < 100          -> new claim as usual
//	100 <= pending_count < 500   -> item claims consolidate per
//	                              owner+item_id+effective_binding; new
//	                              claims may exceed the soft cap
//	pending_count >= 500         -> non-preventable item/equipment
//	                              rolls are skipped; EXP and currency
//	                              still settle; escrow-expiry, PvP/Guild
//	                              and compensation claims still create
//
// Preventable sources never reach Create — their action handler checks
// PendingCount at action start and rejects CLAIM_CAP_REACHED; Create
// re-checks fail-closed.
const (
	// SoftCap rejects preventable source actions (CLAIM_CAP_REACHED).
	SoftCap = int64(softCap)
	// HardCeiling skips non-preventable item/equipment rolls.
	HardCeiling = int64(hardCeiling)
)

// alwaysSettles lists source_types exempt from the hard-ceiling item
// skip (ADR-0062: escrow expiry, PvP/Guild settlement, compensation).
var alwaysSettles = map[string]struct{}{
	"AUCTION_ESCROW_EXPIRY": {},
	"PVP":                   {},
	"GUILD_WAR":             {},
	"GUILD":                 {},
	"ADMIN_COMPENSATION":    {},
}

// ceilingApplies reports whether the hard ceiling affects this source
// (escrow expiry, PvP/Guild settlements and compensation still create
// claims at any count — ADR-0062).
func (in *Input) ceilingApplies() bool {
	_, exempt := alwaysSettles[in.SourceType]
	return !exempt
}

// filterAtCeiling drops item lines at the hard ceiling: the roll is not
// performed so nothing earned is lost, while currency lines still
// settle through their aggregate claim. It reports the surviving input.
func filterAtCeiling(in *Input, pending int64) (out *Input, kept, skipped int) {
	if pending < hardCeiling || !in.ceilingApplies() {
		return in, len(in.Lines), 0
	}
	cp := *in
	cp.Lines = make([]LineInput, 0, len(in.Lines))
	for _, l := range in.Lines {
		if l.Kind == lineItem {
			skipped++
			continue
		}
		cp.Lines = append(cp.Lines, l)
		kept++
	}
	return &cp, kept, skipped
}

// classifyKind picks the claim_kind at creation (ADR-0063):
// currency-only payloads always aggregate per
// owner+currency_id+source_family (the overflow mechanism is
// unconditional); stackable item lines consolidate per
// owner+item_id+effective_binding only once pending_count reaches the
// soft cap; per-instance and multi-line payloads stay SINGLE.
func classifyKind(in *Input, pending int64) string {
	if len(in.Lines) != 1 {
		return ClaimKindSingle
	}
	l := in.Lines[0]
	switch {
	case l.Kind == lineCurrency:
		return ClaimKindCurrencyAggregate
	case l.Kind == lineItem && !l.PerInstance && pending >= softCap:
		return ClaimKindItemConsolidated
	default:
		return ClaimKindSingle
	}
}

// consolidationKey builds the row's key token (no colon inside either
// side — the column CHECK relies on the split).
func consolidationKey(kind string, l *LineInput, in *Input) *string {
	switch kind {
	case ClaimKindCurrencyAggregate:
		k := l.CurrencyID + ":" + in.SourceType
		return &k
	case ClaimKindItemConsolidated:
		k := l.ItemID + ":" + l.EffectiveBinding
		return &k
	default:
		return nil
	}
}
