package id

import "crypto/sha1"

// V5 returns the RFC 4122 §4.3 name-based UUID v5: SHA-1 over the
// namespace's 16 network-order bytes concatenated with the UTF-8 name,
// truncated to 16 bytes with version 5 and RFC variant bits set
// (ids.md § Deterministic Content-Grant Idempotency Keys).
func V5(namespace UUID, name string) UUID {
	h := sha1.New()
	h.Write(namespace[:])
	h.Write([]byte(name))
	sum := h.Sum(nil)
	var u UUID
	copy(u[:], sum[:16])
	u[6] = (u[6] & 0x0f) | 0x50
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}
