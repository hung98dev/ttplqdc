package protocoltest

import (
	"bytes"
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"

	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// roundtrip encodes deterministically, decodes strictly, and re-encodes —
// the journal contract requires byte-identical re-encode.
func roundtrip(t *testing.T, msg proto.Message) proto.Message {
	t.Helper()
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := msg.ProtoReflect().Type().New().Interface()
	if err := (proto.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if u := out.ProtoReflect().GetUnknown(); len(u) != 0 {
		t.Fatalf("decoded record carries %d unknown bytes", len(u))
	}
	data2, err := proto.MarshalOptions{Deterministic: true}.Marshal(out)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(data, data2) {
		t.Fatal("roundtrip not byte-identical")
	}
	return out
}

// --- journal validation (schema invariants from protobuf_conventions.md §7) ---

var validOwnerByFamily = map[string]journalv1.JournalOwnerKind{
	"character.create":            journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_ACCOUNT,
	"inventory.mutate":            journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"loadout.change":              journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"craft.create":                journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"enhance.apply":               journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"reward.claim":                journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"trade.finalise":              journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"auction.list":                journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"placement.portal":            journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"instance.enter":              journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"match.settle":                journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD,
	"guild.event":                 journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_GUILD,
	"maintenance.retention_purge": journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD,
	"checkpoint.transition":       journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"chat.log":                    journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"competitive.admission":       journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD,
	"reward.grant":                journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"world.consequence":           journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD,
	"boss.eligibility":            journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"boss.chest":                  journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
	"schedule.transition":         journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD,
}

func validChatChannel(c string) bool {
	switch c {
	case "WORLD", "PARTY", "GUILD", "WHISPER", "LOCAL":
		return true
	}
	return false
}

// validateRecord checks the closed-schema invariants that are decidable at the
// codec layer (presence, sizes, per-kind rules).
func validateRecord(r *journalv1.DurableCommandRecord) error {
	if r.GetSchemaVersion() != 1 {
		return fmt.Errorf("schema_version=%d, want 1", r.GetSchemaVersion())
	}
	if len(r.GetOperationId()) != 16 {
		return fmt.Errorf("operation_id len %d, want 16", len(r.GetOperationId()))
	}
	if len(r.GetRequestFingerprint()) != 32 {
		return fmt.Errorf("request_fingerprint len %d, want 32", len(r.GetRequestFingerprint()))
	}
	if len(r.GetOwnerId()) == 0 {
		return fmt.Errorf("owner_id required")
	}
	if len(r.GetContentRevision()) != 64 {
		return fmt.Errorf("content_revision must be 64 lowercase hex")
	}
	if r.GetCommand() == nil {
		return fmt.Errorf("command oneof unset")
	}
	if want, ok := validOwnerByFamily[r.GetOperationFamily()]; ok && want != r.GetOwnerKind() {
		return fmt.Errorf("family %s requires owner %s, got %s",
			r.GetOperationFamily(), want, r.GetOwnerKind())
	}
	switch cmd := r.GetCommand().(type) {
	case *journalv1.DurableCommandRecord_Client:
		if err := validateClient(cmd.Client); err != nil {
			return err
		}
	case *journalv1.DurableCommandRecord_Reward:
		for _, s := range cmd.Reward.GetSlots() {
			if err := validateRewardSlot(s); err != nil {
				return err
			}
		}
	case *journalv1.DurableCommandRecord_BossChest:
		for _, s := range cmd.BossChest.GetSlots() {
			if err := validateRewardSlot(s); err != nil {
				return err
			}
		}
	case *journalv1.DurableCommandRecord_Checkpoint:
		cp := cmd.Checkpoint
		if src := cp.GetSource(); src != nil && src.GetTick() != cp.GetSourceTick() {
			return fmt.Errorf("checkpoint source_tick=%d != source.tick=%d",
				cp.GetSourceTick(), src.GetTick())
		}
	case *journalv1.DurableCommandRecord_PublicSchedule:
		d := cmd.PublicSchedule.GetRespawnDelaySeconds()
		if cmd.PublicSchedule.RespawnDelaySeconds != nil && (d < 1800 || d > 2700) {
			return fmt.Errorf("respawn_delay_seconds %d out of 1800..2700", d)
		}
	case *journalv1.DurableCommandRecord_Match:
		m := cmd.Match
		switch m.GetMatchState() {
		case "COMPLETED", "VOID":
		default:
			return fmt.Errorf("match_state %s not terminal", m.GetMatchState())
		}
		if m.GetMatchState() == "VOID" && m.GetResult() != "NONE" {
			return fmt.Errorf("VOID match must have result NONE")
		}
		for _, p := range m.GetParticipants() {
			switch p.GetResult() {
			case "WIN", "LOSS", "DRAW", "NONE":
			default:
				return fmt.Errorf("participant result %q invalid", p.GetResult())
			}
		}
	case *journalv1.DurableCommandRecord_ChatLog:
		if !validChatChannel(cmd.ChatLog.GetChannel()) {
			return fmt.Errorf("chat channel %q invalid", cmd.ChatLog.GetChannel())
		}
		l := len([]rune(cmd.ChatLog.GetContent()))
		if l < 1 || l > 240 {
			return fmt.Errorf("chat content %d graphemes out of 1..240", l)
		}
	case *journalv1.DurableCommandRecord_Job:
		if cmd.Job.GetTarget() == nil {
			return fmt.Errorf("job target oneof unset")
		}
	case *journalv1.DurableCommandRecord_CompetitiveAdmission:
		ca := cmd.CompetitiveAdmission
		switch ca.GetScope() {
		case "RANKED_DUEL", "FIVE_ELEMENT_ARENA", "GUILD_WAR":
		default:
			return fmt.Errorf("competitive scope %q invalid", ca.GetScope())
		}
	}
	return nil
}

func validateClient(c *journalv1.JournalClientCommand) error {
	if len(c.GetAccountId()) != 16 {
		return fmt.Errorf("client.account_id len %d, want 16", len(c.GetAccountId()))
	}
	if c.GetRequest() == nil {
		return fmt.Errorf("client.request oneof unset")
	}
	if tr := c.GetTrade(); tr != nil {
		req, ok := c.GetRequest().(*journalv1.JournalClientCommand_C2STradeFinalise)
		if !ok {
			return fmt.Errorf("trade snapshot requires C2S_TRADE_FINALISE request")
		}
		if !bytes.Equal(tr.GetTradeId(), req.C2STradeFinalise.GetTradeId()) ||
			tr.GetExpectedRevision() != req.C2STradeFinalise.GetExpectedRevision() {
			return fmt.Errorf("trade snapshot mismatches request")
		}
		if !bytes.Equal(tr.GetInitiatingClientOperationId(), req.C2STradeFinalise.GetOperationId()) {
			return fmt.Errorf("initiating_client_operation_id != request operation_id")
		}
	}
	if c.GetCraft() != nil {
		cr := c.GetCraft()
		if _, ok := c.GetRequest().(*journalv1.JournalClientCommand_C2SCraft); !ok {
			if _, isInteract := c.GetRequest().(*journalv1.JournalClientCommand_C2SInteract); !isInteract {
				return fmt.Errorf("craft snapshot only for 404/103 COOK")
			}
		}
		if cr.GetBatchQuantity() < 1 || cr.GetBatchQuantity() > 99 {
			return fmt.Errorf("batch_quantity %d out of 1..99", cr.GetBatchQuantity())
		}
	}
	return nil
}

// validateRewardSlot applies the soul-acquisition batch invariants.
func validateRewardSlot(s *journalv1.JournalRewardSlot) error {
	if s.GetRewardSlot() == "" {
		return fmt.Errorf("reward_slot empty")
	}
	seen := map[string]bool{}
	for _, it := range s.GetItems() {
		if it.GetItemId() == "" || it.GetQuantity() == 0 {
			return fmt.Errorf("item missing id/quantity")
		}
	}
	for _, acq := range s.GetSoulAcquisitions() {
		key := string(acq.GetSoulInstanceId())
		if seen[key] {
			return fmt.Errorf("duplicate soul_instance_id in batch")
		}
		seen[key] = true
		if len(acq.GetSoulInstanceId()) != 16 {
			return fmt.Errorf("soul_instance_id len %d, want 16", len(acq.GetSoulInstanceId()))
		}
		res := acq.GetResonance()
		if res != nil {
			if !acq.GetDuplicate() || !acq.GetAtlasPageMasteredBefore() {
				return fmt.Errorf("resonance requires duplicate && mastered_before")
			}
			if res.GetMemoryCountAfter() != res.GetMemoryCountBefore()+1 {
				return fmt.Errorf("resonance after=%d != before+1", res.GetMemoryCountAfter())
			}
		}
		// A newly acquired soul must never be a soul_exp recipient.
		for _, se := range s.GetSoulExp() {
			if bytes.Equal(se.GetSoulInstanceId(), acq.GetSoulInstanceId()) {
				return fmt.Errorf("new soul %x also in soul_exp", acq.GetSoulInstanceId())
			}
		}
	}
	return nil
}

// validateSameBatch checks whole-recipient soul batch rules across slots:
// same expected_soul_revision, unique instance UUIDs.
func validateSameBatch(slots []*journalv1.JournalRewardSlot) error {
	var wantRev uint64
	first := true
	seen := map[string]bool{}
	for _, s := range slots {
		for _, acq := range s.GetSoulAcquisitions() {
			if first {
				wantRev = acq.GetExpectedSoulRevision()
				first = false
			} else if acq.GetExpectedSoulRevision() != wantRev {
				return fmt.Errorf("batch expected_soul_revision mismatch")
			}
			key := string(acq.GetSoulInstanceId())
			if seen[key] {
				return fmt.Errorf("soul_instance_id repeated across slots")
			}
			seen[key] = true
		}
	}
	return nil
}

func baseRecord(family string, owner journalv1.JournalOwnerKind) *journalv1.DurableCommandRecord {
	return &journalv1.DurableCommandRecord{
		SchemaVersion: 1, OperationFamily: family, OwnerKind: owner,
		OwnerId: uuid(0x01), OperationId: uuid(0x02),
		EnqueuedAtMs: 1760000000000, ContentRevision: testContentRevision,
		RequestFingerprint: bytes.Repeat([]byte{0xAB}, 32),
		AdmissionSequence:  1, ProducerIncarnationId: uuid(0x03),
	}
}

func soulAcquisition(id byte, dup bool, masteredBefore bool, rev uint64) *journalv1.JournalSoulAcquisition {
	return &journalv1.JournalSoulAcquisition{
		SoulInstanceId: uuid(id), SoulId: "soul.test", InitialLevel: 1,
		Duplicate: dup, AtlasProgress: &journalv1.JournalAtlas{PageId: "atlas.hon_giam.test", Delta: 1},
		AtlasPageMasteredBefore: masteredBefore, ExpectedSoulRevision: rev,
	}
}

// --- named tests (20) ---

func TestNewAndDuplicateSoulAcquisitionRoundtrip(t *testing.T) {
	rec := baseRecord("reward.grant", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.Command = &journalv1.DurableCommandRecord_Reward{Reward: &journalv1.JournalRewardCommand{
		Kind: "BOSS",
		Slots: []*journalv1.JournalRewardSlot{{RewardSlot: "boss.test.main",
			SourceType: "source.boss", SourceReference: "boss.test",
			SoulAcquisitions: []*journalv1.JournalSoulAcquisition{
				soulAcquisition(0x10, false, false, 9),
				soulAcquisition(0x11, true, true, 9),
			}}}}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	roundtrip(t, rec)
}

func TestMasteredBossSoulResonanceNineToTenAndExistingSheen(t *testing.T) {
	sheen := int64(1750000000000)
	cases := []*journalv1.JournalSoulAcquisition{
		// First reaching 10: before absent, after present.
		{Resonance: &journalv1.JournalSoulResonance{
			MemoryCountBefore: 9, MemoryCountAfter: 10,
			SheenUnlockedAfterMs: proto.Int64(1760000000000)}},
		// Existing sheen preserved byte-for-byte.
		{Resonance: &journalv1.JournalSoulResonance{
			MemoryCountBefore: 12, MemoryCountAfter: 13,
			SheenUnlockedBeforeMs: &sheen, SheenUnlockedAfterMs: &sheen}},
	}
	for i, acq := range cases {
		base := soulAcquisition(byte(0x20+i), true, true, 4)
		base.Resonance = acq.Resonance
		rec := baseRecord("reward.grant", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
		rec.Command = &journalv1.DurableCommandRecord_Reward{Reward: &journalv1.JournalRewardCommand{
			Kind: "BOSS", Slots: []*journalv1.JournalRewardSlot{
				{RewardSlot: "boss.test", SoulAcquisitions: []*journalv1.JournalSoulAcquisition{base}}}}}
		if err := validateRecord(rec); err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		out := roundtrip(t, rec).(*journalv1.DurableCommandRecord)
		got := out.GetReward().GetSlots()[0].GetSoulAcquisitions()[0].GetResonance()
		if got.GetMemoryCountBefore() != acq.Resonance.GetMemoryCountBefore() ||
			got.GetMemoryCountAfter() != acq.Resonance.GetMemoryCountAfter() {
			t.Fatalf("case %d resonance not preserved", i)
		}
	}
}

func TestNewSoulExcludedFromSameSettlementExp(t *testing.T) {
	rec := baseRecord("reward.grant", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	acq := soulAcquisition(0x30, false, false, 5)
	rec.Command = &journalv1.DurableCommandRecord_Reward{Reward: &journalv1.JournalRewardCommand{
		Kind: "BOSS", Slots: []*journalv1.JournalRewardSlot{
			{RewardSlot: "boss.test", SoulAcquisitions: []*journalv1.JournalSoulAcquisition{acq},
				SoulExp: []*journalv1.JournalSoulExp{{SoulInstanceId: uuid(0x30), Amount: 50}}}}}}
	if err := validateRecord(rec); err == nil {
		t.Fatal("expected rejection: new soul in soul_exp")
	}
}

func TestInvalidSoulAcquisitionQualificationOrRevisionRejected(t *testing.T) {
	// resonance present but not duplicate -> reject
	bad := soulAcquisition(0x40, false, true, 5)
	bad.Resonance = &journalv1.JournalSoulResonance{MemoryCountBefore: 1, MemoryCountAfter: 2}
	rec := baseRecord("reward.grant", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.Command = &journalv1.DurableCommandRecord_Reward{Reward: &journalv1.JournalRewardCommand{
		Kind: "SOUL", Slots: []*journalv1.JournalRewardSlot{
			{RewardSlot: "soul.test", SoulAcquisitions: []*journalv1.JournalSoulAcquisition{bad}}}}}
	if err := validateRecord(rec); err == nil {
		t.Fatal("expected rejection: resonance on non-duplicate")
	}
	// broken revision chain across batch -> reject
	slots := []*journalv1.JournalRewardSlot{
		{RewardSlot: "a", SoulAcquisitions: []*journalv1.JournalSoulAcquisition{soulAcquisition(0x41, false, false, 3)}},
		{RewardSlot: "b", SoulAcquisitions: []*journalv1.JournalSoulAcquisition{soulAcquisition(0x42, false, false, 4)}},
	}
	if err := validateSameBatch(slots); err == nil {
		t.Fatal("expected rejection: mismatched expected_soul_revision")
	}
}

func TestCraftEquipmentAndConsumedMaterialSnapshotRoundtrip(t *testing.T) {
	snap := &journalv1.JournalCraftSnapshot{
		RecipeId: "recipe.sword.hon_kiem", BatchQuantity: 2,
		CreatedItems: []*journalv1.JournalItem{
			{ItemId: "item.sword.hon_kiem", Quantity: 1, EffectiveBinding: "CHARACTER_BOUND",
				ContentRevision: testContentRevision, ItemInstanceId: uuid(0x50),
				BaseRolls:   []*journalv1.JournalStat{{StatId: "stat.atk", Value: 55, Scale: 1}},
				CreatedAtMs: proto.Int64(1760000000100)},
			{ItemId: "item.sword.hon_kiem", Quantity: 1, EffectiveBinding: "CHARACTER_BOUND",
				ContentRevision: testContentRevision, ItemInstanceId: uuid(0x51),
				BaseRolls:   []*journalv1.JournalStat{{StatId: "stat.atk", Value: 61, Scale: 1}},
				CreatedAtMs: proto.Int64(1760000000100)}},
		Consumed: []*journalv1.JournalConsumedItem{
			{ItemInstanceId: uuid(0x52), ItemId: "item.ore.sat", Quantity: 4},
			{ItemInstanceId: uuid(0x53), ItemId: "item.ore.bac", Quantity: 2}},
		CurrencyDelta: []*journalv1.JournalCurrency{
			{CurrencyId: "currency.common", Amount: -1500}},
		CharacterExp: 0,
	}
	rec := baseRecord("craft.create", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.Command = &journalv1.DurableCommandRecord_Client{Client: &journalv1.JournalClientCommand{
		AccountId: uuid(0x54), CharacterId: uuid(0x55), SessionEpoch: 7,
		OwnershipEpoch: proto.Uint64(2), AdmittedAtMs: 1760000000200, IssuedAtMs: 1760000000100,
		Craft: snap,
		Request: &journalv1.JournalClientCommand_C2SCraft{C2SCraft: &protocolv1.C2SCraft{
			OperationId: uuid(0x02), NpcId: "npc.smith", RecipeId: "recipe.sword.hon_kiem",
			BatchQuantity: 2}}}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	roundtrip(t, rec)
}

func TestCookFoodKindlingLifeSkillSnapshotRoundtrip(t *testing.T) {
	snap := &journalv1.JournalCraftSnapshot{
		RecipeId: "recipe.cook.pho", BatchQuantity: 1,
		CreatedItems: []*journalv1.JournalItem{
			{ItemId: "item.food.pho", Quantity: 3, EffectiveBinding: "UNBOUND",
				ContentRevision: testContentRevision, ItemInstanceId: uuid(0x60),
				CreatedAtMs: proto.Int64(1760000000100)}},
		Consumed: []*journalv1.JournalConsumedItem{
			{ItemInstanceId: uuid(0x61), ItemId: "item.mat.noodle", Quantity: 1}},
		CharacterExp: 12, // authored LIFE_SKILL per-unit award
	}
	rec := baseRecord("interaction.cook", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.Command = &journalv1.DurableCommandRecord_Client{Client: &journalv1.JournalClientCommand{
		AccountId: uuid(0x62), CharacterId: uuid(0x63), SessionEpoch: 8,
		AdmittedAtMs: 1760000000200, IssuedAtMs: 1760000000100,
		Craft: snap,
		SpatialSource: &journalv1.JournalSource{
			MapId: "map.ha_noi", ChannelId: 1, PartitionIncarnationId: uuid(0x64),
			SourceEventId: 10, Tick: 300, OccurredAtMs: 1760000000050,
			SourceContentId: "hearth.bonfire.a"},
		Request: &journalv1.JournalClientCommand_C2SInteract{C2SInteract: &protocolv1.C2SInteract{
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_COOK,
			TargetId:     "hearth.bonfire.a", OperationId: uuid(0x02),
			RecipeId: proto.String("recipe.cook.pho")}}}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	roundtrip(t, rec)
}

func TestStandaloneTradeCannotReplayClientReceipt(t *testing.T) {
	rec := baseRecord("trade.finalise", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	trade := &journalv1.JournalTrade{
		TradeId: uuid(0x70), ExpectedRevision: 4,
		Initiator:    &journalv1.JournalTradeSide{CharacterId: uuid(0x71), AccountId: uuid(0x72), CommonAmount: 100},
		Counterpart:  &journalv1.JournalTradeSide{CharacterId: uuid(0x73), AccountId: uuid(0x74)},
		SettlementId: uuid(0x75), FinalizedAtMs: 1760000001000,
		InitiatingClientOperationId: uuid(0x02)}
	rec.Command = &journalv1.DurableCommandRecord_Client{Client: &journalv1.JournalClientCommand{
		AccountId: uuid(0x72), CharacterId: uuid(0x71), SessionEpoch: 7,
		AdmittedAtMs: 1760000000900, IssuedAtMs: 1760000000800,
		Trade: trade,
		Request: &journalv1.JournalClientCommand_C2STradeFinalise{C2STradeFinalise: &protocolv1.C2STradeFinalise{
			OperationId: uuid(0x02), TradeId: uuid(0x70), ExpectedRevision: 4}}}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	roundtrip(t, rec)
	// Mismatched trade snapshot must reject — no separate trade command identity.
	rec.GetClient().GetTrade().ExpectedRevision = 9
	if err := validateRecord(rec); err == nil {
		t.Fatal("expected rejection: trade snapshot disagrees with request")
	}
}

func TestSameSoulBatchChainAcrossRewardSlotsRoundtrip(t *testing.T) {
	slots := []*journalv1.JournalRewardSlot{
		{RewardSlot: "boss.a", SoulAcquisitions: []*journalv1.JournalSoulAcquisition{
			soulAcquisition(0x80, false, false, 12)}},
		{RewardSlot: "boss.b", SoulAcquisitions: []*journalv1.JournalSoulAcquisition{
			soulAcquisition(0x81, true, true, 12)}},
	}
	if err := validateSameBatch(slots); err != nil {
		t.Fatal(err)
	}
	rec := baseRecord("reward.grant", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.Command = &journalv1.DurableCommandRecord_Reward{Reward: &journalv1.JournalRewardCommand{
		Kind: "BOSS", Slots: slots}}
	roundtrip(t, rec)
}

func TestSameSoulBatchRejectsRepeatedInstanceUuidAndBrokenPrefix(t *testing.T) {
	slots := []*journalv1.JournalRewardSlot{
		{RewardSlot: "a", SoulAcquisitions: []*journalv1.JournalSoulAcquisition{
			soulAcquisition(0x90, false, false, 3), soulAcquisition(0x90, true, true, 3)}},
	}
	if err := validateSameBatch(slots); err == nil {
		t.Fatal("expected rejection: repeated soul_instance_id")
	}
	if err := validateRewardSlot(slots[0]); err == nil {
		t.Fatal("expected per-slot rejection too")
	}
}

func TestCompetitiveAdmissionSnapshotRoundtrip(t *testing.T) {
	rec := baseRecord("competitive.admission", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD)
	rec.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_COMPETITIVE_ADMISSION
	rec.Command = &journalv1.DurableCommandRecord_CompetitiveAdmission{
		CompetitiveAdmission: &journalv1.JournalCompetitiveAdmission{
			Scope: "RANKED_DUEL", MatchId: uuid(0xA0), SeasonId: 4,
			AdmittedAtMs: 1760000000000, DeadlineAtMs: 1760000060000,
			State: "RESOLVING", ContentRevision: testContentRevision,
			ResolutionSnapshot: &journalv1.JournalMatch{
				MatchId: uuid(0xA0), ModeId: "pvp.mode.ranked_duel", SeasonId: 4,
				MatchState: "COMPLETED", Result: "WIN", SettlementType: "PVP",
				ResolvedAtMs: 1760000055000,
				Source: &journalv1.JournalSource{
					MapId: "map.arena", ChannelId: 1, PartitionIncarnationId: uuid(0xA1),
					SourceEventId: 1, Tick: 10, OccurredAtMs: 1760000054000},
				Participants: []*journalv1.JournalParticipant{
					{CharacterId: uuid(0xA2), Participation: "NORMAL", TeamIndex: 0,
						Result: "WIN", ExpectedRatingRevision: 1},
					{CharacterId: uuid(0xA3), Participation: "ABANDONED", TeamIndex: 1,
						Result: "LOSS", ExpectedRatingRevision: 1}}}}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	roundtrip(t, rec)
}

func TestGuildWarPerRecipientResultsAndWeeklyProgressionRoundtrip(t *testing.T) {
	match := &journalv1.JournalMatch{
		MatchId: uuid(0xB0), ModeId: "pvp.mode.guild_war", SeasonId: 4,
		MatchState: "COMPLETED", Result: "WIN", SettlementType: "GUILD_PROGRESSION",
		ResolvedAtMs: 1760000060000,
		Source: &journalv1.JournalSource{MapId: "map.war", ChannelId: 1,
			PartitionIncarnationId: uuid(0xB1), SourceEventId: 2, Tick: 20,
			OccurredAtMs: 1760000059000},
		Participants: []*journalv1.JournalParticipant{
			{CharacterId: uuid(0xB2), GuildId: uuid(0xB4), Participation: "NORMAL",
				TeamIndex: 0, Result: "WIN", MembershipId: uuid(0xB6), GuildContribution: 30},
			{CharacterId: uuid(0xB3), GuildId: uuid(0xB5), Participation: "NORMAL",
				TeamIndex: 1, Result: "LOSS", MembershipId: uuid(0xB7), GuildContribution: 30}},
		GuildOutputs: []*journalv1.JournalGuildMatchOutput{
			{GuildId: uuid(0xB4), Result: "WIN", ExpectedRatingRevision: 5,
				MmrBefore: 1600, MmrAfter: 1625,
				ProgressionWeekMondayMs: proto.Int64(1760054400000), ProgressionSlot: proto.Uint32(1),
				GuildExp: 40,
				Contributions: []*journalv1.JournalGuildContribution{
					{CharacterId: uuid(0xB2), MembershipId: uuid(0xB6), Amount: 30}}},
			{GuildId: uuid(0xB5), Result: "LOSS", ExpectedRatingRevision: 3,
				MmrBefore: 1580, MmrAfter: 1560,
				ProgressionWeekMondayMs: proto.Int64(1760054400000), ProgressionSlot: proto.Uint32(2),
				GuildExp: 30,
				Contributions: []*journalv1.JournalGuildContribution{
					{CharacterId: uuid(0xB3), MembershipId: uuid(0xB7), Amount: 30}}}}}
	rec := baseRecord("match.settle", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD)
	rec.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_MATCH
	rec.Command = &journalv1.DurableCommandRecord_Match{Match: match}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	out := roundtrip(t, rec).(*journalv1.DurableCommandRecord)
	got := out.GetMatch()
	if got.GetParticipants()[0].GetResult() != "WIN" || got.GetParticipants()[1].GetResult() != "LOSS" {
		t.Fatal("per-participant results not preserved")
	}
	if got.GetGuildOutputs()[0].GetGuildExp() != 40 || got.GetGuildOutputs()[1].GetGuildExp() != 30 {
		t.Fatal("weekly progression exp (30/30 +10 winner) not preserved")
	}
}

func TestCheckpointRetainsSourceCounter(t *testing.T) {
	rec := baseRecord("checkpoint.transition", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHECKPOINT
	rec.Command = &journalv1.DurableCommandRecord_Checkpoint{Checkpoint: &journalv1.JournalCheckpoint{
		CharacterId: uuid(0xC0), OwnershipEpoch: 5, CheckpointId: "checkpoint.bonfire.ha_noi",
		SafeMapId: "map.ha_noi", EntrySpawnId: "spawn.gate", MembershipState: "MEMBER",
		RecordedAtMs: 1760000007000, SourceTick: 999,
		InstanceId: uuid(0xC1), InstanceDungeonId: proto.String("dungeon.nui_vo_song"),
		InstanceRunTag: proto.String("NORMAL"), SourceMapId: proto.String("map.ha_noi"),
		SourceChannelId: proto.Uint32(1),
		Source: &journalv1.JournalSource{MapId: "map.ha_noi", ChannelId: 1,
			PartitionIncarnationId: uuid(0xC2), SourceEventId: 7, Tick: 999,
			OccurredAtMs: 1760000006900}}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	out := roundtrip(t, rec).(*journalv1.DurableCommandRecord)
	if out.GetCheckpoint().GetSourceTick() != 999 ||
		out.GetCheckpoint().GetSource().GetTick() != 999 {
		t.Fatal("checkpoint source tick/counter not retained")
	}
	// tick mismatch must reject
	rec.GetCheckpoint().SourceTick = 998
	if err := validateRecord(rec); err == nil {
		t.Fatal("expected rejection: source_tick != source.tick")
	}
}

func TestChatLogReplayDoesNotRedeliverOrReviveErasedSubject(t *testing.T) {
	rec := baseRecord("chat.log", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHAT_LOG
	rec.Command = &journalv1.DurableCommandRecord_ChatLog{ChatLog: &journalv1.JournalChatLog{
		MessageId: uuid(0xD0), SenderAccountId: uuid(0xD1), SenderCharacterId: uuid(0xD2),
		Channel: "GUILD", ScopeId: proto.String("guild:8f2e"), Content: "cap nhat lich war",
		CreatedAtMs: 1760000008000}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	out := roundtrip(t, rec).(*journalv1.DurableCommandRecord)
	if out.GetChatLog().GetCreatedAtMs() != 1760000008000 {
		t.Fatal("chat timestamp changed")
	}
	rec.GetChatLog().Channel = "GLOBAL"
	if err := validateRecord(rec); err == nil {
		t.Fatal("expected rejection: invalid chat channel")
	}
	rec.GetChatLog().Channel = "GUILD"
	rec.GetChatLog().Content = ""
	if err := validateRecord(rec); err == nil {
		t.Fatal("expected rejection: empty chat content")
	}
}

func TestPublicScheduleReplayRetainsSampledDelay(t *testing.T) {
	rec := baseRecord("schedule.transition", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD)
	rec.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_PUBLIC_SCHEDULE
	rec.Command = &journalv1.DurableCommandRecord_PublicSchedule{PublicSchedule: &journalv1.JournalPublicSchedule{
		BossId: "boss.ho_long", ExpectedRevision: 7, Transition: "CLOSE",
		PriorGenerationId: uuid(0xE0), NextSpawnAtMs: proto.Int64(1760002100000),
		TransitionAtMs: 1760000000000, RespawnDelaySeconds: proto.Uint32(2100)}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	out := roundtrip(t, rec).(*journalv1.DurableCommandRecord)
	if out.GetPublicSchedule().GetRespawnDelaySeconds() != 2100 {
		t.Fatal("sampled delay changed on replay")
	}
	rec.GetPublicSchedule().RespawnDelaySeconds = proto.Uint32(5000)
	if err := validateRecord(rec); err == nil {
		t.Fatal("expected rejection: delay out of 1800..2700")
	}
}

func TestPublicChestFallbackUsesOriginalRewardRevision(t *testing.T) {
	rec := baseRecord("boss.chest", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_BOSS_CHEST
	rec.Command = &journalv1.DurableCommandRecord_BossChest{BossChest: &journalv1.JournalBossChest{
		CharacterId: uuid(0xF0), GenerationId: uuid(0xF1), BossId: "boss.ho_long",
		CopyMapId: "map.boss_copy", CopyChannelId: 0, Reason: "RESTART",
		Slots: []*journalv1.JournalRewardSlot{{RewardSlot: "boss.ho_long.fallback",
			SourceType: "source.boss", SourceReference: "boss.ho_long"}}}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	out := roundtrip(t, rec).(*journalv1.DurableCommandRecord)
	// reward_content_revision is the enclosing record's revision — persisted on
	// the defeated-copy row, never re-resolved from current catalog.
	if out.GetContentRevision() != testContentRevision {
		t.Fatal("record content_revision not preserved")
	}
	if out.GetBossChest().GetReason() != "RESTART" {
		t.Fatal("fallback reason changed")
	}
}

func TestJournalTypedProducerRoundtrip(t *testing.T) {
	builders := []struct {
		typ journalv1.JournalCommandType
		cmd func(r *journalv1.DurableCommandRecord)
	}{
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_Client{Client: &journalv1.JournalClientCommand{
					AccountId: uuid(1), SessionEpoch: 1,
					Request: &journalv1.JournalClientCommand_C2SInventoryExpand{
						C2SInventoryExpand: &protocolv1.C2SInventoryExpand{
							OperationId: uuid(0x02), ExpectedCapacity: 96}}}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_REWARD,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_Reward{Reward: &journalv1.JournalRewardCommand{
					Kind: "KILL", Slots: []*journalv1.JournalRewardSlot{{RewardSlot: "kill.slime"}}}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_WORLD_CONSEQUENCE,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_WorldConsequence{WorldConsequence: &journalv1.JournalWorldConsequence{
					MapId: "map.world", ChannelId: 1, RelicId: "relic.a", RegionId: "region.a",
					MarkerBossId: "boss.a", SourceId: "src.a", BuffEffectId: "effect.a",
					SpawnedAtMs: 1, ExpiresAtMs: 2, Transition: "CREATE"}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_BOSS_ELIGIBILITY,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_BossEligibility{BossEligibility: &journalv1.JournalBossEligibility{
					CharacterId: uuid(1), GenerationId: uuid(2), BossId: "boss.ho_long",
					CopyMapId: "map.copy", CopyChannelId: 0, Transition: "DEFEATED",
					EligibleUntilMs: proto.Int64(1760000000000)}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_BOSS_CHEST,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_BossChest{BossChest: &journalv1.JournalBossChest{
					CharacterId: uuid(1), GenerationId: uuid(2), BossId: "boss.ho_long",
					CopyMapId: "map.copy", CopyChannelId: 0, Reason: "INTERACT",
					Slots: []*journalv1.JournalRewardSlot{{RewardSlot: "boss.ho_long.s0"}}}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHECKPOINT,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_Checkpoint{Checkpoint: &journalv1.JournalCheckpoint{
					CharacterId: uuid(1), OwnershipEpoch: 1, CheckpointId: "cp.a",
					MembershipState: "MEMBER", SourceTick: 1,
					Source: &journalv1.JournalSource{MapId: "m", ChannelId: 1,
						PartitionIncarnationId: uuid(2), SourceEventId: 1, Tick: 1}}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_PUBLIC_SCHEDULE,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_PublicSchedule{PublicSchedule: &journalv1.JournalPublicSchedule{
					BossId: "boss.a", ExpectedRevision: 1, Transition: "OPEN",
					NewGenerationId: uuid(3), OpenedAtMs: proto.Int64(1), TransitionAtMs: 1}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_MATCH,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_Match{Match: &journalv1.JournalMatch{
					MatchId: uuid(4), ModeId: "pvp.mode.ranked_duel", SeasonId: 1,
					MatchState: "COMPLETED", Result: "NONE", SettlementType: "PVP",
					ResolvedAtMs: 1, Source: &journalv1.JournalSource{
						MapId: "m", ChannelId: 1, PartitionIncarnationId: uuid(2),
						SourceEventId: 1, Tick: 1}}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_GUILD_EVENT,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_GuildEvent{GuildEvent: &journalv1.JournalGuildEvent{
					GuildId: uuid(5), CharacterId: uuid(6), MembershipId: uuid(7),
					SourceOperationId: uuid(8), SourceKind: "RITUAL", SourceReference: "ritual.a",
					OccurredAtMs: 1, Element: 1, RitualPoints: 10, GuildExp: 10, SeasonId: 1,
					CycleId: "2026-10-01", ExpectedGuildRevision: 2}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_JOB,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_Job{Job: &journalv1.JournalJob{
					Kind: "auction.expire", JobKey: "auction:expire:l1", DueAtMs: 1,
					Target: &journalv1.JournalJob_Auction{Auction: &journalv1.JournalAuctionJob{
						ListingId: uuid(9), ExpectedRevision: 1}}}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_ACTIVITY,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_Activity{Activity: &journalv1.JournalActivity{
					CharacterId: uuid(1), SessionEpoch: 1, Transition: "LOGIN", OccurredAtMs: 1}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_COMPETITIVE_ADMISSION,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_CompetitiveAdmission{CompetitiveAdmission: &journalv1.JournalCompetitiveAdmission{
					Scope: "GUILD_WAR", MatchId: uuid(4), SeasonId: 1, State: "PREPARING",
					ContentRevision: testContentRevision}}
			}},
		{journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHAT_LOG,
			func(r *journalv1.DurableCommandRecord) {
				r.Command = &journalv1.DurableCommandRecord_ChatLog{ChatLog: &journalv1.JournalChatLog{
					MessageId: uuid(10), SenderAccountId: uuid(11), SenderCharacterId: uuid(12),
					Channel: "WORLD", Content: "chao", CreatedAtMs: 1}}
			}},
	}
	for i, b := range builders {
		families := []string{"x.family", "checkpoint.transition", "schedule.transition",
			"match.settle", "guild.event", "x.family", "x.family", "x.family",
			"competitive.admission", "chat.log"}
		family := "x.family"
		switch b.typ {
		case journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHECKPOINT:
			family = "checkpoint.transition"
		case journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_PUBLIC_SCHEDULE:
			family = "schedule.transition"
		case journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_MATCH:
			family = "match.settle"
		case journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_GUILD_EVENT:
			family = "guild.event"
		case journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_COMPETITIVE_ADMISSION:
			family = "competitive.admission"
		case journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHAT_LOG:
			family = "chat.log"
		}
		_ = families
		rec := baseRecord(family, journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD)
		if family == "chat.log" || family == "checkpoint.transition" {
			rec.OwnerKind = journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER
		}
		if family == "guild.event" {
			rec.OwnerKind = journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_GUILD
		}
		rec.CommandType = b.typ
		b.cmd(rec)
		if err := validateRecord(rec); err != nil {
			t.Fatalf("producer %d (%s): %v", i, b.typ, err)
		}
		roundtrip(t, rec)
	}
}

func TestJournalUuidNetworkOrder(t *testing.T) {
	// RFC 9562 network order: the raw bytes appear verbatim in the encoding.
	rec := baseRecord("checkpoint.transition", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.CommandType = journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHECKPOINT
	cp := &journalv1.JournalCheckpoint{CharacterId: uuid(0xC0), OwnershipEpoch: 1,
		CheckpointId: "cp", MembershipState: "MEMBER", SourceTick: 1,
		Source: &journalv1.JournalSource{MapId: "m", ChannelId: 1,
			PartitionIncarnationId: uuid(0xC2), SourceEventId: 1, Tick: 1}}
	rec.Command = &journalv1.DurableCommandRecord_Checkpoint{Checkpoint: cp}
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, uuid(0xC0)) || !bytes.Contains(data, uuid(0xC2)) {
		t.Fatal("UUID bytes not in network (big-endian display) order on the wire")
	}
	// And the decode restores the same 16 bytes.
	out := roundtrip(t, rec).(*journalv1.DurableCommandRecord)
	if !bytes.Equal(out.GetCheckpoint().GetCharacterId(), uuid(0xC0)) {
		t.Fatal("UUID roundtrip changed byte order")
	}
}

func TestJournalFinalizedItemAndSourceRoundtrip(t *testing.T) {
	item := &journalv1.JournalItem{
		ItemId: "item.sword", Quantity: 1, EffectiveBinding: "ACCOUNT_BOUND",
		ContentRevision: testContentRevision, Enhancement: 7,
		BaseRolls: []*journalv1.JournalStat{
			{StatId: "stat.atk", Value: 70, Scale: 1},
			{StatId: "stat.crit_rate", Value: 450, Scale: 10000}},
		SecondaryRolls: []*journalv1.JournalStat{
			{StatId: "stat.speed", Value: 12, Scale: 1}},
		Pity:           []*journalv1.JournalPity{{TargetLevel: 8, FailCount: 3}},
		ItemInstanceId: uuid(0x99),
		SoulId:         proto.String("soul.test"), SoulLevel: 3, SoulExp: 120,
		CreatedAtMs: proto.Int64(1760000000000)}
	src := &journalv1.JournalSource{
		MapId: "map.boss", ChannelId: 2, PartitionIncarnationId: uuid(0x98),
		SourceEventId: 4, Tick: 55, OccurredAtMs: 1760000000000,
		SourceContentId: "boss.a", EncounterInstanceId: uuid(0x97),
		GenerationId: uuid(0x96), ChainId: uuid(0x95),
		RngSeedHi: proto.Uint64(0xFFFF), RngSeedLo: proto.Uint64(0x00FF)}
	slot := &journalv1.JournalRewardSlot{RewardSlot: "boss.a", Items: []*journalv1.JournalItem{item}}
	rec := baseRecord("reward.grant", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER)
	rec.Command = &journalv1.DurableCommandRecord_Reward{Reward: &journalv1.JournalRewardCommand{
		Kind: "BOSS", Source: src, Slots: []*journalv1.JournalRewardSlot{slot}}}
	if err := validateRecord(rec); err != nil {
		t.Fatal(err)
	}
	out := roundtrip(t, rec).(*journalv1.DurableCommandRecord)
	got := out.GetReward().GetSlots()[0].GetItems()[0]
	if got.GetPity()[0].GetFailCount() != 3 || got.GetSoulExp() != 120 {
		t.Fatal("finalized item soul/pity fields changed")
	}
	if out.GetReward().GetSource().GetRngSeedHi() != 0xFFFF {
		t.Fatal("rng seed halves not preserved")
	}
}

func TestJournalUnknownOneofOrEnumRejected(t *testing.T) {
	rec := baseRecord("x.family", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD)
	rec.Command = &journalv1.DurableCommandRecord_Activity{Activity: &journalv1.JournalActivity{
		CharacterId: uuid(1), SessionEpoch: 1, Transition: "LOGIN", OccurredAtMs: 1}}
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	// Inject unknown field 999 (varint) — unknown fields are corruption.
	data = append(data, 0xF8, 0x3E, 0x01) // field 999 varint 1
	out := &journalv1.DurableCommandRecord{}
	if err := (proto.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
	if len(out.ProtoReflect().GetUnknown()) == 0 {
		t.Fatal("unknown field was silently discarded — must be detected")
	}
	// Unknown command_type enum number must not validate.
	bad := baseRecord("x.family", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD)
	bad.Command = &journalv1.DurableCommandRecord_Activity{Activity: &journalv1.JournalActivity{
		CharacterId: uuid(1), SessionEpoch: 1, Transition: "LOGIN", OccurredAtMs: 1}}
	bad.CommandType = journalv1.JournalCommandType(77)
	switch bad.GetCommandType() {
	case journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_REWARD,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_WORLD_CONSEQUENCE,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_BOSS_ELIGIBILITY,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_BOSS_CHEST,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHECKPOINT,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_PUBLIC_SCHEDULE,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_MATCH,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_GUILD_EVENT,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_JOB,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_ACTIVITY,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_COMPETITIVE_ADMISSION,
		journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHAT_LOG:
		t.Fatal("enum 77 must not be a known command type")
	}
}

func TestJournalOwnerFamilyMismatchRejected(t *testing.T) {
	rec := baseRecord("trade.finalise", journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_GUILD)
	rec.Command = &journalv1.DurableCommandRecord_Client{Client: &journalv1.JournalClientCommand{
		AccountId: uuid(0x72), SessionEpoch: 1,
		Request: &journalv1.JournalClientCommand_C2STradeFinalise{
			C2STradeFinalise: &protocolv1.C2STradeFinalise{OperationId: uuid(0x02)}}}}
	if err := validateRecord(rec); err == nil {
		t.Fatal("expected owner/family rejection: trade.finalise under GUILD owner")
	}
	rec.OwnerKind = journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER
	if err := validateRecord(rec); err != nil {
		t.Fatalf("correct owner should validate: %v", err)
	}
}
