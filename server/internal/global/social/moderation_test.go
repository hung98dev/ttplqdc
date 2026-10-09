package social

import (
	"context"
	"testing"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// fakeModeration records each consult and rejects sends while `deny`.
type fakeModeration struct {
	deny    bool
	code    protocolv1.ErrorCode
	calls   int
	sender  id.UUID
	channel protocolv1.ChatChannel
	text    string
}

func (m *fakeModeration) CheckSend(_ context.Context, senderID id.UUID,
	channel protocolv1.ChatChannel, _ id.UUID,
	text string) (protocolv1.ErrorCode, bool) {
	m.calls++
	m.sender, m.channel, m.text = senderID, channel, text
	if m.deny {
		return m.code, false
	}
	return 0, true
}

// TestChatModerationConsult: a bound consult runs at send admission —
// after channel gates, before rate-limit/fanout. A rejection produces
// exactly one 655 carrying the consult's error code and no 601 fanout;
// nil (unbound) admits everything (covered by every other chat test).
func TestChatModerationConsult(t *testing.T) {
	e := newEnv(t)
	acct := e.seedAccount(t)
	a := e.seedCharacter(t, acct, 12)
	e.setActive(t, a, true)
	e.hub.zones[a] = "zone.a"
	c, epoch := e.dial(t, acct)
	attach(t, c, epoch, a)

	mod := &fakeModeration{deny: true,
		code: protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED}
	e.svc.moderation = mod

	wsEnv(t, c, 600, epoch, 3, &protocolv1.C2SChatSend{
		OperationId: opBytes(),
		Channel:     protocolv1.ChatChannel_CHAT_CHANNEL_LOCAL,
		MessageText: "hi there",
	})
	res := &protocolv1.S2CChatSendResult{}
	decode(t, wsReadUntil(t, c, 655), 655, res)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_ERROR ||
		res.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED {
		t.Fatalf("denied send: %v", res.GetResult())
	}
	if mod.calls != 1 || mod.sender != a ||
		mod.channel != protocolv1.ChatChannel_CHAT_CHANNEL_LOCAL ||
		mod.text != "hi there" {
		t.Fatalf("consult args: calls=%d sender=%v ch=%v text=%q",
			mod.calls, mod.sender, mod.channel, mod.text)
	}
	if got := e.hub.seen(a, 601); len(got) != 0 {
		t.Fatalf("fanout delivered %d frames on a denied send", len(got))
	}

	// The same sender passes once the consult admits — and the denied
	// attempt did not consume the LOCAL rate-limit budget.
	mod.deny = false
	wsEnv(t, c, 600, epoch, 4, &protocolv1.C2SChatSend{
		OperationId: opBytes(),
		Channel:     protocolv1.ChatChannel_CHAT_CHANNEL_LOCAL,
		MessageText: "ok now",
	})
	res2 := &protocolv1.S2CChatSendResult{}
	decode(t, wsReadUntil(t, c, 655), 655, res2)
	if res2.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("admitted send: %v", res2.GetResult())
	}
	if got := e.hub.waitSeen(t, a, 601, 1); len(got) != 1 {
		t.Fatal("601 not delivered")
	}
}

// TestWithModerationBinds asserts the Option wires the consult.
func TestWithModerationBinds(t *testing.T) {
	mod := &fakeModeration{}
	svc := New(nil, nil, nil, WithModeration(mod))
	if svc.moderation != mod {
		t.Fatal("WithModeration did not bind")
	}
	if New(nil, nil, nil).moderation != nil {
		t.Fatal("default moderation must be nil (no-op)")
	}
}
