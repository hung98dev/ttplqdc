---
name: fix-unity-bug
description: Debug and fix a Unity client bug — reproduce, trace lifecycle/serialized state, root cause, fix, EditMode/PlayMode verify.
---

# Fix Unity Bug

## Workflow

1. **Reproduce.** Exact repro steps + observed vs expected. Check Unity logs (`client/Logs/` if present — read-only, never edit) and console output.
2. **Classify.** Pure logic bug → EditMode repro. Lifecycle/scene/serialization bug → PlayMode repro. Networking desync → use `network-debugging` instead.
3. **Trace.** Walk the Unity lifecycle (`Awake/OnEnable/Start/OnDisable`) and the `FrameLoop` phase order (`Input -> NetReceive -> Prediction -> Interpolation -> Presentation -> UI -> Camera`) and the layering `Input/UI -> intent -> session -> replicated state -> presentation`. Check serialized references on involved prefabs/scenes — a null/missing reference is a common root cause.
4. **Root cause.** One sentence + file:line. Check whether the bug is really server-side authority vs. client prediction divergence — if the server is wrong, fix the server, not the client.
5. **Fix.** Minimal, in the owning asmdef. Preserve serialized data (`[FormerlySerializedAs]` on renames).
6. **Verify.** Repro test green; `bash .devin/scripts/verify_delta.sh`; review serialized diffs for unrelated churn.

## Acceptance

- Repro confirmed fixed via test; no lifecycle leaks (subscriptions unpaired, handles unreleased).
- No out-of-scope scene/prefab modifications; no client-authoritative logic introduced.
