package idempotency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// Receipt states (durable_command_receipts.state CHECK).
const (
	ReceiptAdmitted           = "ADMITTED"
	ReceiptCommitted          = "COMMITTED"
	ReceiptExpiredUncommitted = "EXPIRED_UNCOMMITTED"
	ReceiptRejected           = "REJECTED"
)

type receiptRow struct {
	family      string
	ownerKind   string
	ownerID     id.UUID
	opID        id.UUID
	fingerprint [32]byte
	admittedAt  time.Time
	issuedAt    time.Time
	replayUntil time.Time
	state       string
	outcomeVer  *int32
	outcome     []byte
	completedAt *time.Time
}

// TrustedReplay is the queued-CLIENT command path (ids.md § Trusted
// Queued-Client Replay). Admission reserves queue capacity before any DB
// write — a database outage enqueues nothing. Replay resolves by receipt
// state: COMMITTED returns the exact retained outcome (even past the
// horizon and after generic purge), terminal states return the retained
// typed error without executing, ADMITTED reconciles or executes the
// frozen original command, and a missing/conflicting proof holds the
// request — it is never rerolled, dropped or reissued.
func (s *Store) TrustedReplay(ctx context.Context, family string, owner Owner, opID id.UUID,
	fingerprint [32]byte, cb Command) (Outcome, error) {
	verr := s.validateID(opID)
	// A retained COMMITTED receipt reconstructs its exact outcome even past
	// the horizon — resolve receipts before an expired ID is rejected. An
	// ADMITTED receipt still terminalizes through the replay path.
	if errors.Is(verr, ErrExpired) {
		r, lerr := s.loadReceipt(ctx, family, owner, opID)
		switch {
		case lerr != nil || r == nil:
			return Outcome{}, verr
		case r.fingerprint != fingerprint:
			return Outcome{}, ErrConflict
		case r.state == ReceiptCommitted:
			return Outcome{SchemaVersion: versionOf(r.outcomeVer), Payload: r.outcome}, nil
		case r.state == ReceiptAdmitted:
			return s.reconcileOrExecute(ctx, r, fingerprint, cb, true)
		default:
			return Outcome{}, &TerminalError{State: r.state, Outcome: r.outcome}
		}
	}
	if verr != nil {
		return Outcome{}, verr
	}
	release, err := s.gate.Reserve(ctx)
	if err != nil {
		return Outcome{}, ErrDependency
	}
	defer release()

	rec, err := s.loadReceipt(ctx, family, owner, opID)
	if err != nil {
		return Outcome{}, fmt.Errorf("idempotency: %w", ErrDependency)
	}
	switch {
	case rec == nil:
		// A queued client command with an existing generic commit but no
		// receipt has ambiguous proof — hold; never execute as new.
		if _, done, lerr := s.lookupOperation(ctx, family, owner, opID, fingerprint); lerr != nil {
			if errors.Is(lerr, ErrConflict) {
				return Outcome{}, ErrConflict
			}
			return Outcome{}, lerr
		} else if done {
			return Outcome{}, ErrReplayHold
		}
	case rec.fingerprint != fingerprint:
		return Outcome{}, ErrConflict
	case rec.state == ReceiptCommitted:
		return Outcome{SchemaVersion: versionOf(rec.outcomeVer), Payload: rec.outcome}, nil
	case rec.state == ReceiptRejected:
		return Outcome{}, &TerminalError{State: rec.state, Outcome: rec.outcome}
	case rec.state == ReceiptExpiredUncommitted:
		return Outcome{}, &TerminalError{State: rec.state, Outcome: rec.outcome}
	case rec.state == ReceiptAdmitted:
		return s.reconcileOrExecute(ctx, rec, fingerprint, cb, true)
	default:
		return Outcome{}, ErrReplayHold
	}

	// Fresh admission: ADMITTED receipt commits first so a crash between
	// admission and the mutation commit is replay-reconcilable.
	if err := s.admit(ctx, family, owner, opID, fingerprint); err != nil {
		if errors.Is(err, ErrConflict) {
			// Concurrent admission won — dispatch on the stored row.
			rec, lerr := s.loadReceipt(ctx, family, owner, opID)
			if lerr != nil || rec == nil {
				return Outcome{}, ErrDependency
			}
			return s.dispatchReceipt(ctx, rec, fingerprint, cb)
		}
		return Outcome{}, err
	}
	rec = &receiptRow{family: family, ownerKind: string(owner.Kind), ownerID: owner.ID,
		opID: opID, fingerprint: fingerprint, state: ReceiptAdmitted,
		issuedAt: issuedTime(opID), replayUntil: issuedTime(opID).AddDate(0, 0, 180)}
	return s.reconcileOrExecute(ctx, rec, fingerprint, cb, true)
}

// dispatchReceipt routes a stored receipt by state.
func (s *Store) dispatchReceipt(ctx context.Context, rec *receiptRow,
	fingerprint [32]byte, cb Command) (Outcome, error) {
	switch {
	case rec.fingerprint != fingerprint:
		return Outcome{}, ErrConflict
	case rec.state == ReceiptCommitted:
		return Outcome{SchemaVersion: versionOf(rec.outcomeVer), Payload: rec.outcome}, nil
	case rec.state == ReceiptAdmitted:
		return s.reconcileOrExecute(ctx, rec, fingerprint, cb, true)
	case rec.state == ReceiptRejected || rec.state == ReceiptExpiredUncommitted:
		return Outcome{}, &TerminalError{State: rec.state, Outcome: rec.outcome}
	default:
		return Outcome{}, ErrReplayHold
	}
}

// admit inserts the ADMITTED receipt (its own transaction). A duplicate
// admission race is reconciled by the caller's reload.
func (s *Store) admit(ctx context.Context, family string, owner Owner,
	opID id.UUID, fingerprint [32]byte) error {
	issued := issuedTime(opID)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO durable_command_receipts
		 (operation_family, owner_kind, owner_id, operation_id, request_fingerprint,
		  admitted_at, issued_at, replay_until, state)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		family, string(owner.Kind), owner.ID.String(), opID.String(), fingerprint[:],
		s.now(), issued, issued.AddDate(0, 0, 180), ReceiptAdmitted)
	if err != nil {
		if isUniqueViolation(err) {
			// Another admission won; replay resolves from the stored row.
			return fmt.Errorf("idempotency: admission raced: %w", ErrConflict)
		}
		return fmt.Errorf("idempotency: admit: %w", ErrDependency)
	}
	return nil
}

// reconcileOrExecute handles the ADMITTED state under the receipt lock:
// recover a generic commit, terminalize at/after the horizon with zero
// writes, or apply the frozen original command — mutation, COMMITTED
// receipt and operations insert in one atomic transaction.
func (s *Store) reconcileOrExecute(ctx context.Context, rec *receiptRow,
	fingerprint [32]byte, cb Command, mayExecute bool) (Outcome, error) {
	// Lock the receipt row (priority 2.5 canonical order).
	if err := s.lockReceipt(ctx, rec); err != nil {
		return Outcome{}, err
	}
	if rec.state != ReceiptAdmitted {
		// Lock re-read saw a concurrent transition.
		switch rec.state {
		case ReceiptCommitted:
			return Outcome{SchemaVersion: versionOf(rec.outcomeVer), Payload: rec.outcome}, nil
		default:
			return Outcome{}, &TerminalError{State: rec.state, Outcome: rec.outcome}
		}
	}

	var genOutcome []byte
	var genFingerprint []byte
	err := s.pool.QueryRow(ctx,
		`SELECT request_fingerprint, outcome FROM operations
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
		rec.family, rec.ownerID.String(), rec.opID.String()).Scan(&genFingerprint, &genOutcome)
	switch {
	case err == nil:
		if string(genFingerprint) != string(fingerprint[:]) {
			return Outcome{}, ErrReplayHold
		}
		// Recover the exact COMMITTED outcome without re-executing.
		if err := s.terminalize(ctx, rec, ReceiptCommitted, genOutcome); err != nil {
			return Outcome{}, err
		}
		return Outcome{SchemaVersion: 1, Payload: genOutcome}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return Outcome{}, fmt.Errorf("idempotency: reconcile lookup: %w", err)
	}

	if !s.now().Before(rec.replayUntil) {
		// Horizon passed with no commit proof: terminal, zero writes.
		if err := s.terminalize(ctx, rec, ReceiptExpiredUncommitted, []byte("OPERATION_EXPIRED")); err != nil {
			return Outcome{}, err
		}
		return Outcome{}, &TerminalError{State: ReceiptExpiredUncommitted,
			Outcome: []byte("OPERATION_EXPIRED")}
	}
	if !mayExecute {
		// Orphan ADMITTED with no live reference: terminalize without
		// executing (REJECTED / TEMPORARY_DEPENDENCY_FAILURE).
		if err := s.terminalize(ctx, rec, ReceiptRejected, []byte("TEMPORARY_DEPENDENCY_FAILURE")); err != nil {
			return Outcome{}, err
		}
		return Outcome{}, &TerminalError{State: ReceiptRejected,
			Outcome: []byte("TEMPORARY_DEPENDENCY_FAILURE")}
	}

	var res Outcome
	committed := false
	err = pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var err error
		if _, err = tx.Exec(ctx,
			`SELECT 1 FROM durable_command_receipts
			 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3 FOR UPDATE`,
			rec.family, rec.ownerID.String(), rec.opID.String()); err != nil {
			return err
		}
		if res, err = cb(ctx, tx); err != nil {
			return err
		}
		if err = s.markReceiptTerminal(ctx, tx, rec, ReceiptCommitted, res.Payload); err != nil {
			return err
		}
		committed = true
		issued, _ := id.OperationIssuedAt(rec.opID)
		return insertOperation(ctx, tx, rec.family,
			Owner{Kind: OwnerKind(rec.ownerKind), ID: rec.ownerUUID()}, rec.opID,
			fingerprint, res, issued, s.now())
	})
	if err != nil {
		if isUniqueViolation(err) {
			out, done, lerr := s.lookupOperation(ctx, rec.family,
				Owner{Kind: OwnerKind(rec.ownerKind), ID: rec.ownerUUID()}, rec.opID,
				fingerprint)
			if lerr == nil && done {
				if err := s.terminalize(ctx, rec, ReceiptCommitted, out.Payload); err != nil {
					return Outcome{}, err
				}
				return out, nil
			}
			return Outcome{}, ErrReplayHold
		}
		// Failure after admission: retain a typed REJECTED — never an
		// infinite retry of a failing command.
		if terr := s.terminalize(ctx, rec, ReceiptRejected, outcomeBytes(err)); terr != nil {
			return Outcome{}, terr
		}
		return Outcome{}, &TerminalError{State: ReceiptRejected, Outcome: outcomeBytes(err)}
	}
	if !committed {
		return Outcome{}, ErrReplayHold
	}
	return res, nil
}

// loadReceipt reads by primary key (no lock — callers lock explicitly).
func (s *Store) loadReceipt(ctx context.Context, family string, owner Owner,
	opID id.UUID) (*receiptRow, error) {
	var r receiptRow
	var fp []byte
	var opIDStr, ownerStr string
	err := s.pool.QueryRow(ctx,
		`SELECT operation_family, owner_kind, owner_id, operation_id, request_fingerprint,
		        admitted_at, issued_at, replay_until, state,
		        outcome_schema_version, outcome, completed_at
		 FROM durable_command_receipts
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
		family, owner.ID.String(), opID.String()).
		Scan(&r.family, &r.ownerKind, &ownerStr, &opIDStr, &fp,
			&r.admittedAt, &r.issuedAt, &r.replayUntil, &r.state,
			&r.outcomeVer, &r.outcome, &r.completedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("idempotency: load receipt: %w", err)
	}
	copy(r.fingerprint[:], fp)
	if r.opID, err = id.ParseUUID(opIDStr); err != nil {
		return nil, err
	}
	if r.ownerID, err = id.ParseUUID(ownerStr); err != nil {
		return nil, err
	}
	return &r, nil
}

// lockReceipt SELECT FOR UPDATEs the receipt row and re-reads its state.
func (s *Store) lockReceipt(ctx context.Context, rec *receiptRow) error {
	var state string
	err := s.pool.QueryRow(ctx,
		`SELECT state FROM durable_command_receipts
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3 FOR UPDATE`,
		rec.family, rec.ownerID.String(), rec.opID.String()).Scan(&state)
	if err != nil {
		return fmt.Errorf("idempotency: lock receipt: %w", err)
	}
	rec.state = state
	return nil
}

// terminalize flips a receipt to a terminal state with retained outcome
// bytes (its own transaction — used post-crash / post-failure where the
// mutation transaction already rolled back).
func (s *Store) terminalize(ctx context.Context, rec *receiptRow, state string, outcome []byte) error {
	return pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return s.markReceiptTerminal(ctx, tx, rec, state, outcome)
	})
}

func (s *Store) markReceiptTerminal(ctx context.Context, tx pgx.Tx, rec *receiptRow,
	state string, outcome []byte) error {
	var ver *int32
	if outcome != nil {
		v := int32(1)
		ver = &v
	}
	tag, err := tx.Exec(ctx,
		`UPDATE durable_command_receipts
		 SET state=$4, outcome_schema_version=$5, outcome=$6, completed_at=$7
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3
		   AND state='ADMITTED'`,
		rec.family, rec.ownerID.String(), rec.opID.String(), state, ver, outcome, s.now())
	if err != nil {
		return fmt.Errorf("idempotency: terminalize: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Already terminal under another writer — re-read decides.
		var cur string
		if err := tx.QueryRow(ctx,
			`SELECT state FROM durable_command_receipts
			 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3`,
			rec.family, rec.ownerID.String(), rec.opID.String()).Scan(&cur); err != nil {
			return err
		}
		if cur == ReceiptCommitted && state == ReceiptCommitted {
			return nil
		}
		return ErrReplayHold
	}
	rec.state = state
	return nil
}

// Acknowledge marks a terminal receipt disposition-acked (required for
// purge; ADMITTED and unacknowledged rows are never purged).
func (s *Store) Acknowledge(ctx context.Context, family string, owner Owner, opID id.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE durable_command_receipts SET disposition_ack_at=$4
		 WHERE operation_family=$1 AND owner_id=$2 AND operation_id=$3
		   AND state<>'ADMITTED' AND disposition_ack_at IS NULL`,
		family, owner.ID.String(), opID.String(), s.now())
	if err != nil {
		return fmt.Errorf("idempotency: ack: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("idempotency: no ackable terminal receipt")
	}
	return nil
}

// PurgeReceipts deletes only terminal, acknowledged rows at/after
// replay_until (ids.md): ADMITTED or NULL-ack rows are never purged.
func (s *Store) PurgeReceipts(ctx context.Context, limit int64) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM durable_command_receipts
		 WHERE ctid IN (
		   SELECT ctid FROM durable_command_receipts
		   WHERE state<>'ADMITTED' AND disposition_ack_at IS NOT NULL
		     AND replay_until <= $1
		   ORDER BY replay_until LIMIT $2)`,
		s.now(), limit)
	if err != nil {
		return 0, fmt.Errorf("idempotency: purge receipts: %w", err)
	}
	return tag.RowsAffected(), nil
}

// PurgeOperations deletes generic outcome rows at/after replay_until whose
// receipts have no unacknowledged or non-terminal reference.
func (s *Store) PurgeOperations(ctx context.Context, limit int64) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM operations o
		 WHERE o.replay_until <= $1
		   AND o.ctid IN (
		     SELECT ctid FROM operations
		     WHERE replay_until <= $1 ORDER BY replay_until LIMIT $2)
		   AND NOT EXISTS (
		     SELECT 1 FROM durable_command_receipts r
		     WHERE r.operation_family=o.operation_family
		       AND r.owner_id=o.owner_id AND r.operation_id=o.operation_id
		       AND (r.disposition_ack_at IS NULL OR r.state='ADMITTED'))`,
		s.now(), limit)
	if err != nil {
		return 0, fmt.Errorf("idempotency: purge operations: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ReconcileOrphans is the quiescent-startup path: every ADMITTED receipt
// without a live journal/queue/in-flight reference first reconciles a
// generic commit, otherwise terminalizes without executing. A missing or
// conflicting proof holds the file — it returns the hold list and never
// rerolls, drops or reissues.
func (s *Store) ReconcileOrphans(ctx context.Context,
	live func(family string, owner Owner, opID id.UUID) bool) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT operation_family, owner_kind, owner_id, operation_id
		 FROM durable_command_receipts WHERE state='ADMITTED'
		 ORDER BY owner_id, operation_family, operation_id`)
	if err != nil {
		return nil, fmt.Errorf("idempotency: orphan scan: %w", err)
	}
	defer rows.Close()
	var holds []string
	for rows.Next() {
		var family, kind, ownerStr, opStr string
		if err := rows.Scan(&family, &kind, &ownerStr, &opStr); err != nil {
			return nil, err
		}
		ownerID, err := id.ParseUUID(ownerStr)
		if err != nil {
			return nil, err
		}
		opID, err := id.ParseUUID(opStr)
		if err != nil {
			return nil, err
		}
		owner := Owner{Kind: OwnerKind(kind), ID: ownerID}
		if live != nil && live(family, owner, opID) {
			continue // referenced admission stays ADMITTED for its executor
		}
		rec, err := s.loadReceipt(ctx, family, owner, opID)
		if err != nil {
			return nil, err
		}
		if _, err := s.reconcileOrExecute(ctx, rec, rec.fingerprint, nil, false); err != nil {
			var te *TerminalError
			if errors.As(err, &te) {
				continue // terminalized cleanly (REJECTED/EXPIRED_UNCOMMITTED)
			}
			holds = append(holds, fmt.Sprintf("%s/%s/%s: %v", family, ownerStr, opStr, err))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return holds, nil
}

// TerminalError carries a retained terminal outcome (typed JournalOutcome
// bytes). It unwraps to the state sentinel.
type TerminalError struct {
	State   string
	Outcome []byte
}

func (e *TerminalError) Error() string {
	return "idempotency: terminal receipt " + e.State + ": " + string(e.Outcome)
}

func (e *TerminalError) Unwrap() error {
	switch e.State {
	case ReceiptRejected:
		return ErrRejected
	case ReceiptExpiredUncommitted:
		return ErrExpiredUncommitted
	default:
		return ErrReplayHold
	}
}

// ---------------------------------------------------------------------------

func issuedTime(opID id.UUID) time.Time {
	if ms, ok := id.OperationIssuedAt(opID); ok {
		return time.UnixMilli(ms).UTC()
	}
	return time.Time{}
}

func versionOf(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func outcomeBytes(err error) []byte {
	if len(err.Error()) > 1024 {
		return []byte(err.Error()[:1024])
	}
	return []byte(err.Error())
}

// opIDOwner returns the receipt's owner UUID for operations inserts.
func (r *receiptRow) ownerUUID() id.UUID { return r.ownerID }
