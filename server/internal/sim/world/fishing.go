package world

import "thinhthan/internal/sim/runtime"

// FishingInteractHandler is the post-commit delegate for one committed
// 103 CAST or HOOK — it runs inside the owning partition's drain, never
// on an edge goroutine. The fishing FSM (IDLE -> CASTING -> HOOK_WINDOW
// -> RESOLVING) is a partition-tick domain owned by sim/fishing
// (IMP-058); the world host only routes the committed kind here.
type FishingInteractHandler func(cmd *Command, p *runtime.Partition, tc *runtime.TickContext)

// FishingHandlers is one channel's CAST/HOOK delegate pair.
type FishingHandlers struct {
	Cast FishingInteractHandler
	Hook FishingInteractHandler
}

// FishingHandlerFactory produces a channel's delegates when its host is
// constructed; IMP-058 binds sim/fishing's channel runtime through it.
// Nil fields fall back to the inert default so a partially bound domain
// still fails closed.
type FishingHandlerFactory func(mapID string, ch uint32) *FishingHandlers

// fishingNoops is the unbound default: a committed CAST/HOOK without a
// fishing domain is inert, never a panic.
var fishingNoops = &FishingHandlers{
	Cast: func(*Command, *runtime.Partition, *runtime.TickContext) {},
	Hook: func(*Command, *runtime.Partition, *runtime.TickContext) {},
}
