# Client Performance & Smoothness
status: LOCKED

## Scope
Canonical player-facing performance and smoothness targets for the Unity client: device tiers, frame pacing, memory/GC, load times, input responsiveness, network smoothness, mobile thermal/battery, the runtime architecture that enforces them by construction, and how each is measured. Server-side targets stay in `../08_scale_ops/capacity.md`. Measurement uses Unity built-ins only (`FrameTimingManager`, `ProfilerRecorder`); no new package.

## Platforms and Device Tiers
Launch platforms: Windows desktop and Android. iOS is not a launch target (CI builds only Windows and Android players, ADR-0058).

| Tier | Reference hardware | Target |
|---|---|---|
| `DESKTOP_MIN` | 4-core x64 CPU, 8 GB RAM, DX11 GPU at Intel UHD 620 level, 1920x1080 | 60 FPS, quality `MEDIUM` |
| `ANDROID_MIN` | Android 10, 4 GB RAM, Snapdragon 665 / Helio G85 class (Adreno 610 / Mali-G52) | 30 FPS locked, quality `LOW` |
| `ANDROID_REC` | Android 12, 6 GB RAM, Snapdragon 778G class | 60 FPS, quality `MEDIUM` |

Quality presets `LOW / MEDIUM / HIGH` are chosen automatically at first launch from a 5-second GPU/CPU benchmark and can be changed in settings. They control only presentation: render scale (`LOW` 0.75, others 1.0), active point Light2D budget (`LOW` 4, `MEDIUM` 8, `HIGH` 16), particle budget, parallax layers shown (`LOW` hides `L3`), and post-processing (bloom only on `HIGH`). Presets never change world scale, colliders, hitboxes or telegraph visibility.

## Frame Pacing
Measured in the canonical client hotspot scene for 5 minutes. A client never renders more than the AOI cap (`MAX_ENTITIES_IN_AOI_PER_CLIENT = 40`, `../05_network/synchronization.md`), so the scene is the worst visible set at the forced-placement channel cap (22 players, `../02_world/world_rules.md` § Forced Placement; ADR-0066):
```text
client hotspot scene = local player + 21 remote players + 19 AI_CLASS_NAMED_MECHANIC monsters (40 replicated entities),
                       all in sustained combat with skill VFX, driven by the recorded hotspot stream (§ Smoothness by Construction item 9)
```
The server-side channel worst case (22 players + 42 monsters) is `../09_testing/load.md` scenario 11.

```text
tier           target   p95 frame     p99 frame     hitches (>50 ms)
DESKTOP_MIN    60 FPS   <= 16.7 ms    <= 25 ms      0 per 5 min in combat   (design target, not CI-gated)
ANDROID_REC    60 FPS   <= 16.7 ms    <= 25 ms      <= 1 per 5 min
ANDROID_MIN    30 FPS   <= 33.3 ms    <= 45 ms      <= 1 per 5 min
```
Desktop CPU budget (`PERF-002`, CI-gated; ADR-0070): hotspot scene in Editor PlayMode under xvfb with Mesa llvmpipe on the Linux CI job (GitHub-hosted `ubuntu-24.04` standard runner for public repositories: 4 vCPU, 16 GB RAM), started with `-job-worker-count 2`, `LP_NUM_THREADS=1`, vSync off, `targetFrameRate` unset; 3 repetitions of 100 s each after a 10 s warm-up (§ Measurement and Gates). Metric = **main-thread CPU excluding rendering**: per frame, `PlayerLoop` marker time minus every main-thread marker named `Gfx.*`, `Camera.Render`, `Render.*`, `Semaphore.WaitForSignal` or `WaitForTargetFPS`, all read with `ProfilerRecorder` (no `FrameTimingManager`). It therefore covers the FrameLoop phases (input, network apply, prediction, interpolation, presentation, UI, camera), animation and physics queries, and excludes software rasterization, whether the editor renders on the main thread or a render thread. Gate: p95 <= 8 ms, p99 <= 12 ms, no frame > 33 ms. Culling/sorting/batching cost is bounded by the `PERF-006` count budgets instead of timing. Desktop GPU frame pacing is an accepted gap: hosted CI has no GPU (ADR-0058); it is mitigated by the `PERF-006` draw budgets, the `PERF-016` overdraw/pass budgets, the `PERF-017` adaptive governor and the `PERF-003` Android GPU device runs (ADR-0059).

Frame budget on `ANDROID_MIN` (33.3 ms): scripts <= 10 ms, rendering <= 12 ms, remainder for OS/GPU. Batches <= 150 on mobile (SpriteAtlas per region/actor group, no per-frame material instancing). Frame-rate control: § Smoothness by Construction item 3.

## Memory and GC
```text
managed GC allocation per frame in steady gameplay (movement, combat, UI HUD) = 0 bytes (main-thread frame code)
network receive/decode (background task)                                     <= PERF-024 budget
allocations allowed only at load, scene transfer, and opening/closing full-screen UI
total resident memory: ANDROID_MIN <= 1.3 GB, DESKTOP_MIN <= 2.5 GB (player process)
```
Desktop CI proxy (`PERF-005` desktop): PlayMode runs inside the editor process, so resident memory is not measurable there. The Linux job gates the growth of Unity-tracked memory instead: `ProfilerRecorder` counters `Total Used Memory` and `Gfx Used Memory` (category `Memory`) are sampled in the empty bootstrap scene right before the hotspot scene loads (baseline) and every second during the run; peak minus baseline <= 1.5 GB for `Total Used Memory` and <= 1.0 GB for `Gfx Used Memory`. The 2.5 GB desktop resident target stays a design target; resident memory is gated only on Android device runs.
Incremental GC is on with a 1 ms time slice (`GarbageCollector.incrementalTimeSliceNanoseconds = 1_000_000`); `GC.Collect` runs only on loading screens. Pooling rules: `../10_implementation/engineering_conventions.md` §2.3. Addressables group budgets: `../07_content/presentation_asset_manifest.md` §1.

## Load and Transfer Times
```text
cold start to login screen            <= 10 s (ANDROID_MIN), <= 6 s (DESKTOP_MIN)
login to in-world (first map)         <= 8 s
map transfer, same region             <= 3 s
map transfer, new region (bundles)    <= 6 s
reconnect resume to controllable      <= 5 s after connection is restored
```
Start and end markers (`PERF-007`): cold start = process launch to first interactive login-screen frame (Android device run only; desktop cold start is a design target, not CI-gated, because CI runs no Windows player); login to in-world = `C2S_CHARACTER_ATTACH` sent to the first `IN_WORLD` frame with a controllable local player; map transfer = transfer command received to the first controllable frame on the destination map; reconnect = WSS reconnected to the first controllable frame. The desktop CI run measures the last three in PlayMode on the Linux job with Addressables play mode `Use Existing Build` (bundles built for the Linux player target in the same job) against the PlayMode harness server.
Every wait longer than 0.5 s shows a progress screen or indicator; no frozen frame longer than 100 ms during loading (async Addressables + incremental instantiation).

## Input Responsiveness
```text
local input -> first visual response (animation/prediction start)  <= 1 rendered frame after input sampling
local movement                                                      predicted immediately (../05_network/synchronization.md)
server-confirmed combat result shown                                <= RTT + 50 ms
```
Input is sampled once per rendered frame before simulation presentation; touch controls have no added debounce.

## Network Smoothness
```text
remote interpolation delay      = 2 snapshot intervals (200 ms at 10 Hz), adaptive 150..300 ms from measured jitter
extrapolation limit             = 250 ms, then freeze/lerp (../05_network/synchronization.md)
local correction                = smooth over 100 ms when error <= 0.5 m; snap above (../04_architecture/physics_geometry_contract.md)
full-quality conditions         = RTT <= 150 ms, jitter <= 30 ms, loss <= 2%: no visible rubber-band, corrections > 0.5 m <= 1 per minute
degraded-but-playable           = RTT <= 300 ms, jitter <= 60 ms, loss <= 5%: client shows the network indicator; no desync
```

## Mobile Sustained Performance
Battery drain cannot be measured in the cloud pipeline, so sustained performance is gated by proxies:
```text
30-minute game-loop run on ANDROID_REC-class device   p50 >= 45 FPS for the whole run (thermal throttling allowed below 60)
ANDROID_MIN-class device, 30 FPS cap                  average CPU utilisation <= 50%, no frame p95 regression > 10% between minute 1 and minute 30
```
A battery-saver toggle caps FPS at 30 on any tier.

## Smoothness by Construction
Canonical runtime architecture that keeps frames smooth without GPU timing in CI (ADR-0059). Coding rules and the Q4 fence: `../10_implementation/engineering_conventions.md` §2.3–§2.7.

1. **Frame driver** (`PERF-014`, `PERF-020`): one `FrameLoop` MonoBehaviour (`ThinhThan.Core`, `Core/Runtime/`) is the only first-party type with Unity frame callbacks. The `ThinhThan.App` composition root (or the PlayMode harness) creates it once. Every rendered frame it runs the registered systems in fixed phase order:
   ```text
   Input -> NetReceive -> Prediction -> Interpolation -> Presentation -> UI -> Camera
   ```
   - Presentation covers animation, VFX and audio triggers; Camera runs from `LateUpdate`.
   - Systems implement `IFrameSystem.Tick(in FrameTime)`; registration happens only at composition or map load, never mid-frame.
   - Replicated entity views live in contiguous index-based arrays (struct state + cached component references) updated by one system per concern. There is no per-entity `MonoBehaviour` logic and no per-frame `GetComponent`.
2. **Timing** (`PERF-014`, `PERF-023`):
   - `FrameTime` is computed once per frame: `delta` = `Time.unscaledDeltaTime` clamped to 100 ms; `now` = `Time.realtimeSinceStartupAsDouble`, which is also the timeline for snapshot interpolation.
   - Remote interpolation and local correction use § Network Smoothness.
   - The camera follows the predicted local player with critically damped smoothing (time constant 0.12 s, no overshoot). It moves once per frame in the Camera phase, is clamped to the active camera region (`physics_geometry_contract.md` §6.2), and snaps on transfer or hard reconciliation.
   - There is no pixel-perfect camera (ADR-0055).
3. **Frame-rate control** (`PERF-019`):
   - Desktop: `QualitySettings.vSyncCount = 1`, `targetFrameRate` unset.
   - Android: `vSyncCount = 0`, `Application.targetFrameRate` = tier target, Player Setting *Optimized Frame Pacing* enabled.
   - Battery saver caps every tier at 30 (`PERF-013`).
   - Player settings: incremental GC on; Physics2D `simulationMode = Script`. The client has no authoritative physics; prediction uses the shared geometry port (`../04_architecture/physics_geometry_contract.md`).
4. **Main-thread budget** (`PERF-015`): all non-urgent main-thread work goes through the `FrameBudget` scheduler (`Core/Runtime/`). This covers instantiation, pool pre-warm, list/UI population, Addressables completion handling and content decode.
   ```text
   gameplay frame      <= 2 ms of FrameBudget work
   loading-screen frame <= 12 ms
   synchronous loads   (Resources.Load, WaitForCompletion) forbidden outside loading screens
   Application.backgroundLoadingPriority = Low in gameplay, High on loading screens
   ```
5. **No first-use hitch** (`PERF-018`): map load pre-sizes that map's pools (actors, projectiles, VFX, floating text, UI rows) from content counts. On the loading screen it warms shader variants (`ShaderVariantCollection.WarmUp`, plus `GraphicsStateCollection` warm-up where the graphics API supports it) and loads the map's Addressables group before the screen closes.
6. **Rendering discipline** (`PERF-021`):
   - Materials and shaders are SRP-Batcher compatible.
   - One SpriteAtlas per region/actor group.
   - No runtime material instances: no `.material` getter and no `new Material` in gameplay; per-renderer tint goes through `SpriteRenderer.color`.
   - Y-sorting uses the 2D renderer's custom transparency sort axis `(0, 1, 0)` and never per-frame script sorting.
   - Sprites whose longer texture side is >= 256 texture px and that have transparent margins use mesh type `Tight`; all others use `Full Rect` (`../07_content/presentation_asset_manifest.md` §3).
   - Animators use `CullCompletely` and SpriteSkin skips off-screen bones.
   - Physics2D queries use preallocated buffers.
7. **Adaptive quality governor** (`PERF-017`): `Core/Performance/` reads GPU frame time from `FrameTimingManager`, falling back to CPU frame time where GPU timing is unsupported. Every value is presentation-only; the governor never exceeds the selected preset and never changes geometry or telegraph readability.
   ```text
   window        120 frames (p95); window resets after every step
   step down     p95 > 110% of tier frame target: render scale -0.05 (floor 0.6), then particle budget -25% (floor 50% of preset)
   step up       p95 < 75% of target continuously for 10 s: reverse order, one step
   hysteresis    >= 3 s between steps
   ```
   The time source is injected, so the governor is tested deterministically.
8. **UI** (`PERF-022`, uGUI Canvas + TextMeshPro per `client_experience_contract.md`):
   - Widgets set dirty flags; the UI phase applies them at most once per frame.
   - Static and dynamic elements sit on separate nested Canvases, so a HUD value change never rebuilds static layout.
   - Numbers use `TMP_Text.SetText(format, value)`, which does not allocate.
   - Non-interactive graphics have `raycastTarget = false`; there are no layout groups or `ContentSizeFitter` on dynamic HUD elements.
   - Long lists are virtualized over pooled rows populated through `FrameBudget`.
9. **Network receive path** (`PERF-024`):
   - One background task per session receives through `System.Net.WebSockets.ClientWebSocket` into pooled buffers and decodes each envelope, reusing message instances where the runtime allows.
   - Decoded messages go to a bounded queue; `NetReceive` applies them on the main thread with 0 allocation.
   - Unity APIs are never called off the main thread.
   ```text
   decode allocation      <= 64 KB per second of the recorded hotspot stream, exact for the fixture
   framing / buffers      0 bytes (pooled)
   ```
   Hotspot stream fixture (registered in `../09_testing/test_and_release_evidence.md` §3): 60 s of the client hotspot scene's replication at the AOI cap (40 replicated entities + local player), 10 Hz snapshot/delta envelopes in the `../05_network/synchronization.md` shapes, produced by a seeded deterministic generator in the PlayMode test assembly (IMP-065) from generated protobuf messages; the same file drives the hotspot scene.

## Measurement and Gates
- Desktop (every PR, Linux job, GitHub-hosted, no GPU; ADR-0058, ADR-0066): PlayMode tests in category `Performance` run the hotspot scene under xvfb with Mesa llvmpipe and read every metric through `ProfilerRecorder`; CPU timing (`PERF-002`) excludes all rendering markers (§ Frame Pacing, ADR-0070). GPU frame time is never measured or gated in CI, so a missing GPU is neither a failure nor an `OPS` blocker.
- Every PR, device-independent budgets (Linux job, always required once the owning task is DONE):
  ```text
  managed GC allocation per frame in the hotspot scene       = 0 bytes
  batches <= 150, SetPass calls <= 60 (LOW preset)          texture memory within presentation_asset_manifest.md §1 budgets
  active point Light2D and particle counts <= preset budget  desktop CPU budget (PERF-002)
  overdraw (LOW, 1280x720) average <= 2.5, p99 pixel <= 8     full-screen passes: LOW <= 1, MEDIUM <= 2, HIGH <= 4
  FrameBudget work <= 2 ms per gameplay frame                 first use of every skill VFX / UI screen: no frame > 50 ms CPU
  ```
- Overdraw (`PERF-016`) is measured on the Linux job: the hotspot scene renders at the 1280x720 reference with every sprite, tilemap and particle renderer temporarily switched to a test-only additive `OverdrawCount` material into an `RFloat` target, so each fragment of the mesh area (including transparent margins) adds 1. The metric is the per-pixel count read back from that target. Full-screen passes are all post-processing passes, blits and renderer-feature passes per camera per frame.
- Timing gates (`PERF-002`, `PERF-007` desktop, `PERF-015`, `PERF-018`) run 3 repetitions inside one job and gate on the median value (a threshold on single frames holds in at least 2 of 3 repetitions). Job reruns never turn a red timing gate green (`../10_implementation/agent_execution_protocol.md` §5b). Allocation, counter, pass and overdraw gates are exact on a single run.
- Structural rules that need no measurement (FrameLoop fence `PERF-020`, API fence) are Q4 checks (`../10_implementation/audit_gates.md`).
- Android device runs: an IL2CPP Android build of the hotspot scene runs as a Unity game-loop test on Firebase Test Lab physical devices (one `ANDROID_MIN`-class and one `ANDROID_REC`-class model, recorded in Owner Setup) from the scheduled `device-perf` workflow on `ubuntu-24.04`; CI downloads the frame timings and gates them. No device is attached to the runner. Cadence stays inside the free quota: at most one scheduled run per day on `main` when client code/assets changed since the last device run, plus the mandatory launch-candidate run. A run blocked by exhausted quota is reported `DEFERRED(quota)` and retried the next day; it never fails or blocks ordinary PRs. The launch-candidate gate requires a passing device run on the release commit and waits (it never skips) until quota allows.
- Network smoothness tests use the client network emulator (latency, jitter, loss) against a local server in PlayMode.
- The 30-minute sustained runs execute on Firebase Test Lab during the launch-candidate gate.
- Any metric above its target fails the gate; targets change only by spec change (gate ratchet, ADR-0050).

## Requirement IDs
Every ID below must be named in at least one task packet's acceptance and covered by a named test; Q0 fails on an uncovered ID (`../10_implementation/audit_gates.md` requirement coverage).

| ID | Requirement (section) | Gate |
|---|---|---|
| `PERF-001` | Auto benchmark picks LOW/MEDIUM/HIGH; presets change presentation only (Device Tiers) | every PR |
| `PERF-002` | desktop main-thread CPU excluding rendering: p95 <= 8 ms, p99 <= 12 ms, no frame > 33 ms (Frame Pacing) | every PR (Linux job) |
| `PERF-003` | ANDROID_MIN and ANDROID_REC frame pacing (Frame Pacing) | device run |
| `PERF-004` | 0 bytes managed GC per frame in steady gameplay (Memory and GC) | every PR |
| `PERF-005` | Android resident memory caps; desktop tracked-memory growth <= 1.5 GB total / 1.0 GB Gfx over baseline (Memory and GC) | every PR (desktop proxy) + device run |
| `PERF-006` | batches <= 150, SetPass <= 60, light/particle budgets per preset (Measurement) | every PR |
| `PERF-007` | login, map transfer and reconnect times with defined markers; cold start on Android only (Load and Transfer Times) | every PR (desktop, no cold start) + device run |
| `PERF-008` | progress shown for waits > 0.5 s; no frozen frame > 100 ms while loading | every PR |
| `PERF-009` | input -> first visual response <= 1 frame; confirmed result <= RTT + 50 ms (Input Responsiveness) | every PR |
| `PERF-010` | interpolation/extrapolation/correction values (Network Smoothness) | every PR |
| `PERF-011` | full-quality and degraded network conditions under emulated latency/jitter/loss | every PR |
| `PERF-012` | 30-minute sustained proxies on ANDROID_REC / ANDROID_MIN (Mobile Sustained Performance) | launch candidate |
| `PERF-013` | battery-saver toggle caps FPS at 30 | every PR |
| `PERF-014` | one `FrameLoop`, fixed phase order, `FrameTime` clamp 100 ms, index-based entity views (Smoothness by Construction 1–2) | every PR |
| `PERF-015` | `FrameBudget` <= 2 ms per gameplay frame, <= 12 ms on loading screens; no synchronous load outside loading screens; loading priority Low/High (item 4) | every PR |
| `PERF-016` | overdraw average <= 2.5 and p99 <= 8 at LOW; full-screen passes <= preset budget (Measurement) | every PR (Linux job) |
| `PERF-017` | adaptive governor step down/up, hysteresis, floors, preset ceiling (item 7) | every PR |
| `PERF-018` | map-load pool pre-size, shader warm-up and group preload; first use of every skill VFX and UI screen has no frame > 50 ms CPU (item 5) | every PR |
| `PERF-019` | frame-rate control and player settings: vSync/targetFrameRate per platform, Optimized Frame Pacing, incremental GC 1 ms slice, Physics2D Script mode (item 3, Memory and GC) | every PR |
| `PERF-020` | no `Update/FixedUpdate/LateUpdate/OnGUI` in first-party runtime code outside `FrameLoop` and the allowlist (item 1) | every PR (Q4) |
| `PERF-021` | rendering discipline: SRP-Batcher compatible materials, transparency sort axis, Tight mesh rule, animator culling, no runtime material instances (item 6) | every PR |
| `PERF-022` | UI: dirty flags applied once per frame, static/dynamic Canvas split, 0-alloc HUD value updates, raycast targets off on non-interactive graphics (item 8) | every PR |
| `PERF-023` | camera: critically damped follow 0.12 s without overshoot, one move per frame, active camera-region clamp, snap on transfer/hard reconciliation (item 2) | every PR |
| `PERF-024` | network decode <= 64 KB/s on the 40-entity hotspot stream fixture, 0-byte framing/buffers, main-thread apply 0 bytes (item 9) | every PR |

## Invariants
```text
presentation quality presets never change gameplay geometry, hitboxes or telegraph readability
zero managed allocation per frame in steady gameplay
every wait > 0.5 s shows progress
client frame rate never changes server simulation (20 Hz)
one FrameLoop drives every first-party frame callback; phase order is fixed
non-urgent main-thread work is time-sliced by FrameBudget
GPU cost is bounded by overdraw/pass/batch/light budgets and the adaptive governor, not by CI GPU timing
```
