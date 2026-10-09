package skills

// cooldown.go — cooldown commit surface (skills.md § Cooldown
// Commitment): the cooldown for all launch skills commits ON_START —
// it starts when the action is accepted, never on completion.
//
// CooldownMs (resolve.go) is the per-level resolved value; this file
// carries the commit-side bookkeeping used by consumers that must hold
// a per-entity cooldown window.

// CooldownWindow is a committed cooldown interval.
type CooldownWindow struct {
	SkillID   string
	StartTick uint64
	EndTick   uint64 // authoritative cooldown end (inclusive)
	Committed bool   // ON_START commit is irrevocable once accepted
}

// CommitCooldown produces the ON_START committed window for a def:
// cooldown_end = accepted_tick + ceil(cooldown_ms(S)/50).
func CommitCooldown(d *Def, level int32, acceptedTick uint64) CooldownWindow {
	return CooldownWindow{
		SkillID:   d.ID,
		StartTick: acceptedTick,
		EndTick:   acceptedTick + ceilTick(CooldownMs(d, level)),
		Committed: true,
	}
}

// ActiveAt reports whether the window still runs at a tick. The
// committed window never rolls back — interruption does not restore it.
func (w CooldownWindow) ActiveAt(tick uint64) bool {
	return w.Committed && tick <= w.EndTick
}
