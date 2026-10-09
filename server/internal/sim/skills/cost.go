package skills

// cost.go — resource cost commit surface (skills.md § Cost Commitment):
// every launch skill's MP cost commits ON_START — deducted once when
// the action is accepted. Rejected actions deduct nothing; an
// interrupted action never refunds or re-charges.

// Committer tracks per-action commit-once cost deduction. Deduction is
// the combat system's job; this guard makes commit-once explicit and
// testable.
type Committer struct {
	committed map[uint64]int64 // action_instance_id -> committed MP
}

// NewCommitter returns an empty commit-once tracker.
func NewCommitter() *Committer {
	return &Committer{committed: map[uint64]int64{}}
}

// Commit records the ON_START deduction for an action instance. The
// first call returns the MP to deduct; subsequent calls for the same
// action return 0 (commit-once).
func (c *Committer) Commit(actionID uint64, d *Def) int64 {
	if _, ok := c.committed[actionID]; ok {
		return 0
	}
	c.committed[actionID] = d.CostMP
	return d.CostMP
}

// Committed reports whether the action's cost was already committed.
func (c *Committer) Committed(actionID uint64) bool {
	_, ok := c.committed[actionID]
	return ok
}

// Forget releases tracking for a completed action instance.
func (c *Committer) Forget(actionID uint64) {
	delete(c.committed, actionID)
}
