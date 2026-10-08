package discovery

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"thinhthan/internal/core/id"
	"thinhthan/internal/sim/spatial/parity"
)

// realPayload compiles the canonical content payload once — the same
// pipeline world tests run.
var realPayload *parity.Payload

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
	tmp, err := os.MkdirTemp("", "imp020")
	if err != nil {
		panic(err)
	}
	pay := filepath.Join(tmp, "payload.json")
	run := exec.Command("go", "run", "./cmd/compiler", "-payload", pay)
	run.Dir = filepath.Join(root, "server")
	if out, err := run.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("compiler payload: %v\n%s", err, out))
	}
	data, err := os.ReadFile(pay)
	if err != nil {
		panic(err)
	}
	p, err := parity.LoadPayload(data)
	if err != nil {
		panic(err)
	}
	realPayload = p
	os.Exit(m.Run())
}

func canonicalPayload(t *testing.T) *parity.Payload {
	t.Helper()
	return realPayload
}

func TestTwentyFourFirstDiscoveryEvents(t *testing.T) {
	cat, err := CatalogFromPayload(canonicalPayload(t))
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	maps := cat.Maps()
	if len(maps) != 24 {
		t.Fatalf("world_map slots = %d, want 24", len(maps))
	}
	wantTowns := []string{
		"map.lang_da.dinh_lang",
		"map.rung_u_minh.xom_rung",
		"map.ben_nuoc_den.cho_ben",
		"map.deo_may.ban_chan_deo",
		"map.thanh_co.cong_ngoai",
		"map.nui_thieng.chan_nui",
	}
	wantExp := map[string]uint64{
		"map.lang_da.dinh_lang":    7700,
		"map.rung_u_minh.xom_rung": 49700,
		"map.ben_nuoc_den.cho_ben": 131700,
		"map.deo_may.ban_chan_deo": 253700,
		"map.thanh_co.cong_ngoai":  415700,
		"map.nui_thieng.chan_nui":  545700,
	}
	towns := 0
	fields := 0
	seen := map[string]bool{}
	for _, m := range maps {
		if m.DiscoveryExp == 0 {
			t.Fatalf("%s: zero discovery EXP", m.MapID)
		}
		if m.EntrySpawn == "" {
			t.Fatalf("%s: missing entry spawn", m.MapID)
		}
		if seen[RewardKey(m.MapID, id.NewV4())] {
			t.Fatalf("%s: duplicate reward key", m.MapID)
		}
		seen[m.MapID] = true
		switch m.Type {
		case MapTown:
			towns++
			if want := wantExp[m.MapID]; want != m.DiscoveryExp {
				t.Fatalf("%s: exp %d, want %d", m.MapID, m.DiscoveryExp, want)
			}
		case MapField:
			fields++
		default:
			t.Fatalf("%s: unknown type %q", m.MapID, m.Type)
		}
	}
	if towns != 6 || fields != 18 {
		t.Fatalf("anchors/fields = %d/%d, want 6/18", towns, fields)
	}
	for _, tid := range wantTowns {
		if _, ok := cat.SafeAnchor(tid); !ok {
			t.Fatalf("%s: not travel-eligible", tid)
		}
	}
	if _, ok := cat.SafeAnchor("map.lang_da.bo_ruong"); ok {
		t.Fatal("FIELD map must not be travel-eligible")
	}
}

func TestProgressionSourceOperationIDs(t *testing.T) {
	char := id.NewV4()
	k := RewardKey("map.lang_da.dinh_lang", char)
	want := "reward.discovery.map.lang_da.dinh_lang." + char.String()
	if k != want {
		t.Fatalf("reward key %q, want %q", k, want)
	}
	// The grant identity is deterministic: same map + character yields
	// the same once-only key across reconnects and replays.
	if RewardKey("map.lang_da.dinh_lang", char) != k {
		t.Fatal("reward key not deterministic")
	}
	other := RewardKey("map.lang_da.bo_ruong", char)
	if other == k {
		t.Fatal("distinct maps share a reward key")
	}
}

func TestIdempotentDiscoveryGrants(t *testing.T) {
	d := NewDetector()
	char := id.NewV4()
	if !d.Observe(char, "map.lang_da.dinh_lang") {
		t.Fatal("first entry must report a first observation")
	}
	// Reconnect restoration and repeat entries never re-emit.
	for i := 0; i < 3; i++ {
		if d.Observe(char, "map.lang_da.dinh_lang") {
			t.Fatalf("entry %d re-emitted", i)
		}
	}
	if !d.Observe(char, "map.lang_da.bo_ruong") {
		t.Fatal("a different map's first entry must emit")
	}
	if !d.Observe(id.NewV4(), "map.lang_da.dinh_lang") {
		t.Fatal("a different character's first entry must emit")
	}
}
