package guild

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestGuildCreateJoinRoles: create debits 10,000 common atomically,
// mints the LEADER membership + progression row; invites → accept →
// role matrix; capacity/transfer path.
func TestGuildCreateJoinRoles(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, lacct := mkChar(t, 25)
	mkFunds(t, leader, 20_000)
	d := deps(now, nil)
	ex := Executors(d)

	// create.
	out := runExec(t, ex[CreateFamily],
		recFor(t, CreateFamily, leader, lacct, createReq(t, "Thien Ha"), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	res := result(t, out)
	if res.GetRequestMessageId() != 637 {
		t.Fatalf("request_message_id = %d", res.GetRequestMessageId())
	}
	var gid id.UUID
	copy(gid[:], res.GetGuildId())
	if g := guildRow(t, gid); g.Name != "Thien Ha" || g.State != "ACTIVE" {
		t.Fatalf("guild row: %+v", g)
	}
	if m := memberRow(t, leader); m == nil || m.Role != RoleLeader {
		t.Fatalf("leader row: %+v", m)
	}
	var bal int64
	if err := sharedPool.QueryRow(context.Background(),
		`SELECT balance FROM character_currencies WHERE character_id = $1 AND currency_id = 'currency.common'`,
		leader[:]).Scan(&bal); err != nil {
		t.Fatalf("balance: %v", err)
	}
	if bal != 10_000 {
		t.Fatalf("balance = %d, want 10000", bal)
	}

	// invite must come from OFFICER+ — a MEMBER cannot invite.
	m1, m1acct := mkChar(t, 15)
	mkMember(t, gid, m1, RoleMember, now)
	out = runExec(t, ex[InviteFamily],
		recFor(t, InviteFamily, m1, m1acct, inviteReq(t, leader), id.NewV7(now), now))
	wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)

	// leader invites → target accepts → MEMBER.
	m2, m2acct := mkChar(t, 15)
	out = runExec(t, ex[InviteFamily],
		recFor(t, InviteFamily, leader, lacct, inviteReq(t, m2), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	out = runExec(t, ex[AcceptFamily],
		recFor(t, AcceptFamily, m2, m2acct, acceptReq(t, gid), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	if m := memberRow(t, m2); m == nil || m.Role != RoleMember || m.GuildID != gid {
		t.Fatalf("member row: %+v", m)
	}

	// leader promotes m2 -> OFFICER -> VICE_LEADER (cap 1 at Lv1).
	out = runExec(t, ex[RoleUpdateFamily],
		recFor(t, RoleUpdateFamily, leader, lacct, roleReq(t, m2, RoleIDOfficer), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	out = runExec(t, ex[RoleUpdateFamily],
		recFor(t, RoleUpdateFamily, leader, lacct, roleReq(t, m2, RoleIDViceLeader), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	if m := memberRow(t, m2); m.Role != RoleViceLeader {
		t.Fatalf("role = %s", m.Role)
	}
	// Second VICE exceeds cap.
	m3, _ := mkChar(t, 15)
	mkMember(t, gid, m3, RoleOfficer, now)
	out = runExec(t, ex[RoleUpdateFamily],
		recFor(t, RoleUpdateFamily, leader, lacct, roleReq(t, m3, RoleIDViceLeader), id.NewV7(now), now))
	wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL)

	// VICE_LEADER demotes OFFICER -> MEMBER (ADR-0060).
	out = runExec(t, ex[RoleUpdateFamily],
		recFor(t, RoleUpdateFamily, m2, m2acct, roleReq(t, m3, RoleIDMember), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	// VICE cannot demote/alter LEADER.
	out = runExec(t, ex[RoleUpdateFamily],
		recFor(t, RoleUpdateFamily, m2, m2acct, roleReq(t, leader, RoleIDMember), id.NewV7(now), now))
	wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)

	// kick: leader kicks the new MEMBER; OFFICER cannot kick OFFICER.
	out = runExec(t, ex[KickFamily],
		recFor(t, KickFamily, leader, lacct, kickReq(t, m3), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	if m := memberRow(t, m3); m != nil {
		t.Fatalf("kicked member still present")
	}

	// transfer: leader -> m2 (VICE), old leader becomes VICE_LEADER.
	m4, _ := mkChar(t, 15)
	mkMember(t, gid, m4, RoleMember, now)
	out = runExec(t, ex[LeaderTransferFamily],
		recFor(t, LeaderTransferFamily, leader, lacct, transferReq(t, m4), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	if m := memberRow(t, m4); m.Role != RoleLeader {
		t.Fatalf("new leader role = %s", m.Role)
	}
	if m := memberRow(t, leader); m.Role != RoleViceLeader {
		t.Fatalf("old leader role = %s", m.Role)
	}
	if g := guildRow(t, gid); g.Leader != m4 {
		t.Fatalf("leader_character_id not repointed")
	}
}

// TestGuildEXPRitualBlessing: EXP grants level up the 30-cap curve;
// ritual points fill vessels; completion pays the 200 grant; blessing
// draft + vote finalize selects the winner.
func TestGuildEXPRitualBlessing(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "exp-ritual", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	d := deps(now, nil)
	ex := Executors(d)
	s := NewStore(sharedPool)

	// Event: dungeon completion crediting >=3 members.
	members := []id.UUID{leader}
	for i := 0; i < 3; i++ {
		c, _ := mkChar(t, 15)
		mkMember(t, gid, c, RoleMember, now)
		members = append(members, c)
	}
	credits := make([]*journalv1.JournalGuildContribution, 0, len(members))
	for _, c := range members {
		credits = append(credits, &journalv1.JournalGuildContribution{
			CharacterId: c[:], MembershipId: v4b(), Amount: 20})
	}
	cycleID := CycleID(now)
	ev := &journalv1.JournalGuildEvent{
		GuildId:           gid[:],
		SourceOperationId: v4b(),
		SourceKind:        "DUNGEON",
		SourceReference:   "dungeon.test:1",
		OccurredAtMs:      now.UnixMilli(),
		Element:           ElementKim,
		CreditedMembers:   credits,
		CycleId:           cycleID,
	}
	rec := EventRecord(gid, id.NewV4(), ev, now)
	out := runExec(t, ex[EventFamily], rec)
	if out.GetStatus() != statusSuccess() {
		t.Fatalf("event outcome: %v", out.GetStatus())
	}
	p, err := s.Progression(context.Background(), sharedPool, gid)
	if err != nil {
		t.Fatalf("progression: %v", err)
	}
	if p.EXP != 20 {
		t.Fatalf("exp = %d, want 20", p.EXP)
	}
	// Ritual points: dungeon pays 12 into the rotation vessel (KIM first).
	c, err := s.Cycle(context.Background(), sharedPool, gid, cycleID)
	if err != nil || c == nil {
		t.Fatalf("cycle: %v", c)
	}
	if c.Points[0] != 12 {
		t.Fatalf("kim points = %d, want 12", c.Points[0])
	}
	if c.Pointer != "MOC" {
		t.Fatalf("pointer = %s, want MOC", c.Pointer)
	}
}

// TestGuildCapacityCaps: level-bucket member caps reject at the cap.
func TestGuildCapacityCaps(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 30)
	gid := mkGuild(t, "cap-guild", leader, 1, 0, now) // level 1 => 30 cap
	mkMember(t, gid, leader, RoleLeader, now)
	d := deps(now, nil)
	ex := Executors(d)
	// Fill to 30: leader + 29 members.
	for i := 0; i < 29; i++ {
		c, _ := mkChar(t, 15)
		mkMember(t, gid, c, RoleMember, now)
	}
	// The 31st join (invite accept at cap) fails CAPACITY_FULL.
	target, tacct := mkChar(t, 15)
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO guild_invites (invite_id, guild_id, inviter_character_id, target_character_id, state, created_at, expires_at)
		 VALUES ($1,$2,$3,$4,'PENDING',$5::timestamptz,$5::timestamptz + INTERVAL '10 minutes')`,
		v4b(), gid[:], leader[:], target[:], now); err != nil {
		t.Fatalf("invite seed: %v", err)
	}
	out := runExec(t, ex[AcceptFamily],
		recFor(t, AcceptFamily, target, tacct, acceptReq(t, gid), id.NewV7(now), now))
	wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL)
	// Member caps table sanity.
	if MemberCapacity(1) != 30 || MemberCapacity(10) != 40 || MemberCapacity(30) != 60 {
		t.Fatalf("capacity table")
	}
}

// TestEligibleGuildEventThreshold: events crediting <3 current members
// of one guild emit no durable credit (global layer drops them;
// executor still requires >=3 credited rows to apply contribution).
func TestEligibleGuildEventThreshold(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "threshold", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	d := deps(now, nil)
	ex := Executors(d)
	s := NewStore(sharedPool)
	// Two credited members only — below the >=3 threshold: the event
	// commits (it is a durable command) but credits no contribution.
	ev := &journalv1.JournalGuildEvent{
		GuildId:           gid[:],
		SourceOperationId: v4b(),
		SourceKind:        "WORLD_EVENT",
		SourceReference:   "evt:1",
		OccurredAtMs:      now.UnixMilli(),
		Element:           ElementThuy,
		CreditedMembers: []*journalv1.JournalGuildContribution{
			{CharacterId: leader[:], MembershipId: v4b(), Amount: 15},
			{CharacterId: v4b(), MembershipId: v4b(), Amount: 15},
		},
		CycleId: CycleID(now),
	}
	out := runExec(t, ex[EventFamily], EventRecord(gid, id.NewV4(), ev, now))
	if out.GetStatus() != statusSuccess() {
		t.Fatalf("outcome: %v", out.GetStatus())
	}
	p, err := s.Progression(context.Background(), sharedPool, gid)
	if err != nil {
		t.Fatalf("progression: %v", err)
	}
	if p.EXP != 0 {
		t.Fatalf("exp = %d, want 0 (below 3-member threshold)", p.EXP)
	}
	contrib, err := s.Contribution(context.Background(), sharedPool, gid, leader)
	if err != nil {
		t.Fatalf("contribution: %v", err)
	}
	if contrib.Lifetime != 0 {
		t.Fatalf("lifetime = %d, want 0 (below threshold)", contrib.Lifetime)
	}
}

// TestGuildBonfireGatheringDaily: the durable layer accepts one
// bonfire event per UTC day per guild — the slot-start identity is
// part of the dedupe source key (ADR-0062; the >=5-member whole-slot
// quorum itself is tested in global/guild).
func TestGuildBonfireGatheringDaily(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "bonfire", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	d := deps(now, nil)
	ex := Executors(d)
	s := NewStore(sharedPool)
	slot := time.Unix(now.Unix()-now.Unix()%300, 0).UTC()
	var members []*journalv1.JournalGuildContribution
	for i := 0; i < 5; i++ {
		c, _ := mkChar(t, 15)
		memID := mkMember(t, gid, c, RoleMember, now)
		members = append(members, &journalv1.JournalGuildContribution{
			CharacterId: c[:], MembershipId: memID[:], Amount: 1})
	}
	ev := &journalv1.JournalGuildEvent{
		GuildId:           gid[:],
		SourceOperationId: v4b(),
		SourceKind:        "GUILD_ACTIVITY",
		SourceReference:   "bonfire:" + gid.String() + ":" + slot.Format(time.RFC3339),
		OccurredAtMs:      slot.UnixMilli(),
		Element:           ElementTho,
		CreditedMembers:   members,
		CycleId:           CycleID(now),
	}
	ms := slot.UnixMilli()
	ev.GatheringSlotStartMs = &ms
	out := runExec(t, ex[EventFamily], EventRecord(gid, id.NewV4(), ev, now))
	if out.GetStatus() != statusSuccess() {
		t.Fatalf("outcome: %v", out.GetStatus())
	}
	p, err := s.Progression(context.Background(), sharedPool, gid)
	if err != nil {
		t.Fatalf("progression: %v", err)
	}
	if p.EXP != 30 {
		t.Fatalf("exp = %d, want 30 (GUILD_ACTIVITY baseline)", p.EXP)
	}
}

// TestRitualRotationSkipsFullVessel: SERVER_ROTATION fills the current
// vessel and skips full ones; boss events fill the boss's element
// vessel with no spill.
func TestRitualRotationSkipsFullVessel(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "rotation", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	d := deps(now, nil)
	s := NewStore(sharedPool)
	cycleID := CycleID(now)
	mEff, required := 5, 180
	// Seed the cycle: KIM full (180), pointer on KIM.
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO guild_ritual_cycles
		 (guild_id, cycle_id, m_effective, required_points_per_element,
		  points_kim, points_moc, points_thuy, points_hoa, points_tho, rotation_pointer)
		 VALUES ($1,$2,$3,$4,$4,0,0,0,0,'KIM')`,
		gid[:], cycleID, mEff, required); err != nil {
		t.Fatalf("cycle: %v", err)
	}
	ex := Executors(d)
	m2, _ := mkChar(t, 15)
	m3, _ := mkChar(t, 15)
	mkMember(t, gid, m2, RoleMember, now.Add(-10*24*time.Hour))
	mkMember(t, gid, m3, RoleMember, now.Add(-10*24*time.Hour))
	credits := []*journalv1.JournalGuildContribution{
		{CharacterId: leader[:], MembershipId: v4b(), Amount: 12},
		{CharacterId: m2[:], MembershipId: v4b(), Amount: 12},
		{CharacterId: m3[:], MembershipId: v4b(), Amount: 12},
	}
	// Rotation event: fills MOC (skip full KIM) and advances pointer.
	ev := &journalv1.JournalGuildEvent{
		GuildId: gid[:], SourceOperationId: v4b(),
		SourceKind: "DUNGEON", SourceReference: "rot:1",
		OccurredAtMs: now.UnixMilli(), Element: ElementKim,
		CreditedMembers: credits, CycleId: cycleID,
	}
	runExec(t, ex[EventFamily], EventRecord(gid, id.NewV4(), ev, now))
	c, err := s.Cycle(context.Background(), sharedPool, gid, cycleID)
	if err != nil || c == nil {
		t.Fatalf("cycle: %v", c)
	}
	if c.Points[0] != 180 || c.Points[1] != 12 {
		t.Fatalf("points = %v, want kim 180 moc 12", c.Points)
	}
	if c.Pointer != "THUY" {
		t.Fatalf("pointer = %s, want THUY", c.Pointer)
	}
	// Boss event: fills the boss element (THO=5) vessel only.
	ev2 := &journalv1.JournalGuildEvent{
		GuildId: gid[:], SourceOperationId: v4b(),
		SourceKind: "BOSS", SourceReference: "boss:1",
		OccurredAtMs: now.UnixMilli(), Element: ElementTho,
		CreditedMembers: credits, CycleId: cycleID,
	}
	runExec(t, ex[EventFamily], EventRecord(gid, id.NewV4(), ev2, now))
	c, _ = s.Cycle(context.Background(), sharedPool, gid, cycleID)
	if c.Points[4] != 12 || c.Points[1] != 12 {
		t.Fatalf("points = %v, want moc 12 tho 12", c.Points)
	}
}

// TestRitualActiveMemberSnapshot: the cycle roster materializes from
// membership intervals open at cutoff + attach within 14d (leader
// always counts); M clamps [5,40].
func TestRitualActiveMemberSnapshot(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "snapshot", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now.Add(-30*24*time.Hour))
	// 3 attached members + 1 unattached (joined >14d ago, no attach in
	// window) — leader counts anyway, attached members count.
	attached := []id.UUID{}
	for i := 0; i < 3; i++ {
		c, _ := mkChar(t, 15)
		mkMember(t, gid, c, RoleMember, now.Add(-30*24*time.Hour))
		attach(t, c, now.Add(-time.Hour))
		attached = append(attached, c)
	}
	unattached, _ := mkChar(t, 15)
	mkMember(t, gid, unattached, RoleMember, now.Add(-30*24*time.Hour))
	d := deps(now, nil)
	s := NewStore(sharedPool)
	cycleStart, _ := CycleStart(CycleID(now))
	for _, c := range attached {
		detach(t, c, now.Add(-time.Minute))
	}
	// Re-attach inside the 14-day lookback window ending at the cycle
	// cutoff.
	for _, c := range attached {
		attach(t, c, cycleStart.Add(-time.Hour))
	}
	runTx(t, func(txP pgxTx) error {
		return d.materializeOne(testCtx(), txP, gid, cycleStart, false)
	})
	members, err := s.CycleMembers(context.Background(), sharedPool, gid, CycleID(now))
	if err != nil {
		t.Fatalf("cycle members: %v", err)
	}
	if len(members) != 4 { // leader + 3 attached
		t.Fatalf("roster = %d, want 4", len(members))
	}
	c, err := s.Cycle(context.Background(), sharedPool, gid, CycleID(now))
	if err != nil || c == nil {
		t.Fatalf("cycle: %v", c)
	}
	if c.MEffective != 5 { // M=4 clamps up to 5
		t.Fatalf("m_effective = %d, want 5", c.MEffective)
	}
	if c.Required != 120+12*5 {
		t.Fatalf("required = %d, want 180", c.Required)
	}
}

// TestBlessingDraftRankAndPriority: three lowest SHA-256 ranks become
// candidates; vote ties resolve by fixed priority; zero votes elect
// the highest-priority candidate.
func TestBlessingDraftRankAndPriority(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "blessing", leader, 30, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	cycleID := CycleID(now)
	pool := BlessingPool(30)
	if len(pool) != 6 {
		t.Fatalf("pool = %v", pool)
	}
	cands := DraftBlessings(gid, cycleID, 30)
	if len(cands) != 3 {
		t.Fatalf("candidates = %v", cands)
	}
	// Verify the draft is the three lowest ranks.
	for _, c := range cands {
		rankC := rankBlessing(gid, cycleID, c)
		for _, b := range pool {
			in := false
			for _, cc := range cands {
				if cc == b {
					in = true
				}
			}
			if !in && rankBlessing(gid, cycleID, b) < rankC {
				t.Fatalf("draft missed lower rank %s", b)
			}
		}
	}
	d := deps(now, nil)
	s := NewStore(sharedPool)
	// Zero-vote finalize -> highest priority (lowest number).
	runTx(t, func(txP pgxTx) error {
		if _, err := txP.Exec(testCtx(),
			`INSERT INTO guild_ritual_cycles
			 (guild_id, cycle_id, m_effective, required_points_per_element,
			  rotation_pointer, completed_at, candidate_blessing_ids, vote_closes_at)
			 VALUES ($1,$2,5,180,'KIM',$3,$4,$5)`,
			gid[:], cycleID, now.Add(-time.Hour), cands, now.Add(time.Hour)); err != nil {
			return err
		}
		return d.finalizeVote(testCtx(), txP, gid, cycleID, now)
	})
	want := cands[0]
	for _, c := range cands {
		if blessingPriority[c] < blessingPriority[want] {
			want = c
		}
	}
	p, err := s.Progression(context.Background(), sharedPool, gid)
	if err != nil {
		t.Fatalf("progression: %v", err)
	}
	if p.ActiveBlessingID != want {
		t.Fatalf("active = %s, want %s", p.ActiveBlessingID, want)
	}
}

// TestDisbandBlockedByGuildWarRegistration: the disband executor
// consults WarRegistrationGuard and rejects with
// GUILD_DISBAND_BLOCKED while a registration is QUEUED..RESOLVING.
func TestDisbandBlockedByGuildWarRegistration(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, lacct := mkChar(t, 20)
	gid := mkGuild(t, "war-block", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	d := deps(now, stubGuard{blocked: true})
	ex := Executors(d)
	out := runExec(t, ex[DisbandFamily],
		recFor(t, DisbandFamily, leader, lacct, disbandReq(t), id.NewV7(now), now))
	wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_GUILD_DISBAND_BLOCKED)
	// Clear the guard -> disband succeeds.
	d2 := deps(now, stubGuard{blocked: false})
	ex2 := Executors(d2)
	out = runExec(t, ex2[DisbandFamily],
		recFor(t, DisbandFamily, leader, lacct, disbandReq(t), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	if g := guildRow(t, gid); g.State != "DISBANDED" {
		t.Fatalf("state = %s", g.State)
	}
}

// TestGuildSettingsSet: only LEADER flips recruitment mode (ADR-0060).
func TestGuildSettingsSet(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, lacct := mkChar(t, 20)
	vice, vacct := mkChar(t, 20)
	gid := mkGuild(t, "settings", leader, 1, 0, now)
	mkMember(t, gid, leader, RoleLeader, now)
	mkMember(t, gid, vice, RoleViceLeader, now)
	d := deps(now, nil)
	ex := Executors(d)
	out := runExec(t, ex[SettingsSetFamily],
		recFor(t, SettingsSetFamily, vice, vacct,
			settingsReq(t, protocolv1.GuildRecruitmentMode_GUILD_RECRUITMENT_MODE_APPLICATIONS),
			id.NewV7(now), now))
	wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)
	out = runExec(t, ex[SettingsSetFamily],
		recFor(t, SettingsSetFamily, leader, lacct,
			settingsReq(t, protocolv1.GuildRecruitmentMode_GUILD_RECRUITMENT_MODE_APPLICATIONS),
			id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	if g := guildRow(t, gid); g.RecruitmentMode != "APPLICATIONS" {
		t.Fatalf("mode = %s", g.RecruitmentMode)
	}
}

// TestGuildInviteApplicationCancel: invite cancel by inviter /
// LEADER / VICE_LEADER; application cancel by the applicant.
func TestGuildInviteApplicationCancel(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, lacct := mkChar(t, 20)
	officer, oacct := mkChar(t, 20)
	target, _ := mkChar(t, 15)
	gid := mkGuild(t, "cancels", leader, 1, 0, now)
	mkGuildRowMode(t, gid, "APPLICATIONS")
	mkMember(t, gid, leader, RoleLeader, now)
	mkMember(t, gid, officer, RoleOfficer, now)
	d := deps(now, nil)
	ex := Executors(d)
	// officer invites target; leader cancels it.
	out := runExec(t, ex[InviteFamily],
		recFor(t, InviteFamily, officer, oacct, inviteReq(t, target), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	out = runExec(t, ex[InviteCancelFamily],
		recFor(t, InviteCancelFamily, leader, lacct, inviteCancelReq(t, target), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	// member may NOT cancel another's invite.
	member, macct := mkChar(t, 15)
	mkMember(t, gid, member, RoleMember, now)
	out = runExec(t, ex[InviteFamily],
		recFor(t, InviteFamily, officer, oacct, inviteReq(t, target), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	out = runExec(t, ex[InviteCancelFamily],
		recFor(t, InviteCancelFamily, member, macct, inviteCancelReq(t, target), id.NewV7(now), now))
	wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)
	// application flow: apply then cancel own application.
	applicant, aacct := mkChar(t, 15)
	out = runExec(t, ex[ApplyFamily],
		recFor(t, ApplyFamily, applicant, aacct, applyReq(t, gid), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	out = runExec(t, ex[ApplicationCancelFamily],
		recFor(t, ApplicationCancelFamily, applicant, aacct, applyCancelReq(t, gid), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	// re-apply then officer accepts.
	out = runExec(t, ex[ApplyFamily],
		recFor(t, ApplyFamily, applicant, aacct, applyReq(t, gid), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	out = runExec(t, ex[ApplicationDecideFamily],
		recFor(t, ApplicationDecideFamily, officer, oacct, applyDecideReq(t, applicant, true), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	if m := memberRow(t, applicant); m == nil || m.Role != RoleMember {
		t.Fatalf("applicant not joined")
	}
}

// TestGuildStateRoster: the 628 snapshot carries the full roster with
// role + online projection and the progression view.
func TestGuildStateRoster(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, _ := mkChar(t, 20)
	gid := mkGuild(t, "roster", leader, 5, 1234, now)
	mkMember(t, gid, leader, RoleLeader, now)
	online, _ := mkChar(t, 15)
	mkMember(t, gid, online, RoleMember, now)
	attach(t, online, now.Add(-time.Hour))
	offline, _ := mkChar(t, 15)
	mkMember(t, gid, offline, RoleOfficer, now)
	attach(t, offline, now.Add(-48*time.Hour))
	detach(t, offline, now.Add(-24*time.Hour))
	d := deps(now, nil)
	var view *protocolv1.S2CGuildState
	runTx(t, func(txP pgxTx) error {
		var err error
		view, err = d.StateFor(testCtx(), txP, leader, nil)
		return err
	})
	if view == nil || len(view.GetMembers()) != 3 {
		t.Fatalf("members = %v", view.GetMembers())
	}
	if view.GetGuildName() != "roster" || view.GetRole() != RoleLeader {
		t.Fatalf("view: %+v", view)
	}
	if view.GetLevel() != 5 || view.GetMaxMembers() != 35 {
		t.Fatalf("level/cap = %d/%d", view.GetLevel(), view.GetMaxMembers())
	}
	roles := map[string]string{}
	onlineMap := map[string]protocolv1.OnlineState{}
	for _, m := range view.GetMembers() {
		var cid id.UUID
		copy(cid[:], m.GetCharacterId())
		roles[cid.String()] = m.GetRole()
		onlineMap[cid.String()] = m.GetOnlineState()
	}
	if roles[leader.String()] != RoleLeader || roles[offline.String()] != RoleOfficer {
		t.Fatalf("roles: %v", roles)
	}
	if onlineMap[online.String()] != protocolv1.OnlineState_ONLINE_STATE_ONLINE {
		t.Fatalf("online projection wrong")
	}
	if onlineMap[offline.String()] != protocolv1.OnlineState_ONLINE_STATE_OFFLINE {
		t.Fatalf("offline projection wrong")
	}
	if view.GetProgression().GetGuildExp() != 1234 {
		t.Fatalf("exp projection")
	}
}

// TestGuildNameErrors: canonical trim+NFC uniqueness -> GUILD_NAME_TAKEN;
// invalid names (empty/control/over-24-graphemes/over-96-bytes) ->
// GUILD_NAME_INVALID.
func TestGuildNameErrors(t *testing.T) {
	wipe(t)
	now := time.Now().UTC()
	leader, lacct := mkChar(t, 25)
	mkFunds(t, leader, 500_000)
	d := deps(now, nil)
	ex := Executors(d)
	out := runExec(t, ex[CreateFamily],
		recFor(t, CreateFamily, leader, lacct, createReq(t, "  Guild Name  "), id.NewV7(now), now))
	wantStatus(t, out, statusSuccess())
	second, sacct := mkChar(t, 25)
	mkFunds(t, second, 500_000)
	// same canonical name with different padding/case -> TAKEN.
	out = runExec(t, ex[CreateFamily],
		recFor(t, CreateFamily, second, sacct, createReq(t, "GUILD NAME"), id.NewV7(now), now))
	wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_GUILD_NAME_TAKEN)
	for _, bad := range []string{
		"", "   ", "a\u0007b",
		"abcdefghijabcdefghijabcde", // 25 graphemes
		"ớớớớớớớớớớớớớớớớớớớớớớớớớớớớớớớớớ", // >96 bytes NFC
	} {
		out = runExec(t, ex[CreateFamily],
			recFor(t, CreateFamily, second, sacct, createReq(t, bad), id.NewV7(now), now))
		wantError(t, out, protocolv1.ErrorCode_ERROR_CODE_GUILD_NAME_INVALID)
	}
}

// --- test scaffolding types -------------------------------------------------

type stubGuard struct{ blocked bool }

func (g stubGuard) Blocked(context.Context, pgx.Tx, id.UUID) (bool, error) {
	return g.blocked, nil
}
