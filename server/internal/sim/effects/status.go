package effects

// status.go — the status reapply engine: immunity pre-check, the
// REFRESH_DURATION/STACK/REPLACE_STRONGER/IGNORE rules of
// status_effects.md L48-51, dispel ordering, and the DoT anchor
// bookkeeping of class_skill_catalog.md L136-138.

// tickMsPerTick is the sim tick duration (20 Hz).
const tickMsPerTick = 50

func msToTicks(ms int64) uint64 {
	if ms <= 0 {
		return 0
	}
	t := uint64(ms) / tickMsPerTick
	if uint64(ms)%tickMsPerTick != 0 {
		t++
	}
	if t == 0 {
		t = 1
	}
	return t
}

// applyStatus applies one status/DoT request. Immunity is checked
// pre-creation: SLOW_IMMUNE rejects SLOW instances and
// DISPLACEMENT_IMMUNE rejects DISPLACEMENT instances and
// forced-position results while the rest of the hit resolves.
func (s *System) applyStatus(src StatsView, tgt TargetView, t *Template, req Request, ctx Ctx) []Result {
	book := s.book(tgt.EntityID)
	now := ctx.Now

	// Forced-position requests are suppressed on DISPLACEMENT_IMMUNE
	// while the rest of the hit resolves.
	if req.ForcedPosition && book.immune(now)&TagDisplacementImmune != 0 {
		return []Result{{
			Kind: ResultForcedPositionSuppressed, TargetID: tgt.EntityID,
			SourceID: src.EntityID, EffectID: req.EffectID,
			RejectedBy: TagDisplacementImmune,
		}}
	}

	// Immunity pre-check: active wards reject matching instances
	// before creation.
	imm := book.immune(now)
	switch {
	case t.Tags&TagSlow != 0 && imm&TagSlowImmune != 0:
		return []Result{{
			Kind: ResultRejected, TargetID: tgt.EntityID,
			SourceID: src.EntityID, EffectID: req.EffectID,
			RejectedBy: TagSlowImmune,
		}}
	case t.Tags&TagDisplacement != 0 && imm&TagDisplacementImmune != 0:
		return []Result{{
			Kind: ResultRejected, TargetID: tgt.EntityID,
			SourceID: src.EntityID, EffectID: req.EffectID,
			RejectedBy: TagDisplacementImmune,
		}}
	}

	durMs := req.DurationMs
	if durMs == 0 {
		durMs = t.DurationMs
	}
	var expires uint64
	if durMs == 0 {
		if t.LinkedShield == "" {
			// No authored lifetime and no request override — nothing
			// to create.
			return []Result{{
				Kind: ResultRejected, TargetID: tgt.EntityID,
				SourceID: src.EntityID, EffectID: req.EffectID,
			}}
		}
		// Ward lifetime follows the linked shield; the shield's
		// expiry/break/removal unlinks it.
		expires = ^uint64(0)
	} else {
		expires = now + msToTicks(durMs)
	}
	key := InstanceKey{EffectID: t.ID}
	if t.PerSource {
		key.SourceID = src.EntityID
	}

	existing := book.Get(key)
	var out []Result
	if existing != nil && existing.active(now) {
		out = s.reapply(book, existing, src, t, req, now, expires)
	} else {
		inst := &Instance{
			EffectID:            t.ID,
			SourceID:            src.EntityID,
			TargetID:            tgt.EntityID,
			Stacks:              req.Stacks,
			StartedTick:         now,
			ExpiresAtTick:       expires,
			AnchorTick:          now,
			DamageExpiresAtTick: expires,
			AttackSnapshot:      src.Attack,
			Kind:                t.Kind,
			Tags:                t.Tags,
			Dispellable:         t.Dispellable,
			ReapplyMode:         t.Reapply,
			seq:                 s.nextSeq(),
			tmpl:                t,
		}
		if inst.Stacks < 1 {
			inst.Stacks = 1
		}
		if req.AttackSnapshot != 0 {
			inst.AttackSnapshot = req.AttackSnapshot
		}
		inst.MagHealingReceived = req.HealingReceivedMult
		inst.MagAttackBP = req.AttackAddBP
		inst.MagDefenseBP = req.DefenseAddBP
		if t.ResidualExtensionMs > 0 {
			inst.ExpiresAtTick = expires + msToTicks(t.ResidualExtensionMs)
		}
		book.inst[key] = inst
		out = []Result{{
			Kind: ResultApplied, TargetID: tgt.EntityID,
			SourceID: src.EntityID, EffectID: t.ID,
			Stacks: inst.Stacks, ExpiresAt: inst.ExpiresAtTick,
			Instance: inst,
		}}
	}

	// Application-side tag stripping (luu_bo_haste removes SLOW).
	if t.RemoveTagsOnApply != 0 {
		for _, i := range book.ordered() {
			if i.Tags&t.RemoveTagsOnApply != 0 && i.Tags&TagNegative != 0 && i.active(now) {
				book.remove(i)
				out = append(out, Result{
					Kind: ResultDispelled, TargetID: tgt.EntityID,
					SourceID: i.SourceID, EffectID: i.EffectID,
					ExpiresAt: now,
				})
			}
		}
	}
	return out
}

// reapply resolves one re-application against an active instance.
func (s *System) reapply(book *Book, inst *Instance, src StatsView, t *Template, req Request, now, newExpires uint64) []Result {
	base := Result{
		TargetID: inst.TargetID, SourceID: src.EntityID,
		EffectID: t.ID, Instance: inst,
	}
	switch t.Reapply {
	case Stack:
		if inst.Stacks < t.MaxStacks {
			inst.Stacks++
		}
		if req.Stacks > 1 && inst.Stacks < t.MaxStacks {
			inst.Stacks += req.Stacks - 1
			if inst.Stacks > t.MaxStacks {
				inst.Stacks = t.MaxStacks
			}
		}
		inst.ExpiresAtTick = newExpires
		if t.Kind == KindDoT {
			inst.DamageExpiresAtTick = newExpires
			inst.AttackSnapshot = src.Attack
			if req.AttackSnapshot != 0 {
				inst.AttackSnapshot = req.AttackSnapshot
			}
		}
		base.Kind, base.Stacks, base.ExpiresAt = ResultStackChanged, inst.Stacks, inst.ExpiresAtTick
		return []Result{base}
	case RefreshDuration:
		// Unconditional now+duration reset, except control templates
		// which never shorten.
		if t.Control {
			if newExpires > inst.ExpiresAtTick {
				inst.ExpiresAtTick = newExpires
			}
		} else {
			inst.ExpiresAtTick = newExpires
		}
		// Magnitude and source snapshot take the new application;
		// the DoT anchor stays at first application.
		inst.SourceID = src.EntityID
		if req.HealingReceivedMult != 0 {
			inst.MagHealingReceived = req.HealingReceivedMult
		}
		if req.AttackAddBP != 0 {
			inst.MagAttackBP = req.AttackAddBP
		}
		if req.DefenseAddBP != 0 {
			inst.MagDefenseBP = req.DefenseAddBP
		}
		if t.Kind == KindDoT {
			inst.DamageExpiresAtTick = inst.ExpiresAtTick
			inst.AttackSnapshot = src.Attack
			if req.AttackSnapshot != 0 {
				inst.AttackSnapshot = req.AttackSnapshot
			}
		}
		base.Kind, base.Stacks, base.ExpiresAt = ResultRefreshed, inst.Stacks, inst.ExpiresAtTick
		return []Result{base}
	case ReplaceStronger:
		// Keep the larger absolute magnitude; tie goes to later expiry.
		if req.AttackSnapshot > inst.AttackSnapshot ||
			(req.AttackSnapshot == inst.AttackSnapshot && newExpires > inst.ExpiresAtTick) {
			if req.AttackSnapshot != 0 {
				inst.AttackSnapshot = req.AttackSnapshot
			}
			inst.ExpiresAtTick = newExpires
			inst.SourceID = src.EntityID
			base.Kind, base.Stacks, base.ExpiresAt = ResultReplaced, inst.Stacks, inst.ExpiresAtTick
			return []Result{base}
		}
		base.Kind = ResultIgnored
		return []Result{base}
	case Ignore:
		base.Kind = ResultIgnored
		return []Result{base}
	}
	base.Kind = ResultIgnored
	return []Result{base}
}

// Dispel removes the n most recently applied dispellable NEGATIVE
// instances — order: dispel_priority desc, oldest start first,
// lexical effect_id (status_effects.md § Dispel).
func (s *System) Dispel(target uint64, n int, now uint64) []Result {
	book := s.book(target)
	var cand []*Instance
	for _, i := range book.inst {
		if i.Dispellable && i.Tags&TagNegative != 0 && i.active(now) {
			cand = append(cand, i)
		}
	}
	for a := 1; a < len(cand); a++ {
		for c := a; c > 0; c-- {
			x, y := cand[c-1], cand[c]
			if y.tmpl.DispelPriority > x.tmpl.DispelPriority ||
				(y.tmpl.DispelPriority == x.tmpl.DispelPriority && y.StartedTick < x.StartedTick) ||
				(y.tmpl.DispelPriority == x.tmpl.DispelPriority && y.StartedTick == x.StartedTick && y.EffectID < x.EffectID) {
				cand[c-1], cand[c] = cand[c], cand[c-1]
			}
		}
	}
	var out []Result
	for _, i := range cand {
		if n <= 0 {
			break
		}
		book.remove(i)
		out = append(out, Result{
			Kind: ResultDispelled, TargetID: target,
			SourceID: i.SourceID, EffectID: i.EffectID,
			ExpiresAt: now,
		})
		n--
	}
	return out
}

// ClearOnDeath strips every instance without persist_through_death.
func (s *System) ClearOnDeath(target uint64, now uint64) []Result {
	return s.clearWhere(target, now, func(i *Instance) bool {
		return !i.tmpl.PersistThroughDeath
	})
}

// ClearOnMap strips every instance without persist_across_map.
func (s *System) ClearOnMap(target uint64, now uint64) []Result {
	return s.clearWhere(target, now, func(i *Instance) bool {
		return !i.tmpl.PersistAcrossMap
	})
}

func (s *System) clearWhere(target uint64, now uint64, drop func(*Instance) bool) []Result {
	book := s.book(target)
	var out []Result
	for _, i := range book.ordered() {
		if drop(i) {
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

// Book exposes the live instance store for an entity (read-only use:
// stat aggregation, Snap.Statuses fill, test fixtures).
func (s *System) Book(target uint64) *Book { return s.book(target) }
