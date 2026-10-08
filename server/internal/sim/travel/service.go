package travel

import (
	"thinhthan/internal/sim/world"
)

// ServiceID is the NPC_SERVICE sub-registry id this packet owns
// (npc_shop_catalog.md § Shared Service Shape; ADR-0068).
const ServiceID = "travel"

// Register adds the travel service to a dispatcher — the exported
// post-construction seam spec #215 F2 assigns to IMP-020. Production
// wiring (Runtime.Dispatcher().RegisterService) is IMP-069's
// composition step; tests call Register on a locally constructed
// NewInteractDispatcher instead.
func Register(d *world.InteractDispatcher) {
	d.RegisterService(ServiceID)
}
