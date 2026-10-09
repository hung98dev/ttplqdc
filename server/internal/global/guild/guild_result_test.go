package guild

import (
	"testing"
)

// TestGuildResultCoversEveryRequest: the wire contract requires exactly
// one S2C_GUILD_RESULT(649) per guild C2S — the closed request set in
// registry.go must cover every landed guild client id (storage ids are
// IMP-037's, the cosmetic id is IMP-038's, both still 649-answered).
func TestGuildResultCoversEveryRequest(t *testing.T) {
	want := []uint32{
		608, 610, 623, 624, 625, 626, 627,
		629, 630, // IMP-037 storage (registry still owns 649 coverage)
		637, 638, 639, 640,
		642, 643, 644, 645, 646, // IMP-037 storage
		648,
		650, 651, 652,
		656, // IMP-038 cosmetics
	}
	for _, id := range want {
		if !Covers(id) {
			t.Fatalf("request %d missing from 649 coverage", id)
		}
	}
	if len(GuildResultRequests) != len(want) {
		t.Fatalf("coverage set = %d, want %d", len(GuildResultRequests), len(want))
	}
	if Covers(649) || Covers(628) || Covers(601) {
		t.Fatalf("S2C/non-guild ids must not be covered")
	}
}
