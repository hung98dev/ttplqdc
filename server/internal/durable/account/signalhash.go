package account

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"net"
)

// Signal hash helpers (auth.md § Device ID, external_integrations.md § 3):
// security/risk signals are HMAC-SHA-256 keyed with ACCOUNT_SIGNAL_SALT;
// login-history device/prefix rows store SHA-256(salt || value).

// Salt is the decoded 32-byte ACCOUNT_SIGNAL_SALT.
type Salt [32]byte

// RateKey builds the L2 limiter key_hash: HMAC-SHA-256 over the canonical
// action:scope_kind:scope_value tuple.
func (s Salt) RateKey(action, scopeKind, scopeValue string) [32]byte {
	m := hmac.New(sha256.New, s[:])
	_, _ = m.Write([]byte(action))
	_, _ = m.Write([]byte{':'})
	_, _ = m.Write([]byte(scopeKind))
	_, _ = m.Write([]byte{':'})
	_, _ = m.Write([]byte(scopeValue))
	var out [32]byte
	copy(out[:], m.Sum(nil))
	return out
}

// SaltedHash stores SHA-256(salt || raw) — device_id and IP-prefix rows in
// account_login_history.
func (s Salt) SaltedHash(raw []byte) [32]byte {
	h := sha256.New()
	_, _ = h.Write(s[:])
	_, _ = h.Write(raw)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// DeviceIDHash hashes one client device_id (16-byte UUID or its canonical
// string form; bytes are hashed verbatim per auth.md § Device ID).
func (s Salt) DeviceIDHash(deviceID []byte) [32]byte {
	return s.SaltedHash(deviceID)
}

// IPPrefixHash hashes the /16 (IPv4) or /48 (IPv6) network prefix of ip.
func (s Salt) IPPrefixHash(ip net.IP) [32]byte {
	return s.SaltedHash(IPPrefix(ip))
}

// IPPrefix returns the canonical prefix bytes: first 2 octets for IPv4,
// first 6 for IPv6 (data_model.md § account_login_history).
func IPPrefix(ip net.IP) []byte {
	if v4 := ip.To4(); v4 != nil {
		return v4[:2]
	}
	v6 := ip.To16()
	if v6 == nil {
		return nil
	}
	return v6[:6]
}

// IPScopeValue renders the rate-limit IP scope value: the raw /32 IPv4 or
// the /64 IPv6 subnet (external_integrations.md §3 scope table).
func IPScopeValue(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	v6 := ip.To16()
	if v6 == nil {
		return ""
	}
	for i := 8; i < 16; i++ {
		v6[i] = 0
	}
	return v6.String()
}

// Int64ScopeValue renders a positive int64 scope suffix.
func Int64ScopeValue(v int64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(v))
	return string(b[:])
}
