package queue

import (
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/proto"

	journalv1 "thinhthan/internal/durable/journal/v1"
)

// FrozenInventory is the immutable snapshot of every queued + in-flight +
// journal-referenced command the composition-root journal writer encodes
// durably. Records are deep clones: later queue resolution never mutates
// the snapshot.
type FrozenInventory struct {
	// Records is the complete inventory in no particular order — the
	// journal writer serializes in admission_sequence order itself.
	Records []*journalv1.DurableCommandRecord
	keys    []recordKey
}

// Encode serializes the inventory as a bounded record stream:
// uvarint count, then each record as uvarint length + deterministic proto
// bytes. Journal file framing (TTJRN001 header/CRC32C) belongs to the
// composition-root writer (IMP-069); this is record marshal only.
func (f *FrozenInventory) Encode() ([]byte, error) {
	var out []byte
	var hdr [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(hdr[:], uint64(len(f.Records)))
	out = append(out, hdr[:n]...)
	for _, rec := range f.Records {
		b, err := proto.MarshalOptions{Deterministic: true}.Marshal(rec)
		if err != nil {
			return nil, fmt.Errorf("queue: encode inventory: %w", err)
		}
		if len(b) > 1048576 {
			return nil, fmt.Errorf("%w: record %d bytes exceeds 1 MiB", ErrUnknownProducer, len(b))
		}
		n = binary.PutUvarint(hdr[:], uint64(len(b)))
		out = append(out, hdr[:n]...)
		out = append(out, b...)
	}
	return out, nil
}
