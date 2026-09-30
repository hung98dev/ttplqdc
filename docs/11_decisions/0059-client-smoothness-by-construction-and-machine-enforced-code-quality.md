# ADR-0059: Client Smoothness by Construction and Machine-Enforced Code Quality
status: ACCEPTED

## Context
ADR-0058 removed every GPU from CI, so desktop GPU frame pacing is never measured. The owner requires smoothness to be controlled in code and code to be professional, consistent and efficient. Rules that only live in prose are not enforced for AI implementers; each rule below is either a runtime architecture contract with named tests or a deterministic CI check.

## Decision
1. **One frame driver.** A single `FrameLoop` MonoBehaviour (`client/Assets/Scripts/Core/Runtime/`, IMP-065; instantiated once by the `ThinhThan.App` composition root, IMP-067, or the PlayMode harness) runs ordered phases every rendered frame: `Input → NetReceive → Prediction → Interpolation → Presentation → UI → Camera`. First-party runtime code defines no `Update/FixedUpdate/LateUpdate/OnGUI` outside `FrameLoop` (Q4 fence, allowlisted exceptions only). Replicated entity views are updated from contiguous index-based arrays.
2. **Timing.** Presentation reads one `FrameTime` per frame (unscaled delta clamped to 100 ms; snapshot timeline on `Time.realtimeSinceStartupAsDouble`). The camera follows the predicted local player once per frame in the Camera phase with critically damped smoothing (time constant 0.12 s). No pixel-perfect camera (ADR-0055 painted art).
3. **Main-thread budget.** Non-urgent main-thread work runs through the `FrameBudget` scheduler: ≤ 2 ms per gameplay frame, ≤ 12 ms per loading-screen frame. Synchronous asset loads exist only on loading screens.
4. **No first-use hitch.** Map load pre-sizes pools from content counts, warms shader variants and loads the map's Addressables group before the loading screen closes.
5. **Adaptive quality governor.** `FrameTimingManager` p95 over 120 frames drives render scale (step 0.05, floor 0.6) and then particle budget down/up with hysteresis; presentation only.
6. **GPU-load proxies in CI.** Overdraw (fragments per pixel, llvmpipe, LOW) average ≤ 2.5 and 99th percentile ≤ 8; full-screen pass count ≤ preset budget; plus the existing batch/SetPass/light/particle budgets. With the governor and Android device runs these replace desktop GPU timing.
7. **Network receive path.** The Unity client uses the BCL `System.Net.WebSockets.ClientWebSocket` (no package). Receive and protobuf decode run on one background task into pooled buffers and a bounded queue; the main thread applies messages in `NetReceive`. `Task` is allowed only in `ThinhThan.Net`; everything else uses Unity `Awaitable`. Decode allocation is budgeted (`PERF-024`); main-thread frame code stays at 0 bytes (`PERF-004`).
8. **Timing gates on hosted runners** use the median of 3 repetitions inside one job; allocation, counter and overdraw gates are exact.
9. **Machine-enforced code quality** (`../10_implementation/engineering_conventions.md` § Requirement IDs):
   - C#: `client/Assets/csc.rsp` = `-warnaserror+` + `-nullable:enable`; the generated C# header is `#nullable disable` + protobuf pragmas.
   - Formatting: root `.editorconfig` and `.gitattributes` (`* text=auto eol=lf`), plus a deterministic C# style check in the Go verifier (no .NET SDK, no Roslyn tooling).
   - Go: `gofmt`, `go vet`, and `staticcheck` 2026.2.1 (`honnef.co/go/tools v0.8.1`).
   - Client API fence and single-canonical-implementation checks in Q4 (IMP-083).
10. **Server hot paths.** Allocation budgets are exact `testing.AllocsPerRun` gates (`../08_scale_ops/capacity.md` § Hot-Path Allocation Budgets). Benchmarks run with fixed `-benchtime=Nx`; ns/op is reported only.

## Consequences
- The following specs change:
  - `../04_architecture/client_performance.md` (Smoothness by Construction, Measurement, Requirement IDs `PERF-014..024`),
  - `../10_implementation/engineering_conventions.md` (§1.1, §1.7, §2.3–§2.7, Requirement IDs `CODE-001..006`),
  - `../08_scale_ops/capacity.md` (Hot-Path Allocation Budgets, `HOT-001..003`),
  - `../00_context/technology_versions.md` (staticcheck pin, editor-bound uGUI/TextMeshPro row),
  - `../10_implementation/audit_gates.md` (requirement-coverage scope, Q1/Q3/Q4, bootstrap, protected paths),
  - `../10_implementation/architecture_conformance.md` (§3, §4),
  - `../10_implementation/definition_of_done.md` (Code Quality / Smoothness),
  - `../04_architecture/client.md` (Performance),
  - `../09_testing/test_and_release_evidence.md` (command matrix),
  - `../10_implementation/repository_layout.md`,
  - `../10_implementation/spec_traceability.md`,
  - `../10_implementation/task_queue.md` (IMP-000, IMP-018, IMP-061, IMP-063, IMP-065, IMP-066, IMP-067, IMP-079, IMP-081, IMP-083, IMP-095, IMP-099, IMP-101),
  - root `AGENTS.md`,
  - `.devin/**` (Unity/Go rules, reviewer checklist, client skills).
- Requirement coverage (Q0) also scans `../10_implementation/engineering_conventions.md`.
- Desktop GPU frame pacing is still not timed in CI; the mitigations are item 6, the governor and `PERF-003`.
- No new Unity package and no .NET SDK are added (the uGUI row records the editor-bound package the Canvas UI already uses); staticcheck is the only new tool.
