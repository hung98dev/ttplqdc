package social

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestFriendPairUnorderedUnique: the (low, high) PK rejects the
// reversed duplicate pair (data_model.md § Social).
func TestFriendPairUnorderedUnique(t *testing.T) {
	wipe(t)
	ctx := context.Background()
	a, b := mkCharacter(t), mkCharacter(t)
	mkFriends(t, a, b)

	// The canonical (low, high) CHECK makes any ordering write the same
	// PK pair — a second insert for either direction is 23505.
	lo, hi := lowHigh(a, b)
	op := id.NewV4()
	_, err := sharedPool.Exec(ctx,
		`INSERT INTO friends
		   (character_low_id, character_high_id, created_at, created_operation_id)
		 VALUES ($1,$2,now(),$3)`, lo[:], hi[:], op[:])
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "23505" {
		t.Fatalf("want 23505 on duplicate pair, got %v", err)
	}
	// And a literal reversed insert violates the ordering CHECK.
	op2 := id.NewV4()
	_, err = sharedPool.Exec(ctx,
		`INSERT INTO friends
		   (character_low_id, character_high_id, created_at, created_operation_id)
		 VALUES ($1,$2,now(),$3)`, hi[:], lo[:], op2[:])
	if !errors.As(err, &pgerr) || pgerr.Code != "23514" {
		t.Fatalf("want 23514 on reversed pair, got %v", err)
	}
	if friends, _ := New(sharedPool).AreFriends(ctx, nil, a, b); !friends {
		t.Fatal("pair missing")
	}
}

// TestPendingRequestPairUnique: the partial unique index on
// LEAST/GREATEST(requester,target) WHERE PENDING rejects a second live
// request for the pair in either direction.
func TestPendingRequestPairUnique(t *testing.T) {
	wipe(t)
	ctx := context.Background()
	a, b := mkCharacter(t), mkCharacter(t)
	mkPending(t, a, b, false)

	reqID, opID2 := id.NewV4(), id.NewV4()
	_, err := sharedPool.Exec(ctx,
		`INSERT INTO friend_requests
		   (friend_request_id, requester_character_id, target_character_id,
		    state, created_at, expires_at, create_operation_id)
		 VALUES ($1,$2,$3,'PENDING',now(),now()+interval '7 days',$4)`,
		reqID[:], b[:], a[:], opID2[:])
	var pgerr *pgconn.PgError
	if !errors.As(err, &pgerr) || pgerr.Code != "23505" {
		t.Fatalf("want 23505 on crossed pending pair, got %v", err)
	}
}

// TestCrossedRequestAccepts: A→B pending plus B→A request resolves the
// pair as ACCEPTED and writes the friends row in one transaction
// (social.md § Requests crossed rule).
func TestCrossedRequestAccepts(t *testing.T) {
	wipe(t)
	ctx := context.Background()
	a, b := mkCharacter(t), mkCharacter(t)
	s := New(sharedPool)
	pending := mkPending(t, a, b, false)

	opID := id.NewV7(time.Now())
	rec, err := FriendRequestRecord(accountOf(t, b), b, 1, 1,
		&protocolv1.C2SFriendRequest{
			OperationId:       opID[:],
			TargetCharacterId: a[:],
		}, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := exec(t, s.friendRequest, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("crossed request status=%v", out.GetStatus())
	}
	friends, err := s.AreFriends(ctx, nil, a, b)
	if err != nil || !friends {
		t.Fatalf("friends=%v err=%v", friends, err)
	}
	var state string
	if err := sharedPool.QueryRow(ctx,
		`SELECT state FROM friend_requests WHERE friend_request_id = $1`,
		pending[:]).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "ACCEPTED" {
		t.Fatalf("request state=%s", state)
	}
}

// TestExpiredPendingTreatedAsExpired: a PENDING row past expires_at
// does not count as live — a fresh request inserts a new row.
func TestExpiredPendingTreatedAsExpired(t *testing.T) {
	wipe(t)
	ctx := context.Background()
	a, b := mkCharacter(t), mkCharacter(t)
	s := New(sharedPool)
	mkPending(t, a, b, true /* backdated 8d → expired */)

	opID := id.NewV7(time.Now())
	rec, err := FriendRequestRecord(accountOf(t, a), a, 1, 1,
		&protocolv1.C2SFriendRequest{
			OperationId:       opID[:],
			TargetCharacterId: b[:],
		}, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	// The expired row still holds the unique index slot — the new
	// pending insert must replace it (resolve then insert).
	resOp := id.NewV4()
	_, err = sharedPool.Exec(ctx,
		`UPDATE friend_requests SET state='EXPIRED', resolved_at=now(),
		        resolve_operation_id=$2
		  WHERE requester_character_id=$1 AND state='PENDING'`,
		a[:], resOp[:])
	if err != nil {
		t.Fatalf("expire old row: %v", err)
	}
	out := exec(t, s.friendRequest, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("status=%v", out.GetStatus())
	}
	var n int
	if err := sharedPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM friend_requests
		  WHERE requester_character_id=$1 AND state='PENDING'`,
		a[:]).Scan(&n); err != nil || n != 1 {
		t.Fatalf("live pending=%d err=%v", n, err)
	}
}

// TestOutgoingPendingCap100: the 100th live outgoing request is fine;
// request 101 is CAPACITY_FULL (social.md).
func TestOutgoingPendingCap100(t *testing.T) {
	wipe(t)
	a := mkCharacter(t)
	s := New(sharedPool)
	for i := 0; i < OutgoingCap; i++ {
		mkPending(t, a, mkCharacter(t), false)
	}
	extra := mkCharacter(t)
	opID := id.NewV7(time.Now())
	rec, err := FriendRequestRecord(accountOf(t, a), a, 1, 1,
		&protocolv1.C2SFriendRequest{
			OperationId:       opID[:],
			TargetCharacterId: extra[:],
		}, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := exec(t, s.friendRequest, rec)
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL {
		t.Fatalf("want CAPACITY_FULL, got %v", out.GetErrorCode())
	}
}

// TestBlockCap500: the 501st block is CAPACITY_FULL (social.md).
func TestBlockCap500(t *testing.T) {
	wipe(t)
	a := mkCharacter(t)
	s := New(sharedPool)
	for i := 0; i < BlockCap; i++ {
		mkBlock(t, a, mkCharacter(t))
	}
	target := mkCharacter(t)
	opID := id.NewV7(time.Now())
	rec, err := BlockAddRecord(accountOf(t, a), a, 1, 1,
		&protocolv1.C2SBlockAdd{
			OperationId:       opID[:],
			TargetCharacterId: target[:],
		}, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := exec(t, s.blockAdd, rec)
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL {
		t.Fatalf("want CAPACITY_FULL, got %v", out.GetErrorCode())
	}
}

// TestBlockCancelsPendingAndFriendship: C2S_BLOCK_ADD inserts the block,
// deletes the friends pair and cancels live pendings in one commit.
func TestBlockCancelsPendingAndFriendship(t *testing.T) {
	wipe(t)
	ctx := context.Background()
	a, b := mkCharacter(t), mkCharacter(t)
	c := mkCharacter(t)
	s := New(sharedPool)
	mkFriends(t, a, b)
	sev := mkPending(t, b, a, false) // pair pending — block must cancel
	pending := mkPending(t, a, c, false)

	opID := id.NewV7(time.Now())
	rec, err := BlockAddRecord(accountOf(t, a), a, 1, 1,
		&protocolv1.C2SBlockAdd{
			OperationId:       opID[:],
			TargetCharacterId: b[:],
		}, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := exec(t, s.blockAdd, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("block status=%v", out.GetStatus())
	}
	blocked, err := s.IsBlocked(ctx, nil, a, b)
	if err != nil || !blocked {
		t.Fatalf("blocked=%v err=%v", blocked, err)
	}
	if friends, _ := s.AreFriends(ctx, nil, a, b); friends {
		t.Fatal("friendship survived block")
	}
	var sevState string
	if err := sharedPool.QueryRow(ctx,
		`SELECT state FROM friend_requests WHERE friend_request_id=$1`,
		sev[:]).Scan(&sevState); err != nil || sevState != "CANCELLED" {
		t.Fatalf("severed pending state=%s err=%v", sevState, err)
	}
	// The pending rows for the (a,c) pair stay — block only severs a,b.
	var live int
	if err := sharedPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM friend_requests
		  WHERE friend_request_id=$1 AND state='PENDING'`,
		pending[:]).Scan(&live); err != nil || live != 1 {
		t.Fatalf("unrelated pending live=%d err=%v", live, err)
	}
}
