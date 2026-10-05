package auth

import (
	"strings"
	"unicode/utf8"
)

// Username/email/password validation of auth.md § Password Provider.

// ValidateUsername returns the canonical username_key: trimmed, lowercase,
// 4–20 chars of [a-z0-9_], starting with a letter.
func ValidateUsername(username string) (string, error) {
	u := strings.ToLower(strings.TrimSpace(username))
	if len(u) < 4 || len(u) > 20 {
		return "", ErrUsernameInvalid
	}
	for i := 0; i < len(u); i++ {
		c := u[i]
		if c >= 'a' && c <= 'z' {
			continue
		}
		if (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return "", ErrUsernameInvalid
	}
	if u[0] < 'a' || u[0] > 'z' {
		return "", ErrUsernameInvalid
	}
	return u, nil
}

// ValidateEmail returns (email_key, canonical email): trimmed ≤254, one
// '@', local part 1–64 chars, domain with ≥1 '.', no label starting or
// ending with '.'/'-'. Keys lowercase without folding.
func ValidateEmail(email string) (string, string, error) {
	e := strings.TrimSpace(email)
	if len(e) == 0 || len(e) > 254 {
		return "", "", ErrEmailInvalid
	}
	at := strings.IndexByte(e, '@')
	if at < 0 || strings.IndexByte(e[at+1:], '@') >= 0 {
		return "", "", ErrEmailInvalid
	}
	local, domain := e[:at], e[at+1:]
	if len(local) < 1 || len(local) > 64 {
		return "", "", ErrEmailInvalid
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", "", ErrEmailInvalid
	}
	for _, l := range labels {
		if l == "" || l[0] == '-' || l[0] == '.' || l[len(l)-1] == '-' || l[len(l)-1] == '.' {
			return "", "", ErrEmailInvalid
		}
	}
	return strings.ToLower(e), e, nil
}

// ValidatePassword: 8–128 code points (NFC counted); never equal to the
// username_key or email_key case-insensitively.
func ValidatePassword(password, usernameKey, emailKey string) error {
	n := utf8.RuneCountInString(password)
	if n < 8 || n > 128 {
		return ErrPasswordInvalid
	}
	p := strings.ToLower(password)
	if p == usernameKey || p == emailKey {
		return ErrPasswordInvalid
	}
	return nil
}
