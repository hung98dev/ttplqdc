// Package id implements the canonical identity primitives of
// docs/06_data/ids.md: durable UUID v4 generation, name-based UUID v5,
// client operation UUID v7, static content IDs, runtime entity IDs,
// operation fingerprints and content revision validation.
package id

import (
	"crypto/rand"
	"fmt"
)

// UUID is a 16-byte RFC 4122/9562 universally unique identifier in network
// order, matching the protobuf UUID wire scalar (protobuf_conventions.md §6).
type UUID [16]byte

// NewV4 returns a durable entity UUID: RFC 4122 v4 generated from crypto/rand
// (ids.md § Durable IDs). The platform CSPRNG failing is unrecoverable.
func NewV4() UUID {
	var u UUID
	if _, err := rand.Read(u[:]); err != nil {
		panic(fmt.Sprintf("id: crypto/rand failed: %v", err))
	}
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}

// IsNil reports whether u is the all-zero UUID, which is never a valid ID.
func (u UUID) IsNil() bool { return u == UUID{} }

// Version reports the RFC 4122 version nibble (0 for the nil UUID).
func (u UUID) Version() int { return int(u[6] >> 4) }

// HasRFC4122Variant reports whether u carries the RFC 4122 variant bits (10xx).
func (u UUID) HasRFC4122Variant() bool { return u[8]&0xc0 == 0x80 }

// String returns the canonical lowercase 8-4-4-4-12 text form
// (ids.md § Serialization).
func (u UUID) String() string {
	const hexdigits = "0123456789abcdef"
	var buf [36]byte
	j := 0
	for i := 0; i < 16; i++ {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			buf[j] = '-'
			j++
		}
		buf[j] = hexdigits[u[i]>>4]
		buf[j+1] = hexdigits[u[i]&0x0f]
		j += 2
	}
	return string(buf[:])
}

// Bytes returns the 16-byte network-order binary form.
func (u UUID) Bytes() [16]byte { return u }

// ParseUUID parses the canonical lowercase 8-4-4-4-12 text form. Malformed
// input (wrong length or grouping, non-lowercase-hex characters, display
// strings) and the nil UUID are rejected (ids.md § Validation).
func ParseUUID(s string) (UUID, error) {
	var u UUID
	if len(s) != 36 {
		return u, fmt.Errorf("id: malformed UUID %q", s)
	}
	j := 0
	for i := 0; i < 36; i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return u, fmt.Errorf("id: malformed UUID %q", s)
			}
			continue
		}
		var v byte
		switch {
		case '0' <= c && c <= '9':
			v = c - '0'
		case 'a' <= c && c <= 'f':
			v = c - 'a' + 10
		default:
			return u, fmt.Errorf("id: malformed UUID %q", s)
		}
		if j%2 == 0 {
			u[j/2] = v << 4
		} else {
			u[j/2] |= v
		}
		j++
	}
	if u.IsNil() {
		return u, fmt.Errorf("id: nil UUID is not a valid ID")
	}
	return u, nil
}
