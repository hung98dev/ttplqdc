# ADR-0051: First-Party Username/Password Login
status: ACCEPTED

## Context
`../07_security/external_integrations.md` §1.2 disabled first-party password login at launch; only Apple, Google and Steam federated login existed. Owner decision (2026-09-24): players may also register and log in with a username and password. Registration requires an email address, which is not verified.

## Decision
- A new provider `password` is added beside `apple`, `google`, `steam`. It creates a new canonical `account_id` at registration (ADR-0009 unchanged: provider = credential, not identity).
- Registration input: `username`, `password`, `email`. Login input: `username`, `password`. Email is never a login identifier.
- Email is required, any syntactically valid address (not limited to Gmail), **not verified**, and unique per account (case-insensitive key).
- Username is unique (case-insensitive key). Rules are canonical in `../07_security/auth.md`.
- Passwords are hashed with Argon2id from `golang.org/x/crypto/argon2` with versioned parameters and rehash-on-login. Pinned as `golang.org/x/crypto v0.57.0` in `../00_context/technology_versions.md`.
- There is no password reset or recovery at launch (no email link, no support reset). An account that also links a federated provider can still log in through that provider.
- Login failures return one generic error; registration may report `USERNAME_TAKEN` / `EMAIL_TAKEN` and relies on rate limits against enumeration.

## Consequences
- `../07_security/auth.md` defines registration/login rules, username/email/password validation and Argon2id parameters.
- `../07_security/external_integrations.md` §1.2 enables the `password` provider.
- `../06_data/data_model.md` and `../06_data/physical_schema_contract.md` add `account_password_credentials`.
- `../05_network/errors.md` adds registration error codes.
- `../07_security/rate_limits.md` adds registration/login limit classes.
- `../07_security/data_protection.md` and `personal_data_register.md` record username/email/password hash as Category A and delete them on erasure.
- `../00_context/technology_versions.md` pins `golang.org/x/crypto v0.57.0`.
- `../10_implementation/task_queue.md` IMP-006 implements and tests the provider.
- A forgotten password with no linked provider means the account cannot be recovered; the client must state this at registration.
