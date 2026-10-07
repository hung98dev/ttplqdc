package world

import (
	"sync"
	"time"

	"thinhthan/internal/core/id"
)

// ChannelState is one channel-instance lifecycle state.
type ChannelState int

const (
	ChannelStopped ChannelState = iota
	ChannelStarting
	ChannelRunning
	ChannelDraining
)

// Channel is one map's channel-instance registry row.
type Channel struct {
	Index         uint32
	State         ChannelState
	Occupancy     int // reservations + admitted players
	IdleSince     time.Time
	Quarantined   bool // loader/hook timeout — excluded from selection
	LastErr       error
	host          *ChannelHost
}

// member is the director's per-character location ledger.
type member struct {
	MapID       string
	Channel     uint32
	LastSwitch  time.Time // last CHANNEL_SWITCH admission (cooldown)
}

// Director is the single-writer registry of every normal-world map's
// channels (map id → channels 1..30), every resident character, every
// pending placement and every in-flight transfer. It is a pure state
// machine — effects (outbound, starts, stops) are returned for the
// runtime to execute.
type Director struct {
	mu        sync.Mutex
	maps      MapCatalog
	channels  map[string][]*Channel // mapID → index 1..30 (slot 0 unused)
	members   map[id.UUID]*member
	pending   map[id.UUID]*PendingWait
	transfers map[id.UUID]*transfer
	now       func() time.Time
}

// NewDirector builds the registry for the catalog's maps; every channel
// starts stopped.
func NewDirector(maps MapCatalog, now func() time.Time) *Director {
	d := &Director{
		maps:      maps,
		channels:  make(map[string][]*Channel),
		members:   make(map[id.UUID]*member),
		pending:   make(map[id.UUID]*PendingWait),
		transfers: make(map[id.UUID]*transfer),
		now:       now,
	}
	for _, m := range maps.Maps() {
		chs := make([]*Channel, ChannelsPerMap+1)
		for i := 1; i <= ChannelsPerMap; i++ {
			chs[i] = &Channel{Index: uint32(i), State: ChannelStopped}
		}
		d.channels[m.MapID] = chs
	}
	return d
}

// Channel exposes one channel row (read under the registry lock by the
// runtime; never mutated outside this file).
func (d *Director) Channel(mapID string, ch uint32) (*Channel, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.channel(mapID, ch)
}

func (d *Director) channel(mapID string, ch uint32) (*Channel, bool) {
	chs, ok := d.channels[mapID]
	if !ok || ch == 0 || ch > ChannelsPerMap {
		return nil, false
	}
	return chs[ch], true
}

// Member returns the character's current channel membership.
func (d *Director) Member(characterID id.UUID) (mapID string, channel uint32, ok bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	m, ok := d.members[characterID]
	if !ok {
		return "", 0, false
	}
	return m.MapID, m.Channel, true
}

// occupancy counts reservations and admitted players alike so a starting
// channel's members already consume capacity.
func (c *Channel) free(cap_ int) bool { return c.Occupancy < cap_ }

// autoOrder returns the auto-placement candidate order (sharding.md):
// most-populated running below SoftCap first — packing keeps idle
// channels free to stop — then lowest-index stopped, then nothing.
func (d *Director) autoOrder(chs []*Channel) []*Channel {
	var running []*Channel
	for i := 1; i <= ChannelsPerMap; i++ {
		c := chs[i]
		if c.State == ChannelRunning && c.free(SoftCap) && !c.Quarantined {
			running = append(running, c)
		}
	}
	sortChannels(running, true)
	out := running
	for i := 1; i <= ChannelsPerMap; i++ {
		c := chs[i]
		if c.State == ChannelStopped && !c.Quarantined {
			out = append(out, c)
		}
	}
	return out
}

// forcedOrder returns the forced-placement candidate order: preferred
// below HardCap → least-populated running below SoftCap → lowest stopped
// → running below HardCap → none (caller goes pending).
func (d *Director) forcedOrder(chs []*Channel, preferred uint32) []*Channel {
	var out []*Channel
	if preferred >= 1 && preferred <= ChannelsPerMap {
		c := chs[preferred]
		if c.State != ChannelDraining && c.free(HardCap) && !c.Quarantined {
			out = append(out, c)
		}
	}
	var running []*Channel
	for i := 1; i <= ChannelsPerMap; i++ {
		c := chs[i]
		if c.State == ChannelRunning && c.free(SoftCap) && !c.Quarantined {
			running = append(running, c)
		}
	}
	sortChannels(running, false)
	out = append(out, running...)
	for i := 1; i <= ChannelsPerMap; i++ {
		c := chs[i]
		if c.State == ChannelStopped && !c.Quarantined {
			out = append(out, c)
		}
	}
	for i := 1; i <= ChannelsPerMap; i++ {
		c := chs[i]
		if c.State == ChannelRunning && !c.free(SoftCap) && c.free(HardCap) && !c.Quarantined {
			out = append(out, c)
		}
	}
	return out
}

// sortChannels orders by occupancy (desc for packing, asc for
// spread-first-fit), ties by index ascending.
func sortChannels(cs []*Channel, pack bool) {
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0; j-- {
			a, b := cs[j-1], cs[j]
			swap := a.Occupancy < b.Occupancy && pack ||
				a.Occupancy > b.Occupancy && !pack ||
				a.Occupancy == b.Occupancy && a.Index > b.Index
			if !swap {
				break
			}
			cs[j-1], cs[j] = b, a
		}
	}
}

// Select evaluates one placement request. A successful select reserves
// capacity immediately (Occupancy++); the caller admits the player once
// the channel runs. Forced requests that find nothing return a
// PendingWait instead of failing.
func (d *Director) Select(req PlacementRequest) (PlacementResult, *PendingWait, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	chs, ok := d.channels[req.MapID]
	if !ok {
		return PlacementResult{}, nil, ErrNoMap
	}
	var order []*Channel
	if req.Kind == PlaceForced {
		order = d.forcedOrder(chs, req.Preferred)
	} else {
		order = d.autoOrder(chs)
	}
	if len(order) == 0 {
		if req.Kind == PlaceForced {
			return PlacementResult{}, NewPendingWait(d.now(), req.CharacterID,
				0, pendingReason(req.ForcedReason), req), nil
		}
		return PlacementResult{}, nil, ErrCapacityFull
	}
	c := order[0]
	c.Occupancy++
	c.IdleSince = time.Time{}
	start := c.State == ChannelStopped
	if start {
		c.State = ChannelStarting
	}
	return PlacementResult{MapID: req.MapID, ChannelIndex: c.Index, Start: start}, nil, nil
}

func pendingReason(r ForcedReason) PendingReason {
	switch r {
	case ForcedRespawn:
		return PendingRespawn
	case ForcedInstanceReturn:
		return PendingInstanceReturn
	case ForcedRecovery, ForcedReattach:
		return PendingReconnect
	default:
		return PendingFirstLogin
	}
}

// Admitted records a player's entry into a channel (drain-side, after the
// partition admits the entity).
func (d *Director) Admitted(characterID id.UUID, mapID string, ch uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.members[characterID] = &member{MapID: mapID, Channel: ch}
	delete(d.pending, characterID)
}

// OnPlayerLeave removes the character's membership and frees the
// channel's occupancy; an empty running channel starts its idle clock.
func (d *Director) OnPlayerLeave(characterID id.UUID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	m, ok := d.members[characterID]
	if !ok {
		return
	}
	delete(d.members, characterID)
	if c, ok := d.channel(m.MapID, m.Channel); ok && c.Occupancy > 0 {
		c.Occupancy--
		if c.Occupancy == 0 && c.State == ChannelRunning {
			c.IdleSince = d.now()
		}
	}
}

// MarkSwitch records a completed channel switch for cooldown checks.
func (d *Director) MarkSwitch(characterID id.UUID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if m, ok := d.members[characterID]; ok {
		m.LastSwitch = d.now()
	}
}

// Pending returns the character's queued forced placement, if any.
func (d *Director) Pending(characterID id.UUID) (*PendingWait, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, ok := d.pending[characterID]
	return p, ok
}

// AddPending queues a pending placement; retries re-select every tick
// until the request resolves.
func (d *Director) AddPending(p *PendingWait) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pending[p.CharacterID] = p
}

// DropPending removes a pending entry without resolving it.
func (d *Director) DropPending(characterID id.UUID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.pending, characterID)
}

// hostRunning marks a started channel running with its host handle.
func (d *Director) hostRunning(mapID string, ch uint32, h *ChannelHost) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if c, ok := d.channel(mapID, ch); ok {
		c.State = ChannelRunning
		c.host = h
	}
}

// hostStopped marks the channel stopped and clears its host.
func (d *Director) hostStopped(mapID string, ch uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if c, ok := d.channel(mapID, ch); ok {
		c.State = ChannelStopped
		c.host = nil
	}
}

// hostFailed drops a starting channel to stopped+quarantined.
func (d *Director) hostFailed(mapID string, ch uint32, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if c, ok := d.channel(mapID, ch); ok {
		c.State = ChannelStopped
		c.Quarantined = true
		c.LastErr = err
		c.host = nil
	}
}

// RestartAllStopped returns every stopped channel to un-quarantined
// stopped — the post-restart state (sharding.md: restart leaves all
// channels stopped and clean).
func (d *Director) RestartAllStopped() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, chs := range d.channels {
		for i := 1; i <= ChannelsPerMap; i++ {
			c := chs[i]
			if c.State == ChannelStopped || c.State == ChannelDraining {
				c.Quarantined = false
				c.LastErr = nil
				c.host = nil
				c.State = ChannelStopped
			}
		}
	}
}

// idleStop names one channel to drain-and-stop (map + index).
type idleStop struct {
	MapID   string
	Channel uint32
}

// tickEffects collects the work the runtime executes this tick: pending
// retries that are due, and idle channels past IdleStop to stop. The
// director stays side-effect-free: stop executes through the runtime so
// pending durable writes drain first (sharding.md stop semantics).
type tickEffects struct {
	pending []*PendingWait // due for retry re-selection
	idle    []idleStop     // idle past IdleStop
}

// peekAuto previews the auto placement without reserving — consult reads
// only (ADR-0083 read-only consults).
func (d *Director) peekAuto(mapID string) (uint32, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	chs, ok := d.channels[mapID]
	if !ok {
		return 0, false
	}
	order := d.autoOrder(chs)
	if len(order) == 0 {
		return 0, false
	}
	return order[0].Index, true
}

// switchCooldown reports the character's channel-switch cooldown end, if
// still active. ChannelSwitchCooldown is the post-switch lockout
// (world_rules.md: channel transfer enforces a 10s cooldown).
const ChannelSwitchCooldown = 10 * time.Second

func (d *Director) switchCooldown(characterID id.UUID) (time.Time, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	m, ok := d.members[characterID]
	if !ok || m.LastSwitch.IsZero() {
		return time.Time{}, false
	}
	until := m.LastSwitch.Add(ChannelSwitchCooldown)
	return until, d.now().Before(until)
}

// Tick evaluates pending retries and idle channels for the given now.
func (d *Director) Tick() tickEffects {
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()
	var ef tickEffects
	for _, p := range d.pending {
		if !now.Before(p.nextAt) {
			ef.pending = append(ef.pending, p)
		}
	}
	for mapID, chs := range d.channels {
		for i := 1; i <= ChannelsPerMap; i++ {
			c := chs[i]
			if c.State == ChannelRunning && c.Occupancy == 0 &&
				!c.IdleSince.IsZero() && now.Sub(c.IdleSince) >= IdleStop {
				c.State = ChannelDraining
				c.IdleSince = time.Time{}
				ef.idle = append(ef.idle, idleStop{MapID: mapID, Channel: c.Index})
			}
		}
	}
	return ef
}
