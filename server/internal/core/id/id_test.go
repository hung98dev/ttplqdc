package id

import (
	"errors"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"
)

func TestJournalReplayRetainsSourceIdentity(t *testing.T) {
	inc := NewIncarnation()
	instanceID := NewV4()
	const tick = uint64(4242)

	name, opID, err := inc.NextSourceEvent("map.rung_u_minh", 7, instanceID, tick)
	if err != nil {
		t.Fatalf("NextSourceEvent: %v", err)
	}
	wantName := "sim:map.rung_u_minh:0:" + instanceID.String() + ":" + inc.ID.String() + ":1:4242"
	if name != wantName {
		t.Fatalf("source event name = %q, want %q", name, wantName)
	}

	// JRN-004: journal replay and retries recompute from the retained
	// incarnation/counter/tick and reproduce the identical operation.
	replayName := SourceEventName("map.rung_u_minh", 7, instanceID, inc.ID, 1, tick)
	replayOp := SourceOperationID("map.rung_u_minh", 7, instanceID, inc.ID, 1, tick)
	if replayName != name || replayOp != opID {
		t.Fatalf("replay identity diverged: %q/%v vs %q/%v", replayName, replayOp, name, opID)
	}

	nextName, nextOp, err := inc.NextSourceEvent("map.rung_u_minh", 7, instanceID, tick)
	if err != nil {
		t.Fatalf("NextSourceEvent second: %v", err)
	}
	if nextOp == opID || nextName == name {
		t.Fatal("counter advanced but operation identity did not change")
	}
	if !strings.Contains(nextName, ":"+inc.ID.String()+":2:4242") {
		t.Fatalf("second event lacks counter 2: %q", nextName)
	}
}

func TestWorldOwnerUsesReservedConstant(t *testing.T) {
	if WorldOwnerID.IsNil() {
		t.Fatal("WorldOwnerID must not be the nil UUID")
	}
	if got := WorldOwnerID.String(); got != "00000000-0000-0000-0000-000000000002" {
		t.Fatalf("WorldOwnerID = %q, want 00000000-0000-0000-0000-000000000002", got)
	}
	if _, err := ParseUUID(WorldOwnerID.String()); err != nil {
		t.Fatalf("WorldOwnerID must parse as a valid UUID: %v", err)
	}
}

func TestUUIDv4EntityIdentity(t *testing.T) {
	a := NewV4()
	b := NewV4()
	if a == b {
		t.Fatal("two UUIDv4 draws collided")
	}
	if a.IsNil() {
		t.Fatal("NewV4 returned the nil UUID")
	}
	if a.Version() != 4 {
		t.Fatalf("version = %d, want 4", a.Version())
	}
	if !a.HasRFC4122Variant() {
		t.Fatal("RFC 4122 variant bits not set")
	}
	parsed, err := ParseUUID(a.String())
	if err != nil {
		t.Fatalf("ParseUUID round trip: %v", err)
	}
	if parsed != a {
		t.Fatalf("round trip mismatch: %v vs %v", parsed, a)
	}
	if a.Bytes() != [16]byte(a) {
		t.Fatal("Bytes() must return the network-order binary form")
	}
}

func TestUUIDv7OperationTimestampAndCryptoRandomness(t *testing.T) {
	now := time.UnixMilli(1759300000000).UTC()
	a := NewV7(now)
	b := NewV7(now)
	if a == b {
		t.Fatal("two UUIDv7 draws in the same millisecond collided; the 74 random bits must distinguish intents")
	}
	if a.Version() != 7 || !a.HasRFC4122Variant() {
		t.Fatalf("not an RFC 9562 UUIDv7: version %d", a.Version())
	}
	ms, ok := OperationIssuedAt(a)
	if !ok || ms != now.UnixMilli() {
		t.Fatalf("OperationIssuedAt = %d,%v; want %d", ms, ok, now.UnixMilli())
	}
	if err := ValidateOperationID(a, now); err != nil {
		t.Fatalf("fresh operation ID rejected: %v", err)
	}

	// issued_at == now+60s is the admitted skew edge; now+60s+1ms is malformed.
	if err := ValidateOperationID(NewV7(now.Add(OperationFutureSkew)), now); err != nil {
		t.Fatalf("60s skew boundary rejected: %v", err)
	}
	err := ValidateOperationID(NewV7(now.Add(OperationFutureSkew+time.Millisecond)), now)
	var opErr *OperationIDError
	if !errors.As(err, &opErr) || opErr.Kind != OperationIDFutureSkew {
		t.Fatalf("future skew want OperationIDFutureSkew, got %v", err)
	}

	// now >= issued_at+180d expires; one millisecond earlier is still live.
	if err := ValidateOperationID(a, now.Add(OperationReplayHorizon).Add(-time.Millisecond)); err != nil {
		t.Fatalf("ID one millisecond inside horizon rejected: %v", err)
	}
	err = ValidateOperationID(a, now.Add(OperationReplayHorizon))
	if !errors.As(err, &opErr) || opErr.Kind != OperationIDExpired {
		t.Fatalf("expiry want OperationIDExpired, got %v", err)
	}

	if err := ValidateOperationID(UUID{}, now); !errors.As(err, &opErr) || opErr.Kind != OperationIDMalformed {
		t.Fatalf("nil ID want OperationIDMalformed, got %v", err)
	}
	if err := ValidateOperationID(NewV4(), now); !errors.As(err, &opErr) || opErr.Kind != OperationIDMalformed {
		t.Fatalf("non-v7 ID want OperationIDMalformed, got %v", err)
	}
}

func TestParseUUIDRejections(t *testing.T) {
	canonical := "f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2b"
	u, err := ParseUUID(canonical)
	if err != nil {
		t.Fatalf("canonical UUID rejected: %v", err)
	}
	if u.String() != canonical {
		t.Fatalf("round trip = %q, want %q", u.String(), canonical)
	}

	rejects := []string{
		"",
		"00000000-0000-0000-0000-000000000000",   // nil UUID
		"f7a3d2b14e8c4a2f9b3e6d1c5f8e7a2b",       // no dashes
		"f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2",    // short
		"f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2bc",  // long
		"F7A3D2B1-4E8C-4A2F-9B3E-6D1C5F8E7A2B",   // uppercase non-canonical
		"f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2g",   // non-hex
		"f7a3d2b1_4e8c_4a2f_9b3e_6d1c5f8e7a2b",   // wrong separators
		"Kiem Quang",                             // display name is not an ID
		"  f7a3d2b1-4e8c-4a2f-9b3e-6d1c5f8e7a2b", // whitespace
	}
	for _, s := range rejects {
		if _, err := ParseUUID(s); err == nil {
			t.Fatalf("ParseUUID(%q) accepted", s)
		}
	}
}

func TestUUIDv5DeterministicContentGrant(t *testing.T) {
	scope := "season.3.cosmetic.kimono_bac.9d8f7e6c-5b4a-4932-a1b0-c9d8e7f6a5b4"
	a := V5(ContentGrantNamespaceUUID, scope)
	b := V5(ContentGrantNamespaceUUID, scope)
	if a != b {
		t.Fatal("content-grant derivation is not deterministic")
	}
	if a.IsNil() || a.Version() != 5 || !a.HasRFC4122Variant() {
		t.Fatal("grant ID is not a well-formed RFC 4122 v5 UUID")
	}
	if c := V5(ContentGrantNamespaceUUID, "season.3.cosmetic.kimono_bac.other"); c == a {
		t.Fatal("different grant scope produced the same idempotency key")
	}
	if c := V5(ServerJobNamespaceUUID, scope); c == a {
		t.Fatal("namespace not bound in derivation")
	}
}

func TestValidateStaticContentID(t *testing.T) {
	valid := []string{
		"skill.kim.kiem_quang",
		"item.u_minh.chien_than",
		"monster.rung_u_minh.boss_1",
		"zone.rung_u_minh.a",
		"a.b",
		"0.1",
		strings.Repeat("a", 127) + ".b", // 129 bytes is too long; 128 ok below
	}
	// 128 bytes exactly is the wire bound and must pass.
	edge := "a." + strings.Repeat("b", 126)
	if err := ValidateStaticContentID(edge); err != nil {
		t.Fatalf("128-byte ID rejected: %v", err)
	}
	for _, s := range valid[:6] {
		if err := ValidateStaticContentID(s); err != nil {
			t.Fatalf("valid ID %q rejected: %v", s, err)
		}
	}
	if err := ValidateStaticContentID(valid[6]); err == nil {
		t.Fatal("129-byte ID accepted")
	}

	rejects := []string{
		"",
		"abc", // single segment
		".a.b", "a.b.", "a..b",
		"Skill.kim", // uppercase
		"a.B",
		"a_b.c",   // underscore in first segment
		"sim:x.y", // ':' forbidden in components
		"a.b:c",
		"a.b c", // space
		"a.á",   // non-ASCII
	}
	for _, s := range rejects {
		if err := ValidateStaticContentID(s); err == nil {
			t.Fatalf("ValidateStaticContentID(%q) accepted", s)
		}
	}
}

func TestRuntimeEntityID(t *testing.T) {
	scope := RuntimeEntityID{Owner: 5, Epoch: 9}
	if scope != (RuntimeEntityID{Owner: 5, Epoch: 9}) {
		t.Fatal("same owner/epoch scope must compare equal")
	}
	if scope == (RuntimeEntityID{Owner: 6, Epoch: 9}) || scope == (RuntimeEntityID{Owner: 5, Epoch: 10}) {
		t.Fatal("entity scope must differ across owner or epoch")
	}
}

func TestContentRevisionDiagnosticContext(t *testing.T) {
	rev := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	if err := ValidateContentRevision(rev); err != nil {
		t.Fatalf("canonical revision rejected: %v", err)
	}
	rejects := []string{
		"",
		rev[:63],
		rev + "0",
		strings.ToUpper(rev),
		"sha256:" + rev[:56],
		strings.Repeat("g", 64),
	}
	for _, s := range rejects {
		if err := ValidateContentRevision(s); err == nil {
			t.Fatalf("revision %q accepted", s)
		}
	}

	attr := RevisionLogAttr(rev)
	if attr.Key != "revision" || attr.Value.String() != rev {
		t.Fatalf("RevisionLogAttr = %v", attr)
	}
	if got := slog.Any("x", 1).Key; got != "x" {
		t.Fatalf("slog sanity: %q", got)
	}
}

func TestOperationPayloadConsistency(t *testing.T) {
	owner := NewV4()
	op := NewV7(time.Now().UTC())
	request := []byte("typed-request-bytes")

	fp := OperationFingerprint("inventory.move", owner, op, request)
	if fp != OperationFingerprint("inventory.move", owner, op, request) {
		t.Fatal("identical retry must reproduce the committed fingerprint")
	}
	if fp == OperationFingerprint("inventory.move", owner, op, []byte("conflicting-payload")) {
		t.Fatal("conflicting payload under one operation ID must change the fingerprint")
	}
	if fp == OperationFingerprint("inventory.destroy", owner, op, request) ||
		fp == OperationFingerprint("inventory.move", NewV4(), op, request) ||
		fp == OperationFingerprint("inventory.move", owner, NewV7(time.Now().UTC()), request) {
		t.Fatal("fingerprint must bind family, owner and operation UUID")
	}
}

func TestIncarnationRecreationSameCounterTickDifferentOperation(t *testing.T) {
	a := NewIncarnation()
	b := NewIncarnation()
	if a.ID == b.ID {
		t.Fatal("two incarnations minted the same crypto-v4 UUID")
	}
	if a.ID.Version() != 4 {
		t.Fatal("incarnation ID is not crypto-v4")
	}
	const tick = uint64(99)
	opA := SourceOperationID("map.saigon", 3, UUID{}, a.ID, 1, tick)
	opB := SourceOperationID("map.saigon", 3, UUID{}, b.ID, 1, tick)
	if opA == opB {
		t.Fatal("identical counter/tick across incarnations must yield different operations")
	}
	if SourceOperationID("map.saigon", 3, UUID{}, a.ID, 1, tick) != opA {
		t.Fatal("replay of the same incarnation/counter/tick must retain the operation")
	}

	// The counter starts at 1, advances monotonically and never wraps.
	exhausted := Incarnation{ID: NewV4(), counter: math.MaxUint64}
	if _, _, err := exhausted.NextSourceEvent("map.saigon", 3, UUID{}, tick); err != nil {
		t.Fatalf("final counter slot must still emit: %v", err)
	}
	if _, _, err := exhausted.NextSourceEvent("map.saigon", 3, UUID{}, tick); !errors.Is(err, ErrSourceEventCounterExhausted) {
		t.Fatalf("overflow must stop the partition, got %v", err)
	}
}
