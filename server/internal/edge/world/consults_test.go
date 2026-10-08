package world

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	simworld "thinhthan/internal/sim/world"
)

// fakeConsults is a scriptable Consults stub: every call is recorded so
// handler tests can assert the ADR-0083 consult→submit→post ordering.
type fakeConsults struct {
	mu    sync.Mutex
	calls []string

	tick    uint64
	tickOK  bool
	replies map[string]simworld.ConsultReply
	errs    map[string]error
}

func newFakeConsults() *fakeConsults {
	return &fakeConsults{
		tickOK:  true,
		replies: make(map[string]simworld.ConsultReply),
		errs:    make(map[string]error),
	}
}

func (f *fakeConsults) answer(key string) (simworld.ConsultReply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, key)
	if err, ok := f.errs[key]; ok {
		return simworld.ConsultReply{}, err
	}
	rep, ok := f.replies[key]
	if !ok {
		rep = simworld.ConsultReply{OK: true}
	}
	return rep, nil
}

func (f *fakeConsults) PartitionTick(context.Context, id.UUID) (uint64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "PartitionTick")
	if err, ok := f.errs["PartitionTick"]; ok {
		return 0, false, err
	}
	return f.tick, f.tickOK, nil
}

func (f *fakeConsults) NpcServiceValid(_ context.Context, _ id.UUID, npcID, serviceID string) (simworld.ConsultReply, error) {
	return f.answer("NpcServiceValid:" + npcID + ":" + serviceID)
}

func (f *fakeConsults) TalkAdmission(_ context.Context, _ id.UUID, npcID string) (simworld.ConsultReply, error) {
	return f.answer("TalkAdmission:" + npcID)
}

func (f *fakeConsults) PortalAdmission(_ context.Context, _ id.UUID, portalID string) (simworld.ConsultReply, error) {
	return f.answer("PortalAdmission:" + portalID)
}

func (f *fakeConsults) ChannelAdmission(_ context.Context, _ id.UUID, ch uint32) (simworld.ConsultReply, error) {
	return f.answer("ChannelAdmission")
}

func (f *fakeConsults) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// TestConsultReplySingleShot: one mailbox consult yields exactly one
// reply — a second reply must not arrive (single-shot channel per
// ADR-0083).
func TestConsultReplySingleShot(t *testing.T) {
	e := newEnv(t, nil)
	_, charID := e.seedCharacter(t)
	e.admit(t, charID)

	rc := NewConsults(e.w).(*runtimeConsults)
	tick, ok, err := rc.PartitionTick(context.Background(), charID)
	if err != nil {
		t.Fatalf("partition tick consult: %v", err)
	}
	if !ok {
		// membered → answered by the owning partition with the live tick.
		t.Fatalf("membered consult replied !ok")
	}
	// Second consult to the same member gets its own fresh answer —
	// nothing replays the first.
	tick2, ok2, err := rc.PartitionTick(context.Background(), charID)
	if err != nil || !ok2 || tick2 < tick {
		t.Fatalf("second consult tick=%d ok=%v err=%v, want >=%d", tick2, ok2, err, tick)
	}
}

// TestConsultMemberlessFailsClosed: a consult for a character no
// partition owns answers INVALID_STATE immediately — never fabricated.
func TestConsultMemberlessFailsClosed(t *testing.T) {
	e := newEnv(t, nil)
	rc := NewConsults(e.w)
	tick, ok, err := rc.PartitionTick(context.Background(), id.NewV4())
	if err != nil {
		t.Fatalf("memberless consult: %v", err)
	}
	if ok || tick != 0 {
		t.Fatalf("memberless consult tick=%d ok=%v, want 0/false", tick, ok)
	}
}

// TestConsultBoundedAwaitTimeout: a consult posted into a mailbox that
// never drains fails the bounded await with ErrConsultTimeout.
func TestConsultBoundedAwaitTimeout(t *testing.T) {
	// SimNow frozen at zero + SimSleep parked on a gate: the partition
	// loop accepts mailbox entries but never runs a drain.
	gate := make(chan struct{})
	clocks := newTestClocks()
	w := simworld.NewRuntime(simworld.Config{
		Maps:            testCatalog(),
		Loader:          &fakeLoader{checkpointID: "checkpoint.alpha.one", mapID: "map.alpha", anchorID: "anchor.alpha.spawn"},
		Now:             clocks.Now,
		SimNow:          clocks.SimNow,
		SimSleep:        func(time.Duration) { <-gate },
		ContentRevision: "c" + strings.Repeat("c", 63),
		Seed:            7,
		SyncStarts:      true,
		ManualTick:      true,
	})
	w.Start(context.Background())
	t.Cleanup(func() {
		close(gate)
		w.Stop()
	})

	charID := id.NewV4()
	res, pw, err := w.PlaceAttach(charID, "map.alpha", "anchor.alpha.spawn", 7, nil)
	if err != nil || pw != nil {
		t.Fatalf("place attach res=%v pw=%v err=%v", res, pw, err)
	}
	w.AdmitCharacter(charID, res, "anchor.alpha.spawn",
		simworld.Vitals{MaxHP: 100, HP: 100, MaxMP: 50, MP: 50})
	// Membership normally lands inside the drain; the parked loop cannot
	// run it, so mark the member at the director for consult routing.
	w.Director().Admitted(charID, res.MapID, res.ChannelIndex)

	rc := NewConsults(w).(*runtimeConsults)
	rc.timeout = 25 * time.Millisecond
	_, _, err = rc.PartitionTick(context.Background(), charID)
	if err != ErrConsultTimeout {
		t.Fatalf("undrained consult err = %v, want ErrConsultTimeout", err)
	}
}

// TestConsultsFailClosedUnbound: no world runtime → ErrConsultUnbound,
// which maps to the session/state-class rejection (never fabricated).
func TestConsultsFailClosedUnbound(t *testing.T) {
	rc := NewConsults(nil).(*runtimeConsults)
	_, _, err := rc.PartitionTick(context.Background(), id.NewV4())
	if err != ErrConsultUnbound {
		t.Fatalf("unbound consult err = %v, want ErrConsultUnbound", err)
	}
	if got := consultError(err); got != protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("unbound consult maps to %v, want INVALID_STATE", got)
	}
}
