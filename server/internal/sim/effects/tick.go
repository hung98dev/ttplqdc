package effects

// tick.go — the PhaseTimersStatus system: anchored DoT ticks in the
// determinism order of status_effects.md L209-214, expiry events and
// ward lifetime linkage.

import (
	"sort"

	"thinhthan/internal/sim/runtime"
)

// System is the effects engine: the per-entity book store, the shield
// books, the creation-seq counter, the outbound port and the
// lifesteal rolling windows.
type System struct {
	books   map[uint64]*Book
	shields map[uint64]*shieldBook
	seq     uint64

	// Outbound receives 205 emissions; nil in pure-sim fixtures.
	Outbound runtime.OutboundPort
}

// New constructs the engine. Deps are injected so tests run without
// a partition.
func New(outbound runtime.OutboundPort) *System {
	return &System{
		books:    make(map[uint64]*Book),
		shields:  make(map[uint64]*shieldBook),
		Outbound: outbound,
	}
}

func (s *System) nextSeq() uint64 {
	s.seq++
	return s.seq
}

func (s *System) book(target uint64) *Book {
	b, ok := s.books[target]
	if !ok {
		b = newBook(target)
		s.books[target] = b
	}
	return b
}

func (s *System) shieldBookFor(target uint64) *shieldBook {
	b, ok := s.shields[target]
	if !ok {
		b = &shieldBook{target: target}
		s.shields[target] = b
	}
	return b
}

// Tick implements the PhaseTimersStatus contract: expire elapsed
// instances, run anchored DoT ticks and expire elapsed shields, all
// in determinism order (source_id, effect_id, creation seq per
// target; targets ascending).
//
// Damage produced by DoT ticks is returned as ResultDotTick entries —
// the owning damage pipeline applies them (F-16-1 seam).
func (s *System) Tick(p *runtime.Partition, tc *runtime.TickContext) []Result {
	now := tc.Tick
	targets := make([]uint64, 0, len(s.books)+len(s.shields))
	for id := range s.books {
		targets = append(targets, id)
	}
	for id := range s.shields {
		if _, seen := s.books[id]; !seen {
			targets = append(targets, id)
		}
	}
	sort.Slice(targets, func(a, b int) bool { return targets[a] < targets[b] })

	var out []Result
	for _, target := range targets {
		out = append(out, s.tickTarget(target, now)...)
	}
	if s.Outbound != nil {
		s.emitAll(out, now)
	}
	return out
}

func (s *System) tickTarget(target uint64, now uint64) []Result {
	var out []Result
	if book, ok := s.books[target]; ok {
		for _, i := range book.ordered() {
			if !i.active(now) {
				book.remove(i)
				out = append(out, Result{
					Kind: ResultExpired, TargetID: target,
					SourceID: i.SourceID, EffectID: i.EffectID,
					ExpiresAt: now,
				})
				continue
			}
			if i.Kind != KindDoT || now < i.AnchorTick {
				continue
			}
			// Anchor ticks: anchor + k*TickMs, through
			// DamageExpiresAtTick inclusive.
			elapsed := now - i.AnchorTick
			tickEvery := msToTicks(i.tmpl.TickMs)
			if tickEvery == 0 {
				continue
			}
			if elapsed%tickEvery != 0 || elapsed == 0 {
				continue
			}
			if now > i.DamageExpiresAtTick {
				continue
			}
			dmg := (i.AttackSnapshot * int64(i.tmpl.PerTickAttackRatio*1000)) / 1000 * int64(i.Stacks)
			out = append(out, Result{
				Kind: ResultDotTick, TargetID: target,
				SourceID: i.SourceID, EffectID: i.EffectID,
				Stacks: i.Stacks, Amount: dmg, Element: i.tmpl.Element,
				Instance: i,
			})
		}
	}
	if sb, ok := s.shields[target]; ok {
		out = append(out, s.tickShields(sb, now)...)
	}
	return out
}

// ConsumeChill removes one CHILL instance after the 3-stack
// consumption of class_skill_catalog.md § CHILL Consumption.
func (s *System) ConsumeChill(target, source uint64, now uint64) []Result {
	book := s.book(target)
	key := InstanceKey{EffectID: "effect.skill.thuy.chill_3s", SourceID: source}
	i := book.Get(key)
	if i == nil || !i.active(now) || i.Stacks < 3 {
		return nil
	}
	book.remove(i)
	return []Result{{
		Kind: ResultConsumed, TargetID: target,
		SourceID: source, EffectID: i.EffectID, Stacks: 3,
	}}
}
