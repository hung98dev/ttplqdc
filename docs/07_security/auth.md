# Authentication
status: LOCKED

## Scope
Defines account identity, login credentials, access/refresh lifecycle, gameplay ticket issuance, linking boundaries, and revocation.

Decision: `../11_decisions/0009-account-session-credentials.md`.

## Canonical Identity
`account_id` is the persistent account identity.

Login provider identity is a credential/link to an account, not the account itself.

Characters, currencies, cosmetics, account storage, and other persistent ownership reference canonical account/character IDs rather than email, platform handle, or provider username.

## HTTPS Login Flow
1. Unity connects to authenticated HTTPS endpoint.
2. Client presents a configured login-provider credential.
3. Go backend/provider adapter verifies credential.
4. Backend resolves exactly one canonical `account_id`.
5. Backend issues access + rotating refresh credentials.
6. Client may request a short-lived gameplay connection ticket.
7. Gameplay ticket is consumed by the WSS handshake.

Realtime gameplay packets never carry a password or long-lived refresh credential.

## HTTPS Endpoints (ADR-0064)
Control plane only (never gameplay). Bodies are JSON (UTF-8); UUIDs are canonical lowercase strings and timestamps are int64 Unix milliseconds on this surface. `Bearer` = `Authorization: Bearer <access_token>`.
```text
endpoint                                   auth     request body                                      response
POST /api/v1/auth/login/{provider}         none     provider_token, device_id, client_build, platform  TokenResponse
     provider = apple | google | steam              (apple: identity token JWT + nonce (the raw nonce whose
                                                     SHA-256 the client put in the Apple request); google: OIDC
                                                     ID token; steam: GetAuthTicketForWebApi ticket, hex, created
                                                     with identity "thinhthan-login")
POST /api/v1/auth/password/register        none     username, password, email, device_id,             TokenResponse
                                                     client_build, platform
POST /api/v1/auth/password/login           none     username, password, device_id, client_build,       TokenResponse
                                                     platform
POST /api/v1/auth/refresh                  none     refresh_token, device_id                           TokenResponse
POST /api/v1/auth/logout                   Bearer   scope : SESSION | ALL                              204
POST /api/v1/auth/password/change          Bearer   current_password, new_password                     TokenResponse
POST /api/v1/auth/link/{provider}          Bearer   provider_token, current_password (password         providers
                                                     accounts only)
POST /api/v1/auth/unlink/{provider}        Bearer   current_password or provider_token of another      providers
                                                     linked provider (reauthentication)
GET  /api/v1/account                       Bearer   -                                                  account_id, providers,
                                                                                                       status, pending_deletion,
                                                                                                       deletion_scheduled_at
POST /api/v1/gameplay/ticket               Bearer   client_build, platform, protocol_major,            ticket, ticket_expires_at,
                                                     protocol_minor, content_revision                  wss_url, protocol_minor_min,
                                                                                                       client_build_min,
                                                                                                       content_revision
POST /api/v1/account/delete                Bearer   ../07_security/data_protection.md § Erasure        data_protection.md
POST /api/v1/account/delete/cancel         Bearer   -                                                  204
POST /api/v1/iap/verify, /iap/steam/init   Bearer   validation.md § IAP Receipt Verification           validation.md
```
`TokenResponse` = `account_id, access_token, access_expires_at, refresh_token, refresh_expires_at, is_new_account`. `providers` = list of `{provider_id, linked_at}`.

Rules:
- Federated login with an unknown `provider_id + provider_subject` creates a new account (federated sign-up); a known one logs into its account.
- `link` fails `PROVIDER_ALREADY_LINKED` when the subject belongs to another account; `unlink` fails `LAST_LOGIN_METHOD` when it would leave no usable login method; both fail `CREDENTIAL_CHANGE_LOCKED` while `accounts.credential_guard_until > now` unless `current_password` is supplied (`anti_cheat.md` § Account takeover).
- `refresh` rotates per § Refresh Rotation; a lost-response retry is served by the grace rule there; any other reuse of a rotated token returns `AUTH_INVALID` and revokes the family.
- `gameplay/ticket` returns `CLIENT_UPDATE_REQUIRED`, `CONTENT_INCOMPATIBLE`, `SERVER_DRAINING`, `ACCOUNT_BANNED`, `ACCOUNT_SUSPENDED`, or `SERVER_OVERLOADED` with `queue_position` and `retry_after_ms` (login queue, `session.md`; skipped when a character of the account is live or inside reconnect grace); a pending-deletion account still gets a ticket (attach then returns `ACCOUNT_PENDING_DELETION`).
- `account/delete/cancel` returns the account from `PENDING_DELETION` to `ACTIVE` and clears `deletion_scheduled_at`; 204 also when the account is not pending (idempotent); 409 `INVALID_STATE` once erasure has started. Login (any provider or password) never cancels a pending deletion by itself; the client shows only "cancel deletion" / "log out" while `pending_deletion = true` (`data_protection.md` § Erasure). Rate limit `account.delete_cancel` (`rate_limits.md`).
- Errors: HTTP 400 validation (`USERNAME_INVALID`, `EMAIL_INVALID`, `PASSWORD_INVALID`), 401 `AUTH_INVALID` / `AUTH_EXPIRED`, 403 `ACCOUNT_BANNED` / `ACCOUNT_SUSPENDED` / `CREDENTIAL_CHANGE_LOCKED`, 409 `USERNAME_TAKEN` / `EMAIL_TAKEN` / `PROVIDER_ALREADY_LINKED` / `LAST_LOGIN_METHOD`, 426 `CLIENT_UPDATE_REQUIRED` / `CONTENT_INCOMPATIBLE`, 429 `RATE_LIMITED`, 503 `SERVER_OVERLOADED` / `SERVER_DRAINING` / `TEMPORARY_DEPENDENCY_FAILURE`. Body: `error_code, retryability, retry_after_ms, queue_position, safe_message_key` (`../05_network/errors.md`).
- Character create/list/select are WSS messages 12..14 and 6 (`../05_network/messages.md`); the resume credential is presented only in `C2S_HELLO`.

### Device ID
`device_id` = a random UUID v4 the client generates on first launch and keeps in local app storage (never a hardware fingerprint). It is a risk signal only (`session.md` § Device Metadata): the server stores `SHA-256(ACCOUNT_SIGNAL_SALT || device_id)` and the salted /16 IPv4 or /48 IPv6 prefix hash in `account_login_history` (`../06_data/data_model.md`, 90-day retention; one row per successful login, refresh excluded).

## Credential Types
Launch defaults:
```text
access credential TTL     = 15 minutes
gameplay ticket TTL       = 60 seconds, single-purpose
resume credential TTL     = 10 minutes from issue; re-issued every 300 s by S2C_RESUME_CREDENTIAL (16) while the session is live
refresh credential TTL    = 30 days sliding from last refresh, rotating; family absolute cap 90 days from login
```

TTL values may be shortened by security config, but widening them requires security review.

## Opaque Tokens
Public bearer credentials are high-entropy opaque values.

Server stores only the representation required for verification/revocation; refresh/resume secrets are never stored in plaintext recoverable form.

Do not place gameplay authority claims such as currency, inventory, role, map, or character stats inside client-trusted token payloads.

## Refresh Rotation
On successful refresh:
- issue a new refresh credential,
- invalidate/rotate the previous credential,
- detect reuse of an already-rotated credential,
- revoke the affected session family on suspicious reuse.

Retry safety must distinguish a lost response from true token replay (ADR-0069):
```text
grace         a refresh presenting generation N-1 is accepted once when generation N was issued <= 60 s ago, N has never
              been presented, and device_id matches the family; the server marks N rotated, issues N+1, and returns it
reuse         any other presentation of a rotated generation = AUTH_INVALID and revokes the whole family
absolute cap  a family never refreshes past 90 days after its initial login; then AUTH_EXPIRED and a new login
```

## Login Providers
The account model supports verified external/OIDC/platform identities through explicit provider records.

A provider record is unique by:
```text
provider_id + provider_subject
```

Linking/unlinking requires reauthentication/verification and must never remove the account's last usable login method (there is no recovery path at launch).

## Password Provider (ADR-0051)
Provider id `password`. Enabled at launch beside `apple`, `google`, `steam`.

HTTPS endpoints (same TLS/HTTPS surface as federated login):
```text
POST /api/v1/auth/password/register   body: username, password, email (+ device_id, client_build, platform; § HTTPS Endpoints)
POST /api/v1/auth/password/login      body: username, password (+ device_id, client_build, platform)
```
Both return the same access + rotating refresh credentials as federated login (steps 5-7 of the HTTPS Login Flow).

Registration creates a new `accounts` row and one `account_password_credentials` row in one transaction. Adding a password to an existing federated account is not supported at launch; a password account may link federated providers under the normal linking rules.

Username:
```text
input trimmed of leading/trailing whitespace
length 4..20
charset after lowercasing: [a-z0-9_]
must start with a letter
username_key = ASCII lowercase(username)
UNIQUE(username_key)
reserved/profanity check per ../06_data/text.md on username_key
```

Email:
```text
required; trimmed; length <= 254
exactly one '@'; local part 1..64; domain has >= 1 '.', no leading/trailing '.' or '-' per label
any provider (not limited to Gmail)
email_key = ASCII lowercase(email); no dot/plus-alias folding
UNIQUE(email_key)
NOT verified; never a login identifier; not used for recovery
```

Password:
```text
length 8..128 Unicode code points after NFC (../06_data/text.md)
must not equal username_key or email_key case-insensitively
no composition rules; no silent truncation
```

Hashing:
```text
algorithm        = Argon2id (golang.org/x/crypto/argon2, pin in ../00_context/technology_versions.md)
params_version 1 = memory 19456 KiB, iterations 2, parallelism 1, salt 16 bytes crypto/rand, key 32 bytes
stored           = PHC string in password_hash + params_version
comparison       = constant time
rehash           = on successful login when params_version < current
```

Errors:
- login with unknown username or wrong password -> `AUTH_INVALID` (same shape and similar latency; hash a dummy value for unknown usernames),
- registration -> `USERNAME_INVALID`, `EMAIL_INVALID`, `PASSWORD_INVALID`, `USERNAME_TAKEN`, `EMAIL_TAKEN` (`../05_network/errors.md`),
- banned/suspended accounts -> `ACCOUNT_BANNED` / `ACCOUNT_SUSPENDED` after a correct password.

Recovery: none at launch. No password reset by email or support. An account with a linked federated provider can still log in through it. The client registration screen states that a forgotten password cannot be recovered unless a provider is linked.

Password change while logged in requires the current password and revokes all other refresh/session families of the account.

Rate limits: `../07_security/rate_limits.md` (Authentication).

## Operator (Admin/GM) Accounts
Operators are not player accounts and never share credentials with them.
```text
table              operators(operator_id, login_key, password_hash (Argon2id, same params), totp_secret_encrypted (AES-256-GCM, key from secret env OPERATOR_TOTP_KEY),
                             role, status, created_at, last_login_at)
login              password + TOTP (RFC 6238, 30 s step, 6 digits; implemented with Go crypto/hmac + crypto/sha1)
network            admin HTTPS API bound to the private operations network only, never the public listener
session            8 h absolute, 30 min idle; separate from player sessions
roles              SUPPORT     read-only lookup
                   MODERATOR   mute / suspend / BANNED status changes
                   ECONOMY     rollback or grant of items/currency
                   ADMIN       operator management, config
two-person rule    ECONOMY grants/rollbacks above 1,000,000 common or any item of tier >= T5 need a second operator's approval
audit              every call writes audit_events(actor = operator_id, reason, ticket_id, before/after, operation_id)
```
Setting `accounts.status = BANNED` or `SUSPENDED_*` is only possible through this API and revokes all player sessions.

## Revocation
Revocation scopes include:
- one access/session family,
- one device/session family,
- all sessions for account,
- one linked provider credential.

Security-sensitive account changes may revoke all refresh/gameplay/resume credentials.

Already-bound realtime sessions are notified/closed when their session authority is revoked.

## Storage
PostgreSQL may store:
- canonical accounts,
- provider links,
- hashed verifier material,
- session/refresh family metadata,
- revocation state,
- audit metadata.

Do not query PostgreSQL to revalidate the access token on every gameplay message; the accepted WSS session is bound to a current session epoch.

## Recovery
Account recovery is a privileged authentication flow and must not bypass normal ownership checks.

Recovery actions are audited and invalidate superseded credentials.

## Invariants
- canonical identity = account_id,
- provider handle/email is not persistent ownership identity,
- gameplay WSS never receives password/refresh credential,
- refresh credentials rotate,
- gameplay ticket is short-lived and single-purpose,
- credential revocation can terminate active authority,
- password plaintext storage is forbidden,
- username_key and email_key are each unique across accounts,
- email is unverified and never used for login or recovery.
