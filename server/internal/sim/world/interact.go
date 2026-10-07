package world

// InteractDispatcher is the per-world registry of serviceable 103 kinds
// and NPC services. Kinds outside the handled set and services outside an
// NPC's allowed set fail closed — the consult rejects the admission and
// exactly one 116 (TARGET_INVALID) goes out, never silence.
type InteractDispatcher struct {
	services map[string]bool // registered service_id values
}

// NewInteractDispatcher registers the IMP-018 service set.
func NewInteractDispatcher() *InteractDispatcher {
	d := &InteractDispatcher{services: map[string]bool{}}
	d.RegisterService(ServiceSetCheckpoint)
	return d
}

// IMP-018 service ids (npc_shop_catalog.md); `travel` registers in
// IMP-020 against the same dispatcher surface.
const ServiceSetCheckpoint = "set_checkpoint"

// RegisterService adds a service id to the admitted set.
func (d *InteractDispatcher) RegisterService(serviceID string) {
	d.services[serviceID] = true
}

// ServiceRegistered reports the id admitted by the dispatcher.
func (d *InteractDispatcher) ServiceRegistered(serviceID string) bool {
	return d.services[serviceID]
}
