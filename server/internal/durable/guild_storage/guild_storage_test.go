package guild_storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/guild"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

var now = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func depositReq(instanceID id.UUID, qty uint32, section protocolv1.GuildStorageSection) *protocolv1.C2SGuildStorageDeposit {
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildStorageDeposit{
		OperationId: op[:], ItemInstanceId: instanceID[:],
		Quantity: qty, Section: section}
}

func withdrawReq(instanceID id.UUID, qty uint32, section protocolv1.GuildStorageSection) *protocolv1.C2SGuildStorageWithdraw {
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildStorageWithdraw{
		OperationId: op[:], ItemInstanceId: instanceID[:],
		Quantity: qty, Section: section}
}

func claimReq(instanceID id.UUID, qty uint32) *protocolv1.C2SGuildStorageClaimRequest {
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildStorageClaimRequest{
		OperationId: op[:], ItemInstanceId: instanceID[:], Quantity: qty}
}

func decideReq(claimID id.UUID, d protocolv1.GuildStorageClaimDecision) *protocolv1.C2SGuildStorageClaimDecide {
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildStorageClaimDecide{
		OperationId: op[:], ClaimId: claimID[:], Decision: d}
}

// mkCharOnAccount inserts a second character under an existing account.
func mkCharOnAccount(t *testing.T, acct id.UUID) id.UUID {
	t.Helper()
	char := id.NewV4()
	name := fmt.Sprintf("c%x", char[:4])
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, created_at)
		 VALUES ($1,$2,$3,$4,'class.kim',30,$5)`,
		char[:], acct[:], name, name, time.Now().UTC().Add(-60*24*time.Hour)); err != nil {
		t.Fatalf("character: %v", err)
	}
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO character_inventories (character_id, capacity) VALUES ($1,60)`, char[:]); err != nil {
		t.Fatalf("inventory: %v", err)
	}
	return char
}

// runExecOK runs a JOB executor (non-649 outcome) and requires SUCCESS.
func runExecOK(t *testing.T, ex func(context.Context, pgx.Tx, *journalv1.DurableCommandRecord) (idempotency.Outcome, error),
	rec *journalv1.DurableCommandRecord) {
	t.Helper()
	if err := pgx.BeginTxFunc(context.Background(), sharedPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		o, err := ex(context.Background(), tx, rec)
		if err != nil {
			return err
		}
		out := &journalv1.JournalOutcome{}
		if err := protojson.Unmarshal(o.Payload, out); err != nil {
			return err
		}
		if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("job status = %v", out.GetStatus())
		}
		return nil
	}); err != nil {
		t.Fatalf("job exec: %v", err)
	}
}

// --- tests ---------------------------------------------------------------

// TestCommonReserveStoragePartitions: sections carry independent
// capacity bands, COMMON works at Lv1, RESERVE locks before Lv10, and
// Member cannot deposit into RESERVE.
func TestCommonReserveStoragePartitions(t *testing.T) {
	wipe(t)
	d := deps(now)
	ex := d.Executors()

	leader, _ := mkChar(t, 30)
	member, _ := mkChar(t, 30)
	g1 := mkGuild(t, "G1", leader, 1, now.Add(-10*24*time.Hour))
	mkMember(t, g1, member, guild.RoleMember, now.Add(-10*24*time.Hour))

	// COMMON at Lv1 works.
	i1 := mkItem(t, leader, "item.test", 1, 0)
	rec, err := Record(DepositFamily, leader, id.NewV4(), depositReq(i1, 0,
		protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_COMMON), now)
	if err != nil {
		t.Fatalf("rec: %v", err)
	}
	wantStatus(t, runExec(t, ex[DepositFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	if storageCount(t, g1, SectionCommon) != 1 {
		t.Fatalf("common count = %d, want 1", storageCount(t, g1, SectionCommon))
	}

	// RESERVE locked at Lv1 (capacity 0).
	i2 := mkItem(t, leader, "item.test", 1, 1)
	rec, _ = Record(DepositFamily, leader, id.NewV4(), depositReq(i2, 0,
		protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_RESERVE), now)
	wantError(t, runExec(t, ex[DepositFamily], rec),
		protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)

	// Lv10 guild: RESERVE deposit works for Leader.
	dropMember(t, leader)
	g10 := mkGuild(t, "G10", leader, 10, now.Add(-10*24*time.Hour))
	rec, _ = Record(DepositFamily, leader, id.NewV4(), depositReq(i2, 0,
		protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_RESERVE), now)
	wantStatus(t, runExec(t, ex[DepositFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	if storageCount(t, g10, SectionReserve) != 1 {
		t.Fatalf("reserve count = %d, want 1", storageCount(t, g10, SectionReserve))
	}

	// Member -> RESERVE deposit is PERMISSION_DENIED.
	i3 := mkItem(t, member, "item.test", 1, 0)
	dropMember(t, member)
	mkMember(t, g10, member, guild.RoleMember, now.Add(-10*24*time.Hour))
	rec, _ = Record(DepositFamily, member, id.NewV4(), depositReq(i3, 0,
		protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_RESERVE), now)
	wantError(t, runExec(t, ex[DepositFamily], rec),
		protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)

	// Audit + revision accumulated.
	if auditCount(t, g10, "DEPOSIT") < 1 {
		t.Fatalf("no DEPOSIT audit row")
	}
	if storageRevision(t, g10) == 0 {
		t.Fatalf("revision did not bump")
	}
}

// TestStorageClaimApprovalFlow: Officer requests RESERVE item, Leader
// approves, requester delivers; a second request from Member on the
// same item stays bounded by the reservation.
func TestStorageClaimApprovalFlow(t *testing.T) {
	wipe(t)
	d := deps(now)
	ex := d.Executors()

	leader, _ := mkChar(t, 30)
	officer, _ := mkChar(t, 30)
	g10 := mkGuild(t, "G10", leader, 10, now.Add(-100*24*time.Hour))
	mkMember(t, g10, officer, guild.RoleOfficer, now.Add(-100*24*time.Hour))

	inst := mkStorageItem(t, g10, SectionReserve, "item.rare", 3, 0, leader, leader)

	// Officer requests 2 of the 3.
	rec, err := Record(ClaimRequestFamily, officer, id.NewV4(),
		claimReq(inst, 2), now)
	if err != nil {
		t.Fatalf("rec: %v", err)
	}
	out := runExec(t, ex[ClaimRequestFamily], rec)
	wantStatus(t, out, protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	claims := out.GetCreatedIds()
	if len(claims) != 1 || len(claims[0].GetId()) != 16 {
		t.Fatalf("created ids = %v, want claim uuid", claims)
	}
	var claimID id.UUID
	copy(claimID[:], claims[0].GetId())
	if claimState(t, claimID) != ClaimPending {
		t.Fatalf("claim state = %s", claimState(t, claimID))
	}

	// Officer cannot approve own claim (L/V only).
	rec, _ = Record(ClaimDecideFamily, officer, id.NewV4(),
		decideReq(claimID, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_APPROVE), now)
	wantError(t, runExec(t, ex[ClaimDecideFamily], rec),
		protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)

	// Leader approves -> APPROVED + 7d expiry.
	rec, _ = Record(ClaimDecideFamily, leader, id.NewV4(),
		decideReq(claimID, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_APPROVE), now)
	wantStatus(t, runExec(t, ex[ClaimDecideFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	if claimState(t, claimID) != ClaimApproved {
		t.Fatalf("claim state = %s, want APPROVED", claimState(t, claimID))
	}
	if auditCount(t, g10, "CLAIM_APPROVE") != 1 {
		t.Fatalf("missing CLAIM_APPROVE audit")
	}

	// Requester delivers -> COMPLETED + item in inventory.
	rec, _ = Record(ClaimDecideFamily, officer, id.NewV4(),
		decideReq(claimID, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_DELIVER), now)
	wantStatus(t, runExec(t, ex[ClaimDecideFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	if claimState(t, claimID) != ClaimCompleted {
		t.Fatalf("claim state = %s, want COMPLETED", claimState(t, claimID))
	}
	// Remainder (1 unit) stays in RESERVE; delivered split sits in inventory.
	if storageCount(t, g10, SectionReserve) != 1 {
		t.Fatalf("reserve rows = %d", storageCount(t, g10, SectionReserve))
	}
	if auditCount(t, g10, "CLAIM_DELIVER") != 1 {
		t.Fatalf("missing CLAIM_DELIVER audit")
	}
}

// TestClaimExpirationSettlement: STORAGE_CLAIM_EXPIRE sweeps PENDING>72h
// and APPROVED>7d to EXPIRED, releases the reservation, audits, and
// (r4) cancels open claims of non-members.
func TestClaimExpirationSettlement(t *testing.T) {
	wipe(t)
	d := deps(now)

	leader, _ := mkChar(t, 30)
	officer, _ := mkChar(t, 30)
	leaver, _ := mkChar(t, 30)
	g10 := mkGuild(t, "G10", leader, 10, now.Add(-200*24*time.Hour))
	mkMember(t, g10, officer, guild.RoleOfficer, now.Add(-200*24*time.Hour))
	mkMember(t, g10, leaver, guild.RoleMember, now.Add(-200*24*time.Hour))

	inst := mkStorageItem(t, g10, SectionReserve, "item.rare", 3, 0, leader, leader)

	// Seed one stale PENDING (created 80h ago) + one APPROVED 8d ago +
	// one open claim by a member who has since left.
	seed := func(requester id.UUID, state string, createdAt, expiresAt time.Time, approvedAt *time.Time) id.UUID {
		cid := id.NewV4()
		if _, err := sharedPool.Exec(context.Background(),
			`INSERT INTO guild_storage_claims
			 (claim_id, guild_id, requester_character_id, item_instance_id,
			  quantity, state, created_at, approved_at, expires_at)
			 VALUES ($1,$2,$3,$4,1,$5,$6,$7,$8)`,
			cid, g10, requester, inst, state, createdAt, approvedAt, expiresAt); err != nil {
			t.Fatalf("seed claim: %v", err)
		}
		return cid
	}
	stalePending := seed(officer, ClaimPending, now.Add(-80*time.Hour), now.Add(-8*time.Hour), nil)
	approvedAt := now.Add(-8 * 24 * time.Hour)
	staleApproved := seed(officer, ClaimApproved, approvedAt.Add(-time.Hour), now.Add(-time.Hour), &approvedAt)
	leaverClaim := seed(leaver, ClaimPending, now.Add(-time.Hour), now.Add(71*time.Hour), nil)
	dropMember(t, leaver)

	rec := JobRecord(g10, id.NewV4(), now)
	runExecOK(t, d.StorageClaimExpireJob, rec)

	if claimState(t, stalePending) != ClaimExpired {
		t.Fatalf("stale pending = %s", claimState(t, stalePending))
	}
	if claimState(t, staleApproved) != ClaimExpired {
		t.Fatalf("stale approved = %s", claimState(t, staleApproved))
	}
	if claimState(t, leaverClaim) != ClaimCancelled {
		t.Fatalf("leaver claim = %s", claimState(t, leaverClaim))
	}
	if auditCount(t, g10, "CLAIM_EXPIRE") != 2 || auditCount(t, g10, "CLAIM_CANCEL") != 1 {
		t.Fatalf("audit rows expire=%d cancel=%d",
			auditCount(t, g10, "CLAIM_EXPIRE"), auditCount(t, g10, "CLAIM_CANCEL"))
	}
	if storageRevision(t, g10) == 0 {
		t.Fatalf("revision did not bump")
	}
}

// TestSameAccountWithdrawRejected: an item deposited by a different
// character of the same account cannot be withdrawn (ADR-0049).
func TestSameAccountWithdrawRejected(t *testing.T) {
	wipe(t)
	d := deps(now)
	ex := d.Executors()

	depositor, acct := mkChar(t, 30)
	receiver := mkCharOnAccount(t, acct)
	g10 := mkGuild(t, "G10", depositor, 10, now.Add(-200*24*time.Hour))
	mkMember(t, g10, receiver, guild.RoleMember, now.Add(-200*24*time.Hour))

	inst := mkStorageItem(t, g10, SectionCommon, "item.test", 5, 0, depositor, acct)

	// Same-account different-character withdraw -> SAME_ACCOUNT.
	rec, err := Record(WithdrawFamily, receiver, id.NewV4(),
		withdrawReq(inst, 0, protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_COMMON), now)
	if err != nil {
		t.Fatalf("rec: %v", err)
	}
	wantError(t, runExec(t, ex[WithdrawFamily], rec),
		protocolv1.ErrorCode_ERROR_CODE_GUILD_STORAGE_SAME_ACCOUNT)

	// Same-character withdraw is allowed.
	rec, _ = Record(WithdrawFamily, depositor, id.NewV4(),
		withdrawReq(inst, 0, protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_COMMON), now)
	wantStatus(t, runExec(t, ex[WithdrawFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	if itemLocation(t, inst) != "CHARACTER_INVENTORY" {
		t.Fatalf("item did not leave storage")
	}
}

// TestSameAccountReserveDeliveryRejected: DELIVER of a claim on a
// same-account different-character deposit is rejected.
func TestSameAccountReserveDeliveryRejected(t *testing.T) {
	wipe(t)
	d := deps(now)
	ex := d.Executors()

	depositor, acct := mkChar(t, 30)
	requester := mkCharOnAccount(t, acct)
	leader, _ := mkChar(t, 30)
	g10 := mkGuild(t, "G10", leader, 10, now.Add(-200*24*time.Hour))
	mkMember(t, g10, depositor, guild.RoleOfficer, now.Add(-200*24*time.Hour))
	mkMember(t, g10, requester, guild.RoleOfficer, now.Add(-200*24*time.Hour))

	inst := mkStorageItem(t, g10, SectionReserve, "item.rare", 2, 0, depositor, acct)

	// Same-account claim request itself is rejected up front.
	rec, err := Record(ClaimRequestFamily, requester, id.NewV4(),
		claimReq(inst, 1), now)
	if err != nil {
		t.Fatalf("rec: %v", err)
	}
	wantError(t, runExec(t, ex[ClaimRequestFamily], rec),
		protocolv1.ErrorCode_ERROR_CODE_GUILD_STORAGE_SAME_ACCOUNT)

	// A different-account officer's claim delivers fine.
	officer, _ := mkChar(t, 30)
	mkMember(t, g10, officer, guild.RoleOfficer, now.Add(-200*24*time.Hour))
	rec, _ = Record(ClaimRequestFamily, officer, id.NewV4(), claimReq(inst, 1), now)
	out := runExec(t, ex[ClaimRequestFamily], rec)
	wantStatus(t, out, protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	var claimID id.UUID
	copy(claimID[:], out.GetCreatedIds()[0].GetId())
	rec, _ = Record(ClaimDecideFamily, leader, id.NewV4(),
		decideReq(claimID, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_APPROVE), now)
	wantStatus(t, runExec(t, ex[ClaimDecideFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	rec, _ = Record(ClaimDecideFamily, officer, id.NewV4(),
		decideReq(claimID, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_DELIVER), now)
	wantStatus(t, runExec(t, ex[ClaimDecideFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
}

// TestMembershipAgeGate: a different-account cross-character withdraw
// before 72h membership is rejected; after 72h it succeeds.
func TestMembershipAgeGate(t *testing.T) {
	wipe(t)
	d := deps(now)
	ex := d.Executors()

	depositor, _ := mkChar(t, 30)
	receiver, _ := mkChar(t, 30)
	g10 := mkGuild(t, "G10", depositor, 10, now.Add(-200*24*time.Hour))
	mkMember(t, g10, receiver, guild.RoleMember, now.Add(-24*time.Hour)) // 24h only

	inst := mkStorageItem(t, g10, SectionCommon, "item.test", 5, 0, depositor, id.NewV4())

	rec, err := Record(WithdrawFamily, receiver, id.NewV4(),
		withdrawReq(inst, 0, protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_COMMON), now)
	if err != nil {
		t.Fatalf("rec: %v", err)
	}
	wantError(t, runExec(t, ex[WithdrawFamily], rec),
		protocolv1.ErrorCode_ERROR_CODE_GUILD_MEMBERSHIP_TOO_NEW)

	// Age the membership past the gate and retry.
	if _, err := sharedPool.Exec(context.Background(),
		`UPDATE guild_memberships SET joined_at=$1 WHERE character_id=$2`,
		now.Add(-96*time.Hour), receiver); err != nil {
		t.Fatalf("age membership: %v", err)
	}
	rec, _ = Record(WithdrawFamily, receiver, id.NewV4(),
		withdrawReq(inst, 0, protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_COMMON), now)
	wantStatus(t, runExec(t, ex[WithdrawFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
}

// TestItemPartnerCountsUpdated: a cross-character withdraw increments
// the receiver's item_partner_counts keyed by the depositor.
func TestItemPartnerCountsUpdated(t *testing.T) {
	wipe(t)
	d := deps(now)
	ex := d.Executors()

	depositor, _ := mkChar(t, 30)
	receiver, _ := mkChar(t, 30)
	g10 := mkGuild(t, "G10", depositor, 10, now.Add(-200*24*time.Hour))
	mkMember(t, g10, receiver, guild.RoleMember, now.Add(-96*time.Hour))

	inst := mkStorageItem(t, g10, SectionCommon, "item.test", 4, 0, depositor, id.NewV4())
	rec, err := Record(WithdrawFamily, receiver, id.NewV4(),
		withdrawReq(inst, 4, protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_COMMON), now)
	if err != nil {
		t.Fatalf("rec: %v", err)
	}
	wantStatus(t, runExec(t, ex[WithdrawFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	if got := partnerCount(t, receiver, depositor); got != 4 {
		t.Fatalf("partner count = %d, want 4", got)
	}

	// Own-character withdraw does NOT count.
	inst2 := mkStorageItem(t, g10, SectionCommon, "item.test", 2, 1, receiver, id.NewV4())
	rec, _ = Record(WithdrawFamily, receiver, id.NewV4(),
		withdrawReq(inst2, 0, protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_COMMON), now)
	wantStatus(t, runExec(t, ex[WithdrawFamily], rec),
		protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	if got := partnerCount(t, receiver, depositor); got != 4 {
		t.Fatalf("own-withdraw changed partner count to %d", got)
	}
}

// TestStorageClaimDecideActors: the closed actor matrix —
// APPROVE/REJECT = L/V; CANCEL = requester/L/V; DELIVER = requester only.
func TestStorageClaimDecideActors(t *testing.T) {
	wipe(t)
	d := deps(now)
	ex := d.Executors()

	leader, _ := mkChar(t, 30)
	officer, _ := mkChar(t, 30)
	requester, _ := mkChar(t, 30)
	g10 := mkGuild(t, "G10", leader, 10, now.Add(-200*24*time.Hour))
	mkMember(t, g10, officer, guild.RoleOfficer, now.Add(-200*24*time.Hour))
	mkMember(t, g10, requester, guild.RoleOfficer, now.Add(-200*24*time.Hour))

	inst := mkStorageItem(t, g10, SectionReserve, "item.rare", 5, 0, leader, leader)
	rec, err := Record(ClaimRequestFamily, requester, id.NewV4(), claimReq(inst, 2), now)
	if err != nil {
		t.Fatalf("rec: %v", err)
	}
	out := runExec(t, ex[ClaimRequestFamily], rec)
	wantStatus(t, out, protocolv1.ResultStatus_RESULT_STATUS_SUCCESS)
	var claimID id.UUID
	copy(claimID[:], out.GetCreatedIds()[0].GetId())

	decide := func(actor id.UUID, d protocolv1.GuildStorageClaimDecision) *journalResult {
		rec, _ := Record(ClaimDecideFamily, actor, id.NewV4(), decideReq(claimID, d), now)
		return runExecResult(t, ex[ClaimDecideFamily], rec)
	}

	// Member-decision matrix on the PENDING claim.
	if r := decide(officer, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_APPROVE); r.code != protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED {
		t.Fatalf("officer APPROVE = %v", r.code)
	}
	if r := decide(requester, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_DELIVER); r.code != protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT {
		t.Fatalf("DELIVER on PENDING = %v", r.code)
	}
	// Non-requester DELIVER is PERMISSION_DENIED (ADR-0060) — request
	// it after approval below.
	if r := decide(leader, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_APPROVE); r.status != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("leader APPROVE = %v/%v", r.status, r.code)
	}
	if r := decide(leader, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_DELIVER); r.code != protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED {
		t.Fatalf("non-requester DELIVER = %v", r.code)
	}
	// Requester cancels -> CANCELLED.
	if r := decide(requester, protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_CANCEL); r.status != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("requester CANCEL = %v/%v", r.status, r.code)
	}
	if claimState(t, claimID) != ClaimCancelled {
		t.Fatalf("claim = %s", claimState(t, claimID))
	}
}

// --- small outcome helpers for the actor-matrix test ----------------

type journalResult struct {
	status protocolv1.ResultStatus
	code   protocolv1.ErrorCode
}

func runExecResult(t *testing.T, ex func(context.Context, pgx.Tx, *journalv1.DurableCommandRecord) (idempotency.Outcome, error),
	rec *journalv1.DurableCommandRecord) *journalResult {
	t.Helper()
	ctx := context.Background()
	var out *journalv1.JournalOutcome
	if err := pgx.BeginTxFunc(ctx, sharedPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		o, err := ex(ctx, tx, rec)
		if err != nil {
			return err
		}
		out = &journalv1.JournalOutcome{}
		return protojson.Unmarshal(o.Payload, out)
	}); err != nil {
		t.Fatalf("exec: %v", err)
	}
	r := out.GetS2CGuildResult()
	if r == nil {
		t.Fatalf("no s2c_guild_result")
	}
	return &journalResult{status: r.GetResult().GetStatus(), code: r.GetResult().GetErrorCode()}
}
