package reward

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

func restExp(t *testing.T, charID id.UUID) (level, exp, ticks int64) {
	t.Helper()
	if err := pool(t).QueryRow(context.Background(),
		`SELECT c.level, c.current_exp,
		        COALESCE(d.rest_ticks_gained, 0)
		   FROM characters c
		   LEFT JOIN character_rest_daily d
		     ON d.character_id = c.character_id AND d.utc_date = CURRENT_DATE
		  WHERE c.character_id = $1`, charID.String()).Scan(&level, &exp, &ticks); err != nil {
		t.Fatalf("rest state: %v", err)
	}
	return level, exp, ticks
}

// TestRestTickGrant: each bonfire-rest tick intent grants the slot's
// rest_exp and bumps rest_ticks_gained atomically under the character
// lock.
func TestRestTickGrant(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	prog := progression.NewStore(pool(t))
	ex := RestExecutor(d.Store, prog)

	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := ex(ctx, tx, rewardRecord(char, newOp(), RestKind, 1000))
		if err != nil {
			t.Fatalf("exec: %v", err)
		}
		out := outcome(t, o)
		if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("status %v", out.GetStatus())
		}
	})
	level, exp, ticks := restExp(t, char)
	if exp != 1000 || ticks != 1 || level != 1 {
		t.Fatalf("level=%d exp=%d ticks=%d", level, exp, ticks)
	}
	// Multi-slot command: each slot consumes one tick.
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		rec := rewardRecord(char, newOp(), RestKind, 500)
		rec.GetReward().Slots = append(rec.GetReward().GetSlots(),
			rec.GetReward().GetSlots()[0])
		if _, err := ex(ctx, tx, rec); err != nil {
			t.Fatalf("exec2: %v", err)
		}
	})
	_, exp, ticks = restExp(t, char)
	if exp != 2000 || ticks != 3 {
		t.Fatalf("exp=%d ticks=%d want 2000/3", exp, ticks)
	}
}

// TestRestDailyCapNoGrant: a tick at the 180/day cap commits the
// journal row but grants nothing — enforced inside the settlement tx,
// never by skipping the row.
func TestRestDailyCapNoGrant(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_rest_daily (character_id, utc_date, rest_ticks_gained)
		 VALUES ($1, CURRENT_DATE, 180)`, char.String()); err != nil {
		t.Fatalf("seed cap: %v", err)
	}
	prog := progression.NewStore(pool(t))
	ex := RestExecutor(d.Store, prog)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		o, err := ex(ctx, tx, rewardRecord(char, newOp(), RestKind, 1000))
		if err != nil {
			t.Fatalf("exec at cap: %v", err)
		}
		if outcome(t, o).GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("cap tick must commit success")
		}
	})
	_, exp, ticks := restExp(t, char)
	if exp != 0 || ticks != 180 {
		t.Fatalf("exp=%d ticks=%d — cap granted value", exp, ticks)
	}
}

// TestRestReplayIdempotent: a replayed settlement record returns the
// committed outcome through TrustedReplay — the tick is not granted
// twice (operation_id dedup at the durable boundary).
func TestRestReplayIdempotent(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	prog := progression.NewStore(pool(t))
	ex := RestExecutor(d.Store, prog)
	rec := rewardRecord(char, newOp(), RestKind, 700)

	out1 := replay(t, ex, rec)
	out2 := replay(t, ex, rec)
	if out1.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
		out2.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("replay status %v/%v", out1.GetStatus(), out2.GetStatus())
	}
	_, exp, ticks := restExp(t, char)
	if exp != 700 || ticks != 1 {
		t.Fatalf("replay granted twice: exp=%d ticks=%d", exp, ticks)
	}
}

// TestRestKindMuxFailClosed: a JournalRewardCommand with an
// unregistered kind resolves terminally through the mux — never
// silently re-executed (save_rules.md §7 fail-closed).
func TestRestKindMuxFailClosed(t *testing.T) {
	d := newDeps(t)
	acct := mkAccount(t)
	char := mkCharacter(t, acct)
	prog := progression.NewStore(pool(t))
	mux := KindMux(map[string]queue.Executor{RestKind: RestExecutor(d.Store, prog)})
	rec := rewardRecord(char, newOp(), "beast", 100)
	tx(t, func(ctx context.Context, tx pgx.Tx) {
		if _, err := mux(ctx, tx, rec); err == nil {
			t.Fatalf("unknown kind must fail closed")
		}
	})
	if _, _, ticks := restExp(t, char); ticks != 0 {
		t.Fatalf("unknown kind mutated rest_ticks")
	}
}
