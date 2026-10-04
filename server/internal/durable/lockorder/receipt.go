package lockorder

import (
	"bytes"
	"sort"

	"thinhthan/internal/core/id"
)

// Receipt is one durable_command_receipts key triple. The canonical
// key order is owner UUID bytes, then operation_family ASCII, then
// operation UUID bytes (database.md § Lock Order, priority 2.5).
type Receipt struct {
	Owner     id.UUID
	Family    string
	Operation id.UUID
}

// key encodes the receipt triple as a byte string whose lexical order
// equals the canonical tuple order: the 0x00 separator terminates each
// variable-length ASCII family before the operation bytes.
func (r Receipt) key() []byte {
	k := make([]byte, 0, 16+len(r.Family)+1+16)
	o := r.Owner.Bytes()
	k = append(k, o[:]...)
	k = append(k, r.Family...)
	k = append(k, 0x00)
	op := r.Operation.Bytes()
	k = append(k, op[:]...)
	return k
}

// ReceiptKeys expands receipt triples into priority-2.5 advisory locks
// in canonical key order. Queued CLIENT admission/replay collects every
// multiowner receipt key here before any mutation.
func ReceiptKeys(receipts ...Receipt) []Lock {
	rs := append([]Receipt(nil), receipts...)
	sort.Slice(rs, func(i, j int) bool {
		if c := bytes.Compare(rs[i].Owner[:], rs[j].Owner[:]); c != 0 {
			return c < 0
		}
		if rs[i].Family != rs[j].Family {
			return rs[i].Family < rs[j].Family
		}
		return bytes.Compare(rs[i].Operation[:], rs[j].Operation[:]) < 0
	})
	locks := make([]Lock, 0, len(rs))
	for _, r := range rs {
		locks = append(locks, AdvisoryLock("durable_command_receipts", r.key()))
	}
	return locks
}

// AccountCharacterSet returns the complete account/character lock set
// for queued CLIENT admission/replay (database.md: "locks the complete
// account/character set, then priority-2.5 receipt keys"). Priority-1
// locks cover the account's auth rows; auth_refresh_credentials is
// keyed by session family, not account, so it locks advisory by
// account key. Priority-2 locks cover each character's projections in
// UUID order. The returned list is already canonical.
func AccountCharacterSet(account id.UUID, chars []id.UUID) []Lock {
	ab := account.Bytes()
	locks := []Lock{
		RowLock("accounts", account),
		RowLock("account_password_credentials", account),
		RowLock("account_identities", account),
		RowLock("auth_session_families", account),
		AdvisoryLock("auth_refresh_credentials", ab[:]),
		RowLock("auth_revocations", account),
		RowLock("account_login_history", account),
		RowLock("erasure_intents", account),
	}
	cs := append([]id.UUID(nil), chars...)
	sort.Slice(cs, func(i, j int) bool { return bytes.Compare(cs[i][:], cs[j][:]) < 0 })
	// Within a priority, tables lock in listed order — every characters
	// lock precedes all character_activity locks, not per-character.
	for _, table := range []string{"characters", "character_activity", "character_attach_events", "character_chivalry", "character_chat_restrictions"} {
		for _, c := range cs {
			locks = append(locks, RowLock(table, c))
		}
	}
	return locks
}

// OperationInsertLock is the trailing operations admission: it sorts
// after every aggregate lock (Operations priority) so the idempotent
// operation row is always the last thing a transaction serializes on
// before its commit-time insert.
func OperationInsertLock(family string, owner, op id.UUID) Lock {
	k := make([]byte, 0, len(family)+1+16+16)
	k = append(k, family...)
	k = append(k, 0x00)
	o := owner.Bytes()
	k = append(k, o[:]...)
	p := op.Bytes()
	k = append(k, p[:]...)
	return Lock{Table: "operations", Key: k, Mode: Advisory, priority: Operations}
}
