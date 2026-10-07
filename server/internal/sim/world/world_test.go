package world

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/geometry"
	"thinhthan/internal/sim/spatial/parity"
)

// --- test doubles -----------------------------------------------------------

// testClocks supplies the injected clocks: Now is wall time (lifecycle,
// pending retries, transfer budgets), SimNow/SimSleep drive partition
// loops — SimSleep advances sim time exactly the slept amount so each
// partition iteration produces exactly one tick without real waiting.
type testClocks struct {
	mu  sync.Mutex
	now time.Time
	sim time.Duration
}

func newTestClocks() *testClocks {
	return &testClocks{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *testClocks) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClocks) SimNow() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sim
}

func (c *testClocks) Sleep(d time.Duration) {
	c.Advance(d)
	time.Sleep(50 * time.Microsecond) // yield the partition goroutine
}

func (c *testClocks) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.sim += d
	c.mu.Unlock()
}

// capOutbound captures every world-control emission.
type capOutbound struct {
	mu   sync.Mutex
	msgs []runtime.Outbound
}

func (o *capOutbound) Enqueue(m runtime.Outbound) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.msgs = append(o.msgs, m)
	return nil
}

func (o *capOutbound) all() []runtime.Outbound {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]runtime.Outbound(nil), o.msgs...)
}

func (o *capOutbound) forChar(charID id.UUID, msgID uint32) []runtime.Outbound {
	var out []runtime.Outbound
	for _, m := range o.all() {
		if m.To == SessionTarget(charID) && m.MessageID == msgID {
			out = append(out, m)
		}
	}
	return out
}

// capEmit records every emitted durable command.
type capEmit struct {
	mu   sync.Mutex
	cmds []runtime.DurableCommand
}

func (e *capEmit) Emit(_ context.Context, cmd runtime.DurableCommand) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cmds = append(e.cmds, cmd)
	return nil
}

func (e *capEmit) all() []runtime.DurableCommand {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]runtime.DurableCommand(nil), e.cmds...)
}

// noopResults drains nothing (no durable results in these tests).
type noopResults struct{}

func (noopResults) DrainInto(buf []runtime.Result) int { return 0 }

// fakeLoader is the durable-read port: clean consequence rows, one fixed
// checkpoint row.
type fakeLoader struct {
	mu      sync.Mutex
	cpID    string
	cpMap   string
	cpAnchr string
	loadErr error
	loads   int
}

func (l *fakeLoader) LoadConsequences(ctx context.Context, mapID string,
	channelID uint64) ([]ConsequenceRow, error) {
	l.mu.Lock()
	l.loads++
	l.mu.Unlock()
	if l.loadErr != nil {
		return nil, l.loadErr
	}
	return nil, nil
}

func (l *fakeLoader) LoadCheckpoint(ctx context.Context, charID id.UUID) (string, string, string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cpID, l.cpMap, l.cpAnchr, nil
}

func (l *fakeLoader) loaded() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loads
}

// fakeCatalog is a hand-built MapCatalog.
type fakeCatalog struct {
	recs        map[string]MapRecord
	order       []string
	portals     map[string]PortalDef
	checkpoints map[string]CheckpointDef
}

func newFakeCatalog(recs []MapRecord) *fakeCatalog {
	c := &fakeCatalog{
		recs:        map[string]MapRecord{},
		portals:     map[string]PortalDef{},
		checkpoints: map[string]CheckpointDef{},
	}
	for _, r := range recs {
		c.recs[r.MapID] = r
		c.order = append(c.order, r.MapID)
		for _, p := range r.Portals {
			c.portals[p.PortalID] = p
		}
		for _, cp := range r.Checkpoints {
			c.checkpoints[cp.CheckpointID] = cp
		}
	}
	return c
}

func (c *fakeCatalog) Lookup(mapID string) (MapRecord, bool) {
	r, ok := c.recs[mapID]
	return r, ok
}
func (c *fakeCatalog) Maps() []MapRecord {
	out := make([]MapRecord, len(c.order))
	for i, m := range c.order {
		out[i] = c.recs[m]
	}
	return out
}
func (c *fakeCatalog) Portal(pid string) (PortalDef, bool) {
	p, ok := c.portals[pid]
	return p, ok
}
func (c *fakeCatalog) Checkpoint(cid string) (CheckpointDef, bool) {
	cp, ok := c.checkpoints[cid]
	return cp, ok
}

// anchor builds a geometry anchor.
func anchor(aid string, x, y int64) geometry.Anchor {
	return geometry.Anchor{ID: aid, X: x, Y: y}
}

// testMaps returns the two-map fake world: map.test.alpha (a town with a
// guide NPC + checkpoint) and map.test.beta (a field map) linked by a
// bidirectional portal pair.
func testMaps() []MapRecord {
	alpha := MapRecord{
		MapID:      "map.test.alpha",
		Kind:       MapTown,
		Region:     "test",
		BoundsMaxX: 10000, BoundsMaxY: 10000,
		EntrySpawn: "spawn.entry.test.alpha",
		Anchors: []geometry.Anchor{
			anchor("spawn.entry.test.alpha", 1000, 1000),
			anchor("npc.test.alpha.guide", 1100, 1000),
			anchor("npc.test.alpha.far", 9000, 9000),
			anchor("checkpoint.test.alpha", 2000, 2000),
			anchor("portal.test.alpha.to.test.beta", 3000, 3000),
		},
		Checkpoints: []CheckpointDef{
			{CheckpointID: "checkpoint.test.alpha", MapID: "map.test.alpha",
				AnchorID: "checkpoint.test.alpha"},
		},
		Portals: []PortalDef{
			{PortalID: "portal.test.alpha.to.test.beta", SourceMap: "map.test.alpha",
				DestMap: "map.test.beta", DestSpawn: "spawn.entry.test.beta",
				AnchorID: "portal.test.alpha.to.test.beta"},
		},
		Npcs: []NpcDef{
			{NpcID: "npc.test.alpha.guide", MapID: "map.test.alpha",
				AnchorID: "npc.test.alpha.guide",
				Services: []string{ServiceSetCheckpoint, "travel"}},
			{NpcID: "npc.test.alpha.far", MapID: "map.test.alpha",
				AnchorID: "npc.test.alpha.far",
				Services: []string{ServiceSetCheckpoint}},
		},
	}
	beta := MapRecord{
		MapID:      "map.test.beta",
		Kind:       MapField,
		Region:     "test",
		BoundsMaxX: 20000, BoundsMaxY: 15000,
		EntrySpawn: "spawn.entry.test.beta",
		Anchors: []geometry.Anchor{
			anchor("spawn.entry.test.beta", 500, 500),
			anchor("checkpoint.test.beta", 600, 600),
			anchor("portal.test.beta.to.test.alpha", 700, 700),
		},
		Checkpoints: []CheckpointDef{
			{CheckpointID: "checkpoint.test.beta", MapID: "map.test.beta",
				AnchorID: "checkpoint.test.beta"},
		},
		Portals: []PortalDef{
			{PortalID: "portal.test.beta.to.test.alpha", SourceMap: "map.test.beta",
				DestMap: "map.test.alpha", DestSpawn: "spawn.entry.test.alpha",
				AnchorID: "portal.test.beta.to.test.alpha"},
		},
	}
	return []MapRecord{alpha, beta}
}

// testEnv wires a Runtime with test doubles.
type testEnv struct {
	w       *Runtime
	clocks  *testClocks
	out     *capOutbound
	emit    *capEmit
	loader  *fakeLoader
	catalog *fakeCatalog
}

func newTestEnv(t *testing.T, maps []MapRecord) *testEnv {
	t.Helper()
	clocks := newTestClocks()
	out := &capOutbound{}
	em := &capEmit{}
	ld := &fakeLoader{
		cpID:    "checkpoint.test.alpha",
		cpMap:   "map.test.alpha",
		cpAnchr: "checkpoint.test.alpha",
	}
	cat := newFakeCatalog(maps)
	w := NewRuntime(Config{
		Maps:            cat,
		Loader:          ld,
		Durable:         em,
		Results:         noopResults{},
		Outbound:        out,
		Now:             clocks.Now,
		SimNow:          clocks.SimNow,
		SimSleep:        clocks.Sleep,
		ContentRevision: "0101010101010101010101010101010101010101010101010101010101010101",
		SyncStarts:      true,
		ManualTick:      true,
	})
	w.dispatcher.RegisterService(ServiceSetCheckpoint)
	w.dispatcher.RegisterService("travel")
	w.Start(context.Background())
	t.Cleanup(w.Stop)
	return &testEnv{w: w, clocks: clocks, out: out, emit: em, loader: ld, catalog: cat}
}

// waitFor polls a condition until it holds (partition goroutines tick as
// fast as the injected sim clock advances).
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// charID builds a deterministic character id from an int seed.
func charID(n int) id.UUID {
	var u id.UUID
	u[0] = byte(n >> 8)
	u[1] = byte(n)
	u[15] = 0x5A
	return u
}

// admit places and admits a character at a map's entry spawn and waits for
// director membership. Vitals default to 100/100 hp, 50/50 mp.
func (e *testEnv) admit(t *testing.T, n int, mapID, anchorID string) (id.UUID, PlacementResult) {
	t.Helper()
	cid := charID(n)
	res, pw, err := e.w.PlaceAttach(cid, mapID, anchorID, 6, nil)
	if err != nil {
		t.Fatalf("PlaceAttach(%s): %v", mapID, err)
	}
	if pw != nil {
		t.Fatalf("PlaceAttach(%s): unexpected pending", mapID)
	}
	e.w.AdmitCharacter(cid, res, anchorID, Vitals{MaxHP: 100, HP: 100, MaxMP: 50, MP: 50})
	waitFor(t, "membership", func() bool {
		_, _, ok := e.w.director.Member(cid)
		return ok
	})
	return cid, res
}

// memberHost returns the running host of the character's channel.
func (e *testEnv) memberHost(t *testing.T, cid id.UUID) *ChannelHost {
	t.Helper()
	mapID, ch, ok := e.w.director.Member(cid)
	if !ok {
		t.Fatalf("character has no membership")
	}
	h, ok := e.w.hostFor(mapID, ch)
	if !ok {
		t.Fatalf("no running host for %s ch%d", mapID, ch)
	}
	return h
}

// consultSync posts a consult and waits for its single-shot reply.
func (e *testEnv) consultSync(t *testing.T, c Consult) ConsultReply {
	t.Helper()
	reply := make(chan ConsultReply, 1)
	c.Reply = reply
	if err := e.w.Consult(c); err != nil {
		t.Fatalf("Consult: %v", err)
	}
	select {
	case r := <-reply:
		return r
	case <-time.After(10 * time.Second):
		t.Fatalf("consult reply timeout")
		return ConsultReply{}
	}
}

// killMember marks the member dead inside the tick pipeline — a mailbox
// Fn entry, so the mutation runs on the partition goroutine (race-free).
func (e *testEnv) killMember(t *testing.T, cid id.UUID) {
	t.Helper()
	h := e.memberHost(t, cid)
	killed := make(chan struct{}, 1)
	if err := h.mb.Post(Entry{Fn: func(p *runtime.Partition, tc *runtime.TickContext) {
		if pl, ent, ok := h.playerEntity(cid, p); ok {
			ent.Dead = true
			ent.Snap.HP = 0
			pl.RespawnAtTick = 0 // delay already elapsed
		}
		close(killed)
	}}); err != nil {
		t.Fatalf("post kill: %v", err)
	}
	select {
	case <-killed:
	case <-time.After(10 * time.Second):
		t.Fatal("kill entry never drained")
	}
}

// npcEntityID waits for the npc roster and returns one npc's entity id.
func npcEntityID(t *testing.T, h *ChannelHost, npcID string) uint64 {
	t.Helper()
	waitFor(t, "npc spawned", func() bool { return len(h.npcs) > 0 })
	for eid, n := range h.npcs {
		if n.Def.NpcID == npcID {
			return eid
		}
	}
	t.Fatalf("npc %s not spawned", npcID)
	return 0
}

// --- interact / consult tests ------------------------------------------------

func TestInteractSetCheckpointService(t *testing.T) {
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	h := e.memberHost(t, cid)
	npcEID := npcEntityID(t, h, "npc.test.alpha.guide")

	// TALK opens the NPC session through the post-commit command drain.
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdInteract, CharacterID: cid,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_TALK),
		TargetID:     npcEID,
	}); err != nil {
		t.Fatalf("post talk: %v", err)
	}
	waitFor(t, "session open", func() bool {
		return h.sessions.Valid(npcEID, cid, h.part.TickN())
	})

	rep := e.consultSync(t, Consult{
		Kind:         ConsultNpcServiceValid,
		CharacterID:  cid,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE),
		TargetID:     npcEID,
		ServiceID:    ServiceSetCheckpoint,
	})
	if !rep.OK {
		t.Fatalf("NpcServiceValid rejected: %v", rep.Code)
	}
	if rep.CheckpointID != "checkpoint.test.alpha" || rep.MapID != "map.test.alpha" ||
		rep.AnchorID != "checkpoint.test.alpha" {
		t.Fatalf("set_checkpoint reply fields wrong: %+v", rep)
	}

	// The post-commit command touch keeps the session alive.
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdInteract, CharacterID: cid,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE),
		TargetID:     npcEID,
		ServiceID:    ServiceSetCheckpoint,
	}); err != nil {
		t.Fatalf("post npc_service: %v", err)
	}
}

func TestInteractUnregisteredServiceRejected(t *testing.T) {
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	h := e.memberHost(t, cid)
	npcEID := npcEntityID(t, h, "npc.test.alpha.guide")

	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdInteract, CharacterID: cid,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_TALK),
		TargetID:     npcEID,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "session open", func() bool {
		return h.sessions.Valid(npcEID, cid, h.part.TickN())
	})

	// A service absent from the npc's allowed set rejects TARGET_INVALID.
	rep := e.consultSync(t, Consult{
		Kind:         ConsultNpcServiceValid,
		CharacterID:  cid,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE),
		TargetID:     npcEID,
		ServiceID:    "service.not_registered",
	})
	if rep.OK || rep.Code != protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("unregistered service: want TARGET_INVALID, got %+v", rep)
	}
}

func TestInteractOutOfRangeAndCombatRejected(t *testing.T) {
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	h := e.memberHost(t, cid)
	farEID := npcEntityID(t, h, "npc.test.alpha.far")

	rep := e.consultSync(t, Consult{
		Kind:         ConsultNpcServiceValid,
		CharacterID:  cid,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE),
		TargetID:     farEID,
		ServiceID:    ServiceSetCheckpoint,
	})
	if rep.OK || rep.Code != protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE {
		t.Fatalf("out-of-range npc: want OUT_OF_RANGE, got %+v", rep)
	}

	// In-combat member: rejected IN_COMBAT.
	guideEID := npcEntityID(t, h, "npc.test.alpha.guide")
	done := make(chan struct{}, 1)
	if err := h.mb.Post(Entry{Fn: func(p *runtime.Partition, tc *runtime.TickContext) {
		if _, ent, ok := h.playerEntity(cid, p); ok {
			ent.InCombatWith = 1
		}
		close(done)
	}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("combat flag entry never drained")
	}
	rep = e.consultSync(t, Consult{
		Kind:         ConsultNpcServiceValid,
		CharacterID:  cid,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE),
		TargetID:     guideEID,
		ServiceID:    ServiceSetCheckpoint,
	})
	if rep.OK || rep.Code != protocolv1.ErrorCode_ERROR_CODE_IN_COMBAT {
		t.Fatalf("in-combat member: want IN_COMBAT, got %+v", rep)
	}
}

func TestConsultFifoObservesPriorCommands(t *testing.T) {
	// ADR-0083 FIFO: a consult drained after a command observes its
	// effects. Post TALK then immediately consult the same npc — the
	// reply must see the opened session.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	h := e.memberHost(t, cid)
	npcEID := npcEntityID(t, h, "npc.test.alpha.guide")

	reply := make(chan ConsultReply, 1)
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdInteract, CharacterID: cid,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_TALK),
		TargetID:     npcEID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.w.Consult(Consult{
		Kind:         ConsultNpcServiceValid,
		CharacterID:  cid,
		Reply:        reply,
		InteractKind: uint32(protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE),
		TargetID:     npcEID,
		ServiceID:    ServiceSetCheckpoint,
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case rep := <-reply:
		if !rep.OK {
			t.Fatalf("consult must observe the TALK-opened session, got %+v", rep)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("consult timeout")
	}
	// Single-shot: exactly one reply.
	select {
	case r := <-reply:
		t.Fatalf("second reply on single-shot channel: %+v", r)
	default:
	}
}

func TestConsultMemberlessFailsClosed(t *testing.T) {
	e := newTestEnv(t, testMaps())
	rep := e.consultSync(t, Consult{
		Kind:        ConsultPartitionTick,
		CharacterID: charID(999),
	})
	if rep.OK || rep.Code != protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("memberless consult: want INVALID_STATE, got %+v", rep)
	}
}

func TestConsultMailboxFullOverloads(t *testing.T) {
	// A consult that cannot enter the bounded mailbox fails closed
	// SERVER_OVERLOADED (the mailbox owns the cap; the producer replies
	// before dropping). Stop the partition loop first so the drain can't
	// race the fill.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	h := e.memberHost(t, cid)
	h.cancel()
	select {
	case <-h.done:
	case <-time.After(10 * time.Second):
		t.Fatal("partition loop did not stop")
	}
	for i := 0; i < MailboxCap; i++ {
		if err := h.mb.Post(Entry{Con: &Consult{Kind: ConsultPartitionTick,
			CharacterID: cid, Reply: make(chan ConsultReply, 1)}}); err != nil {
			t.Fatalf("post %d below cap: %v", i, err)
		}
	}
	reply := make(chan ConsultReply, 1)
	if err := e.w.Consult(Consult{Kind: ConsultPartitionTick,
		CharacterID: cid, Reply: reply}); err != nil {
		t.Fatal(err)
	}
	select {
	case rep := <-reply:
		if rep.OK || rep.Code != protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED {
			t.Fatalf("full mailbox: want SERVER_OVERLOADED, got %+v", rep)
		}
	case <-time.After(time.Second):
		t.Fatal("overload reply timeout")
	}
}

// --- portal / transfer tests --------------------------------------------------

func TestPortalTransition(t *testing.T) {
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	h := e.memberHost(t, cid)
	eID := h.players[cid].EntityID

	var op [16]byte
	op[0] = 0xAA
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdPortal, CharacterID: cid,
		OperationID: op, PortalID: "portal.test.alpha.to.test.beta",
	}); err != nil {
		t.Fatal(err)
	}
	// The 105 goes out with the destination + deterministic transfer id.
	var prep *protocolv1.S2CTransferPrepare
	waitFor(t, "105 emitted", func() bool {
		for _, m := range e.out.forChar(cid, MsgS2CTransferPrepare) {
			p, ok := m.Msg.(*protocolv1.S2CTransferPrepare)
			if !ok {
				continue
			}
			if p.GetMapId() == "map.test.beta" {
				prep = p
				return true
			}
		}
		return false
	})
	if prep.GetChannelIndex() != 1 || len(prep.GetTransferId()) != 16 {
		t.Fatalf("bad 105: %+v", prep)
	}
	if prep.GetContentRevision() != "0101010101010101010101010101010101010101010101010101010101010101" ||
		prep.GetReadyDeadlineMs() != uint32(TransferBudgetWorld.Milliseconds()) {
		t.Fatalf("105 fields: %+v", prep)
	}

	// The client's 106 completes the transfer: member lands on beta.
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdPresentationReady, CharacterID: cid,
		TransferID: [16]byte(prep.GetTransferId()),
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "member on beta", func() bool {
		m, _, ok := e.w.director.Member(cid)
		return ok && m == "map.test.beta"
	})
	// The source partition no longer owns the entity.
	if _, err := h.part.Entity(eID); err == nil {
		t.Fatal("source partition still holds the entity")
	}
}

func TestTransferTimeoutFallback(t *testing.T) {
	// An expired transfer recovers to the character's checkpoint under
	// the forced order (source recovery, maps_zones.md).
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")

	var op [16]byte
	op[0] = 0xBB
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdPortal, CharacterID: cid,
		OperationID: op, PortalID: "portal.test.alpha.to.test.beta",
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "105 emitted", func() bool {
		return len(e.out.forChar(cid, MsgS2CTransferPrepare)) > 0
	})
	// The checkpoint read resolves the recovery destination.
	e.loader.cpMap = "map.test.beta"
	e.loader.cpID = "checkpoint.test.beta"
	e.loader.cpAnchr = "checkpoint.test.beta"

	// Advance wall time past the 30s world transfer budget; TickOnce
	// runs the expired-transfer recovery.
	e.clocks.Advance(TransferBudgetWorld + time.Second)
	e.w.TickOnce()

	// Recovery emits a FORCED 105 re-arming the client machine and
	// re-places the member at the checkpoint channel.
	waitFor(t, "forced 105 + re-placement", func() bool {
		for _, m := range e.out.forChar(cid, MsgS2CTransferPrepare) {
			p := m.Msg.(*protocolv1.S2CTransferPrepare)
			if p.GetReason() == protocolv1.TransferReason_TRANSFER_REASON_FORCED &&
				p.GetMapId() == "map.test.beta" {
				m2, _, ok := e.w.director.Member(cid)
				return ok && m2 == "map.test.beta"
			}
		}
		return false
	})
}

func TestCheckpointTransferHandshake(t *testing.T) {
	// Respawn is a checkpoint-mediated transfer: the dead member's 208
	// runs forced placement at the durable checkpoint, the destination
	// emits the committed 207 and queues the sim.checkpoint write.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	e.killMember(t, cid)

	var op [16]byte
	op[0] = 0x77
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdRespawn, CharacterID: cid, OperationID: op,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "207 emitted", func() bool {
		return len(e.out.forChar(cid, MsgS2CRespawn)) > 0
	})
	resp := e.out.forChar(cid, MsgS2CRespawn)[0].Msg.(*protocolv1.S2CRespawn)
	if resp.GetCheckpointId() != "checkpoint.test.alpha" {
		t.Fatalf("207 checkpoint: %q", resp.GetCheckpointId())
	}
	// Vitals: 40% of max floored, hp min 1 — admit set max 100/50 → 40/20.
	if resp.GetHpAfter() != 40 || resp.GetMpAfter() != 20 {
		t.Fatalf("207 vitals: %+v", resp)
	}
	if resp.GetEntityId() == 0 || resp.GetMapId() != "map.test.alpha" {
		t.Fatalf("207 fields: %+v", resp)
	}
	// The sim.checkpoint durable write is queued with the client op id.
	waitFor(t, "checkpoint write", func() bool {
		for _, c := range e.emit.all() {
			if c.Kind == runtime.CmdCheckpoint && c.OperationID == id.UUID(op) &&
				c.Family == FamilySimCheckpoint {
				return true
			}
		}
		return false
	})
	// The ledger payload is registered for the emit adapter.
	cp, ok := e.w.CheckpointPayload(op)
	if !ok || cp.CheckpointID != "checkpoint.test.alpha" ||
		cp.SourceMapID != "map.test.alpha" || cp.MembershipState != "respawn" {
		t.Fatalf("checkpoint ledger: %v %+v", ok, cp)
	}

	// Retry with the same operation_id re-sends the committed 207
	// verbatim — no second durable write.
	before := len(e.emit.all())
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdRespawn, CharacterID: cid, OperationID: op,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "committed 207 re-sent", func() bool {
		return len(e.out.forChar(cid, MsgS2CRespawn)) >= 2
	})
	if len(e.emit.all()) != before {
		t.Fatal("retry produced a second durable write")
	}
}

func TestRespawnNotDeadRejected(t *testing.T) {
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	var op [16]byte
	op[0] = 0x66
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdRespawn, CharacterID: cid, OperationID: op,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "204 emitted", func() bool {
		for _, m := range e.out.forChar(cid, MsgS2CActionRejected) {
			r := m.Msg.(*protocolv1.S2CActionRejected)
			if r.GetRequestMessageId() == 208 {
				return true
			}
		}
		return false
	})
}

func TestRespawnDelayGate(t *testing.T) {
	// Dead but delay not elapsed → 204, not a respawn.
	e := newTestEnv(t, testMaps())
	cid, _ := e.admit(t, 1, "map.test.alpha", "spawn.entry.test.alpha")
	h := e.memberHost(t, cid)
	done := make(chan struct{}, 1)
	if err := h.mb.Post(Entry{Fn: func(p *runtime.Partition, tc *runtime.TickContext) {
		if pl, ent, ok := h.playerEntity(cid, p); ok {
			ent.Dead = true
			ent.Snap.HP = 0
			pl.RespawnAtTick = tc.Tick + RespawnDelayTick + 100
		}
		close(done)
	}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("dead-mark entry never drained")
	}
	var op [16]byte
	op[0] = 0x55
	if err := e.w.PostCommand(cid, &Command{
		Kind: CmdRespawn, CharacterID: cid, OperationID: op,
	}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "204 delay reject", func() bool {
		for _, m := range e.out.forChar(cid, MsgS2CActionRejected) {
			r := m.Msg.(*protocolv1.S2CActionRejected)
			if r.GetRequestMessageId() == 208 {
				return true
			}
		}
		return false
	})
	if n := len(e.out.forChar(cid, MsgS2CRespawn)); n > 0 {
		t.Fatal("respawn emitted before the delay elapsed")
	}
}

// --- real-catalog test ---------------------------------------------------------

// realPayload compiles the canonical payload once (same pipeline the
// parity suite runs).
var realPayload *parity.Payload

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
	tmp, err := os.MkdirTemp("", "imp018")
	if err != nil {
		panic(err)
	}
	pay := filepath.Join(tmp, "payload.json")
	run := exec.Command("go", "run", "./cmd/compiler", "-payload", pay)
	run.Dir = filepath.Join(root, "server")
	if out, err := run.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("compiler payload: %v\n%s", err, out))
	}
	data, err := os.ReadFile(pay)
	if err != nil {
		panic(err)
	}
	p, err := parity.LoadPayload(data)
	if err != nil {
		panic(err)
	}
	realPayload = p
	os.Exit(m.Run())
}

func TestTwentyFourMapBoundsAndDistinctTopologies(t *testing.T) {
	cat, err := CatalogFromPayload(realPayload)
	if err != nil {
		t.Fatalf("catalog build: %v", err)
	}
	maps := cat.Maps()
	if len(maps) != 24 {
		t.Fatalf("normal world: want 24 maps, got %d", len(maps))
	}
	profiles := map[string]string{}
	topos := map[string]string{}
	portals := 0
	checkpoints := 0
	for _, m := range maps {
		if m.BoundsMaxX <= 0 || m.BoundsMaxY <= 0 {
			t.Fatalf("%s: non-positive bounds %dx%d", m.MapID, m.BoundsMaxX, m.BoundsMaxY)
		}
		if prev, dup := profiles[m.LayoutProfile]; dup {
			t.Fatalf("layout_profile %q shared by %s and %s", m.LayoutProfile, prev, m.MapID)
		}
		profiles[m.LayoutProfile] = m.MapID
		if prev, dup := topos[m.RequiredTopology]; dup {
			t.Fatalf("required_topology %q shared by %s and %s",
				m.RequiredTopology, prev, m.MapID)
		}
		topos[m.RequiredTopology] = m.MapID
		if m.Region == "" {
			t.Fatalf("%s: no region key", m.MapID)
		}
		if _, ok := m.Anchor(m.EntrySpawn); !ok {
			t.Fatalf("%s: entry_spawn %q not in anchors", m.MapID, m.EntrySpawn)
		}
		portals += len(m.Portals)
		checkpoints += len(m.Checkpoints)
		for _, p := range m.Portals {
			dst, ok := cat.Lookup(p.DestMap)
			if !ok {
				// Portals into non-world spaces (dungeons) are authored
				// but unreachable by this runtime — keep the row, skip
				// the destination-anchor pin.
				continue
			}
			if _, ok := dst.Anchor(p.DestSpawn); !ok {
				t.Fatalf("portal %s dest spawn %q not anchored on %s",
					p.PortalID, p.DestSpawn, p.DestMap)
			}
		}
	}
	// The route graph is anti-softlock: every map reachable from every
	// other through directed portals (each catalog edge emits both ways).
	reach := map[string]bool{"map.lang_da.dinh_lang": true}
	for grown := true; grown; {
		grown = false
		for _, m := range maps {
			if !reach[m.MapID] {
				continue
			}
			for _, p := range m.Portals {
				if !reach[p.DestMap] {
					reach[p.DestMap] = true
					grown = true
				}
			}
		}
	}
	for _, m := range maps {
		if !reach[m.MapID] {
			t.Fatalf("%s unreachable from the launch hub", m.MapID)
		}
	}
	if checkpoints != 6 {
		t.Fatalf("want 6 world checkpoints, got %d", checkpoints)
	}
	// Every NPC on a world map anchors and carries its service route set.
	npcs := 0
	for _, m := range maps {
		for _, n := range m.Npcs {
			npcs++
			if _, ok := m.Anchor(n.AnchorID); !ok {
				t.Fatalf("npc %s anchor %q missing on %s", n.NpcID, n.AnchorID, m.MapID)
			}
		}
	}
	if npcs == 0 {
		t.Fatal("no world NPCs in the catalog")
	}
}
