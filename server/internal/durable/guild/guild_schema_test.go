package guild

import (
	"context"
	"testing"
	"time"

	"thinhthan/internal/core/id"
)

// TestSingleLeaderIndex pins the partial unique index: at most one
// LEADER row per guild (ADR-0065).
func TestSingleLeaderIndex(t *testing.T) {
	wipe(t)
	ctx := context.Background()
	now := time.Now().UTC()
	l1, _ := mkChar(t, 20)
	l2, _ := mkChar(t, 20)
	gid := mkGuild(t, "leader-index", l1, 1, 0, now)
	mkMember(t, gid, l1, RoleLeader, now)
	mkMember(t, gid, l2, RoleMember, now)
	if _, err := sharedPool.Exec(ctx,
		`UPDATE guild_memberships SET role = 'LEADER' WHERE character_id = $1`,
		l2[:]); err == nil {
		t.Fatalf("second LEADER accepted")
	}
}

// TestContributionSurvivesLeave: leave ends the membership interval
// but lifetime/cycle contribution rows persist (ADR-0065).
func TestContributionSurvivesLeave(t *testing.T) {
	wipe(t)
	ctx := context.Background()
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	member, acct := mkChar(t, 15)
	_ = acct
	gid := mkGuild(t, "contrib-survives", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	mkMember(t, gid, member, RoleMember, now)
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guild_member_contributions (guild_id, character_id, lifetime_contribution, cycle_id, cycle_contribution)
		 VALUES ($1,$2,55,'2026-09-28',55)`, gid[:], member[:]); err != nil {
		t.Fatalf("contribution seed: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`DELETE FROM guild_memberships WHERE character_id = $1`, member[:]); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`UPDATE guild_membership_history SET left_at = now()
		 WHERE character_id = $1 AND left_at IS NULL`, member[:]); err != nil {
		t.Fatalf("interval end: %v", err)
	}
	var lifetime, cycle int64
	if err := sharedPool.QueryRow(ctx,
		`SELECT lifetime_contribution, cycle_contribution
		 FROM guild_member_contributions WHERE guild_id = $1 AND character_id = $2`,
		gid[:], member[:]).Scan(&lifetime, &cycle); err != nil {
		t.Fatalf("contribution row gone: %v", err)
	}
	if lifetime != 55 || cycle != 55 {
		t.Fatalf("contribution = %d/%d, want 55/55", lifetime, cycle)
	}
}

// TestOneVotePerAccount pins the (guild,cycle,account) PK — a second
// ballot from the same account rejects even on a different character.
func TestOneVotePerAccount(t *testing.T) {
	wipe(t)
	ctx := context.Background()
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "vote-once", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	a, account := mkChar(t, 15)
	b, _ := mkChar(t, 15)
	// Same account: move b onto a's account to exercise the PK.
	if _, err := sharedPool.Exec(ctx,
		`UPDATE characters SET account_id = $1 WHERE character_id = $2`,
		account[:], b[:]); err != nil {
		t.Fatalf("account move: %v", err)
	}
	cycle := "2026-09-28"
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guild_ritual_cycles
		 (guild_id, cycle_id, m_effective, required_points_per_element,
		  rotation_pointer, candidate_blessing_ids, vote_closes_at)
		 VALUES ($1,$2,5,180,'KIM',$3,$4)`,
		gid[:], cycle, []string{"guild.blessing.advancement"},
		now.Add(time.Hour)); err != nil {
		t.Fatalf("cycle seed: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guild_blessing_votes (guild_id, cycle_id, account_id, character_id, blessing_id, voted_at)
		 VALUES ($1,$2,$3,$4,'guild.blessing.advancement',$5)`,
		gid[:], cycle, account[:], a[:], now); err != nil {
		t.Fatalf("first vote: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guild_blessing_votes (guild_id, cycle_id, account_id, character_id, blessing_id, voted_at)
		 VALUES ($1,$2,$3,$4,'guild.blessing.advancement',$5)`,
		gid[:], cycle, account[:], b[:], now); err == nil {
		t.Fatalf("second vote accepted")
	}
}

// TestDisbandedNameNotReusable: a disbanded guild's name_key stays
// claimed forever — a new create with the same canonical name rejects
// GUILD_NAME_TAKEN (guild.md § Identity).
func TestDisbandedNameNotReusable(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, acct := mkChar(t, 30)
	mkFunds(t, leader, 100_000)
	gid := mkGuild(t, "Eternal Name", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	d := deps(now, nil)
	ex := Executors(d)
	rec := recFor(t, DisbandFamily, leader, acct,
		disbandReq(t), id.NewV7(now), now)
	out := runExec(t, ex[DisbandFamily], rec)
	wantStatus(t, out, statusSuccess())
	if g := guildRow(t, gid); g.State != "DISBANDED" {
		t.Fatalf("state = %s", g.State)
	}
	fresh, facct := mkChar(t, 30)
	mkFunds(t, fresh, 100_000)
	rec2 := recFor(t, CreateFamily, fresh, facct,
		createReq(t, "ETERNAL NAME"), id.NewV7(now), now)
	out2 := runExec(t, ex[CreateFamily], rec2)
	wantError(t, out2, errorTaken())
}
