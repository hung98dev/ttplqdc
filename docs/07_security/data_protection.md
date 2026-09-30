# Data Protection
status: LOCKED

## Scope and Legal Framework
Defines legal framework, category definitions and data-subject-request handling for personal data held by this system. `personal_data_register.md` § 1 is the sole retention/at-erasure schedule; this document owns the export projection, not a second schedule.

Primary legal framework:
1. **Luật Bảo vệ dữ liệu cá nhân số 91/2025/QH15** (ban hành ngày 26/06/2025, có hiệu lực từ ngày 01/01/2026): đạo luật nền tảng quy định quyền của chủ thể dữ liệu, nghĩa vụ của bên kiểm soát/xử lý dữ liệu cá nhân và bảo vệ dữ liệu xuyên biên giới.
2. **Nghị định 356/2025/NĐ-CP**: launch implementing framework for Luật 91/2025/QH15, including the data-subject-request procedure referenced below; earlier Nghị định 13/2023/NĐ-CP is not a competing launch retention schedule.
3. **Luật Kế toán số 88/2015/QH13** (Điều 41): quy định thời hạn lưu trữ tài liệu kế toán, áp dụng cho chứng từ thanh toán và biên lai IAP.

This document does not duplicate authentication or payment-provider security controls (see `07_security/validation.md` and `03_systems/monetization.md`); it governs what data is held, for how long, and what happens when a player requests deletion.

## Authority
The **Privacy/DPO Owner** (a named role in the operations team) keeps this document current, authorises new data categories before collection begins, manages mandatory compliance dossiers, and processes data-subject requests. Launch retention and transfer numbers below are the implementation contract. A later legal amendment requires an ADR.
## Persistence
All personal data is held in server-authoritative storage. Client caches may hold session-scoped subsets; they are not considered durable stores for retention purposes.

## Data Inventory

| Category | Description | Examples | Basis for Collection |
|---|---|---|---|
| A — Account / Operator Credentials | Player identity and authentication; operator authentication secrets | `account_password_credentials.*`, `accounts.*`; `operators.password_hash`, `.totp_secret_encrypted` | Contract (account/operator access); security purpose; credentials never exported |
| B — Platform Provider Links | Provider-issued stable subjects; verification tokens are transient, not a new durable token store | `account_identities.provider_id`, `.provider_subject`, `.linked_at` (Apple, Google, Steam) | Contract (platform authentication) |
| C — Session & Device Metadata | Session families, refresh credential hashes, platform, app version, device model class | `auth_session_families.*`, `auth_refresh_credentials.*` | Contract (login security) |
| D — IP / Security Signals | Revocations, salted device and IP-prefix hashes of logins, rate-limit keys | `auth_revocations.*`, `account_login_history.*`, `rate_limit_counters.*`, `auth_failure_backoff.*` | Legitimate interest (security) |
| E — Payment Receipt Tokens | Opaque platform transaction tokens; not raw card data (card data never held by this server) | `entitlement.platform_receipt` | Contract + Legal obligation (financial records) |
| F — Chat Logs | Text content of WORLD, PARTY, GUILD, and direct messages sent in-game | `chat_messages.*` | Legitimate interest (moderation / safety) |
| G — Gameplay Records & Event Logs | Account-linked characters/progression and event/settlement records; the erasure lifecycle removes ordinary account/name attribution | `characters.*`, settlements, AH listings | Contract / legitimate interest (economy integrity); identifiable state is personal data, not excluded by calling it game state |
| H — Support / Audit / Erasure Compliance | Reports, audit evidence, operator identity/accountability and pseudonymous erasure/recovery metadata | `audit_events.*`, `player_reports.*`, non-credential `operators` fields, `erasure_intents.*`, immutable external erasure-ledger metadata | Moderation/security/accountability legitimate interest; erasure-compliance legal obligation; case-specific enforcement legal obligation only if documented, never a blanket exemption |

This table is canonical for category letters; `personal_data_register.md` maps them to exact columns and owns retention periods.

## Retention Periods
Retention anchors, periods, cleanup gates and per-field erasure actions are canonical only in `personal_data_register.md` § 1 (ADR-0065), including reports, disabled operators and pending/completed erasure metadata. Consumers must follow that schedule rather than applying one generic deadline to Category H. `../08_scale_ops/backup_recovery.md` owns restore-point limits, object representation and recovery mechanics, not an alternative personal-data schedule.

Data that has passed its retention period is deleted or irreversibly anonymised on a scheduled basis. Anonymisation must be genuine (no re-identification path); pseudonymisation does not satisfy a deletion obligation.

## Deletion and Data-Subject Requests

### Account Closure (Player-Initiated, In-App)
Account deletion can be started in the game client (required by Apple App Store rules) and via support:
```text
POST /api/v1/account/delete   (authenticated; requires re-authentication in the last 5 minutes:
                               password for provider `password`, fresh provider token otherwise)
```
1. The account enters `PENDING_DELETION` for a **7-day cancel window**; only `POST /api/v1/account/delete/cancel` (after login) cancels it; a login alone never cancels (ADR-0069). All session families are revoked at request time.
2. After the window the erasure transaction in `../06_data/data_model.md` § Account Erasure executes (always within 15 calendar days of the request); per-category actions are in `personal_data_register.md` § 1.
3. Closure and legal erasure are the same flow; there is no separate slower closure path.

### Erasure Request Under Law 91/2025/QH15 and Decree 356/2025/ND-CP
A data subject may request erasure of their personal data (Luật 91/2025/QH15 và Nghị định 356/2025/NĐ-CP có hiệu lực từ 01/01/2026). The following constraints apply:
**Permanent characters are a product invariant**, not a legal exemption (see `../00_context/non_goals.md`). An erasure request triggers the lifecycle below; the DPO documents any restricted retained personal data and its specific purpose/basis:

- Player identity credentials (A) and provider links (B) are deleted as described in the owning lifecycle and register. Operator credentials belong to a different subject and are never processed through a player's account erasure.
- Character records retain economy/progression integrity using `../06_data/data_model.md` § Account Erasure: reassign the account link to `TOMBSTONE_ACCOUNT_ID` and rename to `Anonymized_` + 32-hex UUID, releasing the original name. This removes ordinary player attribution; retained H account/character identifiers or notes can still allow re-identification by authorized personnel. Hashing, renaming and severing one FK do not certify global anonymization while such a path exists.
- Chat logs (Category F) authored by the requesting account are deleted within 15 calendar days of a valid erasure request (legal deadline per Decree 356/2025/NĐ-CP; scheduled purge must not exceed this deadline).
- Local journal/queue/in-flight references containing the subject's original F/H/G text must be terminally reconciled and durably disposed before erasure completion; receipt outcome scrubbing follows disposal under the locked admission fence (`../06_data/data_model.md` § Account Erasure, PRIV-008/PRIV-009). Restricted retained receipt keys are not anonymization.
- Payment receipt tokens (Category E) are retained for the financial record period with the account reference severed.
- The Privacy/DPO Owner must respond to procedure within **2 business days** and complete valid erasure execution within **15 calendar days** per Decree 356/2025/NĐ-CP requirements, and must document the handling decision and its legal basis.
- The DPO informs the subject of any retained report/audit/financial/recovery data, its processing basis and retention anchor/end or completion-dependent gate in the register. Retention does not postpone credential/chat erasure or grant investigative disclosure; an undocumented "legal hold" cannot extend the canonical schedule.

### Access and Portability Requests
The following is the **single canonical player-export definition**. All schemas, request endpoints, DPO procedures, runbooks and tests reference this section, not their own category/field copies. The Privacy/DPO Owner responds within 2 business days and fulfils a valid request within 15 calendar days per the launch request procedure.

Resolve the requesting subject from authenticated `account_id`, or verified DPO support intake when ordinary login is unavailable. Never trust a supplied reporter/operator ID or join by username, target character, operator, salted hash or `TOMBSTONE_ACCOUNT_ID`. A retained erased UUID does not authenticate a requester; DPO verification must independently establish that exact subject. Select rows owned by that subject and then construct the allowlisted projection, never serialize a table row followed by removing known secrets. Omitted rows/fields are not replaced with other subjects' data.

| Category / JSON section | Owned source rows | Exact exported fields |
|---|---|---|
| A / `account` | `accounts.account_id = subject`; own password-provider row if present | `account_id`, `status`, `created_at`, `deletion_requested_at`, `erased_at`; optional own `username_key`, `email` |
| B / `provider_links` | `account_identities.account_id = subject` | `provider_id`, `provider_subject`, `linked_at` |
| D / `login_history` | `account_login_history.account_id = subject` | `observed_at`, `is_new_origin` only; no device/IP hash or security-review signals |
| E / `purchases` | `account_iap_entitlements.account_id = subject`, never shared tombstone rows | `entitlement_id`, `product_id`, `entitlement_type`, `platform`, `grant_state`, `created_at`, `granted_at`, `ended_at`, `season_number`, `claim_deadline_at`; no receipt token/provider notification payload |
| F / `sent_chat` | `chat_messages.sender_account_id = subject` | `message_id`, `channel`, `content`, `created_at`; no recipient/group scope or third-party sender identity |
| H / `submitted_reports` | **only** `player_reports.reporter_account_id = subject` | `report_id`, `created_at`, `reason`, `reporter_notes`, `status`, `resolved_at`; these are own submission notes and coarse case state, not a sanction/result disclosure |

Every listed field is sourced from its named existing column; nullable fields serialize JSON `null`. Optional absent password identity fields are omitted; array sections with no eligible retained rows are `[]`. Arrays sort by their source timestamp ascending, then stable source key (`provider_id` for provider links; `observed_at` for login history; UUID for purchases/chat/reports). No retained secrets, operator identities, audit payloads or recovery metadata are exported. C is not an export section. H is only the reporter-own projection: omit `operation_id`, `reporter_account_id`, `reporter_character_id`, `target_character_id`, `chat_message_id`, `handled_by_operator_id`, `resolution_code` and any investigative notes/joins. Being a reported target, audit subject or operator does not authorize a player report/audit-wide export.

Before release the DPO checks submitted notes and sent-chat free text for third-party personal data and redacts only that content, preserving the requesting subject's own non-disclosing text; a withheld/redacted portion and its basis are explained in the response without identifying another subject. Reporter notes remain bounded by the existing 200-grapheme submission rule. `status`/`resolved_at` describe only OPEN/RESOLVED/DISMISSED handling of the own submission; no resolution/sanction reason, target action, investigator attribution or evidence content is inferred or joined. New schema fields are excluded until this owning projection is explicitly amended.

Characters/inventory/progression may be an optional courtesy game summary, separate from these required sections and constrained to the verified subject's owned state with third-party attribution removed. Identifiable gameplay state is personal data; a courtesy label is not a legal assertion that it is exempt.

### Operator-Own Data-Subject Requests

Operators are separate data subjects, not player accounts (`auth.md` § Operator). Use the existing Privacy/DPO intake/runbook, verifying the person's operator identity independently of any player login; **no new public/operator HTTP route is introduced**. A valid access request returns only that operator's `operator_id`, `login_key`, `role`, `status`, `created_at`, `last_login_at`, `disabled_at`, never `password_hash`, `totp_secret_encrypted`, decrypted TOTP, sessions or audit/report rows. The same response/fulfilment procedure applies; disabled operators use DPO identity verification, not wiped credentials. An operator-own erasure/closure request disables access and wipes credentials immediately in the disable transaction; the restricted identity/audit-continuity remainder follows the single register schedule, with purpose/basis and retention communicated to the subject. No player request can disable an operator or erase another subject's evidence.

## Cross-Border Transfer and Regulatory Dossiers
Launch durable personal data (Categories A–H) is stored in Vietnam. The only extra-territorial processors authorised at launch are Apple (Sign in with Apple token verification, App Store receipts), Google (OIDC token verification, Google Play receipts) and Valve (Steam `AuthenticateUserTicket` and Steam Microtransactions order verification); each receives only the opaque token/receipt it issued, never Categories A, C–H or card data. Asset CDN/Addressables do not carry Categories A–H.

### Mandatory Regulatory Dossiers
1. **Hồ sơ đánh giá tác động xử lý dữ liệu cá nhân (DPIA)**: Lập theo Luật 91/2025/QH15 và Nghị định 356/2025/NĐ-CP; lưu trữ tại doanh nghiệp và gửi 01 bản chính về Cục An ninh mạng và phòng, chống tội phạm sử dụng công nghệ cao (A05) - Bộ Công an trong thời hạn 60 ngày kể từ ngày tiến hành xử lý dữ liệu.
2. **Hồ sơ đánh giá tác động chuyển dữ liệu cá nhân ra nước ngoài**: Lập theo Luật 91/2025/QH15 và Nghị định 356/2025/NĐ-CP cho luồng xác thực biên lai IAP với Apple, Google và Valve (Steam); gửi 01 bản chính về Bộ Công an (A05) trong 60 ngày kể từ ngày chuyển dữ liệu.

No other cross-border personal-data transfer is authorised at launch.
## Process Ownership
| Activity | Owner |
|---|---|
| Data inventory maintenance | Privacy/DPO Owner |
| Retention schedule enforcement (automated deletion jobs) | Platform/Infrastructure Lead |
| Data-subject request intake and response | Privacy/DPO Owner |
| New data category approval | Privacy/DPO Owner (sign-off before engineering ships collection) |
| Annual review of this document | Privacy/DPO Owner |

## Requirement IDs

Every ID is named in consuming task Acceptance and covered by named behavioral tests; canonical projections remain in the sections above, not in task-local copies.

| ID | Requirement (section) | Gate |
|---|---|---|
| `PRIV-001` | Reporter-own H export selects exact verified reporter ownership and returns only the bounded submission projection; target/third-party/investigator/operator/resolution disclosure is excluded (Access and Portability Requests) | IMP-056, IMP-103 |
| `PRIV-002` | Player export constructs only the canonical allowlist; no credential/TOTP/hash/receipt/audit/recovery leak, shared-tombstone ownership or third-party free-text disclosure (Access and Portability Requests) | IMP-056, IMP-103 |
| `PRIV-003` | Operator-own access uses verified DPO intake separate from player identity and returns only its canonical own identity projection, never credentials or evidence about other subjects (Operator-Own Data-Subject Requests) | IMP-077, IMP-103 |

## Invariants
```text
legal framework = Luat 91/2025/QH15 + Nghi dinh 356/2025/ND-CP + Luat Ke toan 88/2015/QH13
retention periods, anchors and erasure-metadata gates = personal_data_register.md § 1 (single schedule)
account closure revokes sessions immediately; credential destruction follows the canonical erasure lifecycle
erasure request execution deadline = 15 calendar days (Luat 91/2025/QH15 & Decree 356/2025/ND-CP; response within 2 business days)
permanent character state removes ordinary player attribution; retained re-identification paths remain restricted personal/pseudonymous processing
payment account links severed per the register and owning erasure transaction
new personal data category requires Privacy/DPO Owner approval before collection
cross-border transfer of Vietnamese personal data requires documented legal basis
subject export = Access and Portability Requests projection only; no credentials/TOTP or audit-wide export
operator-own requests use verified DPO intake, separate from player accounts
launch personal-data region = Vietnam
launch extra-territorial processors = Apple, Google and Valve (Steam) login and purchase verification only
mandatory dossiers = DPIA and Cross-Border Transfer dossiers filed to A05 BCA within 60 days
```

## Legal Sign-Off and Effective Date
| Attribute | Value |
|---|---|
| Approving Authority | Ban Pháp chế & DPO (Legal / Data Protection Officer) |
| Status | Phê duyệt chính thức cho mốc Launch |
| Effective Date | 2026-01-01 (đồng bộ ngày có hiệu lực của Luật 91/2025/QH15) |
| Review Cadence | Định kỳ hàng năm |
