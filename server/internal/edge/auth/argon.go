package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (auth.md § Password Provider, ADR-0051).
const (
	ArgonParamsVersion int16  = 1
	argonMemoryKiB     uint32 = 19456
	argonIterations    uint32 = 2
	argonParallelism   uint8  = 1
	argonSaltLen              = 16
	argonKeyLen        uint32 = 32
)

// HashPassword returns the PHC string for params_version 1.
func HashPassword(password string) (string, error) {
	var salt [argonSaltLen]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return "", err
	}
	return phcEncode(salt[:], argon2.IDKey([]byte(password), salt[:], argonIterations,
		argonMemoryKiB, argonParallelism, argonKeyLen)), nil
}

func phcEncode(salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key))
}

// VerifyPassword checks password against the stored PHC hash in constant
// time. A malformed stored hash is an error, not a mismatch.
func VerifyPassword(password, phc string) (bool, error) {
	mem, iters, par, salt, want, err := phcDecode(phc)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, iters, mem, par, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func phcDecode(phc string) (mem, iters uint32, par uint8, salt, key []byte, err error) {
	parts := strings.Split(phc, "$")
	// "$argon2id$v=19$m=..,t=..,p=..$salt$key" splits to ["", "argon2id", "v=19", "m=..", "salt", "key"]
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return 0, 0, 0, nil, nil, errors.New("auth: malformed password hash")
	}
	for _, kv := range strings.Split(parts[3], ",") {
		p := strings.SplitN(kv, "=", 2)
		if len(p) != 2 {
			return 0, 0, 0, nil, nil, errors.New("auth: malformed password hash")
		}
		n, nerr := strconv.ParseUint(p[1], 10, 32)
		if nerr != nil {
			return 0, 0, 0, nil, nil, errors.New("auth: malformed password hash")
		}
		switch p[0] {
		case "m":
			mem = uint32(n)
		case "t":
			iters = uint32(n)
		case "p":
			par = uint8(n)
		default:
			return 0, 0, 0, nil, nil, errors.New("auth: malformed password hash")
		}
	}
	if salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return 0, 0, 0, nil, nil, errors.New("auth: malformed password hash")
	}
	if key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil {
		return 0, 0, 0, nil, nil, errors.New("auth: malformed password hash")
	}
	if mem == 0 || iters == 0 || par == 0 || len(salt) == 0 || len(key) == 0 {
		return 0, 0, 0, nil, nil, errors.New("auth: malformed password hash")
	}
	return mem, iters, par, salt, key, nil
}

var dummyHashOnce struct {
	sync.Once
	phc string
	err error
}

// DummyHash returns a valid Argon2id PHC used to keep unknown-username
// logins at the same verification cost as real ones (auth.md § Password
// Provider: no user-enumeration through latency).
func DummyHash() (string, error) {
	dummyHashOnce.Do(func() {
		dummyHashOnce.phc, dummyHashOnce.err = HashPassword("devin-dummy-password-benchmark")
	})
	return dummyHashOnce.phc, dummyHashOnce.err
}
