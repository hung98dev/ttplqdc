//go:build !race

package replication

import "testing"

// TestAllocs_DeltaBuild is HOT-002: building a state delta into the pooled
// arenas at full view saturation allocates nothing.
func TestAllocs_DeltaBuild(t *testing.T) {
	b := NewBuilder()
	var prevArr, curArr [MaxEntitiesPerView]EntitySnapshot
	for i := range prevArr {
		s := richSnapshot()
		s.ID = uint64(i + 1)
		prevArr[i] = s
		c := s
		c.X += int32(i)
		c.HP -= int64(i)
		c.Vx += int32(i * 3)
		c.Facing = 0 // enum flip forces a facing delta without a new fixture
		c.StatusN = 1
		c.CosmeticN = 0
		curArr[i] = c
	}
	prev := &View{Tick: 1, BaselineID: 9, Entities: prevArr[:], SelfPrivate: PrivateSnapshot{}}
	cur := &View{
		Tick:       2,
		BaselineID: 9,
		Entities:   curArr[:],
		SelfPrivate: PrivateSnapshot{
			CurrentMP: 1,
			MaxMP:     2,
		},
		LastProcessedClientSeq: 7,
		SelfCheckpoint:         richCheckpoint(),
	}
	got := testing.AllocsPerRun(1000, func() {
		b.Reset()
		b.NewDelta(prev, cur)
	})
	if got != 0 {
		t.Fatalf("HOT-002: delta build allocates %v allocs/op, want 0", got)
	}
}

// BenchmarkDeltaBuild is the goAlloc gate's measurement point — it must
// report 0 allocs/op.
func BenchmarkDeltaBuild(b *testing.B) {
	builder := NewBuilder()
	var prevArr, curArr [MaxEntitiesPerView]EntitySnapshot
	for i := range prevArr {
		s := richSnapshot()
		s.ID = uint64(i + 1)
		prevArr[i] = s
		c := s
		c.X += int32(i)
		c.HP -= int64(i)
		curArr[i] = c
	}
	prev := &View{Tick: 1, BaselineID: 9, Entities: prevArr[:]}
	cur := &View{Tick: 2, BaselineID: 9, Entities: curArr[:], SelfCheckpoint: richCheckpoint()}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.Reset()
		builder.NewDelta(prev, cur)
	}
}
