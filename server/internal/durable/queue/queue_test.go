package queue_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/testing/pgtest"
)

var (
	sharedDSN string
	setupErr  error
)

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd + "/../../../.."
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	srv, err := pgtest.Ensure(ctx)
	switch {
	case errors.Is(err, pgtest.ErrUnavailable):
		fmt.Fprintf(os.Stderr, "pgtest ensure: %v\n", err)
		os.Exit(m.Run())
	case err != nil:
		setupErr = err
		os.Exit(m.Run())
	}
	name := fmt.Sprintf("queue_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
	dsn, cleanup, err := srv.NewDB(ctx, name)
	if err != nil {
		setupErr = err
		os.Exit(m.Run())
	}
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(repoRoot()), "up"); err != nil {
		setupErr = err
	} else {
		sharedDSN = dsn
	}
	code := m.Run()
	cleanup()
	srv.Close()
	os.Exit(code)
}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if sharedDSN == "" {
		if setupErr != nil {
			t.Fatalf("postgres provisioning failed: %v", setupErr)
		}
		t.Skip("DEFERRED(local-missing): no postgres")
	}
	pool, err := pgxpool.New(context.Background(), sharedDSN)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func fp(payload string) [32]byte { return sha256.Sum256([]byte(payload)) }

func charOwner() idempotency.Owner {
	return idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: id.NewV4()}
}

func ownerKindField(k idempotency.OwnerKind) journalv1.JournalOwnerKind {
	switch k {
	case idempotency.OwnerAccount:
		return journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_ACCOUNT
	case idempotency.OwnerCharacter:
		return journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER
	case idempotency.OwnerGuild:
		return journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_GUILD
	default:
		return journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD
	}
}

// shell builds a record without its command variant set.
func shell(family string, owner idempotency.Owner, payload string) *journalv1.DurableCommandRecord {
	opID := id.NewV7(time.Now())
	fpr := fp(payload + opID.String())
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    family,
		OwnerKind:          ownerKindField(owner.Kind),
		OwnerId:            owner.ID[:],
		OperationId:        opID[:],
		EnqueuedAtMs:       time.Now().UnixMilli(),
		RequestFingerprint: fpr[:],
	}
}

func clientRec(owner idempotency.Owner, family, payload string) *journalv1.DurableCommandRecord {
	r := shell(family, owner, payload)
	r.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT
	r.Command = &journalv1.DurableCommandRecord_Client{
		Client: &journalv1.JournalClientCommand{AccountId: owner.ID[:]},
	}
	return r
}

func worldRec(payload string) *journalv1.DurableCommandRecord {
	r := shell("world.consequence",
		idempotency.Owner{Kind: idempotency.OwnerWorld, ID: id.NewV4()}, payload)
	r.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_WORLD_CONSEQUENCE
	r.Command = &journalv1.DurableCommandRecord_WorldConsequence{
		WorldConsequence: &journalv1.JournalWorldConsequence{MapId: "m1"},
	}
	return r
}

func jobRec(family string, job *journalv1.JournalJob) *journalv1.DurableCommandRecord {
	owner := idempotency.Owner{Kind: idempotency.OwnerWorld, ID: id.NewV4()}
	if family == "job.payment" {
		owner.Kind = idempotency.OwnerAccount
	} else if family == "job.quest" || family == "job.compensation" {
		owner.Kind = idempotency.OwnerCharacter
	} else if family == "job.guild" {
		owner.Kind = idempotency.OwnerGuild
	}
	r := shell(family, owner, job.GetJobKey())
	r.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_JOB
	r.Command = &journalv1.DurableCommandRecord_Job{Job: job}
	return r
}

func maintJobRec(jobKey string) *journalv1.DurableCommandRecord {
	return jobRec("job.maintenance", &journalv1.JournalJob{
		Kind: "RETENTION_PURGE", JobKey: jobKey,
		Target: &journalv1.JournalJob_Maintenance{
			Maintenance: &journalv1.JournalMaintenanceJob{RowFamily: "durable_command_receipts"},
		}})
}

func erasureResumeRec(subject id.UUID) *journalv1.DurableCommandRecord {
	opID := id.NewV7(time.Now())
	h := fp("acct:" + subject.String())
	return jobRec("job.erasure", &journalv1.JournalJob{
		Kind: "ERASURE_RESUME", JobKey: "ERASURE:" + subject.String() + ":resume",
		Target: &journalv1.JournalJob_Erasure{
			Erasure: &journalv1.JournalErasureJob{
				OperationId:   opID[:],
				AccountIdHash: h[:],
				PreparedAtMs:  time.Now().UnixMilli(),
			},
		}})
}

func chatRec(owner idempotency.Owner) *journalv1.DurableCommandRecord {
	r := shell("chat.log", owner, "chat")
	r.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHAT_LOG
	r.Command = &journalv1.DurableCommandRecord_ChatLog{
		ChatLog: &journalv1.JournalChatLog{Channel: "world", Content: "hi"},
	}
	return r
}

func opOf(t *testing.T, rec *journalv1.DurableCommandRecord) id.UUID {
	t.Helper()
	var op id.UUID
	copy(op[:], rec.GetOperationId())
	return op
}

func ownerIDOf(rec *journalv1.DurableCommandRecord) id.UUID {
	var o id.UUID
	copy(o[:], rec.GetOwnerId())
	return o
}

func liveOwner(rec *journalv1.DurableCommandRecord) idempotency.Owner {
	var kind idempotency.OwnerKind
	switch rec.GetOwnerKind() {
	case journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_ACCOUNT:
		kind = idempotency.OwnerAccount
	case journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER:
		kind = idempotency.OwnerCharacter
	case journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_GUILD:
		kind = idempotency.OwnerGuild
	default:
		kind = idempotency.OwnerWorld
	}
	return idempotency.Owner{Kind: kind, ID: ownerIDOf(rec)}
}

// recorder is a blocking-capable executor for tests.
type recorder struct {
	mu       sync.Mutex
	calls    int
	order    []id.UUID
	block    chan struct{}             // non-nil: block ALL executor entries until closed
	blockOps map[id.UUID]chan struct{} // per-op blocking channels
	entered  chan struct{}
	err      error // domain rejection
	out      []byte
}

func newRecorder() *recorder {
	return &recorder{entered: make(chan struct{}, 64), out: []byte(`{"ok":true}`)}
}

func (r *recorder) executor() queue.Executor {
	return func(ctx context.Context, tx pgx.Tx,
		rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
		var opID id.UUID
		copy(opID[:], rec.GetOperationId())
		r.mu.Lock()
		r.calls++
		r.order = append(r.order, opID)
		block := r.block
		opBlock := r.blockOps[opID]
		r.mu.Unlock()
		select {
		case r.entered <- struct{}{}:
		default:
		}
		if block == nil {
			block = opBlock
		}
		if block != nil {
			select {
			case <-block:
			case <-ctx.Done():
				return idempotency.Outcome{}, ctx.Err()
			}
		}
		if r.err != nil {
			return idempotency.Outcome{}, r.err
		}
		return idempotency.Outcome{SchemaVersion: 1, Payload: r.out}, nil
	}
}

func (r *recorder) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *recorder) ran(op id.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range r.order {
		if o == op {
			return true
		}
	}
	return false
}

func (r *recorder) unblock() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.block != nil {
		close(r.block)
		r.block = nil
	}
	for op, ch := range r.blockOps {
		close(ch)
		delete(r.blockOps, op)
	}
}

// blockOp blocks executor entry for the single operation until unblock.
func (r *recorder) blockOp(op id.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.blockOps == nil {
		r.blockOps = map[id.UUID]chan struct{}{}
	}
	r.blockOps[op] = make(chan struct{})
}

// spyGate observes Reserve/release ordering and can force saturation.
type spyGate struct {
	fail     bool
	outstand chan struct{}
	onRes    func()
}

func (g *spyGate) Reserve(ctx context.Context) (func(), error) {
	if g.fail {
		return nil, idempotency.ErrBackpressure
	}
	if g.onRes != nil {
		g.onRes()
	}
	select {
	case g.outstand <- struct{}{}:
	case <-ctx.Done():
		return nil, idempotency.ErrBackpressure
	}
	return func() { <-g.outstand }, nil
}

func newSpyGate() *spyGate { return &spyGate{outstand: make(chan struct{}, 64)} }

func (g *spyGate) depth() int { return len(g.outstand) }

type fixture struct {
	q     *queue.Queue
	store *idempotency.Store
	pool  *pgxpool.Pool
	rec   *recorder
}

func newFixture(t *testing.T, capacity, workers int, gate idempotency.QueueGate) *fixture {
	t.Helper()
	pool := newPool(t)
	rec := newRecorder()
	store := idempotency.NewStore(pool)
	q := queue.New(capacity, store, queue.Deps{
		Pool:       pool,
		Workers:    workers,
		RetryDelay: time.Millisecond,
		Gate:       gate,
	})
	for _, k := range []queue.ProducerKind{
		queue.ProducerClient, queue.ProducerReward, queue.ProducerWorldConsequence,
		queue.ProducerBossEligibility, queue.ProducerBossChest, queue.ProducerCheckpoint,
		queue.ProducerPublicSchedule, queue.ProducerCompetitiveAdmission,
		queue.ProducerMatch, queue.ProducerGuildEvent, queue.ProducerJob,
		queue.ProducerActivity, queue.ProducerChatLog, queue.ProducerErasureResume,
	} {
		q.RegisterExecutor(k, rec.executor())
	}
	f := &fixture{q: q, store: store, pool: pool, rec: rec}
	t.Cleanup(func() {
		rec.unblock()
		_ = q.Shutdown(context.Background())
	})
	return f
}

func waitFor(t *testing.T, cond func() bool, what string) {
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

func waitResolved(t *testing.T, q *queue.Queue, rec *journalv1.DurableCommandRecord) {
	t.Helper()
	waitFor(t, func() bool {
		return !q.Live(rec.GetOperationFamily(), liveOwner(rec), opOf(t, rec))
	}, "record "+rec.GetOperationFamily()+" resolved")
}

func receiptState(t *testing.T, pool *pgxpool.Pool, family string, ownerID, opID id.UUID) (string, *time.Time, *time.Time) {
	t.Helper()
	var state string
	var ackAt, completedAt *time.Time
	err := pool.QueryRow(context.Background(),
		`SELECT state, disposition_ack_at, completed_at FROM durable_command_receipts
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
		family, ownerID.String(), opID.String()).Scan(&state, &ackAt, &completedAt)
	if err != nil {
		t.Fatalf("receiptState: %v", err)
	}
	return state, ackAt, completedAt
}

func receiptExists(t *testing.T, pool *pgxpool.Pool, family string, ownerID, opID id.UUID) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM durable_command_receipts
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
		family, ownerID.String(), opID.String()).Scan(&n); err != nil {
		t.Fatalf("receiptExists: %v", err)
	}
	return n > 0
}

// ---------------------------------------------------------------------------
// TestClosedProducerAdmission — every registry producer admits; everything
// else is rejected as unknown/corrupt.
func TestClosedProducerAdmission(t *testing.T) {
	char := charOwner()
	world := idempotency.Owner{Kind: idempotency.OwnerWorld, ID: id.NewV4()}
	guild := idempotency.Owner{Kind: idempotency.OwnerGuild, ID: id.NewV4()}
	acct := idempotency.Owner{Kind: idempotency.OwnerAccount, ID: id.NewV4()}

	mk := func(family string, owner idempotency.Owner, ct journalv1.JournalCommandType) *journalv1.DurableCommandRecord {
		r := shell(family, owner, "x")
		r.CommandType = ct
		return r
	}
	with := func(r *journalv1.DurableCommandRecord, cmd func(*journalv1.DurableCommandRecord)) *journalv1.DurableCommandRecord {
		cmd(r)
		return r
	}
	job := func(family string, owner idempotency.Owner, set func(*journalv1.JournalJob)) *journalv1.DurableCommandRecord {
		j := &journalv1.JournalJob{Kind: "K", JobKey: "K:1:0"}
		set(j)
		r := mk(family, owner, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_JOB)
		r.Command = &journalv1.DurableCommandRecord_Job{Job: j}
		return r
	}
	JR := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_REWARD
	JW := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_WORLD_CONSEQUENCE
	JBE := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_BOSS_ELIGIBILITY
	JBC := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_BOSS_CHEST
	JCK := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHECKPOINT
	JPS := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_PUBLIC_SCHEDULE
	JM := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_MATCH
	JG := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_GUILD_EVENT
	JA := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_ACTIVITY
	JCA := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_COMPETITIVE_ADMISSION
	JJ := journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_JOB

	valid := []*journalv1.DurableCommandRecord{
		clientRec(acct, "character.create", "a"),
		clientRec(char, "inventory.mutate", "b"),
		clientRec(char, "trade.finalise", "t"),
		clientRec(char, "interaction.chop", "i"),
		clientRec(char, "client.608", "s"),
		with(mk("sim.kill_settlement", char, JR), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_Reward{
				Reward: &journalv1.JournalRewardCommand{Kind: "KILL"}}
		}),
		with(mk("content.cosmetic.grant", char, JR), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_Reward{
				Reward: &journalv1.JournalRewardCommand{Kind: "COSMETIC"}}
		}),
		with(mk("world.consequence", world, JW), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_WorldConsequence{
				WorldConsequence: &journalv1.JournalWorldConsequence{MapId: "m"}}
		}),
		with(mk("boss.eligibility", char, JBE), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_BossEligibility{
				BossEligibility: &journalv1.JournalBossEligibility{}}
		}),
		with(mk("boss.chest", char, JBC), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_BossChest{
				BossChest: &journalv1.JournalBossChest{}}
		}),
		with(mk("sim.checkpoint", char, JCK), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_Checkpoint{
				Checkpoint: &journalv1.JournalCheckpoint{}}
		}),
		with(mk("boss.schedule", world, JPS), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_PublicSchedule{
				PublicSchedule: &journalv1.JournalPublicSchedule{}}
		}),
		with(mk("competitive.admission", world, JCA), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_CompetitiveAdmission{
				CompetitiveAdmission: &journalv1.JournalCompetitiveAdmission{}}
		}),
		with(mk("pvp.settlement", char, JM), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_Match{
				Match: &journalv1.JournalMatch{SettlementType: "pvp"}}
		}),
		with(mk("guild_war.guild_rating", guild, JM), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_Match{
				Match: &journalv1.JournalMatch{SettlementType: "GUILD_RATING"}}
		}),
		with(mk("guild_war.personal_reward", char, JM), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_Match{
				Match: &journalv1.JournalMatch{SettlementType: "PERSONAL_REWARD"}}
		}),
		with(mk("guild.event", guild, JG), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_GuildEvent{
				GuildEvent: &journalv1.JournalGuildEvent{}}
		}),
		job("job.quest", char, func(j *journalv1.JournalJob) {
			j.Target = &journalv1.JournalJob_Quest{Quest: &journalv1.JournalQuestJob{}}
		}),
		job("job.guild", guild, func(j *journalv1.JournalJob) {
			j.Target = &journalv1.JournalJob_Guild{Guild: &journalv1.JournalGuildJob{}}
		}),
		job("job.auction", world, func(j *journalv1.JournalJob) {
			j.Target = &journalv1.JournalJob_Auction{Auction: &journalv1.JournalAuctionJob{}}
		}),
		job("job.season", world, func(j *journalv1.JournalJob) {
			j.Target = &journalv1.JournalJob_Season{Season: &journalv1.JournalSeasonJob{}}
		}),
		job("job.payment", acct, func(j *journalv1.JournalJob) {
			j.Target = &journalv1.JournalJob_Payment{Payment: &journalv1.JournalPaymentJob{}}
		}),
		job("job.maintenance", world, func(j *journalv1.JournalJob) {
			j.Target = &journalv1.JournalJob_Maintenance{
				Maintenance: &journalv1.JournalMaintenanceJob{}}
		}),
		job("job.compensation", char, func(j *journalv1.JournalJob) {
			j.Target = &journalv1.JournalJob_Compensation{
				Compensation: &journalv1.JournalCompensationJob{}}
		}),
		erasureResumeRec(id.NewV4()),
		with(mk("character.activity", char, JA), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_Activity{
				Activity: &journalv1.JournalActivity{}}
		}),
		chatRec(char),
	}
	for i, rec := range valid {
		if _, err := queue.ValidateRecord(rec); err != nil {
			t.Fatalf("valid[%d] family=%s rejected: %v", i, rec.GetOperationFamily(), err)
		}
	}

	bad := []*journalv1.DurableCommandRecord{
		clientRec(char, "admin.shutdown", "x"),
		with(mk("sim.teleport", char, JR), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_Reward{
				Reward: &journalv1.JournalRewardCommand{Kind: "TELEPORT"}}
		}),
		with(mk("world.consequence", char, JW), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_WorldConsequence{
				WorldConsequence: &journalv1.JournalWorldConsequence{}}
		}),
		with(mk("pvp.settlement", world, JM), func(r *journalv1.DurableCommandRecord) {
			r.Command = &journalv1.DurableCommandRecord_Match{
				Match: &journalv1.JournalMatch{SettlementType: "pvp"}}
		}),
		job("job.auction", char, func(j *journalv1.JournalJob) {
			j.Target = &journalv1.JournalJob_Auction{Auction: &journalv1.JournalAuctionJob{}}
		}),
		job("job.quest", char, func(j *journalv1.JournalJob) {
			j.Target = &journalv1.JournalJob_Maintenance{
				Maintenance: &journalv1.JournalMaintenanceJob{}}
		}),
		func() *journalv1.DurableCommandRecord {
			r := clientRec(char, "inventory.mutate", "x")
			r.CommandType = JJ
			return r
		}(),
		shell("inventory.mutate", char, "x"), // no command variant
		func() *journalv1.DurableCommandRecord {
			r := chatRec(char)
			r.GetChatLog().Content = string(make([]byte, 2<<20))
			return r
		}(),
	}
	for i, rec := range bad {
		if _, err := queue.ValidateRecord(rec); !errors.Is(err, queue.ErrUnknownProducer) {
			t.Fatalf("bad[%d] family=%s: want ErrUnknownProducer, got %v",
				i, rec.GetOperationFamily(), err)
		}
	}

	if got := len(queue.Registry()); got != 15 {
		t.Fatalf("registry rows = %d, want 15", got)
	}
}

// TestAdmissionReservedBeforeReceipt — capacity is reserved BEFORE the
// durable receipt row exists (concurrency.md order).
func TestAdmissionReservedBeforeReceipt(t *testing.T) {
	pool := newPool(t)
	gate := newSpyGate()
	rec := clientRec(charOwner(), "inventory.mutate", "p")
	opID := opOf(t, rec)
	ownerID := ownerIDOf(rec)
	var reserved bool
	gate.onRes = func() {
		reserved = true
		if receiptExists(t, pool, "inventory.mutate", ownerID, opID) {
			t.Errorf("receipt row existed before admission reservation")
		}
	}
	f := newFixture(t, 8, 1, gate)
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !reserved {
		t.Fatal("gate never reserved")
	}
	waitFor(t, func() bool {
		return receiptExists(t, pool, "inventory.mutate", ownerID, opID)
	}, "receipt admitted")
}

// TestNoEnqueueWithoutClientReceipt — a CLIENT command never enqueues
// without its receipt admission; a saturated gate or DB outage enqueues
// nothing.
func TestNoEnqueueWithoutClientReceipt(t *testing.T) {
	gate := newSpyGate()
	gate.fail = true
	f := newFixture(t, 8, 1, gate)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "a")
	if err := f.q.Submit(context.Background(), rec); !errors.Is(err, queue.ErrBackpressure) {
		t.Fatalf("saturated gate: want ErrBackpressure, got %v", err)
	}
	if f.q.Live("inventory.mutate", owner, opOf(t, rec)) {
		t.Fatal("record live after rejected submit")
	}
	if receiptExists(t, f.pool, "inventory.mutate", owner.ID, opOf(t, rec)) {
		t.Fatal("receipt written despite gate failure")
	}

	// DB outage on admission: a closed pool fails the INSERT.
	gate2 := newSpyGate()
	pool2, err := pgxpool.New(context.Background(), sharedDSN)
	if err != nil {
		t.Fatalf("pool2: %v", err)
	}
	pool2.Close()
	rec2 := clientRec(charOwner(), "inventory.mutate", "b")
	q2 := queue.New(8, idempotency.NewStore(newPool(t)), queue.Deps{
		Pool: pool2, Workers: 1, Gate: gate2, RetryDelay: time.Millisecond})
	defer func() { _ = q2.Shutdown(context.Background()) }()
	if err := q2.Submit(context.Background(), rec2); !errors.Is(err, queue.ErrAdmissionFailed) {
		t.Fatalf("outage: want ErrAdmissionFailed, got %v", err)
	}
	if f2 := q2.Freeze(); len(f2.Records) != 0 {
		t.Fatal("record enqueued on admission outage")
	}
	if got := gate2.depth(); got != 0 {
		t.Fatalf("gate slot leaked: outstanding=%d", got)
	}
}

// TestFrozenInventoryIncludesInflight — Freeze snapshots queued AND
// in-flight records immutably; release disposes the journal references.
func TestFrozenInventoryIncludesInflight(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	f.rec.block = make(chan struct{})
	rec := worldRec("a")
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-f.rec.entered // in-flight (executor running, blocked)
	rec2 := worldRec("b")
	if err := f.q.Submit(context.Background(), rec2); err != nil {
		t.Fatalf("submit2: %v", err)
	}
	inv := f.q.Freeze()
	if len(inv.Records) != 2 {
		t.Fatalf("inventory = %d records, want 2 (inflight + queued)", len(inv.Records))
	}
	// Deep-cloned: mutating the snapshot must not corrupt the queue.
	inv.Records[0].OperationFamily = "tampered"
	inv2 := f.q.Freeze()
	for _, r := range inv2.Records {
		if r.GetOperationFamily() == "tampered" {
			t.Fatal("frozen inventory aliases live records")
		}
	}
	f.q.ReleaseInventory(inv2)
	blob, err := inv.Encode()
	if err != nil || len(blob) == 0 {
		t.Fatalf("encode: %v len=%d", err, len(blob))
	}
	f.rec.unblock()
	f.q.ReleaseInventory(inv)
	waitResolved(t, f.q, rec)
	waitResolved(t, f.q, rec2)
}

// TestAckWaitsForAllJournalReferences — ack blocks until commit AND the
// journal reference is disposed.
func TestAckWaitsForAllJournalReferences(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "j")
	f.rec.block = make(chan struct{})
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-f.rec.entered
	inv := f.q.Freeze() // journal ref pinned while in-flight
	f.rec.unblock()

	waitFor(t, func() bool {
		st, _, _ := receiptState(t, f.pool, "inventory.mutate", owner.ID, opOf(t, rec))
		return st == "COMMITTED"
	}, "receipt COMMITTED")

	ackDone := make(chan error, 1)
	go func() {
		ackDone <- f.q.Ack(context.Background(), "inventory.mutate", owner, opOf(t, rec))
	}()
	select {
	case err := <-ackDone:
		t.Fatalf("Ack returned %v before journal reference disposed", err)
	case <-time.After(150 * time.Millisecond):
	}
	f.q.ReleaseInventory(inv)
	if err := <-ackDone; err != nil {
		t.Fatalf("Ack after release: %v", err)
	}
	_, ackAt, completedAt := receiptState(t, f.pool, "inventory.mutate", owner.ID, opOf(t, rec))
	if ackAt == nil || completedAt == nil || ackAt.Before(*completedAt) {
		t.Fatalf("disposition ack %v must be >= completed %v", ackAt, completedAt)
	}
}

// TestTerminalReceiptAckWaitsForEveryQueueReference — a domain-rejected
// (terminal REJECTED) receipt's ack still waits for journal references.
func TestTerminalReceiptAckWaitsForEveryQueueReference(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "rej")
	f.rec.err = errors.New("domain: insufficient shards")
	f.rec.block = make(chan struct{})
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-f.rec.entered
	inv := f.q.Freeze()
	f.rec.unblock()
	waitFor(t, func() bool {
		st, _, _ := receiptState(t, f.pool, "inventory.mutate", owner.ID, opOf(t, rec))
		return st == "REJECTED"
	}, "receipt REJECTED")

	ackDone := make(chan error, 1)
	go func() {
		ackDone <- f.q.Ack(context.Background(), "inventory.mutate", owner, opOf(t, rec))
	}()
	select {
	case err := <-ackDone:
		t.Fatalf("Ack returned %v while journal ref held", err)
	case <-time.After(150 * time.Millisecond):
	}
	f.q.ReleaseInventory(inv)
	if err := <-ackDone; err != nil {
		t.Fatalf("Ack on terminal receipt: %v", err)
	}
	_, ackAt, _ := receiptState(t, f.pool, "inventory.mutate", owner.ID, opOf(t, rec))
	if ackAt == nil {
		t.Fatal("disposition_ack_at not set on terminal receipt")
	}
}

// TestPendingReceiptRetainedPastReplayHorizon — PRIV-008: a pending
// (ADMITTED) receipt referenced by the queue survives horizon-driven
// purge/reconcile.
func TestPendingReceiptRetainedPastReplayHorizon(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	f.rec.block = make(chan struct{})
	defer f.rec.unblock()
	if err := f.q.Submit(context.Background(), worldRec("blocker")); err != nil {
		t.Fatalf("blocker submit: %v", err)
	}
	<-f.rec.entered // worker occupied → the pending record stays referenced

	owner := charOwner()
	issued := time.Now().AddDate(0, 0, -181).UTC()
	opID := id.NewV7(issued)
	payload := fp("pending")
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO durable_command_receipts
		 (operation_family, owner_kind, owner_id, operation_id, request_fingerprint,
		  admitted_at, issued_at, replay_until, state)
		 VALUES ('inventory.mutate','CHARACTER',$1,$2,$3,$4,$4,$5,'ADMITTED')`,
		owner.ID.String(), opID.String(), payload[:], issued, issued.AddDate(0, 0, 180)); err != nil {
		t.Fatalf("seed receipt: %v", err)
	}
	rec := shell("inventory.mutate", owner, "pending")
	rec.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT
	rec.Command = &journalv1.DurableCommandRecord_Client{
		Client: &journalv1.JournalClientCommand{AccountId: owner.ID[:]},
	}
	rec.OperationId = opID[:]
	rec.RequestFingerprint = payload[:]
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit pending: %v", err)
	}
	if _, err := f.store.ReconcileOrphans(context.Background(), f.q.Live); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if _, err := f.q.Purge(context.Background(), 1000); err != nil {
		t.Fatalf("purge: %v", err)
	}
	st, _, _ := receiptState(t, f.pool, "inventory.mutate", owner.ID, opID)
	if st != "ADMITTED" {
		t.Fatalf("pending receipt %q destroyed past horizon with live queue ref", st)
	}
}

// TestErasureAdmissionFenceCancelsUncommittedPersonalCommand — the erasure
// fence cancels uncommitted subject commands (ADMITTED→terminal,
// non-executing) and refuses new subject work.
func TestErasureAdmissionFenceCancelsUncommittedPersonalCommand(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	f.rec.block = make(chan struct{})
	defer f.rec.unblock()
	blocker := worldRec("blocker")
	if err := f.q.Submit(context.Background(), blocker); err != nil {
		t.Fatalf("blocker: %v", err)
	}
	<-f.rec.entered
	subject := charOwner()
	rec := clientRec(subject, "inventory.mutate", "s")
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("subject submit: %v", err)
	}
	callsBefore := f.rec.callCount()

	if err := f.q.ErasureFence(subject.ID); err != nil {
		t.Fatalf("fence: %v", err)
	}
	st, _, _ := receiptState(t, f.pool, "inventory.mutate", subject.ID, opOf(t, rec))
	if st != "REJECTED" {
		t.Fatalf("uncommitted subject command state=%q, want REJECTED", st)
	}
	if f.q.Live("inventory.mutate", subject, opOf(t, rec)) {
		t.Fatal("cancelled command still live")
	}
	if err := f.q.Submit(context.Background(),
		clientRec(subject, "inventory.mutate", "s2")); !errors.Is(err, queue.ErrErasureFenced) {
		t.Fatalf("submit under fence: want ErrErasureFenced, got %v", err)
	}
	f.rec.unblock()
	waitResolved(t, f.q, blocker)
	if got := f.rec.callCount(); got != callsBefore {
		t.Fatalf("cancelled command executed (calls=%d, want %d)", got, callsBefore)
	}
}

// TestChatLogEnqueueNeverBlocksDeliveryOrRequiresClientReceipt —
// PRIV-009: CHAT_LOG is not a CLIENT receipt; it bypasses the admission
// gate entirely so chat delivery never blocks on the durable bound.
func TestChatLogEnqueueNeverBlocksDeliveryOrRequiresClientReceipt(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := chatRec(owner)
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("chat submit: %v", err)
	}
	waitResolved(t, f.q, rec)
	if receiptExists(t, f.pool, "chat.log", owner.ID, opOf(t, rec)) {
		t.Fatal("CHAT_LOG wrote a client receipt")
	}
	// A saturated admission gate does not delay chat records.
	gate := newSpyGate()
	gate.fail = true
	f2 := newFixture(t, 8, 1, gate)
	rec2 := chatRec(charOwner())
	if err := f2.q.Submit(context.Background(), rec2); err != nil {
		t.Fatalf("chat submit under saturated gate: %v", err)
	}
	waitResolved(t, f2.q, rec2)
}

// TestErasureResumeQueueDispositionUsesDurableContinuation — JRN-008: the
// ERASURE_RESUME record settles through the queue like any durable
// command (its disposition means continuation ownership only); the
// destructive worker is deferred until every subject reference is gone.
func TestErasureResumeQueueDispositionUsesDurableContinuation(t *testing.T) {
	f := newFixture(t, 8, 2, nil)
	subject := charOwner()
	subjRec := clientRec(subject, "inventory.mutate", "subj")
	resume := erasureResumeRec(subject.ID)
	destructive := maintJobRec("MAINT:" + subject.ID.String() + ":2026-W40")

	// Only the subject blocks in flight; the continuation must still run.
	f.rec.blockOp(opOf(t, subjRec))
	if err := f.q.Submit(context.Background(), subjRec); err != nil {
		t.Fatalf("subject submit: %v", err)
	}
	<-f.rec.entered // subject in-flight
	if err := f.q.ErasureFence(subject.ID); err != nil {
		t.Fatalf("fence: %v", err)
	}
	// ERASURE_RESUME is exempt from the subject gate even though its
	// job_key names the subject: durable continuation ownership only.
	if err := f.q.Submit(context.Background(), resume); err != nil {
		t.Fatalf("resume submit: %v", err)
	}
	if err := f.q.Submit(context.Background(), destructive); err != nil {
		t.Fatalf("destructive submit: %v", err)
	}
	waitFor(t, func() bool { return f.rec.ran(opOf(t, resume)) }, "resume executed")
	waitResolved(t, f.q, resume)
	// Destructive worker stays deferred while the subject's in-flight
	// reference exists — its ack is continuation ownership, not completion.
	time.Sleep(200 * time.Millisecond)
	if f.rec.ran(opOf(t, destructive)) {
		t.Fatal("destructive worker ran while subject references were held")
	}
	if !f.q.Live("job.maintenance", liveOwner(destructive), opOf(t, destructive)) {
		t.Fatal("deferred destructive record dropped instead of parked")
	}
	f.rec.unblock() // subject commits → subject refs clear → deferred runs
	waitResolved(t, f.q, subjRec)
	waitResolved(t, f.q, destructive)
}

// TestPerAggregateOrdering — commands on one (family, owner_kind,
// owner_id) aggregate commit in submission order even with different
// operation IDs.
func TestPerAggregateOrdering(t *testing.T) {
	f := newFixture(t, 8, 2, nil)
	owner := charOwner()
	var opIDs []id.UUID
	var recs []*journalv1.DurableCommandRecord
	for i := 0; i < 3; i++ {
		rec := clientRec(owner, "inventory.mutate", fmt.Sprintf("op%d", i))
		opIDs = append(opIDs, opOf(t, rec))
		recs = append(recs, rec)
		if err := f.q.Submit(context.Background(), rec); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}
	for _, rec := range recs {
		waitResolved(t, f.q, rec)
	}
	f.rec.mu.Lock()
	defer f.rec.mu.Unlock()
	for i, op := range f.rec.order {
		if op != opIDs[i] {
			t.Fatalf("commit order[%d]=%s, want submission order %s", i, op, opIDs[i])
		}
	}
}

// TestBackpressureWhenFull — a full queue returns a typed error; callers
// never block the simulation tick.
func TestBackpressureWhenFull(t *testing.T) {
	f := newFixture(t, 2, 1, nil)
	f.rec.block = make(chan struct{})
	defer f.rec.unblock()
	if err := f.q.Submit(context.Background(), worldRec("a")); err != nil {
		t.Fatalf("submit a: %v", err)
	}
	<-f.rec.entered
	if err := f.q.Submit(context.Background(), worldRec("b")); err != nil {
		t.Fatalf("submit b: %v", err)
	}
	err := f.q.Submit(context.Background(), worldRec("c"))
	if !errors.Is(err, queue.ErrQueueFull) {
		t.Fatalf("full queue: want ErrQueueFull, got %v", err)
	}
}

// TestAckAfterCommit — the disposition ack is sent only after commit.
func TestAckAfterCommit(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "c")
	f.rec.block = make(chan struct{})
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-f.rec.entered // in-flight, uncommitted
	ackDone := make(chan error, 1)
	go func() {
		ackDone <- f.q.Ack(context.Background(), "inventory.mutate", owner, opOf(t, rec))
	}()
	select {
	case err := <-ackDone:
		t.Fatalf("Ack returned %v before commit", err)
	case <-time.After(150 * time.Millisecond):
	}
	f.rec.unblock()
	if err := <-ackDone; err != nil {
		t.Fatalf("Ack: %v", err)
	}
	_, ackAt, completedAt := receiptState(t, f.pool, "inventory.mutate", owner.ID, opOf(t, rec))
	if ackAt == nil || completedAt == nil || ackAt.Before(*completedAt) {
		t.Fatalf("ack %v must be >= completed %v", ackAt, completedAt)
	}
}

// TestCrashRetrySettlesOnce — a crash retry resubmits the same operation
// ID; the stored commit is replayed and the executor never runs twice.
func TestCrashRetrySettlesOnce(t *testing.T) {
	f := newFixture(t, 8, 1, nil)
	owner := charOwner()
	rec := clientRec(owner, "inventory.mutate", "once")
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitResolved(t, f.q, rec)
	st, _, _ := receiptState(t, f.pool, "inventory.mutate", owner.ID, opOf(t, rec))
	if st != "COMMITTED" {
		t.Fatalf("receipt %q, want COMMITTED", st)
	}
	// Crash retry: identical record (same operation ID + fingerprint).
	if err := f.q.Submit(context.Background(), rec); err != nil {
		t.Fatalf("retry submit: %v", err)
	}
	waitResolved(t, f.q, rec)
	if got := f.rec.callCount(); got != 1 {
		t.Fatalf("executor ran %d times; crash retry must settle once", got)
	}
}
