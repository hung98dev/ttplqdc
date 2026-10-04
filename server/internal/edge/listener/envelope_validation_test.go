package listener

import (
	"testing"
	"time"

	protocolv1 "thinhthan/internal/protocol/v1"
)

func testState(phase Phase, epoch uint64) *ConnState {
	now := time.Now()
	return &ConnState{
		Phase:         phase,
		SessionEpoch:  epoch,
		protocolMajor: 1,
		now:           func() time.Time { return now },
	}
}

func env(id uint32, epoch, cseq uint64) *protocolv1.Envelope {
	return &protocolv1.Envelope{
		ProtocolMajor: 1,
		MessageId:     id,
		SessionEpoch:  epoch,
		ClientSeq:     cseq,
	}
}

// TestValidationOrderTable walks the ordered validation table row by row:
// the first matching row decides the outcome, later rows never preempt it
// (protocol.md § Envelope Validation).
func TestValidationOrderTable(t *testing.T) {
	cases := []struct {
		name       string
		st         func() *ConnState
		env        *protocolv1.Envelope
		wantErr    protocolv1.ErrorCode
		wantClose  bool
		wantStatus int
		wantSilent bool
	}{
		{
			name:       "b: wrong protocol_major closes before anything else",
			st:         func() *ConnState { return testState(PhaseInWorld, 9) },
			env:        &protocolv1.Envelope{ProtocolMajor: 99, MessageId: 100, SessionEpoch: 9, ClientSeq: 5},
			wantErr:    protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_UNSUPPORTED,
			wantClose:  true,
			wantStatus: wsCloseProtocol,
		},
		{
			name:       "c: non-HELLO before HELLO_OK closes protocol violation",
			st:         func() *ConnState { return testState(PhasePreHello, 0) },
			env:        env(100, 0, 1),
			wantClose:  true,
			wantStatus: wsCloseProtocol,
		},
		{
			name:       "d: server_seq set by client closes protocol violation",
			st:         func() *ConnState { return testState(PhaseInWorld, 9) },
			env:        &protocolv1.Envelope{ProtocolMajor: 1, MessageId: 100, SessionEpoch: 9, ClientSeq: 5, ServerSeq: 3},
			wantClose:  true,
			wantStatus: wsCloseProtocol,
		},
		{
			name:       "d: S2C-only id from client closes protocol violation",
			st:         func() *ConnState { return testState(PhaseInWorld, 9) },
			env:        env(303, 9, 5),
			wantClose:  true,
			wantStatus: wsCloseProtocol,
		},
		{
			name:       "e: epoch mismatch emits STALE then closes",
			st:         func() *ConnState { return testState(PhaseInWorld, 9) },
			env:        env(100, 7, 5),
			wantErr:    protocolv1.ErrorCode_ERROR_CODE_SESSION_EPOCH_STALE,
			wantClose:  true,
			wantStatus: wsCloseNormal,
		},
		{
			name:    "f: unregistered id rejects without close",
			st:      func() *ConnState { return testState(PhaseInWorld, 9) },
			env:     env(9999, 9, 5),
			wantErr: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_UNKNOWN,
		},
		{
			name: "g: client_seq regression rejects without close",
			st: func() *ConnState {
				s := testState(PhaseInWorld, 9)
				s.LastClientSeq = 10
				return s
			},
			env:     env(100, 9, 5),
			wantErr: protocolv1.ErrorCode_ERROR_CODE_STALE_INPUT,
		},
		{
			name:       "h: realtime input in DEAD drops silently",
			st:         func() *ConnState { return testState(PhaseDead, 9) },
			env:        env(100, 9, 5),
			wantSilent: true,
		},
		{
			name:    "i: character create in IN_WORLD is not allowed",
			st:      func() *ConnState { return testState(PhaseInWorld, 9) },
			env:     env(12, 9, 5),
			wantErr: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE,
		},
		{
			name:    "order: unknown id beats phase illegality (f before i)",
			st:      func() *ConnState { return testState(PhaseDead, 9) },
			env:     env(9999, 9, 5),
			wantErr: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_UNKNOWN,
		},
		{
			name: "order: epoch mismatch beats stale seq (e before g)",
			st: func() *ConnState {
				s := testState(PhaseInWorld, 9)
				s.LastClientSeq = 10
				return s
			},
			env:        env(100, 7, 5),
			wantErr:    protocolv1.ErrorCode_ERROR_CODE_SESSION_EPOCH_STALE,
			wantClose:  true,
			wantStatus: wsCloseNormal,
		},
		{
			name: "dispatch: legal realtime input in IN_WORLD",
			st:   func() *ConnState { return testState(PhaseInWorld, 9) },
			env:  env(100, 9, 5),
		},
		{
			name: "dispatch: TRANSFER admits heartbeat and baseline ack",
			st:   func() *ConnState { return testState(PhaseTransfer, 9) },
			env:  env(306, 9, 5),
		},
		{
			name:    "i: TRANSFER rejects durable ops",
			st:      func() *ConnState { return testState(PhaseTransfer, 9) },
			env:     env(730, 9, 5),
			wantErr: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE,
		},
		{
			name:       "h: realtime input in TRANSFER drops silently",
			st:         func() *ConnState { return testState(PhaseTransfer, 9) },
			env:        env(200, 9, 5),
			wantSilent: true,
		},
		{
			name:    "i: PLACEMENT_PENDING chat requires attach",
			st:      func() *ConnState { return testState(PhasePlacementPending, 9) },
			env:     env(600, 9, 5),
			wantErr: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE,
		},
		{
			name: "dispatch: PLACEMENT_PENDING chat when attached",
			st: func() *ConnState {
				s := testState(PhasePlacementPending, 9)
				s.Attached = true
				return s
			},
			env: env(600, 9, 5),
		},
		{
			name: "dispatch: CHARACTER_SELECT admits attach",
			st:   func() *ConnState { return testState(PhaseCharacterSelect, 9) },
			env:  env(6, 9, 5),
		},
		{
			name:    "i: CHARACTER_SELECT rejects realtime input",
			st:      func() *ConnState { return testState(PhaseCharacterSelect, 9) },
			env:     env(100, 9, 5),
			wantErr: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := tc.st()
			r := Validate(tc.env, st)
			if tc.wantErr == 0 && !tc.wantClose && !tc.wantSilent {
				if r != nil {
					t.Fatalf("expected dispatch, got reject %+v", r)
				}
				return
			}
			if r == nil {
				t.Fatalf("expected reject, got dispatch")
			}
			if tc.wantSilent {
				if !r.SilentDrop {
					t.Fatalf("expected silent drop, got %+v", r)
				}
				return
			}
			if r.ErrCode != tc.wantErr {
				t.Fatalf("err code: got %v want %v", r.ErrCode, tc.wantErr)
			}
			if r.Close != tc.wantClose {
				t.Fatalf("close: got %v want %v", r.Close, tc.wantClose)
			}
			if tc.wantClose && r.CloseStatus != tc.wantStatus {
				t.Fatalf("close status: got %v want %v", r.CloseStatus, tc.wantStatus)
			}
		})
	}
}

func TestUnknownIdRejectNoClose(t *testing.T) {
	st := testState(PhaseInWorld, 9)
	r := Validate(env(12345, 9, 5), st)
	if r == nil || r.Close || r.SilentDrop {
		t.Fatalf("unknown id: want non-closing reject, got %+v", r)
	}
	if r.ErrCode != protocolv1.ErrorCode_ERROR_CODE_MESSAGE_UNKNOWN {
		t.Fatalf("code: got %v", r.ErrCode)
	}
}

func TestEpochMismatchClose(t *testing.T) {
	st := testState(PhaseInWorld, 9)
	r := Validate(env(100, 8, 5), st)
	if r == nil || !r.Close {
		t.Fatalf("epoch mismatch: want close, got %+v", r)
	}
	if r.ErrCode != protocolv1.ErrorCode_ERROR_CODE_SESSION_EPOCH_STALE || r.Retryability != protocolv1.Retryability_RETRYABILITY_RECONNECT {
		t.Fatalf("want SESSION_EPOCH_STALE/RECONNECT, got %+v", r)
	}
}

func TestSeqRegressionStaleInput(t *testing.T) {
	st := testState(PhaseInWorld, 9)
	st.LastClientSeq = 10
	r := Validate(env(100, 9, 10), st) // equal is a regression too
	if r == nil || r.Close || r.ErrCode != protocolv1.ErrorCode_ERROR_CODE_STALE_INPUT {
		t.Fatalf("stale input: got %+v", r)
	}
	if st.LastClientSeq != 10 {
		t.Fatalf("stale frame must not advance LastClientSeq, got %d", st.LastClientSeq)
	}
}

func TestPreHelloFrameClose(t *testing.T) {
	st := testState(PhasePreHello, 0)
	r := Validate(env(4, 0, 1), st) // heartbeat before HELLO
	if r == nil || !r.Close || r.CloseStatus != wsCloseProtocol {
		t.Fatalf("pre-HELLO frame: got %+v", r)
	}
	// A second HELLO post-handshake is phase-illegal, not a protocol
	// violation: MESSAGE_NOT_ALLOWED_IN_STATE, no close (phase table —
	// id 1 dispatches only in PRE_HELLO).
	st2 := testState(PhaseCharacterSelect, 9)
	r2 := Validate(env(1, 9, 2), st2)
	if r2 == nil || r2.Close || r2.ErrCode != protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE {
		t.Fatalf("second HELLO: want MNAS no-close, got %+v", r2)
	}
}

func TestRejectBudgetClose(t *testing.T) {
	base := time.Now()
	st := &ConnState{
		Phase:         PhaseInWorld,
		SessionEpoch:  9,
		protocolMajor: 1,
		now:           func() time.Time { return base },
	}
	cfgWindow := 10 * time.Second
	// 20 non-closing rejections inside the window stay open.
	for i := 0; i < 20; i++ {
		if r := Validate(env(9999, 9, uint64(i+1)), st); r == nil || r.Close {
			t.Fatalf("reject %d: want non-closing unknown-id, got %+v", i, r)
		}
		if over := st.recordReject(st.clock(), cfgWindow); over != nil {
			t.Fatalf("reject %d: budget closed early: %+v", i, over)
		}
	}
	// The 21st inside the same 10 s window trips the budget.
	if r := Validate(env(9999, 9, 21), st); r == nil {
		t.Fatalf("reject 20: want reject")
	}
	if over := st.recordReject(st.clock(), cfgWindow); over == nil || !over.Close || over.CloseReason != "PROTOCOL_VIOLATION" {
		t.Fatalf("budget: want PROTOCOL_VIOLATION close, got %+v", over)
	}
	// Rate-limit rejections never count: 21 RATE_LIMITED frames are fine.
	st2 := &ConnState{
		Phase:         PhaseInWorld,
		SessionEpoch:  9,
		protocolMajor: 1,
		now:           func() time.Time { return base },
	}
	for i := 0; i < 21; i++ {
		r := Validate(env(9999, 9, uint64(i+1)), st2)
		if r == nil {
			t.Fatalf("rl %d: want reject", i)
		}
		// RATE_LIMITED rejects skip the budget (mirrors applyReject).
	}
	if over := st2.recordReject(st2.clock(), cfgWindow); over != nil {
		t.Fatalf("budget must not trip on zero counted rejects")
	}
}
