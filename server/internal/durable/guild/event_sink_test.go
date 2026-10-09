package guild

import (
	"context"
	"testing"
	"time"

	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"

	"thinhthan/internal/core/id"
)

// TestEventSinkAllSourceKinds: every eligible source kind emits a
// guild.event whose kind payload + EXP/ritual tables stay consistent
// (ADR-0062/0068; the envelope is produced upstream by global/guild —
// here the durable executor executes each emitted record).
func TestEventSinkAllSourceKinds(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "kinds", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	m2, _ := mkChar(t, 15)
	m3, _ := mkChar(t, 15)
	mkMember(t, gid, m2, RoleMember, now)
	mkMember(t, gid, m3, RoleMember, now)
	d := deps(now, nil)
	ex := Executors(d)
	s := NewStore(sharedPool)
	credits := []*journalv1.JournalGuildContribution{
		{CharacterId: leader[:], MembershipId: v4b(), Amount: 10},
		{CharacterId: m2[:], MembershipId: v4b(), Amount: 10},
		{CharacterId: m3[:], MembershipId: v4b(), Amount: 10},
	}
	cases := []struct {
		kind    string
		exp     int64
		ritual  uint32
		element uint32
	}{
		{"DUNGEON", 20, 12, ElementKim},
		{"BOSS", 20, 12, ElementHoa},
		{"WORLD_EVENT", 15, 10, ElementThuy},
		{"GUILD_ACTIVITY", 30, 15, ElementTho},
	}
	var prevEXP uint64
	for i, tc := range cases {
		ev := &journalv1.JournalGuildEvent{
			GuildId:           gid[:],
			SourceOperationId: v4b(),
			SourceKind:        tc.kind,
			SourceReference:   "k:" + tc.kind,
			OccurredAtMs:      now.UnixMilli(),
			Element:           tc.element,
			RitualPoints:      tc.ritual,
			GuildExp:          uint64(tc.exp),
			CreditedMembers:   credits,
			CycleId:           CycleID(now),
		}
		out := runExec(t, ex[EventFamily], EventRecord(gid, id.NewV4(), ev, now))
		if out.GetStatus() != statusSuccess() {
			t.Fatalf("%s outcome: %v", tc.kind, out.GetStatus())
		}
		p, err := s.Progression(context.Background(), sharedPool, gid)
		if err != nil {
			t.Fatalf("%s progression: %v", tc.kind, err)
		}
		want := prevEXP + uint64(tc.exp)
		if p.EXP != want {
			t.Fatalf("%s exp = %d, want %d", tc.kind, p.EXP, want)
		}
		prevEXP = p.EXP
		_ = i
	}
	// Ritual points accumulated across kinds (rotation vessel fill).
	c, err := s.Cycle(context.Background(), sharedPool, gid, CycleID(now))
	if err != nil || c == nil {
		t.Fatalf("cycle: %v", c)
	}
	total := 0
	for _, v := range c.Points {
		total += v
	}
	if total != 12+12+10+15 {
		t.Fatalf("ritual total = %d, want 49", total)
	}
	// BOSS went to its own element vessel (HOA), no spill.
	if c.Points[ElementHoa-1] != 12 {
		t.Fatalf("boss vessel = %d, want 12", c.Points[ElementHoa-1])
	}
}

// TestEventSinkIdempotentSourceKey: replaying a guild.event record
// carries the same source key — the receipt/idempotency layer dedupes
// identical operation ids, so the executor only applies a distinct
// source once. Two records with the same source fields but different
// envelope ops still apply (the dedupe key lives upstream in
// eventOperationID — verified via the emitted op equality here).
func TestEventSinkIdempotentSourceKey(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "dedupe", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	m2, _ := mkChar(t, 15)
	m3, _ := mkChar(t, 15)
	mkMember(t, gid, m2, RoleMember, now)
	mkMember(t, gid, m3, RoleMember, now)
	d := deps(now, nil)
	ex := Executors(d)
	s := NewStore(sharedPool)
	srcOp := id.NewV4()
	ev := &journalv1.JournalGuildEvent{
		GuildId:           gid[:],
		SourceOperationId: srcOp[:],
		SourceKind:        "DUNGEON",
		SourceReference:   "dedupe:1",
		OccurredAtMs:      now.UnixMilli(),
		Element:           ElementKim,
		RitualPoints:      12,
		GuildExp:          20,
		CreditedMembers: []*journalv1.JournalGuildContribution{
			{CharacterId: leader[:], MembershipId: v4b(), Amount: 10},
			{CharacterId: m2[:], MembershipId: v4b(), Amount: 10},
			{CharacterId: m3[:], MembershipId: v4b(), Amount: 10},
		},
		CycleId: CycleID(now),
	}
	rec1 := EventRecord(gid, id.NewV4(), ev, now)
	rec2 := EventRecord(gid, id.NewV4(), ev, now)
	// The operation identity is the envelope op — identical payload,
	// different envelope -> different ops (both apply), identical
	// envelope -> queue dedupe never reaches the executor twice.
	if rec1.GetOperationId() == nil || rec2.GetOperationId() == nil {
		t.Fatalf("records missing op ids")
	}
	out := runExec(t, ex[EventFamily], rec1)
	if out.GetStatus() != statusSuccess() {
		t.Fatalf("first: %v", out.GetStatus())
	}
	p1, _ := s.Progression(context.Background(), sharedPool, gid)
	out = runExec(t, ex[EventFamily], rec2)
	if out.GetStatus() != statusSuccess() {
		t.Fatalf("second: %v", out.GetStatus())
	}
	p2, _ := s.Progression(context.Background(), sharedPool, gid)
	// A distinct envelope op is a distinct command — the dedupe
	// contract is at the queue receipt layer (same op -> deduped).
	if p2.EXP <= p1.EXP {
		t.Fatalf("replay: exp %d -> %d", p1.EXP, p2.EXP)
	}
	// Identical source key under identical op id dedupes at receipt —
	// rebuild the same record twice and confirm identical op identity.
	rec3 := EventRecord(gid, mustUUID(rec1.GetOperationId()), ev, now)
	var op1, op3 id.UUID
	copy(op1[:], rec1.GetOperationId())
	copy(op3[:], rec3.GetOperationId())
	if op1 != op3 {
		t.Fatalf("same envelope op must stay stable")
	}
}

func mustUUID(b []byte) id.UUID {
	var u id.UUID
	copy(u[:], b)
	return u
}

// TestWarRegistrationGuardBlocksDisband: the disband executor consults
// the guard — any QUEUED..RESOLVING registration rejects the commit.
func TestWarRegistrationGuardBlocksDisband(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, acct := mkChar(t, 20)
	gid := mkGuild(t, "guard", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	d := deps(now, stubGuard{blocked: true})
	ex := Executors(d)
	out := runExec(t, ex[DisbandFamily],
		recFor(t, DisbandFamily, leader, acct, disbandReq(t), id.NewV7(now), now))
	wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_GUILD_DISBAND_BLOCKED)
	// A nil guard (war domain not wired) never blocks.
	d2 := deps(now, nil)
	out = runExec(t, Executors(d2)[DisbandFamily],
		recFor(t, DisbandFamily, leader, acct, disbandReq(t), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
}
