---
description: Wire contract rules for proto/thinhthan/v1 — immutability, codegen, compatibility
trigger: glob
globs:
  - "proto/**/*.proto"
  - "proto/testdata/**"
---

# Protobuf Wire Contract

Canonical: `docs/05_network/protobuf_conventions.md`, `versioning.md`, `messages.md`. `proto/` is the single wire source of truth for both Go server and C# client.

## File rules

- `syntax = "proto3"`, `package thinhthan.v1`, 2-space indent.
- Required options: `go_package = "thinhthan/internal/protocol/v1;protocolv1"`, `csharp_namespace = "ThinhThan.Protocol.V1"`.
- Nine baseline files own fixed message-ID ranges (`common`, `session` 1–11, `movement` 100–108, `combat` 200–207/304, `durable` 400–409, `content` 500–506, `social` 600–636, `market` 700–736, `pvp` 800–810). New messages go in the owning file's range.

## Immutability (wire compatibility)

- Field numbers are immutable once committed. Never renumber, retype, or reuse a field number or enum value — including deprecated/reserved ones.
- Retire fields via `reserved N;` / `reserved "name";` or `deprecated_` prefix. Never delete-and-reuse.
- Enums: first value `*_UNSPECIFIED = 0`; `SCREAMING_SNAKE_CASE` values prefixed with the enum name.
- proto3 implicit presence by default; use `optional` only when "zero" vs "absent" must differ.

## Evolution policy

- Additive optional fields / new unused message IDs / new feature flags = `protocol_minor`.
- Changed semantics, required-field reinterpretation, message-ID meaning reuse = `protocol_major`.
- Older clients must degrade safely (unknown fields ignored; absent secondary combat fields render nothing, not zero).

## After any proto change

1. Run `pwsh -NoProfile -File scripts/codegen.ps1` (Linux or Windows, ADR-0058); it bootstraps pinned `protoc 36.2` + `protoc-gen-go v1.36.12` under ignored `tools/`.
2. If an affected fixture shape changed, regenerate with `cd server && go test ./internal/testing/protocol -run TestBinaryEncodingParity -update-golden`.
3. Run `bash .devin/scripts/verify_delta.sh --full`: canonical Q2 must report zero drift for `server/internal/protocol/v1/`, C# output, asmdef, and deterministic Unity metadata; protocol registry/golden tests must pass.
4. Check both consumers: `server/internal/` and `client/Assets/Scripts/`; Unity EditMode parity covers C# decode/re-encode.
5. If message IDs/fields require protected `docs/05_network/` changes, record the gap in `docs/10_implementation/known_blockers.md`, mark the task BLOCKED, and hand it to the `spec-owner` agent (spec-change PR + `policy-review`); never edit protected specs as an implementer. The registry in `docs/05_network/messages.md` is complete for launch (ADR-0054).

Never hand-edit `server/internal/protocol/**`, `client/Assets/Scripts/Protocol/**`, their generated parent `.meta` files, or golden binaries. Never add gRPC. `C2S_AUCTION_BID` does not exist — auction is FIXED_PRICE buy only.
