package listener

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestRegistryParityMessagesMd proves the compiled registry table is
// byte-for-byte the canonical message-ID set of docs/05_network/messages.md
// — the spec's registry tables are the single source of truth for wire
// ids (handoff §5.5 item 7).
func TestRegistryParityMessagesMd(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	md, err := os.Open(filepath.Join(root, "docs", "05_network", "messages.md"))
	if err != nil {
		t.Fatalf("messages.md: %v", err)
	}
	defer md.Close()

	// Registry rows live in ```text blocks headed by an "ID Name" header;
	// each row is "<id> <NAME>".
	row := regexp.MustCompile(`^\s*(\d+)\s+((?:C2S|S2C)_[A-Z_0-9]+)\b`)
	inBlock := false
	want := map[uint32]string{}
	sc := bufio.NewScanner(md)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "```") {
			inBlock = !inBlock
			continue
		}
		if !inBlock {
			continue
		}
		m := row.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			continue
		}
		want[uint32(id)] = m[2]
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(want) != 217 {
		t.Fatalf("messages.md yields %d registry rows, want 217", len(want))
	}

	got := map[uint32]registryRow{}
	for _, r := range registryTable {
		if _, dup := got[r.id]; dup {
			t.Fatalf("duplicate id %d in compiled registry", r.id)
		}
		got[r.id] = r
	}
	if len(got) != len(want) {
		t.Fatalf("compiled registry has %d entries, messages.md has %d", len(got), len(want))
	}
	for id, name := range want {
		r, ok := got[id]
		if !ok {
			t.Fatalf("id %d (%s) missing from compiled registry", id, name)
		}
		if r.name != name {
			t.Fatalf("id %d: compiled name %q, messages.md %q", id, r.name, name)
		}
		c2s := strings.HasPrefix(name, "C2S_")
		if r.c2s != c2s {
			t.Fatalf("id %d (%s): direction flag %v, want %v", id, name, r.c2s, c2s)
		}
		// Every registered id resolves in lookup().
		if lookup(id) == nil {
			t.Fatalf("id %d (%s) not resolvable via lookup()", id, name)
		}
	}
}
