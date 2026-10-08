package queue

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

// ProducerKind identifies a closed producer entry from save_rules.md's
// launch producer registry. ERASURE_RESUME is a JOB record whose target is
// the erasure continuation — classified separately because its queue
// acknowledgement means durable continuation ownership only, never
// erasure completion (JRN-008).
type ProducerKind int

const (
	ProducerClient ProducerKind = iota + 1
	ProducerReward
	ProducerWorldConsequence
	ProducerBossEligibility
	ProducerBossChest
	ProducerCheckpoint
	ProducerPublicSchedule
	ProducerCompetitiveAdmission
	ProducerMatch
	ProducerGuildEvent
	ProducerJob
	ProducerActivity
	ProducerChatLog
	ProducerErasureResume
)

func (k ProducerKind) String() string {
	switch k {
	case ProducerClient:
		return "CLIENT"
	case ProducerReward:
		return "REWARD"
	case ProducerWorldConsequence:
		return "WORLD_CONSEQUENCE"
	case ProducerBossEligibility:
		return "BOSS_ELIGIBILITY"
	case ProducerBossChest:
		return "BOSS_CHEST"
	case ProducerCheckpoint:
		return "CHECKPOINT"
	case ProducerPublicSchedule:
		return "PUBLIC_SCHEDULE"
	case ProducerCompetitiveAdmission:
		return "COMPETITIVE_ADMISSION"
	case ProducerMatch:
		return "MATCH"
	case ProducerGuildEvent:
		return "GUILD_EVENT"
	case ProducerJob:
		return "JOB"
	case ProducerActivity:
		return "ACTIVITY"
	case ProducerChatLog:
		return "CHAT_LOG"
	case ProducerErasureResume:
		return "ERASURE_RESUME"
	default:
		return "UNKNOWN"
	}
}

// RegistryEntry mirrors one row of the save_rules.md producer registry:
// the canonical operation_family (pattern where the spec expands one) and
// the required owner kind. RequiresClientReceipt marks the producers that
// admit through a durable_command_receipts row before enqueue (CLIENT).
type RegistryEntry struct {
	Kind                  ProducerKind
	Family                string
	OwnerKind             idempotency.OwnerKind
	RequiresClientReceipt bool
}

// registry mirrors the complete 15-row launch producer table in
// save_rules.md, in table order. Rows sharing a ProducerKind differ in
// their family/owner rule and are resolved inside the kind's validator.
var registry = []RegistryEntry{
	// Edge/Sim: every durable C2S request in the closed §7 client expansion.
	{ProducerClient, "<§7 client expansion>", idempotency.OwnerCharacter, true},
	// World/Instance kill/dungeon/event settlement rewards.
	{ProducerReward, "sim.<kind>_settlement", idempotency.OwnerCharacter, false},
	// Standalone quest/discovery/Atlas/feat/chivalry/Soul/beast/cosmetic/
	// Guild Stone grants (incl. authored content.grant families).
	{ProducerReward, "content.<kind>.grant", idempotency.OwnerCharacter, false},
	{ProducerWorldConsequence, "world.consequence", idempotency.OwnerWorld, false},
	{ProducerBossEligibility, "boss.eligibility", idempotency.OwnerCharacter, false},
	{ProducerBossChest, "boss.chest", idempotency.OwnerCharacter, false},
	{ProducerCheckpoint, "sim.checkpoint", idempotency.OwnerCharacter, false},
	{ProducerPublicSchedule, "boss.schedule", idempotency.OwnerWorld, false},
	{ProducerCompetitiveAdmission, "competitive.admission", idempotency.OwnerWorld, false},
	{ProducerMatch, "pvp.settlement / guild_war.<type>", idempotency.OwnerCharacter, false},
	{ProducerGuildEvent, "guild.event", idempotency.OwnerGuild, false},
	// Direct-trade finalization is a CLIENT 708 request (trade.finalise).
	{ProducerClient, "trade.finalise", idempotency.OwnerCharacter, true},
	{ProducerActivity, "character.activity", idempotency.OwnerCharacter, false},
	{ProducerChatLog, "chat.log", idempotency.OwnerCharacter, false},
	// Durable/worker jobs incl. the ERASURE_RESUME continuation target.
	{ProducerJob, "job.<target>", idempotency.OwnerWorld, false},
}

// Registry returns the closed producer table (copy) mirroring
// save_rules.md.
func Registry() []RegistryEntry {
	out := make([]RegistryEntry, len(registry))
	copy(out, registry)
	return out
}

// ---------------------------------------------------------------------------
// Closed family tables (protobuf_conventions.md §7 / save_rules.md).

// clientFamiliesExact is the literal §7 client expansion minus the
// interaction.<kind> and client.<ID> patterns.
var clientFamiliesExact = map[string]struct{}{
	"character.create": {},
	"placement.portal": {}, "placement.channel": {},
	"instance.enter": {}, "instance.respond": {}, "instance.leave": {}, "instance.cancel": {},
	"inventory.mutate": {}, "loadout.change": {}, "craft.create": {}, "enhance.apply": {},
	"reward.claim": {}, "beast.active": {}, "beast.equip": {}, "beast.unequip": {},
	"beast.feed": {}, "entitlement.claim": {}, "shop.buy": {}, "cosmetic.redeem": {},
	"cosmetic.equip": {}, "shop.sell": {}, "inventory.expand": {}, "beast.level_up": {},
	"quest.accept": {}, "quest.turn_in": {}, "atlas.acknowledge": {}, "quest.abandon": {},
	"story.choose": {}, "skill.upgrade": {}, "potential.allocate": {}, "progression.respec": {},
	"trade.finalise": {},
	"auction.list":   {}, "auction.buy": {}, "auction.cancel": {},
	"auction.reclaim": {}, "auction.proceeds": {},
}

// rewardKinds is the closed JournalRewardCommand kind set.
var rewardKinds = map[string]struct{}{
	"kill": {}, "boss": {}, "dungeon": {}, "event": {},
	"quest": {}, "discovery": {}, "atlas": {}, "feat": {}, "chivalry": {},
	"soul": {}, "beast": {}, "cosmetic": {}, "guild_stone": {}, "rest": {},
}

// jobTargetOwner maps the closed JOB target to its required owner kind
// (save_rules.md: WORLD for auction/season/erasure/maintenance, CHARACTER
// for quest/compensation, GUILD for guild, ACCOUNT for payment).
var jobTargetOwner = map[string]idempotency.OwnerKind{
	"auction":      idempotency.OwnerWorld,
	"quest":        idempotency.OwnerCharacter,
	"guild":        idempotency.OwnerGuild,
	"season":       idempotency.OwnerWorld,
	"erasure":      idempotency.OwnerWorld,
	"payment":      idempotency.OwnerAccount,
	"maintenance":  idempotency.OwnerWorld,
	"compensation": idempotency.OwnerCharacter,
}

func isClientFamily(f string) bool {
	if _, ok := clientFamiliesExact[f]; ok {
		return true
	}
	if rest, ok := strings.CutPrefix(f, "interaction."); ok && rest != "" && rest == strings.ToLower(rest) {
		return true
	}
	if rest, ok := strings.CutPrefix(f, "client."); ok && rest != "" {
		for _, r := range rest {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	return false
}

func jobTargetName(j *journalv1.JournalJob) string {
	if j == nil {
		return ""
	}
	switch j.GetTarget().(type) {
	case *journalv1.JournalJob_Auction:
		return "auction"
	case *journalv1.JournalJob_Quest:
		return "quest"
	case *journalv1.JournalJob_Guild:
		return "guild"
	case *journalv1.JournalJob_Season:
		return "season"
	case *journalv1.JournalJob_Erasure:
		return "erasure"
	case *journalv1.JournalJob_Payment:
		return "payment"
	case *journalv1.JournalJob_Maintenance:
		return "maintenance"
	case *journalv1.JournalJob_Compensation:
		return "compensation"
	default:
		return ""
	}
}

func ownerKindOf(k journalv1.JournalOwnerKind) (idempotency.OwnerKind, bool) {
	switch k {
	case journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_ACCOUNT:
		return idempotency.OwnerAccount, true
	case journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER:
		return idempotency.OwnerCharacter, true
	case journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_GUILD:
		return idempotency.OwnerGuild, true
	case journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_WORLD:
		return idempotency.OwnerWorld, true
	default:
		return "", false
	}
}

// producerOf maps the command oneof variant to its producer kind and the
// expected JournalCommandType discriminator.
func producerOf(rec *journalv1.DurableCommandRecord) (ProducerKind, journalv1.JournalCommandType, error) {
	switch c := rec.GetCommand().(type) {
	case *journalv1.DurableCommandRecord_Client:
		return ProducerClient, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT, nil
	case *journalv1.DurableCommandRecord_Reward:
		return ProducerReward, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_REWARD, nil
	case *journalv1.DurableCommandRecord_WorldConsequence:
		return ProducerWorldConsequence, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_WORLD_CONSEQUENCE, nil
	case *journalv1.DurableCommandRecord_BossEligibility:
		return ProducerBossEligibility, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_BOSS_ELIGIBILITY, nil
	case *journalv1.DurableCommandRecord_BossChest:
		return ProducerBossChest, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_BOSS_CHEST, nil
	case *journalv1.DurableCommandRecord_Checkpoint:
		return ProducerCheckpoint, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHECKPOINT, nil
	case *journalv1.DurableCommandRecord_PublicSchedule:
		return ProducerPublicSchedule, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_PUBLIC_SCHEDULE, nil
	case *journalv1.DurableCommandRecord_Match:
		return ProducerMatch, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_MATCH, nil
	case *journalv1.DurableCommandRecord_GuildEvent:
		return ProducerGuildEvent, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_GUILD_EVENT, nil
	case *journalv1.DurableCommandRecord_Job:
		if c.Job.GetErasure() != nil {
			return ProducerErasureResume, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_JOB, nil
		}
		return ProducerJob, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_JOB, nil
	case *journalv1.DurableCommandRecord_Activity:
		return ProducerActivity, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_ACTIVITY, nil
	case *journalv1.DurableCommandRecord_CompetitiveAdmission:
		return ProducerCompetitiveAdmission, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_COMPETITIVE_ADMISSION, nil
	case *journalv1.DurableCommandRecord_ChatLog:
		return ProducerChatLog, journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHAT_LOG, nil
	default:
		return 0, 0, fmt.Errorf("%w: unregistered command variant %T", ErrUnknownProducer, rec.GetCommand())
	}
}

// familyOwner resolves the closed family/owner rule for a record's
// producer kind.
func familyOwner(kind ProducerKind, rec *journalv1.DurableCommandRecord) (idempotency.OwnerKind, error) {
	fam := rec.GetOperationFamily()
	switch kind {
	case ProducerClient:
		if !isClientFamily(fam) {
			return "", fmt.Errorf("%w: client family %q not in §7 expansion", ErrUnknownProducer, fam)
		}
		if fam == "character.create" {
			return idempotency.OwnerAccount, nil
		}
		return idempotency.OwnerCharacter, nil
	case ProducerReward:
		k := strings.ToLower(rec.GetReward().GetKind())
		if _, ok := rewardKinds[k]; !ok {
			return "", fmt.Errorf("%w: reward kind %q", ErrUnknownProducer, rec.GetReward().GetKind())
		}
		if fam == "sim."+k+"_settlement" || fam == "content."+k+".grant" {
			return idempotency.OwnerCharacter, nil
		}
		return "", fmt.Errorf("%w: reward family %q", ErrUnknownProducer, fam)
	case ProducerWorldConsequence:
		if fam == "world.consequence" {
			return idempotency.OwnerWorld, nil
		}
	case ProducerBossEligibility:
		if fam == "boss.eligibility" {
			return idempotency.OwnerCharacter, nil
		}
	case ProducerBossChest:
		if fam == "boss.chest" {
			return idempotency.OwnerCharacter, nil
		}
	case ProducerCheckpoint:
		if fam == "sim.checkpoint" {
			return idempotency.OwnerCharacter, nil
		}
	case ProducerPublicSchedule:
		if fam == "boss.schedule" {
			return idempotency.OwnerWorld, nil
		}
	case ProducerCompetitiveAdmission:
		if fam == "competitive.admission" {
			return idempotency.OwnerWorld, nil
		}
	case ProducerMatch:
		st := strings.ToLower(rec.GetMatch().GetSettlementType())
		switch st {
		case "pvp":
			if fam == "pvp.settlement" {
				return idempotency.OwnerCharacter, nil
			}
		case "guild_rating", "guild_progression":
			if fam == "guild_war."+st {
				return idempotency.OwnerGuild, nil
			}
		case "personal_reward", "season_participation":
			if fam == "guild_war."+st {
				return idempotency.OwnerCharacter, nil
			}
		}
		return "", fmt.Errorf("%w: match settlement %q family %q", ErrUnknownProducer, st, fam)
	case ProducerGuildEvent:
		if fam == "guild.event" {
			return idempotency.OwnerGuild, nil
		}
	case ProducerJob, ProducerErasureResume:
		target := jobTargetName(rec.GetJob())
		owner, ok := jobTargetOwner[target]
		if !ok || fam != "job."+target {
			return "", fmt.Errorf("%w: job target %q family %q", ErrUnknownProducer, target, fam)
		}
		return owner, nil
	case ProducerActivity:
		if fam == "character.activity" {
			return idempotency.OwnerCharacter, nil
		}
	case ProducerChatLog:
		if fam == "chat.log" {
			return idempotency.OwnerCharacter, nil
		}
	}
	return "", fmt.Errorf("%w: family %q", ErrUnknownProducer, fam)
}

// ValidateRecord enforces closed-registry admission: the record's command
// variant must be a registered producer, command_type must match the
// variant, family/owner must satisfy the registry rule, identity fields
// must be complete, and the record must be protobuf-encodable within the
// journal 1 MiB bound. Returns the per-aggregate ordering key.
func ValidateRecord(rec *journalv1.DurableCommandRecord) (AggKey, error) {
	if rec == nil || rec.GetCommand() == nil {
		return AggKey{}, fmt.Errorf("%w: no typed command", ErrUnknownProducer)
	}
	kind, wantType, err := producerOf(rec)
	if err != nil {
		return AggKey{}, err
	}
	if rec.GetCommandType() != wantType {
		return AggKey{}, fmt.Errorf("%w: command_type %s vs variant %s",
			ErrUnknownProducer, rec.GetCommandType(), wantType)
	}
	owner, ok := ownerKindOf(rec.GetOwnerKind())
	if !ok {
		return AggKey{}, fmt.Errorf("%w: owner_kind %s", ErrUnknownProducer, rec.GetOwnerKind())
	}
	wantOwner, err := familyOwner(kind, rec)
	if err != nil {
		return AggKey{}, err
	}
	if wantOwner != owner {
		return AggKey{}, fmt.Errorf("%w: owner %s, family %q requires %s",
			ErrUnknownProducer, owner, rec.GetOperationFamily(), wantOwner)
	}
	if rec.GetOperationFamily() == "" || len(rec.GetOperationId()) != 16 ||
		len(rec.GetOwnerId()) != 16 || len(rec.GetRequestFingerprint()) != 32 {
		return AggKey{}, fmt.Errorf("%w: incomplete identity fields", ErrUnknownProducer)
	}
	var ownerID id.UUID
	copy(ownerID[:], rec.GetOwnerId())
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	if opID.IsNil() || ownerID.IsNil() {
		return AggKey{}, fmt.Errorf("%w: nil owner/operation id", ErrUnknownProducer)
	}
	enc, err := proto.MarshalOptions{Deterministic: true}.Marshal(rec)
	if err != nil {
		return AggKey{}, fmt.Errorf("%w: unencodable record: %v", ErrUnknownProducer, err)
	}
	if len(enc) > 1048576 {
		return AggKey{}, fmt.Errorf("%w: record %d bytes exceeds 1 MiB", ErrUnknownProducer, len(enc))
	}
	return AggKey{Family: rec.GetOperationFamily(), OwnerKind: owner, OwnerID: ownerID}, nil
}

// ProducerOf exposes the validated producer kind for a record (the record
// must pass ValidateRecord first).
func ProducerOf(rec *journalv1.DurableCommandRecord) ProducerKind {
	kind, _, err := producerOf(rec)
	if err != nil {
		return 0
	}
	return kind
}
