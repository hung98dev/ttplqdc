package reward

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/inventory"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/schema"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/testing/pgtest"
)

var (
	sharedPool *pgxpool.Pool
	setupErr   error
)

func testRepoRoot() string {
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
	name := fmt.Sprintf("reward_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
	dsn, cleanup, err := srv.NewDB(ctx, name)
	if err != nil {
		setupErr = err
		os.Exit(m.Run())
	}
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(testRepoRoot()), "up"); err != nil {
		setupErr = err
	} else if p, err := pgxpool.New(ctx, dsn); err != nil {
		setupErr = err
	} else {
		sharedPool = p
	}
	code := m.Run()
	if sharedPool != nil {
		sharedPool.Close()
	}
	cleanup()
	srv.Close()
	os.Exit(code)
}

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if sharedPool == nil {
		if setupErr != nil {
			t.Fatalf("postgres provisioning failed: %v", setupErr)
		}
		t.Fatal("postgres pool unavailable")
	}
	return sharedPool
}

func newStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(pool(t))
}

func mkAccount(t *testing.T) id.UUID {
	t.Helper()
	acct := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO accounts (account_id) VALUES ($1)`, acct.String()); err != nil {
		t.Fatalf("account: %v", err)
	}
	return acct
}

func mkCharacter(t *testing.T, accountID id.UUID) id.UUID {
	t.Helper()
	char := id.NewV4()
	name := fmt.Sprintf("rw_%d", time.Now().UnixNano()%1_000_000_000)
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id)
		 VALUES ($1,$2,$3,$4,'class.kim')`, char.String(), accountID.String(), name, name); err != nil {
		t.Fatalf("character: %v", err)
	}
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_inventories (character_id, capacity)
		 VALUES ($1, 60)`, char.String()); err != nil {
		t.Fatalf("inventory row: %v", err)
	}
	return char
}

// mkItem inserts one instance on wire slot n.
func mkItem(t *testing.T, charID id.UUID, itemID string, qty int32, slot uint32) id.UUID {
	t.Helper()
	inst := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO item_instances
		  (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
		   item_state, content_revision, created_at)
		 VALUES ($1,$2,$3,'UNBOUND',0,'{}'::jsonb,NULL,NOW())`,
		inst.String(), itemID, qty); err != nil {
		t.Fatalf("instance: %v", err)
	}
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO item_locations (item_instance_id, location_kind, character_id, slot, updated_at)
		 VALUES ($1,'CHARACTER_INVENTORY',$2,$3,NOW())`,
		inst.String(), charID.String(), invSlot(slot)); err != nil {
		t.Fatalf("location: %v", err)
	}
	return inst
}

func invSlot(n uint32) string { return "inv." + fmt.Sprint(n) }

func mkWallet(t *testing.T, charID id.UUID, currencyID string, amount int64) {
	t.Helper()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_currencies (character_id, currency_id, balance, revision)
		 VALUES ($1,$2,$3,0)`, charID.String(), currencyID, amount); err != nil {
		t.Fatalf("wallet: %v", err)
	}
}

// testDefs resolves the fixed test catalog.
func testDefs() inventory.Defs {
	catalog := map[string]inventory.ItemDef{
		"item.potion.hp": {Def: items.Def{
			ItemID: "item.potion.hp", Kind: items.KindConsumable, Stackable: true,
			MaxStack: 99}},
		"item.potion.mp": {Def: items.Def{
			ItemID: "item.potion.mp", Kind: items.KindConsumable, Stackable: true,
			MaxStack: 99}},
		"item.sword.rare": {Def: items.Def{
			ItemID: "item.sword.rare", Kind: items.KindEquipment, Stackable: false,
			MaxStack: 1}},
	}
	return func(_ context.Context, itemID string) (inventory.ItemDef, error) {
		d, ok := catalog[itemID]
		if !ok {
			return inventory.ItemDef{}, fmt.Errorf("defs: unknown %s", itemID)
		}
		return d, nil
	}
}

func newDeps(t *testing.T) Deps {
	t.Helper()
	return Deps{
		Store: newStore(t),
		Items: items.New(pool(t)),
		Inv:   inventory.NewStore(pool(t)),
		Defs:  testDefs(),
	}
}

// tx runs fn in one test transaction.
func tx(t *testing.T, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	txx, err := pool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer txx.Rollback(ctx)
	fn(ctx, txx)
	if err := txx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// makeInput builds a valid creation input.
func makeInput(charID id.UUID, opID id.UUID, srcType string, lines ...LineInput) *Input {
	return &Input{
		OwnerCharacterID:        charID,
		SourceType:              srcType,
		SourceReference:         "test.ref",
		RewardSlot:              "slot.0",
		SourceRewardOperationID: opID,
		Lines:                   lines,
	}
}

// testRevision is a schema-legal content_revision (64 lowercase hex).
const testRevision = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func itemLine(itemID string, qty int64, perInstance bool) LineInput {
	return LineInput{
		Kind:             lineItem,
		ItemID:           itemID,
		Quantity:         big.NewInt(qty),
		EffectiveBinding: "UNBOUND",
		ContentRevision:  testRevision,
		PerInstance:      perInstance,
	}
}

// newOp mints a UUIDv7 operation id (TrustedReplay validates the
// operation_id encoding).
func newOp() id.UUID { return id.NewV7(time.Now()) }

func currencyLine(currencyID string, amt int64) LineInput {
	return LineInput{Kind: lineCurrency, CurrencyID: currencyID, Amount: big.NewInt(amt)}
}

// claimRecord builds the 408 journal record the executor consumes.
func claimRecord(charID, opID, claimID id.UUID) *journalv1.DurableCommandRecord {
	acct := id.NewV4()
	return &journalv1.DurableCommandRecord{
		SchemaVersion:   1,
		OperationFamily: ClaimFamily,
		OwnerKind:       journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:         charID[:],
		OperationId:     opID[:],
		CommandType:     journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				AccountId:   acct[:],
				CharacterId: charID[:],
				Request: &journalv1.JournalClientCommand_C2SRewardClaim{
					C2SRewardClaim: &protocolv1.C2SRewardClaim{
						OperationId:   opID[:],
						RewardClaimId: claimID[:],
					},
				},
			},
		},
	}
}

// rewardRecord builds one REST settlement record.
func rewardRecord(charID, opID id.UUID, kind string, exp uint64) *journalv1.DurableCommandRecord {
	return &journalv1.DurableCommandRecord{
		SchemaVersion:   1,
		OperationFamily: "sim.rest_settlement",
		OwnerKind:       journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:         charID[:],
		OperationId:     opID[:],
		CommandType:     journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_REWARD,
		Command: &journalv1.DurableCommandRecord_Reward{
			Reward: &journalv1.JournalRewardCommand{
				Kind: kind,
				Slots: []*journalv1.JournalRewardSlot{{
					RewardSlot:      "rest",
					CharacterExp:    exp,
					SourceType:      "WORLD_EVENT",
					SourceReference: "bonfire.test",
				}},
			},
		},
	}
}

// outcome decodes a protojson JournalOutcome receipt.
func outcome(t *testing.T, o idempotency.Outcome) *journalv1.JournalOutcome {
	t.Helper()
	out := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(o.Payload, out); err != nil {
		t.Fatalf("outcome decode: %v", err)
	}
	return out
}

// pendingOf is the spec pending_count: PENDING non-aggregate claims
// plus aggregate currency claims (ITEM_CONSOLIDATED excluded, ADR-0062).
func pendingOf(t *testing.T, charID id.UUID) int64 {
	t.Helper()
	var n int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT count(*) FROM reward_claims
		 WHERE owner_character_id=$1 AND state='PENDING'
		   AND claim_kind <> 'ITEM_CONSOLIDATED'`, charID.String()).Scan(&n); err != nil {
		t.Fatalf("pending: %v", err)
	}
	return n
}

// revisions reads (claims_revision) from characters.
func claimsRev(t *testing.T, charID id.UUID) int64 {
	t.Helper()
	var r int64
	if err := pool(t).QueryRow(context.Background(),
		`SELECT claims_revision FROM characters WHERE character_id=$1`, charID.String()).Scan(&r); err != nil {
		t.Fatalf("claims_revision: %v", err)
	}
	return r
}

// padPending bulk-inserts n SINGLE PENDING claims for cap tests.
func padPending(t *testing.T, charID id.UUID, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := pool(t).Exec(context.Background(),
			`INSERT INTO reward_claims
			 (reward_claim_id, owner_character_id, source_type, source_reference,
			  reward_slot, claim_kind, consolidation_key, state, revision, created_at, updated_at)
			 VALUES ($1,$2,'MONSTER','pad','slot.pad','SINGLE',NULL,'PENDING',0,NOW(),NOW())`,
			id.NewV4().String(), charID.String()); err != nil {
			t.Fatalf("pad %d: %v", i, err)
		}
	}
}

// replay drives the durable path the queue executor runs: TrustedReplay
// under the receipt lock invoking the named executor.
func replay(t *testing.T, ex func(context.Context, pgx.Tx,
	*journalv1.DurableCommandRecord) (idempotency.Outcome, error),
	rec *journalv1.DurableCommandRecord) *journalv1.JournalOutcome {
	t.Helper()
	ctx := context.Background()
	idem := idempotency.NewStore(pool(t))
	var opID, charID id.UUID
	copy(opID[:], rec.GetOperationId())
	copy(charID[:], rec.GetOwnerId())
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	out, err := idem.TrustedReplay(ctx, rec.GetOperationFamily(),
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: charID}, opID, fp,
		func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return ex(ctx, tx, rec)
		})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	return outcome(t, out)
}
