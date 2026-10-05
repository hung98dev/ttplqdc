package character

import (
	"testing"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestCreateOnlyUnattached: C2S_CHARACTER_CREATE is legal only while the
// session is unattached — an attached session gets
// S2C_ERROR{MESSAGE_NOT_ALLOWED_IN_STATE}, not a result-13.
func TestCreateOnlyUnattached(t *testing.T) {
	e := newEnv(t, 4)
	acct := e.seedAccount(t)
	charID := e.seedCharacter(t, acct, "AlreadyHave")
	c, epoch := e.dial(t, acct)

	wsEnv(t, c, 6, epoch, 2, &protocolv1.C2SCharacterAttach{CharacterId: charID[:]})
	if env := wsReadUntil(t, c, 7); env.GetMessageId() != 7 {
		t.Fatalf("attach response id = %d, want 7", env.GetMessageId())
	}

	wsEnv(t, c, 12, epoch, 3, &protocolv1.C2SCharacterCreate{
		OperationId:   opIDBytes(),
		CharacterName: "Second Char",
		ClassId:       "class.moc",
	})
	err := decodeError(t, wsReadUntil(t, c, 3))
	if got := err.GetErrorCode(); got != protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE {
		t.Fatalf("error code = %v, want MESSAGE_NOT_ALLOWED_IN_STATE", got)
	}
}

// TestCreateErrorCodes: in-set domain verdicts arrive on
// S2C_CHARACTER_CREATE_RESULT (13), never S2C_ERROR.
func TestCreateErrorCodes(t *testing.T) {
	e := newEnv(t, 8)
	acct := e.seedAccount(t)
	c, epoch := e.dial(t, acct)
	seq := uint64(2)

	send := func(name, classID string) *protocolv1.S2CCharacterCreateResult {
		t.Helper()
		wsEnv(t, c, 12, epoch, seq, &protocolv1.C2SCharacterCreate{
			OperationId:   opIDBytes(),
			CharacterName: name,
			ClassId:       classID,
		})
		seq++
		return decodeCreateResult(t, wsReadUntil(t, c, 13))
	}

	if r := send("Bad Class", "class.sat"); r.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("unknown class code = %v, want TARGET_INVALID", r.GetResult().GetErrorCode())
	}
	if r := send("", "class.kim"); r.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_CHARACTER_NAME_INVALID {
		t.Fatalf("empty name code = %v, want CHARACTER_NAME_INVALID", r.GetResult().GetErrorCode())
	}

	if r := send("Wire Dup", "class.kim"); r.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("first create status = %v, want SUCCESS", r.GetResult().GetStatus())
	}
	if r := send("wire dup", "class.moc"); r.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_CHARACTER_NAME_TAKEN {
		t.Fatalf("dup name code = %v, want CHARACTER_NAME_TAKEN", r.GetResult().GetErrorCode())
	}
}

// TestCharacterListPushes: a fresh session receives the empty list push
// on bind, then SUCCESS on 13, then the updated list push on 14.
func TestCharacterListPushes(t *testing.T) {
	e := newEnv(t, 4)
	acct := e.seedAccount(t)
	c, epoch := e.dial(t, acct)

	wsEnv(t, c, 12, epoch, 2, &protocolv1.C2SCharacterCreate{
		OperationId:   opIDBytes(),
		CharacterName: "List Push",
		ClassId:       "class.thuy",
	})

	if l := decodeList(t, wsReadUntil(t, c, 14)); len(l.GetCharacters()) != 0 {
		t.Fatalf("bind list has %d characters, want 0", len(l.GetCharacters()))
	}
	r := decodeCreateResult(t, wsReadUntil(t, c, 13))
	if r.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("create status = %v", r.GetResult().GetStatus())
	}
	l := decodeList(t, wsReadUntil(t, c, 14))
	if len(l.GetCharacters()) != 1 {
		t.Fatalf("updated list has %d characters, want 1", len(l.GetCharacters()))
	}
	ch := l.GetCharacters()[0]
	if ch.GetCharacterName() != "List Push" || ch.GetClassId() != "class.thuy" ||
		ch.GetLevel() != 1 || ch.GetMapId() != "map.lang_da.dinh_lang" || ch.GetIsAttached() {
		t.Fatalf("summary = %+v", ch)
	}
}

// TestSuspendedCreateUsesS2CError: out-of-set verdicts go out on
// S2C_ERROR (3) with the account code — the client must not see 13.
func TestSuspendedCreateUsesS2CError(t *testing.T) {
	e := newEnv(t, 4)
	acct := e.seedAccount(t)
	e.suspendAccount(t, acct)
	c, epoch := e.dial(t, acct)

	wsEnv(t, c, 12, epoch, 2, &protocolv1.C2SCharacterCreate{
		OperationId:   opIDBytes(),
		CharacterName: "Suspended Acct",
		ClassId:       "class.hoa",
	})
	err := decodeError(t, wsReadUntil(t, c, 3))
	if got := err.GetErrorCode(); got != protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_SUSPENDED {
		t.Fatalf("error code = %v, want ACCOUNT_SUSPENDED", got)
	}
}

// TestCreateRetryReturnsCommittedOutcome: resending the same
// operation_id with the identical payload redelivers the committed
// result-13 — no re-execution, one character row.
func TestCreateRetryReturnsCommittedOutcome(t *testing.T) {
	e := newEnv(t, 4)
	acct := e.seedAccount(t)
	c, epoch := e.dial(t, acct)

	req := &protocolv1.C2SCharacterCreate{
		OperationId:   opIDBytes(),
		CharacterName: "Retry Me",
		ClassId:       "class.tho",
	}
	wsEnv(t, c, 12, epoch, 2, req)
	first := decodeCreateResult(t, wsReadUntil(t, c, 13))
	if first.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("first create status = %v", first.GetResult().GetStatus())
	}

	wsEnv(t, c, 12, epoch, 3, req)
	retry := decodeCreateResult(t, wsReadUntil(t, c, 13))
	if retry.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("retry status = %v", retry.GetResult().GetStatus())
	}
	if string(retry.GetCharacter().GetCharacterId()) != string(first.GetCharacter().GetCharacterId()) {
		t.Fatalf("retry created a second character: %x vs %x",
			retry.GetCharacter().GetCharacterId(), first.GetCharacter().GetCharacterId())
	}
	var n int
	if err := e.accounts.Pool().QueryRow(t.Context(),
		`SELECT count(*) FROM characters WHERE account_id=$1`, acct).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("characters rows = %d, want 1", n)
	}
}
