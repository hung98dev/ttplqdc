// Package rng provides the single server-owned deterministic RNG interface for
// gameplay probability and content rolls (concurrency.md § RNG Concurrency):
// math/rand/v2 PCG-64 streams derived from typed authority inputs. UUID and
// secret generation stay on crypto/rand (ids.md); nothing here accepts
// client-supplied results.
package rng

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"

	"thinhthan/internal/core/id"
)

// ErrInvalidContext is returned when a Context cannot identify an authority
// stream (malformed content revision, non-v7 operation ID, or every identity
// input empty).
var ErrInvalidContext = errors.New("rng: invalid context")

// Context carries the typed authority inputs that derive a deterministic
// stream: content revision, partition/instance/source identity, operation or
// event identity, and the server-owned seed (concurrency.md § RNG
// Concurrency). All fields must describe the same authoritative roll.
type Context struct {
	// ContentRevision is the canonical 64-lowercase-hex content bundle hash.
	ContentRevision string
	// SourceID is the partition/instance/source identity (UUID), optional.
	SourceID id.UUID
	// OperationID is the admitting operation's UUIDv7, optional; when set it
	// must be a well-formed v7 (time-window checks are NOT applied here —
	// admission expiry is the caller's gate).
	OperationID id.UUID
	// EventID is the ordered event/counter identity when no operation UUID
	// applies.
	EventID uint64
	// Seed is the server-owned seed (may be a fixed regression seed).
	Seed uint64
}

// canonical bytes: revision ASCII (64) + source UUID (16) + operation UUID
// (16) + event LE (8) + seed LE (8) = 112 bytes, no separators needed because
// every field is fixed-width.
const contextLen = 64 + 16 + 16 + 8 + 8

func (c Context) canonical() []byte {
	var b [contextLen]byte
	copy(b[0:64], c.ContentRevision)
	s := c.SourceID.Bytes()
	copy(b[64:80], s[:])
	o := c.OperationID.Bytes()
	copy(b[80:96], o[:])
	binary.LittleEndian.PutUint64(b[96:104], c.EventID)
	binary.LittleEndian.PutUint64(b[104:112], c.Seed)
	return b[:]
}

func (c Context) validate() error {
	if err := id.ValidateContentRevision(c.ContentRevision); err != nil {
		return fmt.Errorf("%w: content_revision: %v", ErrInvalidContext, err)
	}
	if !c.OperationID.IsNil() {
		if _, ok := id.OperationIssuedAt(c.OperationID); !ok {
			return fmt.Errorf("%w: operation_id: not a well-formed UUIDv7", ErrInvalidContext)
		}
	}
	if c.SourceID.IsNil() && c.OperationID.IsNil() && c.EventID == 0 && c.Seed == 0 {
		return fmt.Errorf("%w: no identity input (source/operation/event/seed all zero)", ErrInvalidContext)
	}
	return nil
}

// Stream is one deterministic PCG-64 stream owned by a Context. Not safe for
// concurrent use; concurrency.md requires scheduling-independence, so callers
// draw from the owning context's stream in a fixed order.
type Stream struct {
	r *rand.Rand
}

// NewStream derives the deterministic PCG-64 stream for ctx. Identical
// contexts yield byte-identical streams; distinct contexts yield independent
// streams. The seed pair comes from SHA-256 over the canonical context
// encoding — no RNG participates in seeding.
func NewStream(ctx Context) (*Stream, error) {
	if err := ctx.validate(); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(ctx.canonical())
	pcg := rand.NewPCG(binary.LittleEndian.Uint64(sum[0:8]), binary.LittleEndian.Uint64(sum[8:16]))
	return &Stream{r: rand.New(pcg)}, nil
}

// Float64Range returns a half-open sample on [lo, hi) (concurrency.md:
// rand_float is half-open). It panics on lo >= hi or NaN bounds — a caller
// bug, matching math/rand/v2's own contract violations (IntN <= 0 panics).
func (s *Stream) Float64Range(lo, hi float64) float64 {
	if !(lo < hi) || lo != lo || hi != hi {
		panic(fmt.Sprintf("rng: Float64Range invalid bounds [%v, %v)", lo, hi))
	}
	return lo + s.r.Float64()*(hi-lo)
}

// IntN returns a half-open sample on [0, n); panics on n <= 0.
func (s *Stream) IntN(n int) int {
	return s.r.IntN(n)
}
