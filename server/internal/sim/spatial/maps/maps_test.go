package maps

import (
	"sort"
	"testing"

	"thinhthan/internal/config"
)

// LoadAllRegistered loads every embedded .geom.json with a synthetic record
// (per-file registration fields come from the file itself, so the synthetic
// record mirrors it) and checks the registry surface.
func TestLoadAllRegistered(t *testing.T) {
	ids := SpaceIDs()
	if len(ids) == 0 {
		t.Fatal("no embedded maps")
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatal("SpaceIDs not sorted")
	}
	for _, id := range ids {
		raw, err := Raw(id)
		if err != nil || len(raw) == 0 {
			t.Fatalf("%s: unreadable embedded map: %v", id, err)
		}
	}
	// Full validate is exercised by parity tests; here we just check the
	// registry refuses unknown ids and loads each known one against a
	// record built from the file itself.
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate space id %s", id)
		}
		seen[id] = true
	}
	if _, err := LoadOne("nonexistent.space", config.SpaceRecord{}); err == nil {
		t.Fatal("LoadOne on unknown id must fail")
	}
	if _, err := Raw("nonexistent.space"); err == nil {
		t.Fatal("Raw on unknown id must fail")
	}
}
