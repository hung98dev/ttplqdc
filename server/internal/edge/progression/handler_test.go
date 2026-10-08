package progression

import (
	"context"
	"testing"

	protocolv1 "thinhthan/internal/protocol/v1"
)

func TestSkillUpgradeWireFlow(t *testing.T) {
	e := newEnv(t, 64, &fakeConsult{})
	acct := e.seedAccount(t)
	char := e.seedCharacter(t, acct, 25, 2, 0)
	c, epoch := e.dial(t, acct)
	attach(t, c, epoch, char)

	wsEnv(t, c, 511, epoch, 3, &protocolv1.C2SSkillUpgrade{
		OperationId:   opBytes(),
		SkillId:       "skill.kim.active.xuyen_phong",
		ExpectedLevel: 1,
	})
	// 514 then ordered 515 on SUCCESS (wire contract).
	env514 := wsReadUntil(t, c, 514)
	res := decodeMutateResult(t, env514)
	if res.GetRequestMessageId() != 511 {
		t.Fatalf("request_message_id %d", res.GetRequestMessageId())
	}
	got := res.GetResult()
	if got.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("result %v code %v", got.GetStatus(), got.GetErrorCode())
	}
	env515 := wsReadUntil(t, c, 515)
	st := decodeState(t, env515)
	if st.GetProgressionRevision() != 1 {
		t.Fatalf("revision %d want 1", st.GetProgressionRevision())
	}
	if st.GetUnspentSkillPoints() != 1 {
		t.Fatalf("unspent %d want 1", st.GetUnspentSkillPoints())
	}
	var upgraded *protocolv1.SkillView
	for _, s := range st.GetSkills() {
		if s.GetSkillId() == "skill.kim.active.xuyen_phong" {
			upgraded = s
		}
	}
	if upgraded == nil || upgraded.GetLevel() != 2 {
		t.Fatalf("upgraded skill view %v", upgraded)
	}
}

func TestPotentialAllocateWireFlow(t *testing.T) {
	e := newEnv(t, 64, &fakeConsult{})
	acct := e.seedAccount(t)
	char := e.seedCharacter(t, acct, 30, 0, 10)
	c, epoch := e.dial(t, acct)
	attach(t, c, epoch, char)

	wsEnv(t, c, 512, epoch, 3, &protocolv1.C2SPotentialAllocate{
		OperationId: opBytes(),
		Deltas:      &protocolv1.PotentialDelta{Str: 6, Vit: 4},
	})
	res := decodeMutateResult(t, wsReadUntil(t, c, 514))
	if res.GetRequestMessageId() != 512 {
		t.Fatalf("request_message_id %d", res.GetRequestMessageId())
	}
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("result %v code %v", res.GetResult().GetStatus(),
			res.GetResult().GetErrorCode())
	}
	st := decodeState(t, wsReadUntil(t, c, 515))
	if st.GetProgressionRevision() != 1 ||
		st.GetPotentialAllocated().GetStr() != 6 ||
		st.GetPotentialAllocated().GetVit() != 4 ||
		st.GetUnspentPotentialPoints() != 0 ||
		st.GetPotentialEarnedTotal() != 10 {
		t.Fatalf("state = %+v", st)
	}
}

func TestRespecConsultPaths(t *testing.T) {
	fc := &fakeConsult{}
	e := newEnv(t, 64, fc)
	acct := e.seedAccount(t)
	// Level 20 respecs free (progression.md — price is 0 at ≤ Lv20), so
	// the success path needs no currency balance.
	char := e.seedCharacter(t, acct, 20, 0, 0)
	c, epoch := e.dial(t, acct)
	attach(t, c, epoch, char)
	req := func() *protocolv1.C2SRespec {
		return &protocolv1.C2SRespec{
			OperationId: opBytes(), NpcId: "npc.lang_da.nguoi_dan_duong",
			Kind: protocolv1.RespecKind_RESPEC_KIND_POTENTIAL}
	}

	// Unbound verdict → OUT_OF_RANGE rejection.
	fc.set(false, nil)
	wsEnv(t, c, 513, epoch, 3, req())
	if e1 := decodeError(t, wsReadUntil(t, c, 3)); e1.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE {
		t.Fatalf("unbound code %v", e1.GetErrorCode())
	}
	if fc.callCount() != 1 {
		t.Fatalf("consult calls %d", fc.callCount())
	}

	// Bound → commit: 514 SUCCESS + 515.
	fc.set(true, nil)
	wsEnv(t, c, 513, epoch, 4, req())
	res := decodeMutateResult(t, wsReadUntil(t, c, 514))
	if res.GetRequestMessageId() != 513 ||
		res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("respec result %v", res.GetResult())
	}
	st := decodeState(t, wsReadUntil(t, c, 515))
	if st.GetProgressionRevision() != 1 {
		t.Fatalf("revision %d want 1", st.GetProgressionRevision())
	}
}

func TestRespecUnboundConsultPort(t *testing.T) {
	// nil ConsultPort → fail-closed INVALID_STATE, consult never called.
	e := newEnv(t, 64, nil)
	acct := e.seedAccount(t)
	char := e.seedCharacter(t, acct, 25, 0, 0)
	c, epoch := e.dial(t, acct)
	attach(t, c, epoch, char)
	wsEnv(t, c, 513, epoch, 3, &protocolv1.C2SRespec{
		OperationId: opBytes(), NpcId: "npc.lang_da.nguoi_dan_duong",
		Kind: protocolv1.RespecKind_RESPEC_KIND_POTENTIAL})
	if e1 := decodeError(t, wsReadUntil(t, c, 3)); e1.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("unbound consult port code %v", e1.GetErrorCode())
	}
}

func TestProgressionRequiresAttach(t *testing.T) {
	e := newEnv(t, 64, &fakeConsult{})
	acct := e.seedAccount(t)
	c, epoch := e.dial(t, acct) // unattached session
	wsEnv(t, c, 511, epoch, 3, &protocolv1.C2SSkillUpgrade{
		OperationId: opBytes(), SkillId: "skill.kim.basic.kiem_thuc"})
	if e1 := decodeError(t, wsReadUntil(t, c, 3)); e1.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE {
		t.Fatalf("unattached code %v", e1.GetErrorCode())
	}
}

func TestProgressionOpsIdempotentWire(t *testing.T) {
	e := newEnv(t, 64, &fakeConsult{})
	acct := e.seedAccount(t)
	char := e.seedCharacter(t, acct, 25, 2, 0)
	c, epoch := e.dial(t, acct)
	attach(t, c, epoch, char)
	op := opBytes()
	send := func(seq uint64) {
		wsEnv(t, c, 511, epoch, seq, &protocolv1.C2SSkillUpgrade{
			OperationId: op, SkillId: "skill.kim.active.xuyen_phong"})
	}
	send(3)
	res := decodeMutateResult(t, wsReadUntil(t, c, 514))
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("first outcome %v", res.GetResult().GetErrorCode())
	}
	_ = decodeState(t, wsReadUntil(t, c, 515)) // rev 1
	// Same operation_id replays the retained outcome — no second apply.
	send(4)
	res2 := decodeMutateResult(t, wsReadUntil(t, c, 514))
	if res2.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("replay outcome %v", res2.GetResult().GetErrorCode())
	}
	var st2 *protocolv1.S2CProgressionState
	for i := 0; i < 16 && st2 == nil; i++ {
		env := wsRead(t, c)
		if env.GetMessageId() == 515 {
			st2 = decodeState(t, env)
		}
	}
	if st2.GetProgressionRevision() != 1 {
		t.Fatalf("replay bumped revision to %d", st2.GetProgressionRevision())
	}
	// Column reflects a single upgrade.
	var lvl int32
	if err := e.accounts.Pool().QueryRow(
		context.Background(),
		`SELECT level FROM character_skill_levels
		 WHERE character_id = $1 AND skill_id = 'skill.kim.active.xuyen_phong'`,
		char.String()).Scan(&lvl); err != nil {
		t.Fatalf("skill level: %v", err)
	}
	if lvl != 2 {
		t.Fatalf("skill level %d want 2", lvl)
	}
}

func TestRejectErrorsDoNotPush(t *testing.T) {
	e := newEnv(t, 64, &fakeConsult{})
	acct := e.seedAccount(t)
	char := e.seedCharacter(t, acct, 25, 0, 0) // no skill points
	c, epoch := e.dial(t, acct)
	attach(t, c, epoch, char)
	wsEnv(t, c, 511, epoch, 3, &protocolv1.C2SSkillUpgrade{
		OperationId: opBytes(), SkillId: "skill.kim.active.xuyen_phong"})
	res := decodeMutateResult(t, wsReadUntil(t, c, 514))
	if res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_SKILL_POINTS_INSUFFICIENT {
		t.Fatalf("verdict %v", res.GetResult().GetErrorCode())
	}
	// A committed ERROR verdict pushes no 515 — next read is the next
	// message only. Send a second (valid-shaped) intent and ensure the
	// stream carries another 514, not a state push.
	wsEnv(t, c, 512, epoch, 4, &protocolv1.C2SPotentialAllocate{
		OperationId: opBytes(), Deltas: &protocolv1.PotentialDelta{Str: 1}})
	res2 := decodeMutateResult(t, wsReadUntil(t, c, 514))
	if res2.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_POTENTIAL_POINTS_INSUFFICIENT {
		t.Fatalf("verdict %v", res2.GetResult().GetErrorCode())
	}
}
