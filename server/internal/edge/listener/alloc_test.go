//go:build !race

package listener

import (
	"testing"

	"google.golang.org/protobuf/proto"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestAllocs_EnvelopeEncodeFrameWrite is the HOT-003 gate (capacity.md):
// envelope encode + frame write into a pooled buffer = 0 allocs/op.
func TestAllocs_EnvelopeEncodeFrameWrite(t *testing.T) {
	payload, err := proto.Marshal(&protocolv1.S2CHeartbeat{ServerMs: 123})
	if err != nil {
		t.Fatal(err)
	}
	env := &protocolv1.Envelope{
		ProtocolMajor: 1,
		MessageId:     5,
		SessionEpoch:  7,
		ServerSeq:     42,
	}
	dst := make([]byte, 0, 1024)
	if allocs := testing.AllocsPerRun(500, func() {
		dst = EncodeEnvelope(dst[:0], env, payload)
		_ = dst // frame write consumes the pooled buffer
	}); allocs != 0 {
		t.Fatalf("envelope encode + frame write: got %v allocs/op, want 0", allocs)
	}
}

// BenchmarkEnvelopeEncode is the single allowed benchmark under edge/ and
// must stay at 0 allocs/op (goAlloc gate over the whole edge tree).
func BenchmarkEnvelopeEncode(b *testing.B) {
	payload, err := proto.Marshal(&protocolv1.S2CHeartbeat{ServerMs: 123})
	if err != nil {
		b.Fatal(err)
	}
	env := &protocolv1.Envelope{
		ProtocolMajor: 1,
		MessageId:     5,
		SessionEpoch:  7,
		ServerSeq:     42,
	}
	dst := make([]byte, 0, 1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = EncodeEnvelope(dst[:0], env, payload)
		_ = dst
	}
}
