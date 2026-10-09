package equipment

import "thinhthan/internal/config"

// Register wires the equipment expansion checks into the activation
// gate: every candidate snapshot must satisfy the spec's Validation
// reject list before it may activate. Same registration contract as
// config/validation packages (Gate.RegisterCheck).
func Register(g *config.Gate) {
	g.RegisterCheck(Check)
}
