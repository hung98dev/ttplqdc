# TASK: <Name>
id: IMP-XXX
status: NOT_STARTED
claimed_by: ""
branch: ""
claimed_at: ""
blocked_by: ""

specs: []
adrs: []
depends_on: []
owned_paths: []
forbidden_paths: []
contract_inputs: []
contract_outputs: []
consumers_checked: []

## Change
- <what to build>

## Acceptance
- <objectively checkable result; each item has a test below>

## Tests
- `<path inside owned_paths>`: <TestName>, ...

generated_artifacts: []
cleanup_obligations: []
evidence_location: "docs/10_implementation/evidence/IMP-XXX/"

Claim fields are empty until the coordinator claims the task; `blocked_by` names a `BLK-xxx`, `OPS-xxx` or `REVERT-<sha>` when `status: BLOCKED`. `DONE` is defined only by `../10_implementation/definition_of_done.md`.
