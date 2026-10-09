package chat

import (
	"errors"
	"fmt"
)

// ErrMalformedRecord is the terminal-validation sentinel for a record
// that cannot produce a chat_messages write.
var ErrMalformedRecord = errors.New("chat: malformed record")

func errField(field string) error {
	return fmt.Errorf("%w: %s", ErrMalformedRecord, field)
}
