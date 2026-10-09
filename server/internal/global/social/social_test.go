package social

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	sociald "thinhthan/internal/durable/social"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestSocialGraphFriendBlock: request → accept → friendship both
// directions; block severs it and cancels pendings in one commit;
// friend requests are then TARGET_BLOCKED.
func TestSocialGraphFriendBlock(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	aAcct, bAcct := e.seedAccount(t), e.seedAccount(t)
	a := e.seedCharacter(t, aAcct, 5)
	b := e.seedCharacter(t, bAcct, 5)

	execs := sociald.Executors(e.store)
	run := func(fam string, req any, accountID, charID id.UUID) {
		rec, err := buildRec(t, fam, req, accountID, charID)
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		tx, err := e.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		out, err := execs[fam](ctx, tx, rec)
		if err != nil {
			t.Fatalf("executor: %v", err)
		}
		_ = out
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// a→b request; b accepts.
	run(sociald.FriendRequestFamily, &protocolv1.C2SFriendRequest{
		OperationId: opBytes(), TargetCharacterId: b[:]}, aAcct, a)
	run(sociald.FriendAcceptFamily, &protocolv1.C2SFriendAccept{
		OperationId: opBytes(), RequesterCharacterId: a[:]}, bAcct, b)
	friends, err := e.store.AreFriends(ctx, nil, a, b)
	if err != nil || !friends {
		t.Fatalf("friends=%v err=%v", friends, err)
	}
	// c sends a→c pending; then a blocks b → friendship severed.
	c := e.seedCharacter(t, e.seedAccount(t), 5)
	run(sociald.FriendRequestFamily, &protocolv1.C2SFriendRequest{
		OperationId: opBytes(), TargetCharacterId: c[:]}, aAcct, a)
	run(sociald.BlockAddFamily, &protocolv1.C2SBlockAdd{
		OperationId: opBytes(), TargetCharacterId: b[:]}, aAcct, a)
	if friends, _ := e.store.AreFriends(ctx, nil, a, b); friends {
		t.Fatal("friendship survived block")
	}
	blocked, _ := e.store.BlockedEither(ctx, nil, a, b)
	if !blocked {
		t.Fatal("block missing")
	}
	// b→a request now TARGET_BLOCKED.
	tx, _ := e.pool.Begin(ctx)
	defer tx.Rollback(ctx)
	rec, _ := buildRec(t, sociald.FriendRequestFamily, &protocolv1.C2SFriendRequest{
		OperationId: opBytes(), TargetCharacterId: a[:]}, bAcct, b)
	out, err := execs[sociald.FriendRequestFamily](ctx, tx, rec)
	if err != nil {
		t.Fatal(err)
	}
	if code := outcomeErrorCode(t, out); code != protocolv1.ErrorCode_ERROR_CODE_TARGET_BLOCKED {
		t.Fatalf("want TARGET_BLOCKED, got %v", code)
	}
	// A→C pending still live (block only severs the pair).
	var live int
	if err := e.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM friend_requests
		  WHERE requester_character_id=$1 AND target_character_id=$2
		    AND state='PENDING'`, a.String(), c.String()).Scan(&live); err != nil || live != 1 {
		t.Fatalf("unrelated pending live=%d err=%v", live, err)
	}
}

// TestChatRateLimitingChannels: per-channel budgets reject the first
// over-limit send and accept within budget; WHISPER budgets per target.
func TestChatRateLimitingChannels(t *testing.T) {
	l := newLimiter()
	a, b := id.NewV4(), id.NewV4()
	now := time.Now()
	// WORLD: 2/10s.
	for i := 0; i < 2; i++ {
		if !l.allowChat(protocolv1.ChatChannel_CHAT_CHANNEL_WORLD, a, id.UUID{}, now) {
			t.Fatalf("world %d denied", i)
		}
	}
	if l.allowChat(protocolv1.ChatChannel_CHAT_CHANNEL_WORLD, a, id.UUID{}, now) {
		t.Fatal("world third send allowed")
	}
	// LOCAL has its own window.
	if !l.allowChat(protocolv1.ChatChannel_CHAT_CHANNEL_LOCAL, a, id.UUID{}, now) {
		t.Fatal("local denied after world cap")
	}
	// WHISPER is per-target.
	for i := 0; i < 5; i++ {
		if !l.allowChat(protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER, a, b, now) {
			t.Fatalf("whisper %d denied", i)
		}
	}
	if l.allowChat(protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER, a, b, now) {
		t.Fatal("whisper 6th allowed")
	}
	if !l.allowChat(protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER, a, id.NewV4(), now) {
		t.Fatal("whisper to other target denied")
	}
	// Window rolls after 10s.
	if !l.allowChat(protocolv1.ChatChannel_CHAT_CHANNEL_WORLD, a, id.UUID{}, now.Add(11*time.Second)) {
		t.Fatal("world denied after window")
	}
}

// TestCanonicalTextNormalization: NFC canonical, trim, control/separator
// reject, 1..240 graphemes.
func TestCanonicalTextNormalization(t *testing.T) {
	if _, ok := sociald.CanonicalText("  hi  ", maxChatGraphemes); !ok {
		t.Fatal("plain text rejected")
	}
	if _, ok := sociald.CanonicalText("   ", maxChatGraphemes); ok {
		t.Fatal("blank accepted")
	}
	if _, ok := sociald.CanonicalText("a\x00b", maxChatGraphemes); ok {
		t.Fatal("control char accepted")
	}
	// NFC: precomposed é (U+00E9) equals e+combining accent input.
	got, ok := sociald.CanonicalText("é", maxChatGraphemes)
	if !ok {
		t.Fatal("combining accent rejected")
	}
	if got != "é" {
		t.Fatalf("NFC canonical = %q", got)
	}
	long := ""
	for i := 0; i < 241; i++ {
		long += "a"
	}
	if _, ok := sociald.CanonicalText(long, maxChatGraphemes); ok {
		t.Fatal("241 graphemes accepted")
	}
	long = ""
	for i := 0; i < 240; i++ {
		long += "a"
	}
	if _, ok := sociald.CanonicalText(long, maxChatGraphemes); !ok {
		t.Fatal("240 graphemes rejected")
	}
}

// buildRec marshals req into the family record builder for executor-level
// tests.
func buildRec(t *testing.T, family string, req any, accountID,
	charID id.UUID) (*journalv1.DurableCommandRecord, error) {
	t.Helper()
	now := time.Now().UTC()
	switch r := req.(type) {
	case *protocolv1.C2SFriendRequest:
		return sociald.FriendRequestRecord(accountID, charID, 1, 1, r, now)
	case *protocolv1.C2SFriendAccept:
		return sociald.FriendAcceptRecord(accountID, charID, 1, 1, r, now)
	case *protocolv1.C2SFriendDecline:
		return sociald.FriendDeclineRecord(accountID, charID, 1, 1, r, now)
	case *protocolv1.C2SFriendRemove:
		return sociald.FriendRemoveRecord(accountID, charID, 1, 1, r, now)
	case *protocolv1.C2SBlockAdd:
		return sociald.BlockAddRecord(accountID, charID, 1, 1, r, now)
	case *protocolv1.C2SBlockRemove:
		return sociald.BlockRemoveRecord(accountID, charID, 1, 1, r, now)
	case *protocolv1.C2SReportPlayer:
		return sociald.ReportRecord(accountID, charID, 1, 1, r, now)
	}
	return nil, fmt.Errorf("unhandled request %T", req)
}

// outcomeErrorCode decodes the retained JournalOutcome's error code.
func outcomeErrorCode(t *testing.T, out idempotency.Outcome) protocolv1.ErrorCode {
	t.Helper()
	o := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(out.Payload, o); err != nil {
		t.Fatalf("outcome decode: %v", err)
	}
	if cr := o.GetS2CSocialResult(); cr != nil {
		return cr.GetResult().GetErrorCode()
	}
	if cr := o.GetS2CReportPlayerResult(); cr != nil {
		return cr.GetResult().GetErrorCode()
	}
	return o.GetErrorCode()
}

// TestReportReasonsAndLimit: each canonical reason commits, a bad
// reason is TARGET_INVALID, the 11th report in 24h is RATE_LIMITED,
// unknown chat_message_id is ITEM_NOT_FOUND.
func TestReportReasonsAndLimit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	aAcct, bAcct := e.seedAccount(t), e.seedAccount(t)
	a := e.seedCharacter(t, aAcct, 5)
	b := e.seedCharacter(t, bAcct, 5)

	execs := sociald.Executors(e.store)
	report := func(reason protocolv1.ReportReason, chatID []byte, notes *string) protocolv1.ErrorCode {
		req := &protocolv1.C2SReportPlayer{
			OperationId:       opBytes(),
			TargetCharacterId: b[:],
			Reason:            reason,
		}
		if chatID != nil {
			req.ChatMessageId = chatID
		}
		if notes != nil {
			req.ReporterNotes = notes
		}
		rec, err := buildRec(t, sociald.ReportFamily, req, aAcct, a)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := e.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		out, err := execs[sociald.ReportFamily](ctx, tx, rec)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		return outcomeErrorCode(t, out)
	}
	for _, reason := range []protocolv1.ReportReason{
		protocolv1.ReportReason_REPORT_REASON_SPAM,
		protocolv1.ReportReason_REPORT_REASON_HARASSMENT,
		protocolv1.ReportReason_REPORT_REASON_HATE_OR_ABUSE,
		protocolv1.ReportReason_REPORT_REASON_CHEATING,
		protocolv1.ReportReason_REPORT_REASON_SCAM,
		protocolv1.ReportReason_REPORT_REASON_INAPPROPRIATE_NAME,
		protocolv1.ReportReason_REPORT_REASON_OTHER,
	} {
		if code := report(reason, nil, nil); code != 0 {
			t.Fatalf("reason %v rejected: %v", reason, code)
		}
	}
	if code := report(protocolv1.ReportReason_REPORT_REASON_UNSPECIFIED, nil, nil); code != protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("bad reason code=%v", code)
	}
	unknownChat := id.NewV4()
	if code := report(protocolv1.ReportReason_REPORT_REASON_SPAM, unknownChat[:], nil); code != protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND {
		t.Fatalf("unknown chat_message_id code=%v", code)
	}
	over := "x"
	_ = over
	// 7 committed + 2 failed = 9th accepted, 10th accepted, 11th limited.
	var n int
	if err := e.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM player_reports WHERE reporter_account_id=$1`,
		aAcct.String()).Scan(&n); err != nil {
		t.Fatal(err)
	}
	for n < 10 {
		if code := report(protocolv1.ReportReason_REPORT_REASON_OTHER, nil, nil); code != 0 {
			t.Fatalf("report %d rejected: %v", n+1, code)
		}
		n++
	}
	if code := report(protocolv1.ReportReason_REPORT_REASON_OTHER, nil, nil); code != protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED {
		t.Fatalf("11th report code=%v", code)
	}
}
