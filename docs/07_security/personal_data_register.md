# Personal Data Register & Retention Schedule
status: LOCKED

## Scope

Canonical mapping of personal-data categories to tables/columns and the **single canonical retention schedule** (ADR-0065). `data_protection.md` owns the legal framework, category definitions and request handling and references this file for periods. The erasure transaction itself is canonical in `../06_data/data_model.md` § Account Erasure. Executing task: `IMP-056`.

Legal basis: Luật 91/2025/QH15 và Nghị định 356/2025/NĐ-CP (personal data), Luật Kế toán 88/2015/QH13 Điều 41 (financial records).

## 1. Retention Schedule and Column Mapping

| Cat. | Data | Tables / columns | Retention (automatic purge) | At erasure (`data_model.md` § Account Erasure) |
|---|---|---|---|---|
| **A** Account identity | username, email, password hash; account row | `account_password_credentials.*`; `accounts.*` | credentials: account lifetime; `accounts` row: 1 year after `erased_at` (holds no personal column after erasure) | credentials deleted; `accounts.status = TOMBSTONE_ERASED`, `erased_at` set, `created_at` overwritten with the erasure day, guard/review columns cleared (ADR-0070) |
| **B** Provider links | Apple/Google/Steam subject | `account_identities.*` | account lifetime; a link removed by unlink is deleted immediately | deleted |
| **C** Session & device metadata | session families, refresh hashes, platform, app version, device model class | `auth_session_families.*`, `auth_refresh_credentials.*` | 30 days after the family is revoked or expires | deleted (families already revoked at deletion request) |
| **D** Security signals | revocations; login device/IP-prefix hashes; rate-limit counters; auth failure backoff | `auth_revocations.*`, `account_login_history.*`, `rate_limit_counters.*`, `auth_failure_backoff.*` | revocations: until `expires_at` (≤ 30 days); login history: 90 days rolling; rate-limit counters and auth failure backoff: 24 h after last update | revocations and login history deleted; rate-limit counters and auth failure backoff are keyed by HMAC-SHA-256 with the secret `ACCOUNT_SIGNAL_SALT` (`external_integrations.md` § 3), so rows cannot be found from a username/IP without the server secret; they are not erased individually (they expire within 24 h) |
| **E** Payment records | receipts, entitlement history, refund events, IAP notifications | `account_iap_entitlements.*`, `account_refund_consumed_events.*`, `account_entitlement_claims.*`, `iap_notification_dedup.*` | entitlements, claims, refund events: **10 years** from `created_at` (Luật Kế toán), and an entitlement row stays while any cosmetic entitlement row references it; notification dedup: 180 days | account link severed: `account_id` re-pointed to `TOMBSTONE_ACCOUNT_ID` |
| **F** Chat | moderation log | `chat_messages.*` | 90 days rolling | the account's rows deleted |
| **G** Gameplay & economy records | characters and owned state; social graph; operations; settlements; rollups; guild storage audit | `characters.*` and character-owned tables, `friends.*`, `friend_requests.*`, `blocks.*`, `pvp_match_settlements.*`, `pvp_sanctions.*`, `guild_war_settlements.*`, `operations.*`, `auction_listings.*`, `auction_proceeds.*`, `trade_settlement_records.*`, `economy_*_daily_rollups.*`, `guild_storage_audit.*` | characters: permanent (anonymized after erasure); friendships and blocks: while both characters exist; `friend_requests` not `PENDING`: 30 days after `resolved_at`; PvP and Guild War settlements: 180 days after `settled_at`; PvP sanctions: 180 days after `ends_at`; operations, trade settlements, rollups, guild storage audit: 180 days rolling; auction listings in `SOLD`, `RECLAIMED` or `MOVED_TO_CLAIM`: 180 days after `ended_at`, except a `SOLD` listing whose `auction_proceeds` row is still `PENDING` (purged together with those proceeds once they are `CLAIMED` and 180 days old); `CLAIMED` proceeds: 180 days; `ACTIVE`/`CANCELLED`/`EXPIRED` listings (asset still in escrow) and `PENDING` proceeds are never purged (ADR-0070) | characters anonymized (`Anonymized_` + 32-hex UUID); `friends` / `friend_requests` / `blocks` rows naming its characters deleted; account columns re-pointed to the tombstone; account rollups deleted |
| **H** Audit | enforcement, admin, security, refund audit | `audit_events.*` | **3 years** from `occurred_at` | kept unchanged (legal/enforcement evidence); `subject_account_id` has no FK |

A purge job per row family runs daily and deletes rows past their period in bounded batches. Backups follow `../08_scale_ops/backup_recovery.md` (restore points ≤ 6 months; erasure ledger replay).

## 2. Data-Subject Requests
- **Erasure:** procedure response within **2 business days**; execution within **15 calendar days** of a valid request. In-app deletion enters `PENDING_DELETION` (7-day cancel window, all sessions revoked at request), then the erasure transaction runs. The DPO file records the request ID, operation ID and execution time for regulator inspection (A05, Bộ Công an).
- **Access / portability:** JSON export of Categories A, B, D (login history summary), E (purchase history) and F; characters/progression as a courtesy game summary.

## Invariants
```text
this file = the only retention schedule; data_protection.md references it
erasure transaction = data_model.md § Account Erasure (one transaction, deferred composite FKs)
Category E kept 10 years with account link severed at erasure
characters permanent; anonymized name = 'Anonymized_' + 32-hex character UUID; `anonymized_` name_key prefix reserved
Categories A credentials, B, C, D (except rate-limit counters), F deleted at erasure
Category H kept 3 years unchanged
erasure execution <= 15 calendar days; procedure response <= 2 business days
```
