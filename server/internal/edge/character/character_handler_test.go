package character

import (
	"testing"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestCreateListSelectFlow: hello -> create -> 13 SUCCESS + 14 push ->
// C2S_CHARACTER_ATTACH on the created character -> ATTACH_OK.
func TestCreateListSelectFlow(t *testing.T) {
	e := newEnv(t, 4)
	acct := e.seedAccount(t)
	c, epoch := e.dial(t, acct)

	wsEnv(t, c, 12, epoch, 2, &protocolv1.C2SCharacterCreate{
		OperationId:   opIDBytes(),
		CharacterName: "Flow Hero",
		ClassId:       "class.kim",
	})
	r := decodeCreateResult(t, wsReadUntil(t, c, 13))
	if r.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("create status = %v", r.GetResult().GetStatus())
	}
	l := decodeList(t, wsReadUntil(t, c, 14))
	if len(l.GetCharacters()) != 1 {
		t.Fatalf("list has %d characters, want 1", len(l.GetCharacters()))
	}
	if got := l.GetCharacters()[0].GetCharacterId(); string(got) != string(r.GetCharacter().GetCharacterId()) {
		t.Fatalf("list char id %x != created %x", got, r.GetCharacter().GetCharacterId())
	}

	wsEnv(t, c, 6, epoch, 3, &protocolv1.C2SCharacterAttach{CharacterId: r.GetCharacter().GetCharacterId()})
	if env := wsReadUntil(t, c, 7); env.GetMessageId() != 7 {
		t.Fatalf("attach response id = %d, want 7", env.GetMessageId())
	}
}

// TestSelectForeignCharacterRejected: attaching a character owned by a
// different account rejects with S2C_ERROR{NOT_OWNER}.
func TestSelectForeignCharacterRejected(t *testing.T) {
	e := newEnv(t, 4)
	acct := e.seedAccount(t)
	other := e.seedAccount(t)
	foreign := e.seedCharacter(t, other, "Not Yours")
	c, epoch := e.dial(t, acct)

	wsEnv(t, c, 6, epoch, 2, &protocolv1.C2SCharacterAttach{CharacterId: foreign[:]})
	err := decodeError(t, wsReadUntil(t, c, 3))
	if got := err.GetErrorCode(); got != protocolv1.ErrorCode_ERROR_CODE_NOT_OWNER {
		t.Fatalf("error code = %v, want NOT_OWNER", got)
	}
}
