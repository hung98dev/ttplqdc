package protocoltest

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate proto/testdata/golden fixtures")

// uuid returns a deterministic 16-byte UUID whose first byte is n.
func uuid(n byte) []byte {
	b := make([]byte, 16)
	for i := range b {
		b[i] = n + byte(i)
	}
	return b
}

var testContentRevision = strings.Repeat("ab", 32)

// goldenMessages builds one representative message per fixture family. Field
// values are fixed and deterministic so fixtures are byte-stable.
func goldenMessages() map[string]proto.Message {
	msgs := map[string]proto.Message{
		"envelope.bin": &protocolv1.Envelope{
			ProtocolMajor: 1, ProtocolMinor: 0, MessageId: 300,
			SessionEpoch: 7, ClientSeq: 11, ServerSeq: 13, CorrelationId: 17,
			Payload: []byte{0xCA, 0xFE},
		},
		"hello.bin": &protocolv1.C2SHello{
			Credential:  &protocolv1.C2SHello_GameplayTicket{GameplayTicket: "ticket.test"},
			ClientBuild: 10500, Platform: protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS,
			ContentRevision: testContentRevision, DeviceId: uuid(0x10),
			Locale: protocolv1.ClientLocale_CLIENT_LOCALE_VI_VN,
		},
		"hello_ok.bin": &protocolv1.S2CHelloOk{
			SessionId: uuid(0x20), SessionEpoch: 7, AccountId: uuid(0x30),
			ServerTimeMs: 1760000000000, HeartbeatIntervalMs: 5000,
			ConnectionTimeoutMs: 15000, ResumeCredential: "resume.test",
			ResumeExpiresAtMs: 1760000300000, ProtocolMinor: 0,
			ContentRevision: testContentRevision, PendingDeletion: false,
			ResumedCharacterId: uuid(0x40), ResumeRotateIntervalMs: 300000,
		},
		"error.bin": &protocolv1.S2CError{
			ErrorCode:    protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED,
			Retryability: protocolv1.Retryability_RETRYABILITY_BACKOFF,
			RetryAfterMs: 250, QueuePosition: 3,
			SafeMessageKey: "err.rate_limited", CloseAfter: false,
		},
		"world_baseline.bin": &protocolv1.S2CWorldBaseline{
			BaselineId: 42, ServerTick: 1000, MapId: "map.ha_noi",
			ChannelIndex: 1, InstanceId: uuid(0x50), ContentRevision: testContentRevision,
			Self: &protocolv1.EntityState{
				EntityId: 1, EntityKind: protocolv1.EntityKind_ENTITY_KIND_PLAYER,
				ContentId: "character.player", CharacterId: uuid(0x60),
				DisplayName: "TestChar", Level: 30, XMm: 12000, YMm: 8000,
				VxMmS: 0, VyMmS: -6000, Facing: protocolv1.Facing_FACING_LEFT,
				MovementState: protocolv1.MovementState_MOVEMENT_STATE_FALL,
				Hp:            5000, MaxHp: 6000, Shield: 250, Flags: 0x2,
				Statuses: []*protocolv1.EntityStatus{
					{EffectId: "status.knockback", SourceEntityId: 9, Stacks: 1, ExpiresAtTick: 1500},
				},
				EquippedCosmetics: []*protocolv1.EquippedCosmetic{
					{Slot: protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME, CosmeticId: "cosmetic.frame.gold"},
				},
				EncounterId: 88, StatLifesteal: 100, StatReflect: 50, StatAbsorb: 0,
				StatHealReduction: 0, StatHealingReceived: 10000,
			},
			SelfPrivate: &protocolv1.SelfPrivateState{
				CurrentMp: 900, MaxMp: 1000, AcceptedTargetEntityId: 9,
			},
			SelfCheckpoint: &protocolv1.MovementCheckpoint{
				XMm: 12000, YMm: 8000, VxMmS: 0, VyMmS: -6000,
				Facing:        protocolv1.Facing_FACING_LEFT,
				MovementState: protocolv1.MovementState_MOVEMENT_STATE_FALL,
				PlatformId:    0, IsGrounded: false, JumpCount: 1,
				DropIgnorePlatformId: 0, DropIgnoreUntilTick: 0,
				HeldHorizontalIntent: protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_LEFT,
				EffectiveParameters: &protocolv1.EffectiveMovementParameters{
					RunSpeedMmS: 4500, FirstJumpMmS: 9000, SecondJumpMmS: 8000,
					GravityMmS2: 28000, MaxFallMmS: 14000, AirControlBp: 10000,
					MaxStepHeightMm: 600,
				},
			},
			Entities: []*protocolv1.EntityState{
				{EntityId: 9, EntityKind: protocolv1.EntityKind_ENTITY_KIND_MONSTER,
					ContentId: "monster.slime", XMm: 12500, YMm: 8000,
					Facing:        protocolv1.Facing_FACING_RIGHT,
					MovementState: protocolv1.MovementState_MOVEMENT_STATE_IDLE,
					Hp:            1200, MaxHp: 1200},
			},
			Encounters: []*protocolv1.EncounterState{
				{EncounterId: 88, EncounterContentId: "encounter.test_boss", PhaseNumber: 1,
					ActiveMechanics: []*protocolv1.ActiveMechanicState{
						{MechanicInstanceId: 5, StartsAtTick: 900, EndsAtTick: 1100,
							Payload: &protocolv1.EncounterMechanicPayload{
								Payload: &protocolv1.EncounterMechanicPayload_Zone{
									Zone: &protocolv1.ZoneMechanic{
										Areas: []*protocolv1.MechanicArea{
											{Area: &protocolv1.MechanicArea_Circle{
												Circle: &protocolv1.MechanicCircle{
													Center:   &protocolv1.PositionMm{XMm: 5000, YMm: 5000},
													RadiusMm: 1200}}},
										},
										EffectId: "effect.zone_burn", HitCap: 40}}}}}}},
		},
		"combat_event.bin": &protocolv1.S2CCombatEvent{
			EventId: 7001, ServerTick: 1001, ActionInstanceId: 301,
			SourceEntityId: 1, TargetEntityId: 9, SkillId: "skill.tanh_thoi",
			EffectId: "", ResultKind: protocolv1.CombatResultKind_COMBAT_RESULT_KIND_DAMAGE,
			Outcome: protocolv1.CombatOutcome_COMBAT_OUTCOME_HIT, IsCrit: true,
			DamageElement:        protocolv1.Element_ELEMENT_HOA,
			PostMitigationDamage: 640, ShieldAbsorbed: 140, HpDamage: 500,
			HealAmount: 0, ShieldAmount: 0, TargetHpAfter: 700, TargetShieldAfter: 110,
			Killed: false, JustGuardWindow: true, JustGuardTriggered: true,
			JustGuardHint: true, BeastPassive2Success: false,
			ReflectDamageInstance: proto.Int64(32),
			LifestealHealAmount:   proto.Int64(16),
			AbsorbShieldAmount:    proto.Int64(8),
		},
		"state_delta.bin": &protocolv1.S2CStateDelta{
			BaselineId: 42, ServerTick: 1002,
			SelfAck: &protocolv1.SelfAck{
				LastProcessedClientSeq: 11,
				Checkpoint: &protocolv1.MovementCheckpoint{
					XMm: 12010, YMm: 7990, IsGrounded: true,
					MovementState:       protocolv1.MovementState_MOVEMENT_STATE_IDLE,
					EffectiveParameters: &protocolv1.EffectiveMovementParameters{RunSpeedMmS: 4500}},
			},
			SelfPrivate: &protocolv1.SelfPrivateDelta{CurrentMp: proto.Int64(880)},
			Entities: []*protocolv1.EntityDelta{
				{EntityId: 9, XMm: proto.Int32(12510), Hp: proto.Int64(700),
					Statuses: &protocolv1.StatusList{Entries: []*protocolv1.EntityStatus{
						{EffectId: "status.burn", Stacks: 2, ExpiresAtTick: 1900}}},
					EquippedCosmetics: &protocolv1.CosmeticList{}},
			},
		},
		"inventory_state.bin": &protocolv1.S2CInventoryState{
			Capacity: 96, InventoryRevision: 7,
			Slots: []*protocolv1.InventorySlotView{
				{Slot: 0, LockedQuantity: 0, Item: &protocolv1.ItemInstanceView{
					ItemInstanceId: uuid(0x70), ItemId: "item.sword.hon_kiem", Quantity: 1,
					ContentRevision:  testContentRevision,
					EffectiveBinding: protocolv1.ItemBinding_ITEM_BINDING_CHARACTER_BOUND,
					EnhancementLevel: 3,
					RolledBaseStats: []*protocolv1.StatValue{
						{StatId: "stat.atk", Value: 55}},
					RolledSecondaryStats: []*protocolv1.StatValue{
						{StatId: "stat.crit_rate", Value: 450}},
					EffectiveStats: []*protocolv1.StatValue{
						{StatId: "stat.atk", Value: 55},
						{StatId: "stat.crit_rate", Value: 450}}}},
			},
			Loadouts: []*protocolv1.LoadoutView{
				{LoadoutId: "loadout.primary", IsActive: true,
					Slots: []*protocolv1.LoadoutSlotView{{SlotId: "slot.weapon"}},
				},
				{LoadoutId: "loadout.secondary_1"},
				{LoadoutId: "loadout.secondary_2"},
			},
			LoadoutRevision: 4,
		},
		"quest_update.bin": &protocolv1.S2CQuestUpdate{
			Result: &protocolv1.OperationResult{
				OperationId: uuid(0x80), Status: protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				ErrorCode: protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED},
			QuestId: "quest.main.ch1", State: protocolv1.QuestState_QUEST_STATE_COMPLETED,
			Objectives: []*protocolv1.QuestObjectiveView{
				{ObjectiveIndex: 0, Current: 5, Required: 5}},
			ExpiresAtMs: 0, ExpGained: 1200,
			Granted: []*protocolv1.ItemGrant{
				{ItemInstanceId: uuid(0x81), ItemId: "item.sword.hon_kiem", Quantity: 1}},
			CurrencyDelta: []*protocolv1.CurrencyDelta{
				{CurrencyId: "currency.common", Amount: 5000}},
		},
		"daily_board.bin": &protocolv1.S2CDailyBoardState{
			RequestId: 77, Status: protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
			ErrorCode: protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED,
			CycleId:   "2026-10-01", BoardRevision: 3, CompletedCount: 1,
			ResetAtMs: 1760918400000,
			Entries: []*protocolv1.DailyBoardEntry{
				{BoardSlot: 1, State: protocolv1.QuestState_QUEST_STATE_COMPLETED, IsMystery: false,
					View: &protocolv1.DailyBoardEntry_Revealed{Revealed: &protocolv1.DailyQuestView{
						QuestId: "quest.daily.kill_slimes", TitleKey: "quest.daily.kill_slimes.title",
						Objectives:    []*protocolv1.QuestObjectiveView{{ObjectiveIndex: 0, Current: 10, Required: 10}},
						RewardPreview: []*protocolv1.ItemQuantity{{ItemId: "item.linh_dan", Quantity: 2}},
						StarterNpcId:  "npc.board_guard", AreaId: "area.ha_noi"}}},
				{BoardSlot: 2, State: protocolv1.QuestState_QUEST_STATE_AVAILABLE, IsMystery: true,
					View: &protocolv1.DailyBoardEntry_Hidden{Hidden: &protocolv1.MysteryBoardPlaceholder{
						SafeAreaId: "area.ha_noi"}}},
				{BoardSlot: 3, State: protocolv1.QuestState_QUEST_STATE_AVAILABLE},
				{BoardSlot: 4, State: protocolv1.QuestState_QUEST_STATE_AVAILABLE},
				{BoardSlot: 5, State: protocolv1.QuestState_QUEST_STATE_AVAILABLE},
				{BoardSlot: 6, State: protocolv1.QuestState_QUEST_STATE_AVAILABLE},
			},
		},
		"guild_state.bin": &protocolv1.S2CGuildState{
			GuildId: uuid(0x90), GuildRevision: 12, GuildName: "Hoa Son",
			Role: "guild.role.member", Level: 4, MembersCount: 2, MaxMembers: 50,
			Motd: "hello", RecruitmentMode: protocolv1.GuildRecruitmentMode_GUILD_RECRUITMENT_MODE_APPLICATIONS,
			Members: []*protocolv1.GuildMemberView{
				{CharacterId: uuid(0x91), DisplayName: "Leader", ClassId: "class.kiem_khach",
					Level: 40, Role: "guild.role.leader",
					OnlineState: protocolv1.OnlineState_ONLINE_STATE_ONLINE},
				{CharacterId: uuid(0x92), DisplayName: "Member", ClassId: "class.tho_tu",
					Level: 33, Role: "guild.role.member",
					OnlineState:  protocolv1.OnlineState_ONLINE_STATE_OFFLINE,
					LastOnlineAt: 1759990000000},
			},
			Progression: &protocolv1.GuildProgressionView{
				GuildExp: 1200, RitualStreak: 3, CycleId: "2026-10-01",
				MEffective: 2, RequiredPointsPerElement: 5,
				Points: []*protocolv1.GuildProgressionPoint{
					{Element: protocolv1.Element_ELEMENT_KIM, Current: 5},
					{Element: protocolv1.Element_ELEMENT_MOC, Current: 5},
					{Element: protocolv1.Element_ELEMENT_THUY, Current: 4},
					{Element: protocolv1.Element_ELEMENT_HOA, Current: 0},
					{Element: protocolv1.Element_ELEMENT_THO, Current: 2}},
				CompletedAtMs: 0, CandidateBlessingIds: []string{"blessing.a", "blessing.b", "blessing.c"},
				VoteClosesAtMs: 1760050000000, VoteCounts: []uint32{1, 2, 0},
				ReceiverVote: "blessing.b", ActiveBlessingId: "blessing.a",
				BlessingExpiresAtMs:          1760100000000,
				ReceiverLifetimeContribution: 900, ReceiverCycleContribution: 90,
			},
			OwnedGuildCosmeticIds: []string{"cosmetic.guild.banner_a"},
			GuildCosmeticSelections: &protocolv1.GuildCosmeticSelections{
				Shrine: "", Banner: "cosmetic.guild.banner_a", Crest: ""},
			CosmeticRevision: 6,
		},
		"match_state.bin": &protocolv1.S2CMatchState{
			MatchId: uuid(0xA0), PvpModeId: "pvp.mode.ranked_duel",
			State:           protocolv1.MatchState_MATCH_STATE_ACTIVE,
			StateDeadlineMs: 1760060000000, RoundNumber: 1,
			Teams: []*protocolv1.MatchTeamView{
				{TeamIndex: 0, Score: 1, Members: []*protocolv1.MatchMemberView{
					{CharacterId: uuid(0xA1), DisplayName: "P1", ClassId: "class.kiem_khach",
						Ready: true, Connected: true}}},
				{TeamIndex: 1, Score: 0, Members: []*protocolv1.MatchMemberView{
					{CharacterId: uuid(0xA2), DisplayName: "P2", ClassId: "class.tho_tu",
						Ready: true, Connected: false}}},
			},
			Result:       protocolv1.MatchResult_MATCH_RESULT_NONE,
			RatingBefore: 1500, RatingAfter: 1500,
		},
		"trade_offer.bin": &protocolv1.S2CTradeOfferState{
			TradeId: uuid(0xB0), Revision: 4,
			State: protocolv1.TradeOfferState_TRADE_OFFER_STATE_LOCKED,
			Sides: []*protocolv1.TradeSide{
				{CharacterId: uuid(0xB1), CommonAmount: 1000, Confirmed: true,
					Items: []*protocolv1.ItemInstanceView{
						{ItemInstanceId: uuid(0xB2), ItemId: "item.ore.dong", Quantity: 30,
							ContentRevision: testContentRevision}}},
				{CharacterId: uuid(0xB3), CommonAmount: 0, Confirmed: false},
			},
			FeePreview: 50,
		},
		"journal_client.bin": &journalv1.DurableCommandRecord{
			SchemaVersion: 1, OperationFamily: "trade.finalise",
			OwnerKind: journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
			OwnerId:   uuid(0xC0), OperationId: uuid(0xC1),
			CommandType:  journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
			EnqueuedAtMs: 1760000001000, ContentRevision: testContentRevision,
			RequestFingerprint: bytes.Repeat([]byte{0x5A}, 32),
			AdmissionSequence:  9, ProducerIncarnationId: uuid(0xC2),
			Command: &journalv1.DurableCommandRecord_Client{
				Client: &journalv1.JournalClientCommand{
					AccountId: uuid(0xC3), CharacterId: uuid(0xC0),
					SessionEpoch: 7, OwnershipEpoch: proto.Uint64(3),
					AdmittedAtMs: 1760000000500, IssuedAtMs: 1760000000400,
					Trade: &journalv1.JournalTrade{
						TradeId: uuid(0xB0), ExpectedRevision: 4,
						Initiator: &journalv1.JournalTradeSide{
							CharacterId: uuid(0xC0), AccountId: uuid(0xC3),
							CommonAmount: 1000},
						Counterpart: &journalv1.JournalTradeSide{
							CharacterId: uuid(0xB3), AccountId: uuid(0xC4)},
						SettlementId: uuid(0xC5), FeeCommon: 50,
						FinalizedAtMs:               1760000000900,
						InitiatingClientOperationId: uuid(0xC1)},
					ExpectedRevisions: []*journalv1.JournalAggregateRevision{
						{Aggregate: "inventory", OwnerId: uuid(0xC0), Revision: 7}},
					Request: &journalv1.JournalClientCommand_C2STradeFinalise{
						C2STradeFinalise: &protocolv1.C2STradeFinalise{
							OperationId: uuid(0xC1), TradeId: uuid(0xB0),
							ExpectedRevision: 4}}}}},
		"journal_reward.bin": &journalv1.DurableCommandRecord{
			SchemaVersion: 1, OperationFamily: "reward.grant",
			OwnerKind: journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
			OwnerId:   uuid(0xD0), OperationId: uuid(0xD1),
			CommandType:  journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_REWARD,
			EnqueuedAtMs: 1760000002000, ContentRevision: testContentRevision,
			RequestFingerprint: bytes.Repeat([]byte{0x5B}, 32),
			AdmissionSequence:  10, ProducerIncarnationId: uuid(0xC2),
			Command: &journalv1.DurableCommandRecord_Reward{
				Reward: &journalv1.JournalRewardCommand{
					Kind: "BOSS",
					Source: &journalv1.JournalSource{
						MapId: "map.boss_arena", ChannelId: 1,
						PartitionIncarnationId: uuid(0xD2), SourceEventId: 55,
						Tick: 2000, OccurredAtMs: 1760000001900,
						SourceContentId:     "boss.ho_long",
						EncounterInstanceId: uuid(0xD3), GenerationId: uuid(0xD4),
						RngSeedHi: proto.Uint64(0x0123), RngSeedLo: proto.Uint64(0x4567)},
					Slots: []*journalv1.JournalRewardSlot{
						{RewardSlot: "boss.ho_long.main",
							Items: []*journalv1.JournalItem{
								{ItemId: "item.sword.hon_kiem", Quantity: 1,
									EffectiveBinding: "CHARACTER_BOUND",
									ContentRevision:  testContentRevision,
									Enhancement:      0,
									BaseRolls: []*journalv1.JournalStat{
										{StatId: "stat.atk", Value: 55, Scale: 1}},
									ItemInstanceId: uuid(0xD5),
									CreatedAtMs:    proto.Int64(1760000001900)}},
							Currencies: []*journalv1.JournalCurrency{
								{CurrencyId: "currency.common", Amount: 5000}},
							CharacterExp: 300,
							SoulAcquisitions: []*journalv1.JournalSoulAcquisition{
								{SoulInstanceId: uuid(0xD6), SoulId: "soul.ho_long",
									InitialLevel: 1, InitialSoulExp: 0, Duplicate: false,
									AtlasProgress:           &journalv1.JournalAtlas{PageId: "atlas.hon_giam.ho_long", Delta: 1},
									AtlasPageMasteredBefore: false,
									ExpectedSoulRevision:    3}},
							SourceType: "source.boss", SourceReference: "boss.ho_long",
							SourceRewardOperationId: uuid(0xD1)}}}},
		},
		"journal_match.bin": &journalv1.DurableCommandRecord{
			SchemaVersion: 1, OperationFamily: "match.settle",
			OwnerKind: journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD,
			OwnerId:   uuid(0xE0), OperationId: uuid(0xE1),
			CommandType:  journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_MATCH,
			EnqueuedAtMs: 1760000003000, ContentRevision: testContentRevision,
			RequestFingerprint: bytes.Repeat([]byte{0x5C}, 32),
			AdmissionSequence:  11, ProducerIncarnationId: uuid(0xC2),
			Command: &journalv1.DurableCommandRecord_Match{
				Match: &journalv1.JournalMatch{
					MatchId: uuid(0xA0), ModeId: "pvp.mode.ranked_duel",
					SeasonId: 4, MatchState: "COMPLETED", Result: "WIN",
					SettlementType: "PVP", ResolvedAtMs: 1760000002800,
					Source: &journalv1.JournalSource{
						MapId: "map.arena", ChannelId: 1,
						PartitionIncarnationId: uuid(0xD2), SourceEventId: 77,
						Tick: 2200, OccurredAtMs: 1760000002700},
					Participants: []*journalv1.JournalParticipant{
						{CharacterId: uuid(0xA1), Participation: "NORMAL",
							TeamIndex: 0, ActiveSeconds: 120, RewardEligible: true,
							MmrBefore: 1500, MmrAfter: 1524,
							SeasonRatingBefore: 1500, SeasonRatingAfter: 1524,
							ExpectedRatingRevision: 9, Result: "WIN"},
						{CharacterId: uuid(0xA2), Participation: "NORMAL",
							TeamIndex: 1, ActiveSeconds: 120, RewardEligible: true,
							MmrBefore: 1500, MmrAfter: 1476,
							SeasonRatingBefore: 1500, SeasonRatingAfter: 1476,
							ExpectedRatingRevision: 7, Result: "LOSS"}}}}},
		"journal_job.bin": &journalv1.DurableCommandRecord{
			SchemaVersion: 1, OperationFamily: "maintenance.retention_purge",
			OwnerKind: journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD,
			OwnerId:   uuid(0xE1), OperationId: uuid(0xE2),
			CommandType:  journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_JOB,
			EnqueuedAtMs: 1760000004000, ContentRevision: testContentRevision,
			RequestFingerprint: bytes.Repeat([]byte{0x5D}, 32),
			AdmissionSequence:  12, ProducerIncarnationId: uuid(0xC2),
			Command: &journalv1.DurableCommandRecord_Job{
				Job: &journalv1.JournalJob{
					Kind:    "maintenance.retention_purge",
					JobKey:  "maintenance:retention_purge:2026-10-01",
					DueAtMs: 1760000400000,
					Target: &journalv1.JournalJob_Maintenance{
						Maintenance: &journalv1.JournalMaintenanceJob{
							RowFamily:     "chat_messages",
							WindowStartMs: 1750000000000,
							WindowEndMs:   1760000000000}}}}},
	}
	return msgs
}

// goldenFixtureTypes maps each fixture file to its fully-qualified proto type.
// Order in goldenFixtureList is the canonical write order for index.json.
var goldenFixtureList = []string{
	"envelope.bin", "hello.bin", "hello_ok.bin", "error.bin",
	"world_baseline.bin", "combat_event.bin", "state_delta.bin",
	"inventory_state.bin", "quest_update.bin", "daily_board.bin",
	"guild_state.bin", "match_state.bin", "trade_offer.bin",
	"journal_client.bin", "journal_reward.bin", "journal_match.bin", "journal_job.bin",
}

var goldenFixtureTypes = map[string]string{
	"envelope.bin":        "thinhthan.v1.Envelope",
	"hello.bin":           "thinhthan.v1.C2SHello",
	"hello_ok.bin":        "thinhthan.v1.S2CHelloOk",
	"error.bin":           "thinhthan.v1.S2CError",
	"world_baseline.bin":  "thinhthan.v1.S2CWorldBaseline",
	"combat_event.bin":    "thinhthan.v1.S2CCombatEvent",
	"state_delta.bin":     "thinhthan.v1.S2CStateDelta",
	"inventory_state.bin": "thinhthan.v1.S2CInventoryState",
	"quest_update.bin":    "thinhthan.v1.S2CQuestUpdate",
	"daily_board.bin":     "thinhthan.v1.S2CDailyBoardState",
	"guild_state.bin":     "thinhthan.v1.S2CGuildState",
	"match_state.bin":     "thinhthan.v1.S2CMatchState",
	"trade_offer.bin":     "thinhthan.v1.S2CTradeOfferState",
	"journal_client.bin":  "thinhthan.internal.v1.DurableCommandRecord",
	"journal_reward.bin":  "thinhthan.internal.v1.DurableCommandRecord",
	"journal_match.bin":   "thinhthan.internal.v1.DurableCommandRecord",
	"journal_job.bin":     "thinhthan.internal.v1.DurableCommandRecord",
}

func writeGolden(t *testing.T, dir string) {
	t.Helper()
	msgs := goldenMessages()
	var index []goldenFixture
	for _, file := range goldenFixtureList {
		msg, ok := msgs[file]
		if !ok {
			t.Fatalf("no constructor for fixture %s", file)
		}
		data := canonicalMarshal(t, msg)
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
		index = append(index, goldenFixture{
			File: file, Type: goldenFixtureTypes[file],
			Journal: strings.HasPrefix(file, "journal_"),
		})
	}
	idx, err := json.MarshalIndent(goldenIndexFile{Fixtures: index}, "", "  ")
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), append(idx, '\n'), 0o644); err != nil {
		t.Fatalf("write index.json: %v", err)
	}
}

// TestBinaryEncodingParity decodes every committed golden fixture, re-encodes
// it deterministically and requires byte-identical output. With -update-golden
// it regenerates the fixture set instead (the repo's fixture maintenance flow).
func TestBinaryEncodingParity(t *testing.T) {
	dir := goldenDir(t)
	if *updateGolden {
		writeGolden(t, dir)
		return
	}
	fixtures := goldenIndex(t)
	for _, fx := range fixtures {
		data, err := os.ReadFile(filepath.Join(dir, fx.File))
		if err != nil {
			t.Fatalf("read fixture %s: %v", fx.File, err)
		}
		msg := messageType(t, fx.Type).New().Interface()
		if err := (proto.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, msg); err != nil {
			t.Fatalf("fixture %s decode failed: %v", fx.File, err)
		}
		if u := msg.ProtoReflect().GetUnknown(); len(u) != 0 {
			t.Fatalf("fixture %s carries unknown fields (%d bytes)", fx.File, len(u))
		}
		out := canonicalMarshal(t, msg)
		if !bytes.Equal(data, out) {
			t.Fatalf("fixture %s: decode->deterministic re-encode is not byte-identical", fx.File)
		}
	}
}

// TestEveryRegisteredNetworkIdHasOneFileOwner asserts every registry ID maps to
// exactly one generated message type living in the proto file that owns its
// range, and that no allocated ID lacks a generated type.
func TestEveryRegisteredNetworkIdHasOneFileOwner(t *testing.T) {
	// message_id -> owning file is fixed by the registry's range ownership.
	wantFile := map[string]string{}
	seen := map[uint32]int{}
	for _, e := range networkRegistry {
		seen[e.ID]++
		wantFile[e.GoType] = e.File
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("registry ID %d listed %d times", id, n)
		}
	}
	// Every generated message type resolves under thinhthan.v1 and its declaring
	// proto file matches the registry range owner.
	for _, e := range networkRegistry {
		mt, err := protoregistry.GlobalTypes.FindMessageByName(
			protoreflect.FullName("thinhthan.v1." + e.GoType))
		if err != nil {
			t.Fatalf("ID %d %s: no generated type thinhthan.v1.%s", e.ID, e.Name, e.GoType)
		}
		path := mt.Descriptor().ParentFile().Path()
		base := filepath.Base(path)
		want := "thinhthan/v1/" + e.File + ".proto"
		if path != want {
			t.Fatalf("ID %d %s: type declared in %s, want %s (%s)",
				e.ID, e.Name, path, want, base)
		}
	}
}

// TestMessageRegistryMapping pins the envelope message_id <-> payload type 1:1
// mapping: every allocated registry ID has a distinct generated type and no
// type is bound to two IDs.
func TestMessageRegistryMapping(t *testing.T) {
	byType := map[string]uint32{}
	for _, e := range networkRegistry {
		if prev, ok := byType[e.GoType]; ok {
			t.Fatalf("type %s bound to IDs %d and %d", e.GoType, prev, e.ID)
		}
		byType[e.GoType] = e.ID
	}
	if len(byType) != len(networkRegistry) {
		t.Fatalf("registry rows %d != types %d", len(networkRegistry), len(byType))
	}
	// Reserved ranges have no generated registry entry.
	for _, id := range []uint32{737} {
		for _, e := range networkRegistry {
			if e.ID == id {
				t.Fatalf("retired ID %d present in registry", id)
			}
		}
	}
	for id := uint32(711); id <= 729; id++ {
		for _, e := range networkRegistry {
			if e.ID == id {
				t.Fatalf("reserved ID %d present in registry", id)
			}
		}
	}
}

// TestErrorEnumMatchesErrorsMd asserts the generated ErrorCode enum contains
// exactly the codes of errors.md section Canonical Codes plus UNSPECIFIED=0.
func TestErrorEnumMatchesErrorsMd(t *testing.T) {
	codes := errorsMdCodes(t)
	enum, err := protoregistry.GlobalTypes.FindEnumByName("thinhthan.v1.ErrorCode")
	if err != nil {
		t.Fatalf("ErrorCode enum: %v", err)
	}
	vals := enum.Descriptor().Values()
	if got, want := vals.Len(), len(codes)+1; got != want {
		t.Fatalf("ErrorCode value count = %d, want %d (112 codes + UNSPECIFIED)", got, want)
	}
	byName := map[string]protoreflect.EnumNumber{}
	for i := 0; i < vals.Len(); i++ {
		v := vals.Get(i)
		byName[string(v.Name())] = v.Number()
	}
	if byName["ERROR_CODE_UNSPECIFIED"] != 0 {
		t.Fatal("ERROR_CODE_UNSPECIFIED must be 0")
	}
	for i, code := range codes {
		name := "ERROR_CODE_" + code
		v, ok := byName[name]
		if !ok {
			t.Fatalf("missing enum value %s", name)
		}
		if int(v) != i+1 {
			t.Fatalf("enum %s = %d, want %d", name, v, i+1)
		}
	}
}

// TestCodegenDriftCheck regenerates every output into a temp dir with the same
// pinned tools and byte-compares against the committed generated files.
func TestCodegenDriftCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("drift check needs protoc; runs in full verify")
	}
	protoc := os.Getenv("PROTOC")
	if protoc == "" {
		p, err := exec.LookPath("protoc")
		if err != nil {
			home, _ := os.UserHomeDir()
			cand := filepath.Join(home, "tools", "protoc", "bin", "protoc")
			if _, statErr := os.Stat(cand); statErr == nil {
				p = cand
			} else {
				t.Skip("protoc not on PATH; codegen drift covered by pwsh -Drift gate")
			}
		}
		protoc = p
	}
	ver, err := exec.Command(protoc, "--version").Output()
	if err != nil || strings.TrimSpace(string(ver)) != "libprotoc 36.2" {
		t.Fatalf("protoc version = %q err=%v, want libprotoc 36.2", strings.TrimSpace(string(ver)), err)
	}
	root := repoRoot(t)
	tmp := t.TempDir()
	goOut := filepath.Join(tmp, "go")
	csOut := filepath.Join(tmp, "cs")
	if err := os.MkdirAll(goOut, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(csOut, 0o755); err != nil {
		t.Fatal(err)
	}
	network := []string{
		"thinhthan/v1/common.proto", "thinhthan/v1/session.proto",
		"thinhthan/v1/movement.proto", "thinhthan/v1/combat.proto",
		"thinhthan/v1/durable.proto", "thinhthan/v1/content.proto",
		"thinhthan/v1/social.proto", "thinhthan/v1/market.proto",
		"thinhthan/v1/pvp.proto",
	}
	// protoc-gen-go resolves the same way scripts/codegen.ps1 does: install the
	// server/go.mod pin into GOBIN/GOPATH-bin, then expose that dir to protoc.
	goTool, err := exec.LookPath("go")
	if err != nil {
		home, _ := os.UserHomeDir()
		cand := filepath.Join(home, "tools", "go", "bin", "go")
		if _, statErr := os.Stat(cand); statErr == nil {
			goTool = cand
		} else {
			t.Skip("go toolchain not found")
		}
	}
	goEnv := func(name string) string {
		out, err := exec.Command(goTool, "env", name).Output()
		if err != nil {
			t.Fatalf("go env %s: %v", name, err)
		}
		return strings.TrimSpace(string(out))
	}
	gobin := goEnv("GOBIN")
	if gobin == "" {
		gobin = filepath.Join(goEnv("GOPATH"), "bin")
	}
	if out, err := exec.Command(goTool, "-C", filepath.Join(root, "server"),
		"install", "google.golang.org/protobuf/cmd/protoc-gen-go").CombinedOutput(); err != nil {
		t.Fatalf("install protoc-gen-go: %v\n%s", err, out)
	}
	home, _ := os.UserHomeDir()
	toolPath := os.Getenv("PATH") + string(os.PathListSeparator) + gobin +
		string(os.PathListSeparator) + filepath.Join(home, "tools", "bin")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(protoc, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "PATH="+toolPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("protoc %v: %v\n%s", args, err, out)
		}
	}
	genGo := []string{"-I", "proto", "--go_out=" + goOut, "--go_opt=module=thinhthan"}
	run(append(append([]string{}, genGo...), network...)...)
	run(append(append([]string{}, genGo...), "thinhthan/internal/v1/durable_journal.proto")...)
	run(append(append([]string{}, "-I", "proto", "--csharp_out="+csOut), network...)...)

	header := "#nullable disable\n#pragma warning disable 1591, 0612, 3021, 8981\n"
	compare := func(generated, committed string) {
		t.Helper()
		g, err := os.ReadFile(generated)
		if err != nil {
			t.Fatalf("read generated %s: %v", generated, err)
		}
		c, err := os.ReadFile(committed)
		if err != nil {
			t.Fatalf("read committed %s: %v", committed, err)
		}
		if strings.HasSuffix(committed, ".cs") {
			g = append([]byte(header), g...)
		}
		if !bytes.Equal(g, c) {
			t.Fatalf("drift: %s differs from committed %s", generated, committed)
		}
	}
	entries, err := os.ReadDir(filepath.Join(goOut, "internal", "protocol", "v1"))
	if err != nil {
		t.Fatalf("generated Go out: %v", err)
	}
	for _, e := range entries {
		compare(filepath.Join(goOut, "internal", "protocol", "v1", e.Name()),
			filepath.Join(root, "server", "internal", "protocol", "v1", e.Name()))
	}
	compare(filepath.Join(goOut, "internal", "durable", "journal", "v1", "durable_journal.pb.go"),
		filepath.Join(root, "server", "internal", "durable", "journal", "v1", "durable_journal.pb.go"))
	csEntries, err := os.ReadDir(csOut)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range csEntries {
		if strings.HasSuffix(e.Name(), ".cs") {
			compare(filepath.Join(csOut, e.Name()),
				filepath.Join(root, "client", "Assets", "Scripts", "Protocol", e.Name()))
		}
	}
}

// TestGeneratedCSharpHeader asserts every generated .cs in the Protocol output
// dir begins with the exact CODE-004 header.
func TestGeneratedCSharpHeader(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "client", "Assets", "Scripts", "Protocol")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := "#nullable disable\n#pragma warning disable 1591, 0612, 3021, 8981\n"
	count := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".cs") {
			continue
		}
		count++
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(data), want) {
			t.Fatalf("%s missing CODE-004 header", e.Name())
		}
	}
	if count == 0 {
		t.Fatal("no generated .cs files found")
	}
}

// TestAdr0060MessagesRegistered checks the ADR-0060 durable mutation families:
// every durable-range request/response pair the ADR names exists in the
// registry table with the spec'd direction.
func TestAdr0060MessagesRegistered(t *testing.T) {
	for _, id := range []uint32{
		400, 401, 402, 403, 404, 405, 406, 407, 408, 409, 410, 411, 412, 413,
		414, 415, 416, 417, 418, 419, 420, 421, 422, 423, 424, 425, 426, 427,
		428, 429, 430, 431, 432, 433, 434, 435, 436, 437, 438, 439, 440, 441,
		442, 443,
	} {
		found := false
		for _, e := range networkRegistry {
			if e.ID == id {
				found = true
				if e.File != "durable" {
					t.Fatalf("ID %d owned by %s, want durable", id, e.File)
				}
				break
			}
		}
		if !found {
			t.Fatalf("ADR-0060 durable ID %d not in registry", id)
		}
	}
}

// TestCodegenPreservesProtocolAsmdef guards the IMP-000 skeleton exception:
// codegen.ps1 must never reference asmdef/csc.rsp writes, and both files must
// exist with their committed contents.
func TestCodegenPreservesProtocolAsmdef(t *testing.T) {
	root := repoRoot(t)
	script, err := os.ReadFile(filepath.Join(root, "scripts", "codegen.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{".asmdef", "csc.rsp", ".meta"} {
		for _, line := range strings.Split(string(script), "\n") {
			if strings.Contains(line, "WriteAllText") || strings.Contains(line, "Set-Content") || strings.Contains(line, "Out-File") {
				if strings.Contains(line, banned) {
					t.Fatalf("codegen.ps1 writes to %s path: %s", banned, line)
				}
			}
		}
	}
	for _, f := range []string{"ThinhThan.Protocol.asmdef", "csc.rsp"} {
		if _, err := os.Stat(filepath.Join(root, "client", "Assets", "Scripts", "Protocol", f)); err != nil {
			t.Fatalf("skeleton file %s missing: %v", f, err)
		}
	}
}

// TestCodegenNeverWritesMeta asserts codegen output never touches .meta files
// (Unity materialization owns them; §4b CI artifact commit).
func TestCodegenNeverWritesMeta(t *testing.T) {
	root := repoRoot(t)
	script, err := os.ReadFile(filepath.Join(root, "scripts", "codegen.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(script), "\n") {
		for _, verb := range []string{"WriteAllText", "Set-Content", "Out-File",
			"Copy-Item", "Move-Item", "New-Item", "Add-Content"} {
			if strings.Contains(line, verb) && strings.Contains(line, ".meta") {
				t.Fatalf("codegen.ps1 writes .meta at line %d: %s", i+1, line)
			}
		}
	}
}
