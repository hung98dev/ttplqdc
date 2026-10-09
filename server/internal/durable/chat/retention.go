package chat

import "time"

// RetentionWindow is the 90-day rolling retention of chat_messages —
// data_model.md § chat_messages / personal_data_register § 1 F. A row
// is purge-eligible when now >= created_at + RetentionWindow; the purge
// worker (IMP-056) consumes the `created_at` marker directly.
const RetentionWindow = 90 * 24 * time.Hour

// ErasureWindow is the 15-day bound inside which a subject erasure must
// delete the account's chat_messages rows (data_protection.md).
const ErasureWindow = 15 * 24 * time.Hour
