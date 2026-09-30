# ADR-0045: CI Evidence Without Self-Referential Commit SHA
status: ACCEPTED

> **AMENDMENT NOTICE (ADR-0057)**: Identity is `source_tree_hash` only (algorithm and exclusions in ADR-0057); `tested_commit_sha` is retired because squash merge removes branch commits. Manifests are produced by CI as artifact `evidence` and committed unchanged; new fields `run_attempt`, `worktree_clean`; `content_revision` may be `none` before a content compiler exists. The `evidence/M0` note below is obsolete; milestone evidence uses `evidence/M<n>/`.

## Context

The previous evidence manifest required `git_commit_sha` of the commit that contained the manifest. That SHA does not exist until the manifest is committed, so agents invented `null` SHAs or dirty-tree manifests and still claimed Q0–Q6 passed.

## Decision

1. CI is the enforcement source for release/task evidence.
2. A repo-side `docs/10_implementation/evidence/<ID>/manifest.json` is an index to an immutable CI result. It must not require the SHA of the commit that adds the index.
3. Identity of what was tested is exactly one of:
   - `tested_commit_sha`: git object already existing before the evidence index commit,
   - `source_tree_hash`: SHA-256 over source paths excluding `docs/10_implementation/evidence/`.
4. `result` is only `PASSED` or `FAILED`. `IN_PROGRESS` is not evidence.
5. Required fields:

```text
schema_version
task_id
tested_commit_sha | source_tree_hash
toolchain (go, unity, postgresql, protoc, protoc_gen_go)
commands
test_summary.total / passed / failed / skipped
skipped_reasons (id + reason + allowed)
content_revision
ci_run_id (empty string only when CI did not run; then DONE is invalid)
result = PASSED
```

6. `DONE` is valid only if this schema validates and the hash matches the tested source.

## Consequences

- `docs/09_testing/test_and_release_evidence.md` — schema owner.
- `docs/10_implementation/definition_of_done.md` — DONE requires ADR-0045 evidence.
- `docs/10_implementation/task_queue.md` — Q0/Q6 validator.
- `docs/10_implementation/audit_gates.md` — Gate D.
- Invalid `docs/10_implementation/evidence/M0/manifest.json` is not evidence; delete or replace after CI.
