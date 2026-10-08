package combat

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// Just Guard tutorial hint (combat.md § Just Guard): the first
// JUST_GUARD_WINDOW emitted for a character sets just_guard_hint=true
// on that S2C_COMBAT_EVENT and persists the once-only flag
// progression.first_session.just_guard_hint.
//
// The durable write of that flag goes through this single isolated
// port — per the wave-14 plan the flag write stays behind one
// function so the owning lane (progression-flags subaggregate) can
// bind it without touching combat internals.
type HintPort interface {
	// HasHint reports whether the character already emitted its
	// once-only hint.
	HasHint(characterID [16]byte) bool
	// PersistHint records the flag for the character.
	PersistHint(characterID [16]byte)
}

// emitHint marks just_guard_hint on the event and persists the flag
// through the injected port. Returns false when the port is absent,
// the entity is not a player with a character id, or the hint was
// already emitted.
func (s *System) emitHint(e *runtime.Entity, ev *protocolv1.S2CCombatEvent) bool {
	if s.cfg.Hint == nil || e.Snap.CharacterID == [16]byte{} {
		return false
	}
	charID := e.Snap.CharacterID
	if s.cfg.Hint.HasHint(charID) {
		return false
	}
	ev.JustGuardHint = true
	s.cfg.Hint.PersistHint(charID)
	return true
}
