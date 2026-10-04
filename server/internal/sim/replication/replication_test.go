package replication

import (
	"testing"

	protocolv1 "thinhthan/internal/protocol/v1"

	"google.golang.org/protobuf/proto"
)

// TestSnapshotDeltaSequence checks the lifecycle ordering the client sees:
// entrants carry full state via spawn, removals via despawn, and deltas
// only touch still-visible entities with changed fields.
func TestSnapshotDeltaSequence(t *testing.T) {
	b := NewBuilder()
	e1 := richSnapshot()
	e1.ID = 10
	e2 := richSnapshot()
	e2.ID = 20
	e1c := e1
	e1c.X += 1000
	e3 := richSnapshot()
	e3.ID = 30

	prev := &View{Tick: 1, BaselineID: 9, Entities: []EntitySnapshot{e1, e2}}
	cur := &View{Tick: 2, BaselineID: 9, Entities: []EntitySnapshot{e1c, e2, e3}}
	d := b.NewDelta(prev, cur)

	// e1 changed: one delta with the new field. e2 unchanged: absent.
	// e3 is new to the client: spawn carries it, no delta entry.
	if len(d.Entities) != 1 || d.Entities[0].EntityId != 10 {
		t.Fatalf("delta entities wrong: %+v", d.Entities)
	}
	if d.Entities[0].XMm == nil || *d.Entities[0].XMm != e1c.X {
		t.Fatalf("delta lost x change: %+v", d.Entities[0])
	}

	spawn, ok := b.NewSpawn(9, 2, &e3)
	if !ok {
		t.Fatal("spawn arena exhausted")
	}
	if spawn.BaselineId != 9 || spawn.ServerTick != 2 ||
		spawn.Entity == nil || spawn.Entity.EntityId != 30 ||
		spawn.Entity.DisplayName != e3.DisplayName {
		t.Fatalf("spawn wrong: %+v", spawn)
	}

	despawn, ok := b.NewDespawn(9, 2, 20, protocolv1.DespawnReason_DESPAWN_REASON_LEFT_AOI)
	if !ok {
		t.Fatal("despawn arena exhausted")
	}
	if despawn.BaselineId != 9 || despawn.ServerTick != 2 ||
		despawn.EntityId != 20 ||
		despawn.Reason != protocolv1.DespawnReason_DESPAWN_REASON_LEFT_AOI {
		t.Fatalf("despawn wrong: %+v", despawn)
	}
}

// TestDeliveryClassRouting pins the delivery-class table and supersede-key
// rule from messages.md for every replication message.
func TestDeliveryClassRouting(t *testing.T) {
	b := NewBuilder()
	snap := richSnapshot()
	spawn, _ := b.NewSpawn(9, 1, &snap)
	despawn, _ := b.NewDespawn(9, 1, 20, protocolv1.DespawnReason_DESPAWN_REASON_DIED)
	delta := b.NewDelta(&View{BaselineID: 42}, &View{BaselineID: 42})
	resync := b.NewResyncResult(7, protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED, 42, 0)

	cases := []struct {
		name   string
		msg    proto.Message
		id     uint32
		class  DeliveryClass
		hasKey bool
		key    uint64
	}{
		{"baseline", b.NewBaseline(&View{}, &BaselineParams{BaselineID: 42}), 300, DeliveryReplaceableState, false, 0},
		{"spawn", spawn, 301, DeliveryAuthoritativeEvent, false, 0},
		{"despawn", despawn, 302, DeliveryAuthoritativeEvent, false, 0},
		{"delta", delta, 303, DeliveryReplaceableState, true, 42},
		{"ack", &protocolv1.C2SBaselineAck{}, 306, DeliveryControl, false, 0},
		{"resync_req", &protocolv1.C2SBaselineResyncRequest{}, 307, DeliveryDiscreteIntent, false, 0},
		{"resync_result", resync, 308, DeliveryDurableResult, false, 0},
	}
	for _, tc := range cases {
		id, ok := MessageID(tc.msg)
		if !ok || id != tc.id {
			t.Fatalf("%s: MessageID = %d,%v, want %d", tc.name, id, ok, tc.id)
		}
		if c := ClassOf(tc.msg); c != tc.class {
			t.Fatalf("%s: ClassOf = %v, want %v", tc.name, c, tc.class)
		}
		mid, key, hasKey := SupersedeKey(tc.msg)
		if hasKey != tc.hasKey {
			t.Fatalf("%s: supersede hasKey = %v, want %v", tc.name, hasKey, tc.hasKey)
		}
		if hasKey && (mid != tc.id || key != tc.key) {
			t.Fatalf("%s: supersede key = (%d,%d), want (%d,%d)", tc.name, mid, key, tc.id, tc.key)
		}
	}
	// Messages outside the replication family report no id.
	if _, ok := MessageID(&protocolv1.EntityState{}); ok {
		t.Fatal("MessageID accepted a non-replication message")
	}
}
