---
name: implement-unity-feature
description: Implement a Unity/C# client feature — asmdef ownership, authority boundary, lifecycle, tests. Use for client/ work.
---

# Implement Unity Feature

## Workflow

1. **Spec.** Read `docs/04_architecture/client.md` + the owning spec. Confirm what is presentation vs. what is server authority — the client never decides gameplay results.
2. **Locate.** Find the right asmdef (`ThinhThan.Core|Net|Systems|UI`) per `repository_layout.md`. Inspect related scenes/prefabs/scripts before editing.
3. **Contract check.** If the feature needs a new/changed message, switch to `client-server-feature`. Generated Protocol C#/asmdef/meta outputs are never hand-edited.
4. **Implement.** Follow `.devin/rules/11-unity-client.md`. C# 9.0 Allman, warnings as errors, nullable on; logic as `IFrameSystem` in its `FrameLoop` phase with 0-alloc frame code, `FrameBudget` for non-urgent work, `Pool<T>` for transient visuals and no fenced API (`engineering_conventions.md` §2.3–§2.7), Input System for input, UI through the state machine — never direct socket from views.
5. **Serialized data.** Renamed `[SerializeField]` → `[FormerlySerializedAs]`. Keep scene/prefab diffs minimal and in-scope.
6. **Tests.** EditMode for pure logic/data; PlayMode for lifecycle/session flow. Deterministic — no wall-clock dependence.
7. **Verify.** Run `verify_delta.sh --full`; it resolves Unity Editor `6000.6.1f1` from `UNITY_EDITOR_PATH` or the pinned Hub path and runs EditMode (plus PlayMode for scene/prefab changes). Review serialized YAML.

## Acceptance

- Compiles under C# 9.0 / .NET Standard 2.1; asmdef references still acyclic.
- No client-authoritative logic introduced; prediction reconciles to server state.
- No out-of-scope scene/prefab/asset modifications.
