package world

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/db"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/durable/schema"
	durableworld "thinhthan/internal/durable/world"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/geometry"
	simworld "thinhthan/internal/sim/world"
	"thinhthan/internal/testing/pgtest"
)

// fakeCatalog implements simworld.MapCatalog over a hand-built map set.
type fakeCatalog struct {
	recs        map[string]simworld.MapRecord
	order       []string
	checkpoints map[string]simworld.CheckpointDef
	portals     map[string]simworld.PortalDef
}

func (c *fakeCatalog) Lookup(mapID string) (simworld.MapRecord, bool) {
	r, ok := c.recs[mapID]
	return r, ok
}

func (c *fakeCatalog) Maps() []simworld.MapRecord {
	out := make([]simworld.MapRecord, 0, len(c.order))
	for _, m := range c.order {
		out = append(out, c.recs[m])
	}
	return out
}

func (c *fakeCatalog) Checkpoint(id string) (simworld.CheckpointDef, bool) {
	cp, ok := c.checkpoints[id]
	return cp, ok
}

func (c *fakeCatalog) Portal(id string) (simworld.PortalDef, bool) {
	p, ok := c.portals[id]
	return p, ok
}

// testCatalog is a two-map world: alpha hosts the member + NPC, beta is
// the portal/channel destination.
func testCatalog() *fakeCatalog {
	alpha := simworld.MapRecord{
		MapID:            "map.alpha",
		Kind:             simworld.MapField,
		Region:           "region.alpha",
		BoundsMaxX:       8000,
		BoundsMaxY:       8000,
		LayoutProfile:    "layout.alpha",
		RequiredTopology: "topo.alpha",
		EntrySpawn:       "anchor.alpha.spawn",
		Anchors: []geometry.Anchor{
			{ID: "anchor.alpha.spawn", X: 1000, Y: 1000},
			{ID: "anchor.alpha.npc", X: 1500, Y: 1000},
			{ID: "anchor.alpha.portal", X: 2000, Y: 1000},
		},
		Checkpoints: []simworld.CheckpointDef{
			{CheckpointID: "checkpoint.alpha.one", MapID: "map.alpha", AnchorID: "anchor.alpha.spawn"},
		},
		Portals: []simworld.PortalDef{
			{PortalID: "portal.alpha.beta", SourceMap: "map.alpha", DestMap: "map.beta",
				DestSpawn: "anchor.beta.spawn", AnchorID: "anchor.alpha.portal"},
		},
		Npcs: []simworld.NpcDef{
			{NpcID: "npc.alpha.guide", MapID: "map.alpha", AnchorID: "anchor.alpha.npc",
				Services: []string{"set_checkpoint"}},
		},
	}
	beta := simworld.MapRecord{
		MapID:            "map.beta",
		Kind:             simworld.MapField,
		Region:           "region.beta",
		BoundsMaxX:       4000,
		BoundsMaxY:       4000,
		LayoutProfile:    "layout.beta",
		RequiredTopology: "topo.beta",
		EntrySpawn:       "anchor.beta.spawn",
		Anchors: []geometry.Anchor{
			{ID: "anchor.beta.spawn", X: 500, Y: 500},
		},
	}
	return &fakeCatalog{
		recs:  map[string]simworld.MapRecord{"map.alpha": alpha, "map.beta": beta},
		order: []string{"map.alpha", "map.beta"},
		checkpoints: map[string]simworld.CheckpointDef{
			"checkpoint.alpha.one": alpha.Checkpoints[0],
		},
		portals: map[string]simworld.PortalDef{
			"portal.alpha.beta": alpha.Portals[0],
		},
	}
}

// fakeLoader implements simworld.Loader.
type fakeLoader struct {
	checkpointID, mapID, anchorID string
	err                           error
}

func (l *fakeLoader) LoadConsequences(context.Context, string, uint64) ([]simworld.ConsequenceRow, error) {
	return nil, nil
}

func (l *fakeLoader) LoadCheckpoint(context.Context, id.UUID) (string, string, string, error) {
	return l.checkpointID, l.mapID, l.anchorID, l.err
}

// capOutbound records every runtime.Outbound the world runtime emits.
type capOutbound struct {
	mu   sync.Mutex
	rows []runtime.Outbound
}

func (o *capOutbound) Enqueue(m runtime.Outbound) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.rows = append(o.rows, m)
	return nil
}

func (o *capOutbound) forChar(charID id.UUID, msgID uint32) []runtime.Outbound {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []runtime.Outbound
	for _, m := range o.rows {
		if m.To == simworld.SessionTarget(charID) && m.MessageID == msgID {
			out = append(out, m)
		}
	}
	return out
}

// testClocks gives the world runtime an advanceable sim clock; SimSleep
// yields briefly so partition goroutines progress deterministically.
type testClocks struct {
	mu  sync.Mutex
	now time.Time
	sim time.Duration
}

func newTestClocks() *testClocks {
	return &testClocks{now: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}
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

func (c *testClocks) SimSleep(d time.Duration) {
	c.mu.Lock()
	c.sim += d
	c.mu.Unlock()
	time.Sleep(50 * time.Microsecond)
}

func (c *testClocks) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.sim += d
}

// env wires the real ADR-0083 stack for edge/world tests: pgtest pool,
// durable queue with the world executor family-mux, live simworld
// runtime, service + router.
type env struct {
	pool     *pgxpool.Pool
	store    *durableworld.Store
	q        *queue.Queue
	w        *simworld.Runtime
	svc      *Service
	rt       *router.Registry
	out      *capOutbound
	clocks   *testClocks
	consults *fakeConsults
}

func newEnv(t *testing.T, consults Consults) *env {
	t.Helper()
	ctx := context.Background()
	dsn := pgtest.FreshDB(t)
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir("../../../.."), "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	store := durableworld.NewStore(pool)
	q := queue.New(64, idempotency.NewStore(pool), queue.Deps{
		Pool:       pool,
		Now:        func() time.Time { return time.Now().UTC() },
		BackoffMin: time.Millisecond,
		BackoffMax: 8 * time.Millisecond,
	})
	execs := durableworld.Executors(durableworld.Deps{Store: store})
	q.RegisterExecutor(queue.ProducerClient, func(ctx context.Context, tx pgx.Tx,
		rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
		ex, ok := execs[rec.GetOperationFamily()]
		if !ok {
			return idempotency.Outcome{}, fmt.Errorf("no world executor for family %q", rec.GetOperationFamily())
		}
		return ex(ctx, tx, rec)
	})
	q.RegisterExecutor(queue.ProducerCheckpoint,
		durableworld.Deps{Store: store}.CheckpointExecutors()[durableworld.FamilySimCheckpoint])
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = q.Shutdown(c)
	})

	clocks := newTestClocks()
	out := &capOutbound{}
	w := simworld.NewRuntime(simworld.Config{
		Maps:            testCatalog(),
		Loader:          &fakeLoader{checkpointID: "checkpoint.alpha.one", mapID: "map.alpha", anchorID: "anchor.alpha.spawn"},
		Outbound:        out,
		Now:             clocks.Now,
		SimNow:          clocks.SimNow,
		SimSleep:        clocks.SimSleep,
		ContentRevision: strings.Repeat("b", 64),
		Seed:            7,
		SyncStarts:      true,
		ManualTick:      true,
	})
	w.Start(context.Background())
	t.Cleanup(w.Stop)

	svc := New(w, q, consults, WithClock(clocks.Now))
	rt := router.New()
	if err := svc.Register(rt); err != nil {
		t.Fatalf("register: %v", err)
	}
	return &env{pool: pool, store: store, q: q, w: w, svc: svc, rt: rt,
		out: out, clocks: clocks}
}

// seedCharacter inserts one live character row (account + character).
func (e *env) seedCharacter(t *testing.T) (id.UUID, id.UUID) {
	t.Helper()
	ctx := context.Background()
	acct := id.NewV7(time.Now().UTC())
	if _, err := e.pool.Exec(ctx, `INSERT INTO accounts (account_id) VALUES ($1)`, acct); err != nil {
		t.Fatalf("account: %v", err)
	}
	charID := id.NewV7(time.Now().UTC())
	name := "w" + strings.ReplaceAll(charID.String(), "-", "")[:16]
	if _, err := e.pool.Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id)
		 VALUES ($1,$2,$3,$3,'class.kim')`, charID, acct, name); err != nil {
		t.Fatalf("character: %v", err)
	}
	return acct, charID
}

// admit places the character on map.alpha channel 1 and posts the member
// admission; the host drains it on the next partition step.
func (e *env) admit(t *testing.T, charID id.UUID) {
	t.Helper()
	res, pw, err := e.w.PlaceAttach(charID, "map.alpha", "anchor.alpha.spawn", 7, nil)
	if err != nil {
		t.Fatalf("place attach: %v", err)
	}
	if pw != nil {
		t.Fatalf("attach went pending on an empty channel")
	}
	e.w.AdmitCharacter(charID, res, "anchor.alpha.spawn",
		simworld.Vitals{MaxHP: 100, HP: 100, MaxMP: 50, MP: 50})
	waitFor(t, "member admission", func() bool {
		m, ch, ok := e.w.Director().Member(charID)
		return ok && m == "map.alpha" && ch == 1
	})
}

// view builds the router.View the session adapter passes (Conn nil —
// DeliveryControl sends are no-ops; assertions read the recorded durable
// outcome and the world-side effects instead).
func view(acct id.UUID, charID *id.UUID) router.View {
	return router.View{AccountID: acct, SessionEpoch: 9, OwnershipEpoch: 1, CharacterID: charID}
}

// dispatch sends one inbound payload through the registered route table.
func (e *env) dispatch(ctx context.Context, v router.View, msgID uint32, payload proto.Message) error {
	return e.rt.Dispatch(ctx, v, listener.Inbound{MessageID: msgID, Payload: payload})
}

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
