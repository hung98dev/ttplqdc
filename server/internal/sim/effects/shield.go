package effects

// shield.go — the shield book of combat.md § Shields: grant/consume/
// expire/break/remove, consume order earliest expires_at → lexical
// effect_id → creation seq, same source+effect reapply =
// max(remaining, new) + new expiry, SHIELD_BROKEN / SHIELD_EXPIRED /
// SHIELD_REMOVED distinct, ward-linked lifetimes.

// Shield is one live shield grant.
type Shield struct {
	EffectID  string
	SourceID  uint64
	ExpiresAt uint64
	Remaining int64
	Capacity  int64
	seq       uint64
}

type shieldBook struct {
	target  uint64
	entries []*Shield
}

// applyShield resolves a KindShield template request: capacity from
// authored coefficients (support_scale applied) or an explicit
// ShieldAmount (absorb / ally-shared grants).
func (s *System) applyShield(src StatsView, tgt TargetView, t *Template, req Request, ctx Ctx) []Result {
	now := ctx.Now
	stats := src
	// Shield coefficients scale off the TARGET's vitals for self
	// grants; ally-shared and absorb grants carry explicit amounts.
	var capacity int64
	switch {
	case req.ShieldAmount > 0:
		capacity = req.ShieldAmount
	default:
		support := req.SupportScale
		if support == 0 {
			support = 1.0
		}
		capacity = int64(float64(
			float64(stats.MaxHP)*t.ShieldMaxHPCoef+
				float64(stats.Attack)*t.ShieldAttackCoef+
				float64(stats.Defense)*t.ShieldDefenseCoef) * support)
	}
	if capacity <= 0 {
		return []Result{{
			Kind: ResultRejected, TargetID: tgt.EntityID,
			SourceID: src.EntityID, EffectID: t.ID,
		}}
	}
	sb := s.shieldBookFor(tgt.EntityID)
	expires := now + msToTicks(t.DurationMs)

	// Same source+effect reapply: max(remaining, new) + new expiry.
	for _, sh := range sb.entries {
		if sh.EffectID == t.ID && sh.SourceID == src.EntityID && now < sh.ExpiresAt {
			if capacity > sh.Remaining {
				sh.Remaining = capacity
			}
			if capacity > sh.Capacity {
				sh.Capacity = capacity
			}
			sh.ExpiresAt = expires
			return []Result{{
				Kind: ResultShieldGranted, TargetID: tgt.EntityID,
				SourceID: src.EntityID, EffectID: t.ID,
				Amount: sh.Remaining, ExpiresAt: sh.ExpiresAt,
			}}
		}
	}
	sb.entries = append(sb.entries, &Shield{
		EffectID: t.ID, SourceID: src.EntityID,
		ExpiresAt: expires, Remaining: capacity,
		Capacity: capacity, seq: s.nextSeq(),
	})
	return []Result{{
		Kind: ResultShieldGranted, TargetID: tgt.EntityID,
		SourceID: src.EntityID, EffectID: t.ID,
		Amount: capacity, ExpiresAt: expires,
	}}
}

// ConsumeShields absorbs `amount` of damage in spec order: earliest
// expires_at, then lexical effect_id, then creation seq. Returns the
// absorbed total and the ordered results (SHIELD_BROKEN on depletion).
func (s *System) ConsumeShields(target uint64, amount int64, now uint64) (int64, []Result) {
	sb := s.shieldBookFor(target)
	// Consume order: earliest expires_at -> lexical effect_id -> seq.
	for i := 1; i < len(sb.entries); i++ {
		for j := i; j > 0; j-- {
			a, b := sb.entries[j-1], sb.entries[j]
			if b.ExpiresAt < a.ExpiresAt ||
				(b.ExpiresAt == a.ExpiresAt && b.EffectID < a.EffectID) ||
				(b.ExpiresAt == a.ExpiresAt && b.EffectID == a.EffectID && b.seq < a.seq) {
				sb.entries[j-1], sb.entries[j] = sb.entries[j], sb.entries[j-1]
			}
		}
	}
	var out []Result
	remaining := amount
	for i := 0; i < len(sb.entries) && remaining > 0; {
		sh := sb.entries[i]
		if now >= sh.ExpiresAt {
			sb.entries = append(sb.entries[:i], sb.entries[i+1:]...)
			out = append(out, Result{
				Kind: ResultShieldExpired, TargetID: target,
				SourceID: sh.SourceID, EffectID: sh.EffectID,
			})
			continue
		}
		take := sh.Remaining
		if take > remaining {
			take = remaining
		}
		sh.Remaining -= take
		remaining -= take
		out = append(out, Result{
			Kind: ResultShieldConsumed, TargetID: target,
			SourceID: sh.SourceID, EffectID: sh.EffectID,
			Amount: take,
		})
		if sh.Remaining == 0 {
			sb.entries = append(sb.entries[:i], sb.entries[i+1:]...)
			out = append(out, Result{
				Kind: ResultShieldBroken, TargetID: target,
				SourceID: sh.SourceID, EffectID: sh.EffectID,
			})
			out = append(out, s.unlinkWards(target, sh.EffectID, now)...)
			continue
		}
		i++
	}
	return amount - remaining, out
}

// RemoveShield force-removes one shield (explicit removal semantics —
// SHIELD_REMOVED, distinct from broken/expired).
func (s *System) RemoveShield(target uint64, effectID string, sourceID uint64, now uint64) []Result {
	sb := s.shieldBookFor(target)
	for i, sh := range sb.entries {
		if sh.EffectID == effectID && (sourceID == 0 || sh.SourceID == sourceID) {
			sb.entries = append(sb.entries[:i], sb.entries[i+1:]...)
			out := []Result{{
				Kind: ResultShieldRemoved, TargetID: target,
				SourceID: sh.SourceID, EffectID: sh.EffectID,
			}}
			return append(out, s.unlinkWards(target, sh.EffectID, now)...)
		}
	}
	return nil
}

// tickShields expires elapsed shields (SHIELD_EXPIRED) and unlinks
// their wards.
func (s *System) tickShields(sb *shieldBook, now uint64) []Result {
	var out []Result
	for i := 0; i < len(sb.entries); {
		sh := sb.entries[i]
		if now >= sh.ExpiresAt {
			sb.entries = append(sb.entries[:i], sb.entries[i+1:]...)
			out = append(out, Result{
				Kind: ResultShieldExpired, TargetID: sb.target,
				SourceID: sh.SourceID, EffectID: sh.EffectID,
			})
			out = append(out, s.unlinkWards(sb.target, sh.EffectID, now)...)
			continue
		}
		i++
	}
	return out
}

// unlinkWards removes ward instances whose lifetime follows the dead
// shield (thuy_kinh_ward ↔ thuy_kinh_shield, tho_giap_ward ↔
// tho_giap_shield).
func (s *System) unlinkWards(target uint64, shieldID string, now uint64) []Result {
	book, ok := s.books[target]
	if !ok {
		return nil
	}
	var out []Result
	for _, i := range book.ordered() {
		if i.tmpl.LinkedShield == shieldID {
			book.remove(i)
			out = append(out, Result{
				Kind: ResultExpired, TargetID: target,
				SourceID: i.SourceID, EffectID: i.EffectID,
				ExpiresAt: now,
			})
		}
	}
	return out
}

// ShieldTotal reports the live shield absorption pool of a target.
func (s *System) ShieldTotal(target uint64, now uint64) int64 {
	var total int64
	for _, sh := range s.shieldBookFor(target).entries {
		if now < sh.ExpiresAt {
			total += sh.Remaining
		}
	}
	return total
}
