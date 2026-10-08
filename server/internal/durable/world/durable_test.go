package world

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/db"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/schema"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/testing/pgtest"
)

func testPool(t *testing.T) *pgxpool.Pool {
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
	return pool
}

func seedChar(t *testing.T, pool *pgxpool.Pool) id.UUID {
	t.Helper()
	ctx := context.Background()
	acct := id.NewV7(time.Now().UTC())
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (account_id) VALUES ($1)`, acct); err != nil {
		t.Fatalf("account: %v", err)
	}
	charID := id.NewV7(time.Now().UTC())
	if _, err := pool.Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id)
		 VALUES ($1,$2,$3,$3,'class.kim')`, charID, acct, "w"+charID.String()[:12]); err != nil {
		t.Fatalf("character: %v", err)
	}
	return charID
}

func runExec(t *testing.T, pool *pgxpool.Pool, ex func(context.Context, pgx.Tx,
	*journalv1.DurableCommandRecord) (idempotency.Outcome, error),
	rec *journalv1.DurableCommandRecord) *journalv1.JournalOutcome {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	out, err := ex(ctx, tx, rec)
	if err != nil {
		tx.Commit(ctx)
		t.Fatalf("executor: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	j := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(out.Payload, j); err != nil {
		t.Fatalf("outcome decode: %v", err)
	}
	return j
}

func checkpointRow(t *testing.T, pool *pgxpool.Pool, charID id.UUID) (string, string) {
	t.Helper()
	var cp, m string
	if err := pool.QueryRow(context.Background(),
		`SELECT checkpoint_id, map_id FROM characters WHERE character_id=$1`,
		charID).Scan(&cp, &m); err != nil {
		t.Fatalf("checkpointRow: %v", err)
	}
	return cp, m
}

func strp(s string) *string { return &s }

// op7 mints a fresh UUIDv7 operation id as bytes.
func op7() []byte {
	o := id.NewV7(time.Now().UTC())
	return o[:]
}

// TestRecordFamiliesAndTypes: every builder stamps the family + command
// variant the durable registry expects — client families produce
// JOURNAL_COMMAND_TYPE_CLIENT client records, sim.checkpoint a CHECKPOINT
// record under ProducerCheckpoint.
func TestRecordFamiliesAndTypes(t *testing.T) {
	acct, charID := id.NewV7(time.Now().UTC()), id.NewV7(time.Now().UTC())
	now := time.Now().UTC()

	iRec, err := InteractRecord(acct, charID, 3, 7,
		&protocolv1.C2SInteract{OperationId: op7(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
			TargetId:     "npc.alpha.guide", ServiceId: strp("set_checkpoint")},
		&journalv1.JournalCheckpoint{CharacterId: charID[:], CheckpointId: "checkpoint.alpha.one"},
		now)
	if err != nil {
		t.Fatalf("InteractRecord: %v", err)
	}
	if iRec.GetOperationFamily() != FamilyNpcService || FamilyNpcService != "interaction.npc_service" {
		t.Fatalf("interact family %q", iRec.GetOperationFamily())
	}
	if iRec.GetCommandType() != journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT {
		t.Fatal("interact record must be a CLIENT command")
	}
	if iRec.GetClient().GetCheckpoint().GetCheckpointId() != "checkpoint.alpha.one" {
		t.Fatal("set_checkpoint record must carry the checkpoint payload (field 12)")
	}
	if len(iRec.GetRequestFingerprint()) != 32 {
		t.Fatal("fingerprint missing")
	}

	pRec, err := PortalRecord(acct, charID, 3, 7,
		&protocolv1.C2SPortalUse{OperationId: op7(), PortalId: "portal.alpha.beta"}, now)
	if err != nil {
		t.Fatalf("PortalRecord: %v", err)
	}
	if pRec.GetOperationFamily() != FamilyPlacementPortal || FamilyPlacementPortal != "placement.portal" {
		t.Fatalf("portal family %q", pRec.GetOperationFamily())
	}
	if pRec.GetClient().GetC2SPortalUse().GetPortalId() != "portal.alpha.beta" {
		t.Fatal("portal request not embedded")
	}

	cRec, err := ChannelRecord(acct, charID, 3, 7,
		&protocolv1.C2SChannelSwitch{OperationId: op7(), TargetChannelIndex: 4}, now)
	if err != nil {
		t.Fatalf("ChannelRecord: %v", err)
	}
	if cRec.GetOperationFamily() != FamilyPlacementChannel || FamilyPlacementChannel != "placement.channel" {
		t.Fatalf("channel family %q", cRec.GetOperationFamily())
	}

	kRec, err := CheckpointRecord(id.NewV7(now), charID, [32]byte{1},
		"src.evt", "a1b2c3d4"+repeatB(), &journalv1.JournalCheckpoint{
			CharacterId:  charID[:],
			CheckpointId: "checkpoint.alpha.one", SafeMapId: "map.alpha"}, now)
	if err != nil {
		t.Fatalf("CheckpointRecord: %v", err)
	}
	if kRec.GetOperationFamily() != FamilySimCheckpoint || FamilySimCheckpoint != "sim.checkpoint" {
		t.Fatalf("checkpoint family %q", kRec.GetOperationFamily())
	}
	if kRec.GetCommandType() != journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHECKPOINT {
		t.Fatal("sim.checkpoint must be a CHECKPOINT command")
	}
	if kRec.GetClient() != nil {
		t.Fatal("sim.checkpoint must not carry a client payload")
	}
}

func repeatB() string {
	b := make([]byte, 56)
	for i := range b {
		b[i] = 'b'
	}
	return string(b)
}

// TestRecordRejectsBadOperationID: a malformed operation id fails at
// record build, never at the executor.
func TestRecordRejectsBadOperationID(t *testing.T) {
	acct, charID := id.NewV7(time.Now().UTC()), id.NewV7(time.Now().UTC())
	if _, err := InteractRecord(acct, charID, 0, 0,
		&protocolv1.C2SInteract{OperationId: []byte{1, 2, 3}}, nil, time.Now()); err == nil {
		t.Fatal("short operation_id must fail record build")
	}
	if _, err := PortalRecord(acct, charID, 0, 0,
		&protocolv1.C2SPortalUse{OperationId: nil}, time.Now()); err == nil {
		t.Fatal("empty operation_id must fail record build")
	}
}

// TestNpcServiceExecCommitsCheckpoint: the registered set_checkpoint
// service applies characters.checkpoint_id and returns SUCCESS 116.
func TestNpcServiceExecCommitsCheckpoint(t *testing.T) {
	pool := testPool(t)
	charID := seedChar(t, pool)
	cp := &journalv1.JournalCheckpoint{CharacterId: charID[:], CheckpointId: "checkpoint.alpha.one"}
	rec, err := InteractRecord(id.NewV7(time.Now().UTC()), charID, 1, 1,
		&protocolv1.C2SInteract{OperationId: op7(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
			TargetId:     "npc.alpha.guide", ServiceId: strp(SetCheckpointService)},
		cp, time.Now().UTC())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := runExec(t, pool, Deps{Store: NewStore(pool)}.npcServiceExec, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("status %v", out.GetStatus())
	}
	if out.GetS2CInteractResult().GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatal("116 must report SUCCESS")
	}
	got, _ := checkpointRow(t, pool, charID)
	if got != "checkpoint.alpha.one" {
		t.Fatalf("checkpoint_id %q", got)
	}
}

// TestNpcServiceExecUnregistered: a service outside the registry commits
// the terminal 116 TARGET_INVALID — one 116 per 103, never silence.
func TestNpcServiceExecUnregistered(t *testing.T) {
	pool := testPool(t)
	charID := seedChar(t, pool)
	rec, err := InteractRecord(id.NewV7(time.Now().UTC()), charID, 1, 1,
		&protocolv1.C2SInteract{OperationId: op7(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
			TargetId:     "npc.alpha.guide", ServiceId: strp("teleport")},
		nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := runExec(t, pool, Deps{Store: NewStore(pool)}.npcServiceExec, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_ERROR {
		t.Fatalf("status %v", out.GetStatus())
	}
	res := out.GetS2CInteractResult().GetResult()
	if res.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("error code %v", res.GetErrorCode())
	}
	got, _ := checkpointRow(t, pool, charID)
	if got == "checkpoint.alpha.one" {
		t.Fatal("unregistered service must not write checkpoint_id")
	}
}

// TestPortalExecRecordedResult: placement.portal commits receipt +
// recorded 116 PORTAL only — the map write lands on transfer completion.
func TestPortalExecRecordedResult(t *testing.T) {
	pool := testPool(t)
	charID := seedChar(t, pool)
	rec, err := PortalRecord(id.NewV7(time.Now().UTC()), charID, 1, 1,
		&protocolv1.C2SPortalUse{OperationId: op7(),
			PortalId: "portal.alpha.beta"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := runExec(t, pool, Deps{Store: NewStore(pool)}.portalExec, rec)
	res := out.GetS2CInteractResult()
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
		res.GetInteractKind() != protocolv1.InteractKind_INTERACT_KIND_PORTAL ||
		res.GetTargetId() != "portal.alpha.beta" {
		t.Fatalf("unexpected portal outcome %+v", res)
	}
}

// TestChannelExecRecordedResult: placement.channel commits receipt +
// recorded 110 echoing the target channel (R3-4: no row to commit).
func TestChannelExecRecordedResult(t *testing.T) {
	pool := testPool(t)
	charID := seedChar(t, pool)
	rec, err := ChannelRecord(id.NewV7(time.Now().UTC()), charID, 1, 1,
		&protocolv1.C2SChannelSwitch{OperationId: op7(),
			TargetChannelIndex: 9}, time.Now().UTC())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := runExec(t, pool, Deps{Store: NewStore(pool)}.channelExec, rec)
	res := out.GetS2CChannelSwitchResult()
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
		res.GetTargetChannelIndex() != 9 {
		t.Fatalf("unexpected channel outcome %+v", res)
	}
}

// TestCheckpointExecAppliesColumns: the ProducerCheckpoint write applies
// exactly the columns present in the payload.
func TestCheckpointExecAppliesColumns(t *testing.T) {
	pool := testPool(t)
	charID := seedChar(t, pool)
	rec, err := CheckpointRecord(id.NewV7(time.Now().UTC()), charID, [32]byte{1},
		"src", "a1b2c3d4"+repeatB(), &journalv1.JournalCheckpoint{
			CharacterId:  charID[:],
			CheckpointId: "checkpoint.alpha.one",
			SafeMapId:    "map.alpha"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := runExec(t, pool, Deps{Store: NewStore(pool)}.checkpointExec, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("status %v", out.GetStatus())
	}
	cp, m := checkpointRow(t, pool, charID)
	if cp != "checkpoint.alpha.one" || m != "map.alpha" {
		t.Fatalf("checkpoint %q map %q", cp, m)
	}

	// Map-only write (transfer completion) leaves checkpoint_id alone.
	rec2, err := CheckpointRecord(id.NewV7(time.Now().UTC()), charID, [32]byte{2},
		"src", "a1b2c3d4"+repeatB(), &journalv1.JournalCheckpoint{
			CharacterId: charID[:], SafeMapId: "map.beta"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("record2: %v", err)
	}
	runExec(t, pool, Deps{Store: NewStore(pool)}.checkpointExec, rec2)
	cp, m = checkpointRow(t, pool, charID)
	if cp != "checkpoint.alpha.one" || m != "map.beta" {
		t.Fatalf("after map-only write: checkpoint %q map %q", cp, m)
	}
}

// TestCheckpointExecRejectsWrongFamily: family discipline — the executor
// refuses records not stamped sim.checkpoint.
func TestCheckpointExecRejectsWrongFamily(t *testing.T) {
	pool := testPool(t)
	charID := seedChar(t, pool)
	rec, err := CheckpointRecord(id.NewV7(time.Now().UTC()), charID, [32]byte{1},
		"src", "a1b2c3d4"+repeatB(), &journalv1.JournalCheckpoint{
			CharacterId: charID[:], SafeMapId: "map.alpha"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	rec.OperationFamily = FamilyNpcService
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := (Deps{Store: NewStore(pool)}).checkpointExec(context.Background(), tx, rec); err == nil {
		t.Fatal("wrong family must reject")
	}
}

// TestExecutorsRegistry: the client family-mux registers exactly the
// three admitted families; sim.checkpoint lives under CheckpointExecutors.
func TestExecutorsRegistry(t *testing.T) {
	execs := Executors(Deps{})
	for _, f := range []string{FamilyNpcService, FamilyPlacementPortal, FamilyPlacementChannel} {
		if _, ok := execs[f]; !ok {
			t.Fatalf("family %q not registered", f)
		}
	}
	if len(execs) != 3 {
		t.Fatalf("unexpected client executor count %d", len(execs))
	}
	ck := (Deps{}).CheckpointExecutors()
	if len(ck) != 1 {
		t.Fatalf("checkpoint executors %d", len(ck))
	}
	if _, ok := ck[FamilySimCheckpoint]; !ok {
		t.Fatal("sim.checkpoint executor missing")
	}
}
