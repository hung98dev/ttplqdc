// Package chat persists accepted chat messages to chat_messages — the
// moderation log of social.md § Chat History / data_model.md §
// chat_messages. Writes flow through the closed queue producer
// `chat.log` (save_rules.md § Closed Durable Queue Producer Registry):
// one JournalChatLog record per accepted message, applied inside the
// queue's committing transaction. Persistence is asynchronous and
// append-once — chat delivery never waits for it, and the delivery path
// holds no client receipt for these rows.
//
// Retention is 90 days rolling off `created_at` (personal_data_register
// § 1 F); the column doubles as IMP-056's erasure/retention marker —
// this package adds no extra retention state.
package chat
