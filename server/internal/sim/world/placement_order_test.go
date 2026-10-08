package world

import (
	"testing"

	"thinhthan/internal/core/id"
)

// orderTestDirector returns a director over the two-map fake world with
// the caller's channel occupancies applied to map.test.alpha.
func orderTestDirector(occ []int) *Director {
	clk := newTestClocks()
	d := NewDirector(newFakeCatalog(testMaps()), clk.Now)
	for i, n := range occ {
		ch := uint32(i + 1)
		c, _ := d.Channel("map.test.alpha", ch)
		c.State = ChannelRunning
		c.Occupancy = n
	}
	return d
}

// runningIndex returns the channel index a select resolved to.
func selCh(t *testing.T, d *Director, cid id.UUID, kind PlacementKind) PlacementResult {
	t.Helper()
	res, pw, err := d.Select(PlacementRequest{
		CharacterID: cid, MapID: "map.test.alpha", Kind: kind,
		ForcedReason: ForcedReattach,
	})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if pw != nil {
		t.Fatalf("Select: unexpected pending")
	}
	return res
}

func TestPlayerInitiatedEntryStillCapsAt18(t *testing.T) {
	// Auto placement packs into running channels only below SoftCap
	// (world_rules.md: 18 normal); a channel at 18 must not accept
	// player-initiated entry even though forced cap is 22.
	d := orderTestDirector([]int{18})
	res := selCh(t, d, charID(1), PlaceAuto)
	if res.ChannelIndex == 1 {
		t.Fatal("auto entry admitted into an 18-occupancy channel")
	}
	if res.ChannelIndex != 2 {
		t.Fatalf("auto entry should start lowest stopped channel, got ch%d",
			res.ChannelIndex)
	}
}

func TestAutoOrderPacksMostPopulated(t *testing.T) {
	// Running channels below the soft cap are tried most-populated first
	// (packing keeps idle channels free to stop); stopped channels come
	// last in ascending index.
	d := orderTestDirector([]int{3, 9, 1})
	res := selCh(t, d, charID(1), PlaceAuto)
	if res.ChannelIndex != 2 {
		t.Fatalf("pack order: want most-populated ch2, got ch%d", res.ChannelIndex)
	}
	res = selCh(t, d, charID(2), PlaceAuto)
	if res.ChannelIndex != 2 {
		t.Fatalf("pack order again: want ch2 (now 10), got ch%d", res.ChannelIndex)
	}
	// Fill ch2 to the soft cap, then ch1 (3) beats ch3 (1).
	for i := 0; i < 8; i++ {
		selCh(t, d, charID(10+i), PlaceAuto)
	}
	res = selCh(t, d, charID(20), PlaceAuto)
	if res.ChannelIndex != 1 {
		t.Fatalf("after ch2 saturated: want ch1 (3 occupants), got ch%d",
			res.ChannelIndex)
	}
}

func TestAutoOrderFallsToStoppedWhenAllRunningFull(t *testing.T) {
	// Every running channel at SoftCap → the lowest stopped channel
	// starts for the entrant.
	d := orderTestDirector([]int{18, 18, 18})
	res := selCh(t, d, charID(1), PlaceAuto)
	if res.ChannelIndex != 4 || !res.Start {
		t.Fatalf("want ch4 starting, got %+v", res)
	}
}

func TestAutoOrderAllSaturatedRejects(t *testing.T) {
	// All 30 channels running at SoftCap — player-initiated entry fails
	// closed with ErrCapacityFull (MAP_CAPACITY_FULL wire verdict).
	d := orderTestDirector(make([]int, 0))
	for i := 1; i <= ChannelsPerMap; i++ {
		c, _ := d.Channel("map.test.alpha", uint32(i))
		c.State = ChannelRunning
		c.Occupancy = SoftCap
	}
	_, _, err := d.Select(PlacementRequest{
		CharacterID: charID(1), MapID: "map.test.alpha", Kind: PlaceAuto,
	})
	if err != ErrCapacityFull {
		t.Fatalf("want ErrCapacityFull, got %v", err)
	}
}

func TestAutoEntryUnknownMapFails(t *testing.T) {
	d := orderTestDirector(nil)
	_, _, err := d.Select(PlacementRequest{
		CharacterID: charID(1), MapID: "map.test.nope", Kind: PlaceAuto,
	})
	if err != ErrNoMap {
		t.Fatalf("want ErrNoMap, got %v", err)
	}
}

func TestForcedOrderPreferredBelowHardCap(t *testing.T) {
	// Forced placement honors the preferred channel while it stays below
	// the 22 hard cap — even above the 18 soft cap.
	d := orderTestDirector(nil)
	c, _ := d.Channel("map.test.alpha", 5)
	c.State = ChannelRunning
	c.Occupancy = 20 // over soft, under hard
	res, pw, err := d.Select(PlacementRequest{
		CharacterID: charID(1), MapID: "map.test.alpha",
		Kind: PlaceForced, Preferred: 5, ForcedReason: ForcedRespawn,
	})
	if err != nil || pw != nil {
		t.Fatalf("forced select: %v %v", err, pw)
	}
	if res.ChannelIndex != 5 {
		t.Fatalf("preferred ch5 below HardCap must win, got ch%d", res.ChannelIndex)
	}
}

func TestForcedOrderLeastPopulatedThenStopped(t *testing.T) {
	// Preferred at hard cap → least-populated running below SoftCap →
	// lowest stopped → running below HardCap.
	d := orderTestDirector([]int{22, 7, 4})
	res, pw, err := d.Select(PlacementRequest{
		CharacterID: charID(1), MapID: "map.test.alpha",
		Kind: PlaceForced, Preferred: 1, ForcedReason: ForcedRespawn,
	})
	if err != nil || pw != nil {
		t.Fatalf("forced: %v %v", err, pw)
	}
	if res.ChannelIndex != 3 {
		t.Fatalf("want least-populated ch3, got ch%d", res.ChannelIndex)
	}
}

func TestForcedOrderStoppedThenHardCapTail(t *testing.T) {
	// Running channels all at/above SoftCap but below HardCap → a
	// stopped channel still wins before the hard-cap tail.
	d := orderTestDirector(nil)
	for i := 1; i <= ChannelsPerMap; i++ {
		c, _ := d.Channel("map.test.alpha", uint32(i))
		c.State = ChannelRunning
		c.Occupancy = 20
	}
	// Free one stopped slot.
	c, _ := d.Channel("map.test.alpha", 9)
	c.State = ChannelStopped
	c.Occupancy = 0
	res, pw, err := d.Select(PlacementRequest{
		CharacterID: charID(1), MapID: "map.test.alpha", Kind: PlaceForced,
		ForcedReason: ForcedRespawn,
	})
	if err != nil || pw != nil {
		t.Fatalf("forced: %v %v", err, pw)
	}
	if res.ChannelIndex != 9 || !res.Start {
		t.Fatalf("want stopped ch9 starting, got %+v", res)
	}
}

func TestForcedOrderUsesHardCapTail(t *testing.T) {
	// Everything above SoftCap, nothing stopped → the running channel
	// below HardCap admits (forced must not reject while capacity exists).
	d := orderTestDirector(nil)
	for i := 1; i <= ChannelsPerMap; i++ {
		c, _ := d.Channel("map.test.alpha", uint32(i))
		c.State = ChannelRunning
		c.Occupancy = 20
	}
	res, pw, err := d.Select(PlacementRequest{
		CharacterID: charID(1), MapID: "map.test.alpha", Kind: PlaceForced,
		ForcedReason: ForcedRespawn,
	})
	if err != nil || pw != nil {
		t.Fatalf("forced: %v %v", err, pw)
	}
	if res.ChannelIndex != 1 {
		t.Fatalf("want hard-cap tail ch1, got ch%d", res.ChannelIndex)
	}
}

func TestForcedOrderQuarantinedSkipped(t *testing.T) {
	// A quarantined preferred channel is never a forced candidate.
	d := orderTestDirector(nil)
	c, _ := d.Channel("map.test.alpha", 2)
	c.State = ChannelRunning
	c.Occupancy = 1
	c.Quarantined = true
	res, _, err := d.Select(PlacementRequest{
		CharacterID: charID(1), MapID: "map.test.alpha",
		Kind: PlaceForced, Preferred: 2, ForcedReason: ForcedRespawn,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ChannelIndex == 2 {
		t.Fatal("quarantined channel selected")
	}
}
