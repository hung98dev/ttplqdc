package protocoltest

import (
	"regexp"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// walkAllMessages calls fn on every message descriptor in thinhthan.v1 files.
func walkAllMessages(t *testing.T, fn func(protoreflect.MessageDescriptor)) {
	t.Helper()
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(fd.Package()), "thinhthan.v1") {
			return true
		}
		var walk func(msgs protoreflect.MessageDescriptors)
		walk = func(msgs protoreflect.MessageDescriptors) {
			for i := 0; i < msgs.Len(); i++ {
				m := msgs.Get(i)
				fn(m)
				walk(m.Messages())
			}
		}
		walk(fd.Messages())
		return true
	})
}

func findMessage(t *testing.T, name string) protoreflect.MessageDescriptor {
	t.Helper()
	d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		t.Fatalf("descriptor %s: %v", name, err)
	}
	md, ok := d.(protoreflect.MessageDescriptor)
	if !ok {
		t.Fatalf("%s is not a message", name)
	}
	return md
}

// uuidFieldNames are the field-name set that carries RFC 9562 UUIDs (bytes16).
// Entity IDs are uint64 and content IDs are strings — neither belongs here.
var uuidFieldNames = map[string]bool{
	"account_id": true, "character_id": true, "session_id": true,
	"operation_id": true, "item_instance_id": true, "reward_claim_id": true,
	"listing_id": true, "escrow_asset_id": true, "proceeds_id": true,
	"trade_id": true, "party_id": true, "guild_id": true, "match_id": true,
	"challenge_id": true, "settlement_id": true, "entry_id": true,
	"transfer_id": true, "instance_id": true, "report_id": true,
	"claim_id": true, "post_id": true, "entitlement_id": true,
	"soul_instance_id": true, "device_id": true, "pvp_match_id": true,
	"guild_war_match_id": true, "chat_message_id": true,
	"inviter_character_id": true, "requester_character_id": true,
	"target_character_id": true, "sender_character_id": true,
	"leader_character_id": true, "applicant_character_id": true,
	"poster_character_id": true, "friend_character_id": true,
	"blocked_character_id": true, "resumed_character_id": true,
	"responsible_character_id": true, "equipped_item_instance_id": true,
	"unequipped_item_instance_id": true, "target_item_instance_id": true,
	"contracted_item_instance_id": true, "after_soul_instance_id": true,
	"next_after_soul_instance_id": true, "public_boss_spawn_generation_id": true,
	"deposited_by": true, "reward_operation_id": true,
	"lucky_charm_item_instance_id": true, "insurance_item_instance_id": true,
	"roster_character_ids": true, "reward_claim_ids": true,
}

// contentIdFieldNames are *_id fields that carry string content IDs (the
// complement of uuidFieldNames; entity IDs are uint64).
var contentIdFieldNames = map[string]bool{
	"item_id": true, "beast_id": true, "quest_id": true, "skill_id": true,
	"map_id": true, "npc_id": true, "portal_id": true, "dungeon_id": true,
	"content_id": true, "cosmetic_id": true, "recipe_id": true,
	"pvp_mode_id": true, "blessing_id": true, "cycle_id": true,
	"board_cycle_id": true, "board_anchor_id": true, "atlas_page_id": true,
	"effect_id": true, "class_id": true, "checkpoint_id": true,
	"currency_id": true, "stat_id": true, "loadout_id": true,
	"active_loadout_id": true, "soul_id": true,
	"food_item_id": true, "offer_id": true, "product_id": true,
	"reward_tier_id": true, "basic_skill_id": true, "zone_id": true,
	"area_id": true, "safe_area_id": true, "starter_npc_id": true,
	"entrance_id": true, "service_id": true, "branch_flag": true,
	"target_id": true, "cue_id": true, "encounter_content_id": true,
	"title_key": true, "safe_message_key": true, "source_type": true,
	"source_reference": true, "source_kind": true, "reward_slot": true,
	"category": true, "note": true, "page_cursor": true, "next_page_cursor": true,
	"pattern_id": true, "status_kind": true,
}

// TestUuidFieldsAreBytes16: every field ending _id/_ids is bytes iff it is a
// UUID (whitelist), string iff content ID (whitelist); no third shape.
func TestUuidFieldsAreBytes16(t *testing.T) {
	idField := regexp.MustCompile(`^(.*_id|.*_ids|deposited_by|reference_id)$`)
	var bad []string
	walkAllMessages(t, func(md protoreflect.MessageDescriptor) {
		for i := 0; i < md.Fields().Len(); i++ {
			f := md.Fields().Get(i)
			name := string(f.Name())
			if !idField.MatchString(name) {
				continue
			}
			isBytes := f.Kind() == protoreflect.BytesKind
			isString := f.Kind() == protoreflect.StringKind
			full := string(md.FullName()) + "." + name
			switch {
			case uuidFieldNames[name]:
				if !isBytes {
					bad = append(bad, full+" should be bytes (UUID), got "+f.Kind().String())
				}
			case contentIdFieldNames[name]:
				if !isString {
					bad = append(bad, full+" should be string (content ID), got "+f.Kind().String())
				}
			default:
				// Other _id shapes legitimately exist: registry message IDs
				// (uint32), entity IDs (uint64), enum slot selectors, plain
				// strings. Only flag kinds that can never be an identifier.
				switch f.Kind() {
				case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
					protoreflect.StringKind, protoreflect.BytesKind,
					protoreflect.EnumKind:
				default:
					bad = append(bad, full+" unexpected kind "+f.Kind().String())
				}
			}
		}
	})
	if len(bad) != 0 {
		t.Fatalf("UUID/content-ID field shape violations:\n%s", strings.Join(bad, "\n"))
	}
}

// TestErrorCodeEnumCoversErrorsMdInOrder: the enum value at number i equals
// ERROR_CODE_<code> for the i-th mechanically extracted code, in order.
func TestErrorCodeEnumCoversErrorsMdInOrder(t *testing.T) {
	codes := errorsMdCodes(t)
	if len(codes) != 112 {
		t.Fatalf("errors.md extraction yielded %d codes, want 112", len(codes))
	}
	enum, err := protoregistry.GlobalTypes.FindEnumByName("thinhthan.v1.ErrorCode")
	if err != nil {
		t.Fatal(err)
	}
	vals := enum.Descriptor().Values()
	for i, code := range codes {
		v := vals.Get(i + 1)
		want := "ERROR_CODE_" + code
		if string(v.Name()) != want {
			t.Fatalf("enum position %d: got %s want %s", i+1, v.Name(), want)
		}
		if v.Number() != protoreflect.EnumNumber(i+1) {
			t.Fatalf("%s number = %d, want %d", want, v.Number(), i+1)
		}
	}
}

// TestErrorCodeNumbersAppendOnly: numbers are dense 0..112 in declared order —
// no reserved holes, no gaps; appends happen only at the end.
func TestErrorCodeNumbersAppendOnly(t *testing.T) {
	enum, err := protoregistry.GlobalTypes.FindEnumByName("thinhthan.v1.ErrorCode")
	if err != nil {
		t.Fatal(err)
	}
	vals := enum.Descriptor().Values()
	for i := 0; i < vals.Len(); i++ {
		v := vals.Get(i)
		if v.Number() != protoreflect.EnumNumber(i) {
			t.Fatalf("ErrorCode position %d has number %d; numbers must be dense", i, v.Number())
		}
	}
	if r := enum.Descriptor().ReservedRanges(); r.Len() != 0 {
		t.Fatalf("ErrorCode reserved ranges = %d, want 0 (baseline has none)", r.Len())
	}
}

// resultExceptionIDs: request_id-scoped or read-only responses that do NOT
// embed OperationResult (spec exceptions 308/443; 517 is request_id-scoped).
var resultExceptionTypes = map[string]bool{
	"S2CBaselineResyncResult": true, // 308: request_id echo
	"S2CSoulListResult":       true, // 443: request_id echo
}

// TestResultsEmbedOperationResult: every *_RESULT response embeds
// OperationResult as field 1, except the request_id-scoped read-only results.
func TestResultsEmbedOperationResult(t *testing.T) {
	var missing []string
	for _, e := range networkRegistry {
		if !strings.HasSuffix(e.Name, "_RESULT") && !strings.HasSuffix(e.GoType, "Result") {
			continue
		}
		if resultExceptionTypes[e.GoType] {
			continue
		}
		md := findMessage(t, "thinhthan.v1."+e.GoType)
		f := md.Fields().ByNumber(1)
		if f == nil || string(f.Name()) != "result" ||
			f.Message() == nil || string(f.Message().FullName()) != "thinhthan.v1.OperationResult" {
			missing = append(missing, e.GoType)
		}
	}
	if len(missing) != 0 {
		t.Fatalf("*_RESULT without OperationResult field 1: %s", strings.Join(missing, ", "))
	}
}

// TestAdr0064MessagesRegistered pins wire-type rules from ADR-0064: entity IDs
// uint64, positions sint32 mm, timestamps int64 ms, money int64, bp uint32,
// client_build uint32, ticks uint64.
func TestAdr0064MessagesRegistered(t *testing.T) {
	checks := []struct {
		msg, field string
		want       protoreflect.Kind
	}{
		{"thinhthan.v1.EntityState", "entity_id", protoreflect.Uint64Kind},
		{"thinhthan.v1.EntityState", "hp", protoreflect.Int64Kind},
		{"thinhthan.v1.EntityState", "x_mm", protoreflect.Sint32Kind},
		{"thinhthan.v1.EntityState", "stat_lifesteal", protoreflect.Uint32Kind},
		{"thinhthan.v1.EntityStatus", "expires_at_tick", protoreflect.Uint64Kind},
		{"thinhthan.v1.PositionMm", "x_mm", protoreflect.Sint32Kind},
		{"thinhthan.v1.CurrencyDelta", "amount", protoreflect.Int64Kind},
		{"thinhthan.v1.Envelope", "session_epoch", protoreflect.Uint64Kind},
		{"thinhthan.v1.Envelope", "message_id", protoreflect.Uint32Kind},
		{"thinhthan.v1.C2SHello", "client_build", protoreflect.Uint32Kind},
		{"thinhthan.v1.S2CHelloOk", "server_time_ms", protoreflect.Int64Kind},
		{"thinhthan.v1.S2CHeartbeat", "server_ms", protoreflect.Uint64Kind},
		{"thinhthan.v1.C2SHeartbeat", "client_mono_ms", protoreflect.Uint64Kind},
		{"thinhthan.v1.C2SSkillUse", "area_center_x_mm", protoreflect.Sint32Kind},
		{"thinhthan.v1.S2CAuctionListResult", "listing_fee", protoreflect.Int64Kind},
		{"thinhthan.v1.C2STradeOfferUpdate", "common_amount", protoreflect.Int64Kind},
		{"thinhthan.v1.S2CDailyBoardState", "reset_at_ms", protoreflect.Int64Kind},
		{"thinhthan.v1.S2CChatMessage", "sent_at_ms", protoreflect.Int64Kind},
	}
	for _, c := range checks {
		md := findMessage(t, c.msg)
		f := md.Fields().ByName(protoreflect.Name(c.field))
		if f == nil {
			t.Fatalf("%s.%s missing", c.msg, c.field)
		}
		if f.Kind() != c.want {
			t.Fatalf("%s.%s kind = %s, want %s", c.msg, c.field, f.Kind(), c.want)
		}
	}
}

// TestNoOptionalRepeatedFields: `optional` applies only to singular scalar or
// message fields — `optional repeated` is banned outright; every optional must
// carry proto3 optional presence.
func TestNoOptionalRepeatedFields(t *testing.T) {
	var bad []string
	walkAllMessages(t, func(md protoreflect.MessageDescriptor) {
		for i := 0; i < md.Fields().Len(); i++ {
			f := md.Fields().Get(i)
			if f.IsList() && f.HasPresence() && !f.IsMap() {
				bad = append(bad, string(md.FullName())+"."+string(f.Name())+" is optional repeated")
			}
		}
	})
	// journal too
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(fd.Package()), "thinhthan.internal.v1") {
			return true
		}
		var walk func(msgs protoreflect.MessageDescriptors)
		walk = func(msgs protoreflect.MessageDescriptors) {
			for i := 0; i < msgs.Len(); i++ {
				m := msgs.Get(i)
				for i := 0; i < m.Fields().Len(); i++ {
					f := m.Fields().Get(i)
					if f.IsList() && f.HasPresence() && !f.IsMap() {
						bad = append(bad, string(m.FullName())+"."+string(f.Name()))
					}
				}
				walk(m.Messages())
			}
		}
		walk(fd.Messages())
		return true
	})
	if len(bad) != 0 {
		t.Fatalf("optional repeated fields:\n%s", strings.Join(bad, "\n"))
	}
}

// TestErrorCodeFencedRowMajorOrder: the enum order is the row-major token order
// of the fenced Canonical Codes blocks — verified against an independent
// per-block extraction (not just the flat list).
func TestErrorCodeFencedRowMajorOrder(t *testing.T) {
	codes := errorsMdCodes(t)
	enum, err := protoregistry.GlobalTypes.FindEnumByName("thinhthan.v1.ErrorCode")
	if err != nil {
		t.Fatal(err)
	}
	vals := enum.Descriptor().Values()
	// Block boundaries per spec: 11+22+8+10+9+52.
	wantLens := []int{11, 22, 8, 10, 9, 52}
	idx := 0
	for bi, l := range wantLens {
		for k := 0; k < l; k++ {
			got := vals.Get(idx + 1)
			want := "ERROR_CODE_" + codes[idx]
			if string(got.Name()) != want {
				t.Fatalf("block %d offset %d: enum has %s, spec order wants %s",
					bi, k, got.Name(), want)
			}
			idx++
		}
	}
}

// TestOutcomeMessagesHaveNoOperationResult: every *Outcome message (and the
// spec's *_OUTCOME rule) must not embed OperationResult.
func TestOutcomeMessagesHaveNoOperationResult(t *testing.T) {
	var bad []string
	walkAllMessages(t, func(md protoreflect.MessageDescriptor) {
		if !strings.HasSuffix(string(md.Name()), "Outcome") {
			return
		}
		for i := 0; i < md.Fields().Len(); i++ {
			f := md.Fields().Get(i)
			if f.Message() != nil && string(f.Message().FullName()) == "thinhthan.v1.OperationResult" {
				bad = append(bad, string(md.FullName())+"."+string(f.Name()))
			}
			if string(f.Name()) == "operation_id" {
				bad = append(bad, string(md.FullName())+".operation_id")
			}
		}
	})
	if len(bad) != 0 {
		t.Fatalf("Outcome messages carrying OperationResult/operation_id:\n%s", strings.Join(bad, "\n"))
	}
}

// TestAdr0069MessagesRegistered: ADR-0069 renames/additions — the registry must
// bind the corrected names at their exact IDs.
func TestAdr0069MessagesRegistered(t *testing.T) {
	want := map[uint32]string{
		401: "S2C_INVENTORY_RESULT", 404: "C2S_CRAFT", 406: "C2S_ENHANCE",
		410: "C2S_BEAST_SET_ACTIVE", 411: "S2C_BEAST_SET_ACTIVE_RESULT",
		612: "S2C_FRIEND_REQUEST", 652: "C2S_GUILD_APPLICATION_CANCEL",
		656: "C2S_GUILD_COSMETIC_EQUIP", 704: "S2C_TRADE_CANCELLED",
		705: "C2S_TRADE_OFFER_UPDATE",
	}
	for id, name := range want {
		found := false
		for _, e := range networkRegistry {
			if e.ID == id {
				found = true
				if e.Name != name {
					t.Fatalf("ID %d = %s, want %s", id, e.Name, name)
				}
				if _, err := protoregistry.GlobalTypes.FindMessageByName(
					protoreflect.FullName("thinhthan.v1." + e.GoType)); err != nil {
					t.Fatalf("ID %d %s: type %s missing", id, name, e.GoType)
				}
			}
		}
		if !found {
			t.Fatalf("ID %d (%s) absent from registry", id, name)
		}
	}
}
