package social

import "errors"

var (
	// ErrMalformedRecord: a frozen record failed identity checks.
	ErrMalformedRecord = errors.New("social: malformed record")
)
