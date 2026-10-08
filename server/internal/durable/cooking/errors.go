package cooking

import "errors"

// Sentinel domain errors mapped to wire codes at the executor boundary.
var (
	// ErrInsufficientItems — the selected consume set cannot cover the
	// recipe inputs (admission-side and commit-side revalidation).
	ErrInsufficientItems = errors.New("cooking: insufficient input items")
	// ErrInventoryFull — no stack or empty slot can take an output.
	ErrInventoryFull = errors.New("cooking: inventory capacity full")
	// ErrNotFound — a frozen consume instance is gone at commit.
	ErrNotFound = errors.New("cooking: frozen item instance missing")
)
