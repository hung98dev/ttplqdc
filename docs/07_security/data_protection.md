# Data Protection
status: LOCKED

## Scope and Legal Framework
Defines the data inventory, retention periods, deletion and data-subject-request handling, and process ownership for all personal data held by this system.

Primary legal framework:
1. **Luật Bảo vệ dữ liệu cá nhân số 91/2025/QH15** (ban hành ngày 26/06/2025, có hiệu lực từ ngày 01/01/2026): đạo luật nền tảng quy định quyền của chủ thể dữ liệu, nghĩa vụ của bên kiểm soát/xử lý dữ liệu cá nhân và bảo vệ dữ liệu xuyên biên giới.
2. **Nghị định 13/2023/NĐ-CP** về bảo vệ dữ liệu cá nhân: quy định chi tiết các biện pháp kỹ thuật, hồ sơ đánh giá tác động và thủ tục hành chính liên quan.
3. **Luật Kế toán số 88/2015/QH13** (Điều 41): quy định thời hạn lưu trữ tài liệu kế toán, áp dụng cho chứng từ thanh toán và biên lai IAP.

This document does not duplicate authentication or payment-provider security controls (see `07_security/validation.md` and `03_systems/monetization.md`); it governs what data is held, for how long, and what happens when a player requests deletion.

## Authority
The **Privacy/DPO Owner** (a named role in the operations team) keeps this document current, authorises new data categories before collection begins, manages mandatory compliance dossiers, and processes data-subject requests. Launch retention and transfer numbers below are the implementation contract. A later legal amendment requires an ADR.
## Persistence
All personal data is held in server-authoritative storage. Client caches may hold session-scoped subsets; they are not considered durable stores for retention purposes.

## Data Inventory

| Category | Description | Examples | Basis for Collection |
|---|---|---|---|
| A — Account Identity | Username, email address (unverified), password hash, account creation timestamp | `account_password_credentials.username_key`, `.email`, `.password_hash`, `accounts.created_at` | Contract (account creation) |
| B — Platform Provider Links | OAuth tokens, platform user IDs (App Store, Google Play, regional gateway) | Sign in with Apple token, Google account token | Contract (platform authentication) |
| C — Session & Device Metadata | Session families, refresh credential hashes, platform, app version, device model class | `auth_session_families.*`, `auth_refresh_credentials.*` | Contract (login security) |
| D — IP / Security Signals | Revocations, salted device and IP-prefix hashes of logins, rate-limit keys | `auth_revocations.*`, `account_login_history.*`, `rate_limit_counters.*`, `auth_failure_backoff.*` | Legitimate interest (security) |
| E — Payment Receipt Tokens | Opaque platform transaction tokens; not raw card data (card data never held by this server) | `entitlement.platform_receipt` | Contract + Legal obligation (financial records) |
| F — Chat Logs | Text content of WORLD, PARTY, GUILD, and direct messages sent in-game | `chat_messages.*` | Legitimate interest (moderation / safety) |
| G — Gameplay Records & Event Logs | Characters/progression (anonymised on erasure, retained) and the event stream used for anti-cheat/economy analysis (180-day rolling) | `characters.*`, settlements, AH listings | Contract / legitimate interest (economy integrity) |
| H — Support / Audit Records | Enforcement actions, support tickets, anomaly signal records | `audit_events.*`, `ECONOMY_REVIEW` flags | Legal obligation (evidence for enforcement) |

This table is canonical for category letters; `personal_data_register.md` maps them to exact columns and owns retention periods.

## Retention Periods
Retention periods and per-category erasure actions are canonical in `personal_data_register.md` § 1 (ADR-0065). Summary: A credentials and B/C/D/F are deleted at erasure (rate-limit counters expire within 24 h and are not subject-searchable); E is kept 10 years with the account link severed at erasure; G characters are permanent and anonymized, G event/settlement records roll off after 180 days (value-holding rows never); H is kept 3 years.

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
**Characters are a permanent product non-goal** (see `non_goals.md`): character identity and progression records are permanent by design. An erasure request therefore triggers the following handling:

- Account identity (Category A) and provider links (Category B) are deleted or anonymised as described under Account Closure above.
- Character records (name, level, inventory, progression) are **retained in anonymised form** per the lifecycle in `../06_data/data_model.md`: `characters.account_id` is reassigned to `TOMBSTONE_ACCOUNT_ID` (`00000000-0000-0000-0000-000000000001`), the character's link to the data subject's account is permanently severed, and the character is renamed to `Anonymized_` + its 32-hex UUID (releasing the original `name_key`) while preserving in-world economy and trade history integrity.
- Chat logs (Category F) authored by the requesting account are deleted within 15 calendar days of a valid erasure request (legal deadline per Decree 356/2025/NĐ-CP; scheduled purge must not exceed this deadline).
- Payment receipt tokens (Category E) are retained for the financial record period with the account reference severed.
- The Privacy/DPO Owner must respond to procedure within **2 business days** and complete valid erasure execution within **15 calendar days** per Decree 356/2025/NĐ-CP requirements, and must document the handling decision and its legal basis.
- Where a legal basis prevents immediate erasure (e.g., ongoing enforcement action, financial records obligation), the player is informed of the basis and the expected retention end date.

### Access and Portability Requests
Players may request a copy of their personal data (Category A, B, D summary, E summary, F). The Privacy/DPO Owner must respond within 2 business days and fulfil the request within 15 calendar days per Decree 356/2025/NĐ-CP. Game-state data (characters, inventory, progression) is not personal data for portability purposes but may be provided as a courtesy export.

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

## Invariants
```text
legal framework = Luat 91/2025/QH15 + Nghi dinh 356/2025/ND-CP + Luat Ke toan 88/2015/QH13
payment receipt tokens (Category E) retained = 10 years from transaction date (Luat Ke toan 88/2015/QH13)
retention periods = personal_data_register.md § 1 (single schedule)
chat logs retention = 90 days rolling maximum
gameplay event logs retention = 180 days rolling maximum
account closure -> Category A anonymised in public contexts immediately
erasure request execution deadline = 15 calendar days (Luat 91/2025/QH15 & Decree 356/2025/ND-CP; response within 2 business days)
character records retained in anonymised form on erasure (permanent-character non-goal preserved)
account link severed from payment records at erasure (records kept 10 years under TOMBSTONE_ACCOUNT_ID)
new personal data category requires Privacy/DPO Owner approval before collection
cross-border transfer of Vietnamese personal data requires documented legal basis
Category A: credentials deleted at erasure; residual non-personal accounts row purged 1 year after erasure
Category H audit retention = 3 years from record creation
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
