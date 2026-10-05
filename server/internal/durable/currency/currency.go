// Package currency is the durable currency primitive (economy.md):
// three character-scoped balances with canonical caps, atomic
// debit/credit inside the caller's transaction, cross-account-only
// common-currency transfer, and one audit_events row per committed
// mutation. Settlement layers (rewards, trade, auction) call Apply or
// Transfer under lockorder + idempotency.Store.Execute; the primitive
// owns no transaction boundary itself.
package currency

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/lockorder"
)

// ID is the canonical currency identifier (economy.md § currencies).
type ID string

const (
	Common  ID = "currency.common"
	Bound   ID = "currency.bound"
	Special ID = "currency.special"
)

// Caps are the canonical per-character balance ceilings (economy.md).
// A credit that would exceed the cap fails whole — never clamped.
var Caps = map[ID]int64{
	Common:  2_000_000_000,
	Bound:   100_000_000,
	Special: 1_000_000,
}

var (
	ErrInsufficientBalance = errors.New("currency: insufficient balance")
	ErrCapExceeded         = errors.New("currency: credit exceeds cap")
	ErrUnknownCurrency     = errors.New("currency: unknown currency id")
	ErrInvalidDelta        = errors.New("currency: zero delta")
	ErrNonTransferable     = errors.New("currency: currency is not transferable")
	ErrSameAccountTransfer = errors.New("currency: transfer within one account")
	ErrInvalidTransfer     = errors.New("currency: invalid transfer")
)

// ActorKind is the audit_events.actor_kind enum this primitive emits.
type ActorKind string

const (
	ActorPlayer ActorKind = "PLAYER"
	ActorSystem ActorKind = "SYSTEM"
)

// Mutation is one signed balance change under an operation ID.
type Mutation struct {
	CharacterID id.UUID
	CurrencyID  ID
	// Delta is signed: positive credits, negative debits.
	Delta       int64
	OperationID id.UUID
	ReasonCode  string
	SourceRef   string
	Actor       ActorKind
}

// Result reports the committed balance pair.
type Result struct {
	Before int64
	After  int64
}

// Transfer moves Amount of currency.common between two characters on
// different accounts (economy.md § transfers + ADR-0029).
type Transfer struct {
	FromCharacterID id.UUID
	ToCharacterID   id.UUID
	CurrencyID      ID
	// Amount is a positive common-currency amount.
	Amount      int64
	OperationID id.UUID
	ReasonCode  string
	SourceRef   string
	Actor       ActorKind
}

// TransferResult reports both committed balance pairs.
type TransferResult struct {
	From Result
	To   Result
}

// Apply commits one mutation on the caller's tx: it takes the
// character_currencies row lock, validates cap/floor, updates balance
// and bumps revision, then writes the audit row.
func Apply(ctx context.Context, tx pgx.Tx, m Mutation) (Result, error) {
	if err := validateMutation(m); err != nil {
		return Result{}, err
	}
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("character_currencies", m.CharacterID)); err != nil {
		return Result{}, err
	}
	return applyLocked(ctx, tx, m)
}

// Credit commits a positive-delta mutation.
func Credit(ctx context.Context, tx pgx.Tx, m Mutation) (Result, error) {
	if m.Delta <= 0 {
		return Result{}, ErrInvalidDelta
	}
	return Apply(ctx, tx, m)
}

// Debit commits a negative-delta mutation; Delta is given positive.
func Debit(ctx context.Context, tx pgx.Tx, m Mutation) (Result, error) {
	if m.Delta <= 0 {
		return Result{}, ErrInvalidDelta
	}
	m.Delta = -m.Delta
	return Apply(ctx, tx, m)
}

// DoTransfer settles one cross-account currency.common movement in the
// caller's tx: canonical locks on both characters and both currency
// rows, account-equality rejection, then atomic debit+credit+paired
// audit rows. Same-account and same-character transfers both reject
// with ErrSameAccountTransfer; only currency.common is transferable
// (bound/special reject with ErrNonTransferable).
func DoTransfer(ctx context.Context, tx pgx.Tx, t Transfer) (TransferResult, error) {
	if t.Amount <= 0 {
		return TransferResult{}, ErrInvalidTransfer
	}
	if t.CurrencyID != Common {
		return TransferResult{}, ErrNonTransferable
	}
	if err := validateActor(t.Actor); err != nil {
		return TransferResult{}, err
	}
	locks := []lockorder.Lock{
		lockorder.RowLock("characters", t.FromCharacterID),
		lockorder.RowLock("characters", t.ToCharacterID),
		lockorder.RowLock("character_currencies", t.FromCharacterID),
		lockorder.RowLock("character_currencies", t.ToCharacterID),
	}
	if err := lockorder.SortLocks(locks); err != nil {
		return TransferResult{}, err
	}
	if err := lockorder.Acquire(ctx, tx, locks...); err != nil {
		return TransferResult{}, err
	}
	fromAcct, err := accountOf(ctx, tx, t.FromCharacterID)
	if err != nil {
		return TransferResult{}, err
	}
	toAcct, err := accountOf(ctx, tx, t.ToCharacterID)
	if err != nil {
		return TransferResult{}, err
	}
	if fromAcct == toAcct {
		return TransferResult{}, ErrSameAccountTransfer
	}
	m := Mutation{
		CurrencyID:  t.CurrencyID,
		OperationID: t.OperationID,
		ReasonCode:  t.ReasonCode,
		SourceRef:   t.SourceRef,
		Actor:       t.Actor,
	}
	m.CharacterID, m.Delta = t.FromCharacterID, -t.Amount
	from, err := applyLocked(ctx, tx, m)
	if err != nil {
		return TransferResult{}, err
	}
	m.CharacterID, m.Delta = t.ToCharacterID, t.Amount
	to, err := applyLocked(ctx, tx, m)
	if err != nil {
		return TransferResult{}, err
	}
	return TransferResult{From: from, To: to}, nil
}

// validateMutation checks fields before any lock or row read.
func validateMutation(m Mutation) error {
	if _, ok := Caps[m.CurrencyID]; !ok {
		return ErrUnknownCurrency
	}
	if m.Delta == 0 {
		return ErrInvalidDelta
	}
	return validateActor(m.Actor)
}

func validateActor(a ActorKind) error {
	switch a {
	case ActorPlayer, ActorSystem:
		return nil
	}
	return fmt.Errorf("currency: invalid actor kind %q", string(a))
}

// accountOf reads the owning account under the characters row lock the
// caller already holds.
func accountOf(ctx context.Context, tx pgx.Tx, characterID id.UUID) (id.UUID, error) {
	var acct id.UUID
	err := tx.QueryRow(ctx,
		`SELECT account_id FROM characters WHERE character_id=$1`, characterID.String()).
		Scan(&acct)
	if err != nil {
		return id.UUID{}, fmt.Errorf("currency: resolve account: %w", err)
	}
	return acct, nil
}

// applyLocked mutates one balance assuming the caller already holds the
// character_currencies lock for m.CharacterID. The row lock covers all
// of the character's currency rows; a missing row (first credit)
// inserts at balance=delta.
func applyLocked(ctx context.Context, tx pgx.Tx, m Mutation) (Result, error) {
	var before int64
	err := tx.QueryRow(ctx,
		`SELECT balance FROM character_currencies
		 WHERE character_id=$1 AND currency_id=$2`,
		m.CharacterID.String(), string(m.CurrencyID)).Scan(&before)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if m.Delta < 0 {
			return Result{}, ErrInsufficientBalance
		}
		if m.Delta > Caps[m.CurrencyID] {
			return Result{}, ErrCapExceeded
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_currencies (character_id, currency_id, balance)
			 VALUES ($1,$2,$3)`,
			m.CharacterID.String(), string(m.CurrencyID), m.Delta); err != nil {
			return Result{}, fmt.Errorf("currency: first credit insert: %w", err)
		}
		res := Result{Before: 0, After: m.Delta}
		if err := writeAudit(ctx, tx, m, res); err != nil {
			return Result{}, err
		}
		return res, nil
	case err != nil:
		return Result{}, fmt.Errorf("currency: read balance: %w", err)
	}
	after := before + m.Delta
	switch {
	case after < 0:
		return Result{}, ErrInsufficientBalance
	case after > Caps[m.CurrencyID]:
		return Result{}, ErrCapExceeded
	}
	if _, err := tx.Exec(ctx,
		`UPDATE character_currencies SET balance=$3, revision=revision+1
		 WHERE character_id=$1 AND currency_id=$2`,
		m.CharacterID.String(), string(m.CurrencyID), after); err != nil {
		return Result{}, fmt.Errorf("currency: update balance: %w", err)
	}
	res := Result{Before: before, After: after}
	if err := writeAudit(ctx, tx, m, res); err != nil {
		return Result{}, err
	}
	return res, nil
}
