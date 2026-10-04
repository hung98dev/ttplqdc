package listener

import (
	"google.golang.org/protobuf/encoding/protowire"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// wire fields of Envelope (common.proto): every scalar field is varint,
// payload is bytes.
const (
	envFieldProtocolMajor = 1
	envFieldProtocolMinor = 2
	envFieldMessageID     = 3
	envFieldSessionEpoch  = 4
	envFieldClientSeq     = 5
	envFieldServerSeq     = 6
	envFieldCorrelationID = 7
	envFieldPayload       = 8
)

// EncodeEnvelope appends the wire encoding of `env` — with `payload` as its
// already-encoded inner message — to `dst` and returns the extended slice.
// It is the hot-path encoder of HOT-003 (capacity.md): envelope encode +
// frame write into a pooled buffer = 0 allocs/op. Callers pass a pooled
// destination with enough capacity and recycle it themselves; nothing here
// allocates.
func EncodeEnvelope(dst []byte, env *protocolv1.Envelope, payload []byte) []byte {
	dst = protowire.AppendVarint(dst, uint64(envFieldProtocolMajor<<3)|uint64(protowire.VarintType))
	dst = protowire.AppendVarint(dst, uint64(env.ProtocolMajor))
	if env.ProtocolMinor != 0 {
		dst = protowire.AppendVarint(dst, uint64(envFieldProtocolMinor<<3)|uint64(protowire.VarintType))
		dst = protowire.AppendVarint(dst, uint64(env.ProtocolMinor))
	}
	dst = protowire.AppendVarint(dst, uint64(envFieldMessageID<<3)|uint64(protowire.VarintType))
	dst = protowire.AppendVarint(dst, uint64(env.MessageId))
	if env.SessionEpoch != 0 {
		dst = protowire.AppendVarint(dst, uint64(envFieldSessionEpoch<<3)|uint64(protowire.VarintType))
		dst = protowire.AppendVarint(dst, env.SessionEpoch)
	}
	if env.ClientSeq != 0 {
		dst = protowire.AppendVarint(dst, uint64(envFieldClientSeq<<3)|uint64(protowire.VarintType))
		dst = protowire.AppendVarint(dst, env.ClientSeq)
	}
	if env.ServerSeq != 0 {
		dst = protowire.AppendVarint(dst, uint64(envFieldServerSeq<<3)|uint64(protowire.VarintType))
		dst = protowire.AppendVarint(dst, env.ServerSeq)
	}
	if env.CorrelationId != 0 {
		dst = protowire.AppendVarint(dst, uint64(envFieldCorrelationID<<3)|uint64(protowire.VarintType))
		dst = protowire.AppendVarint(dst, env.CorrelationId)
	}
	dst = protowire.AppendTag(dst, envFieldPayload, protowire.BytesType)
	dst = protowire.AppendBytes(dst, payload)
	return dst
}
