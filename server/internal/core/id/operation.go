package id

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"
)

const (
	// OperationFutureSkew is the maximum admitted offset between a client
	// UUIDv7 timestamp and server UTC (ids.md § Operation IDs).
	OperationFutureSkew = 60 * time.Second
	// OperationReplayHorizon is the client operation replay horizon:
	// now >= issued_at + 180 days expires the operation.
	OperationReplayHorizon = 180 * 24 * time.Hour
)

// OperationIDErrorKind classifies a ValidateOperationID rejection so the
// edge maps it to wire codes (errors.md): Malformed and FutureSkew map to
// PROTOCOL_MALFORMED, Expired to OPERATION_EXPIRED.
type OperationIDErrorKind int

const (
	// OperationIDMalformed covers nil, non-v7 and malformed IDs and every
	// value that is not a well-formed RFC 9562 UUIDv7.
	OperationIDMalformed OperationIDErrorKind = iota
	// OperationIDFutureSkew covers issued_at > now + 60 s.
	OperationIDFutureSkew
	// OperationIDExpired covers now >= issued_at + 180 days.
	OperationIDExpired
)

func (k OperationIDErrorKind) String() string {
	switch k {
	case OperationIDMalformed:
		return "malformed"
	case OperationIDFutureSkew:
		return "future_skew"
	case OperationIDExpired:
		return "expired"
	}
	return "unknown"
}

// OperationIDError is the typed rejection of ValidateOperationID; consumers
// read Kind through errors.As to select the wire error code.
type OperationIDError struct {
	Kind OperationIDErrorKind
}

func (e *OperationIDError) Error() string {
	return fmt.Sprintf("id: invalid operation ID: %s", e.Kind)
}

// NewV7 returns a client operation ID: an RFC 9562 UUIDv7 whose first 48
// bits are now's Unix UTC milliseconds, with version/variant bits set and
// the remaining 74 bits crypto-random (ids.md § Operation IDs).
func NewV7(now time.Time) UUID {
	var u UUID
	if _, err := rand.Read(u[6:]); err != nil {
		panic(fmt.Sprintf("id: crypto/rand failed: %v", err))
	}
	ms := uint64(now.UnixMilli())
	u[0] = byte(ms >> 40)
	u[1] = byte(ms >> 32)
	u[2] = byte(ms >> 24)
	u[3] = byte(ms >> 16)
	u[4] = byte(ms >> 8)
	u[5] = byte(ms)
	u[6] = (u[6] & 0x0f) | 0x70
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}

// OperationIssuedAt extracts the 48-bit Unix-millisecond timestamp embedded
// in a UUIDv7. ok is false for a non-v7 or non-RFC-variant value.
func OperationIssuedAt(id UUID) (ms int64, ok bool) {
	if id.Version() != 7 || !id.HasRFC4122Variant() {
		return 0, false
	}
	ms = int64(uint64(id[0])<<40 | uint64(id[1])<<32 | uint64(id[2])<<24 |
		uint64(id[3])<<16 | uint64(id[4])<<8 | uint64(id[5]))
	return ms, true
}

// ValidateOperationID applies the public client operation ID checks of
// ids.md § Operation IDs against the authenticated server instant now: nil,
// non-v7 or malformed IDs and issued_at > now+60s yield
// OperationIDMalformed/OperationIDFutureSkew (PROTOCOL_MALFORMED);
// now >= issued_at+180d yields OperationIDExpired (OPERATION_EXPIRED). An
// expired ID is never reinterpreted as a new operation.
func ValidateOperationID(id UUID, now time.Time) error {
	if id.IsNil() {
		return &OperationIDError{Kind: OperationIDMalformed}
	}
	ms, ok := OperationIssuedAt(id)
	if !ok {
		return &OperationIDError{Kind: OperationIDMalformed}
	}
	issued := time.UnixMilli(ms).UTC()
	if issued.After(now.Add(OperationFutureSkew)) {
		return &OperationIDError{Kind: OperationIDFutureSkew}
	}
	if !now.Before(issued.Add(OperationReplayHorizon)) {
		return &OperationIDError{Kind: OperationIDExpired}
	}
	return nil
}

// OperationFingerprint computes the request fingerprint of the durable
// admission receipt: SHA-256 over the operation family, owner, operation
// UUID and the exact typed request bytes (ids.md § Trusted Queued-Client
// Replay). request is the caller's deterministic serialization of the
// original request; identical retries reproduce the fingerprint while a
// conflicting payload changes it, surfacing OPERATION_CONFLICT at the owner.
func OperationFingerprint(family string, owner UUID, operationID UUID, request []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte(family))
	h.Write([]byte{0})
	h.Write(owner[:])
	h.Write(operationID[:])
	h.Write(request)
	var fp [32]byte
	copy(fp[:], h.Sum(nil))
	return fp
}
