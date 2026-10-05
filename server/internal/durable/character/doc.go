// Package character owns the durable character aggregate: canonical
// name normalization (text.md), the characters-row store, and the
// character.create queue executor (save_rules.md producer row,
// ADR-0081 seam). Characters are permanent — no delete path exists.
package character
