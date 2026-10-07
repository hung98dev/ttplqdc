package world

import "errors"

var (
	// ErrNotFound is returned when a lookup finds no characters row.
	ErrNotFound = errors.New("world: character not found")
)
