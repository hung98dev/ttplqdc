using System;
using System.Collections.Generic;
using System.Net;
using System.Net.Sockets;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Core.Geometry;
using ThinhThan.Core.Input;
using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Movement;
using ThinhThan.Systems.Replication;
using ThinhThan.Tests.PlayMode.Harness;
using ThinhThan.UI.Character;
using ThinhThan.UI.CoreHud;
using ThinhThan.UI.StateMachine;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.InputHudStateMachine
{
    /// <summary>
    /// IMP-066 PlayMode: semantic input → wire intents, §1 UI FSM literal
    /// edge set (post-#185), §5.1 gameplay-edge drain-discard, HUD feeds —
    /// plus the three wired-id flows (10/11/15/206/207) on the real
    /// FakeServer.
    /// </summary>
    public sealed class InputHudStateMachineTests
    {
        // --- Seam fakes ----------------------------------------------------

        private sealed class FakeSender : IMovementSender
        {
            public readonly List<(uint Id, IMessage Msg)> Sent =
                new List<(uint, IMessage)>();

            public ulong NextSeq = 1;

            public Awaitable<ulong> SendAsync(
                uint messageId, IMessage payload, CancellationToken cancel)
            {
                Sent.Add((messageId, payload));
                var src = new AwaitableCompletionSource<ulong>();
                src.SetResult(NextSeq++);
                return src.Awaitable;
            }
        }

        private sealed class FakeInteract : IInteractResolver
        {
            public InteractTarget? Next;

            public bool TryResolve(
                long selfXMm, long selfYMm, out InteractTarget target)
            {
                target = Next ?? default;
                return Next != null;
            }
        }

        private sealed class FakeTargets : ITargetSelector
        {
            public ulong Next;

            public ulong NextHostile(
                long selfXMm, long selfYMm, ulong currentEntityId)
            {
                return Next;
            }
        }

        private sealed class FakeSink : SelfReconciliationSink
        {
            public void NoteInputSent(ulong seq)
            {
            }
        }

        private sealed class FakeFeed : ICombatEventFeed
        {
            private Action<S2CActionStarted>? _actionStarted;
            private Action<S2CActionRejected>? _actionRejected;
            private Action<S2CStatusEvent>? _statusEvent;
            private Action<S2CCombatEvent>? _combatEvent;
            private Action<S2CInteractResult>? _interactResult;

            public event Action<S2CActionStarted> ActionStarted
            {
                add => _actionStarted += value;
                remove => _actionStarted -= value;
            }

            public event Action<S2CActionRejected> ActionRejected
            {
                add => _actionRejected += value;
                remove => _actionRejected -= value;
            }

            public event Action<S2CStatusEvent> StatusEvent
            {
                add => _statusEvent += value;
                remove => _statusEvent -= value;
            }

            public event Action<S2CCombatEvent> CombatEvent
            {
                add => _combatEvent += value;
                remove => _combatEvent -= value;
            }

            public event Action<S2CInteractResult> InteractResult
            {
                add => _interactResult += value;
                remove => _interactResult -= value;
            }

            public void RaiseStarted(S2CActionStarted e)
            {
                _actionStarted?.Invoke(e);
            }

            public void RaiseRejected(S2CActionRejected e)
            {
                _actionRejected?.Invoke(e);
            }

            public void RaiseStatus(S2CStatusEvent e)
            {
                _statusEvent?.Invoke(e);
            }
        }

        // --- Shared rigs ---------------------------------------------------

        private static GeometryData FlatGeometry()
        {
            return new GeometryData(
                "map_test_flat", "map", "flat", "test-revision",
                200000, 50000,
                new[]
                {
                    new GeometryData.Segment(
                        1, GeometryData.SegmentKind.SolidGround,
                        -100000, 0, 200000, 0),
                },
                new GeometryData.CameraRegion[0],
                new GeometryData.Anchor[0]);
        }

        private static FrameTime Frame(double nowSec, float delta = 0.016f)
        {
            return new FrameTime(delta, nowSec, 0);
        }

        private sealed class InputRig
        {
            public InputSemanticState State = new InputSemanticState();
            public GameInputSource Source;
            public FakeSender Sender = new FakeSender();
            public FakeInteract Interact = new FakeInteract();
            public FakeTargets Targets = new FakeTargets();
            public ISkillLoadout Loadout = new FixedSkillLoadout(
                new string?[]
                {
                    "skill.a", "skill.b", "skill.c", "skill.d", "skill.e",
                });
            public MovementPredictionSystem Prediction;
            public ReplicationApplier Replication = new ReplicationApplier();
            public ActionIntentDispatch Actions;

            public InputRig()
            {
                Source = new GameInputSource(State);
                Prediction = new MovementPredictionSystem(
                    new GeometryMath.GeometryWorld(FlatGeometry()));
                Prediction.RestoreFrom(new MovementCheckpoint
                {
                    XMm = 10000,
                    YMm = 0,
                    Facing = Facing.Right,
                    IsGrounded = true,
                });
                Actions = new ActionIntentDispatch(
                    Sender, Interact, Targets, Loadout, Prediction,
                    Replication, () => new byte[16]);
            }
        }

        /// <summary>Feeds a baseline carrying self state into the applier.</summary>
        private static void FeedBaseline(
            ReplicationApplier applier, EntityState self,
            ulong baselineId = 1, SelfPrivateState? selfPrivate = null)
        {
            var baseline = new S2CWorldBaseline
            {
                BaselineId = baselineId,
                ServerTick = 10,
                MapId = "map_test_flat",
                Self = self,
            };
            if (selfPrivate != null)
            {
                baseline.SelfPrivate = selfPrivate;
            }

            applier.Apply(new DecodedFrame
            {
                MessageId = 300,
                BaselineId = baselineId,
                ServerSeq = baselineId,
                Payload = baseline,
            });
        }

        // --- Input dispatch ------------------------------------------------

        [Test]
        public void TestInputEdgeDispatch()
        {
            var rig = new InputRig();
            rig.State.SetDirection(1);
            rig.State.PushEdge(SemanticEdge.BasicAttack);
            rig.State.PushEdge(SemanticEdge.Skill2);
            rig.State.PushEdge(SemanticEdge.ChatOpen);

            // Movement drain sees only the movement edge and maps it to the
            // wire LocalEdge; action/UI edges stay queued for their drains.
            var localEdges = new LocalEdge[8];
            int moved = rig.Source.SampleEdges(localEdges);
            Assert.AreEqual(1, moved);
            Assert.AreEqual(LocalEdge.PressRight, localEdges[0]);

            rig.Actions.Dispatch(Frame(1.0), rig.Source);
            Assert.AreEqual(2, rig.Sender.Sent.Count);
            Assert.AreEqual(201U, rig.Sender.Sent[0].Id);
            var basic = (C2SBasicAttack)rig.Sender.Sent[0].Msg;
            Assert.AreEqual(1000UL, basic.ClientMonoMs);
            Assert.AreEqual(Facing.Right, basic.Facing);
            Assert.AreEqual(0UL, basic.TargetEntityId);
            Assert.AreEqual(200U, rig.Sender.Sent[1].Id);
            var skill = (C2SSkillUse)rig.Sender.Sent[1].Msg;
            Assert.AreEqual("skill.b", skill.SkillId);
            Assert.AreEqual(1000UL, skill.ClientMonoMs);

            // The chat edge survived both gameplay drains (UI-local).
            var uiEdges = new SemanticEdge[4];
            Assert.AreEqual(1, rig.Source.SampleUiEdges(uiEdges));
            Assert.AreEqual(SemanticEdge.ChatOpen, uiEdges[0]);
        }

        [Test]
        public void TestCooldownDisplaySync()
        {
            var replication = new ReplicationApplier();
            FeedBaseline(replication, new EntityState { EntityId = 42 });
            var model = new HudDataModel();
            var feed = new FakeFeed();
            var tracker = new CooldownTracker(
                replication,
                new FixedSkillLoadout(new string?[] { "skill.a", "skill.b" }),
                model, feed);

            feed.RaiseStarted(new S2CActionStarted
            {
                SourceEntityId = 42,
                SkillId = "skill.b",
                CooldownEndsAtTick = 100,
                ServerTick = 40,
            });
            Assert.AreEqual(1, model.Cooldowns.Length);
            Assert.AreEqual(2, model.Cooldowns[0].Slot);
            Assert.AreEqual(100UL, model.Cooldowns[0].CooldownEndsAtTick);
            Assert.Less(model.Cooldowns[0].Fill(60), 1f);
            Assert.Greater(model.Cooldowns[0].Fill(60), 0f);

            // A rejection on the same skill clears the sweep.
            feed.RaiseRejected(new S2CActionRejected
            {
                SkillId = "skill.b",
            });
            Assert.AreEqual(0, model.Cooldowns.Length);

            // An action on another entity never enters the hotbar.
            feed.RaiseStarted(new S2CActionStarted
            {
                SourceEntityId = 99,
                SkillId = "skill.a",
                CooldownEndsAtTick = 200,
                ServerTick = 40,
            });
            Assert.AreEqual(0, model.Cooldowns.Length);

            tracker.Update(250);
            tracker.Dispose();
        }

        [Test]
        public void TestBuffDisplayUpdate()
        {
            var replication = new ReplicationApplier();
            var self = new EntityState
            {
                EntityId = 42,
                Flags = UiEntityFlags.Eligible,
            };
            self.Statuses.Add(new EntityStatus
            {
                EffectId = "e.ignite",
                Stacks = 2,
                ExpiresAtTick = 50,
            });
            FeedBaseline(replication, self);

            var model = new HudDataModel();
            var feed = new FakeFeed();
            var tracker = new StatusTracker(replication, model, feed);
            tracker.Update();

            Assert.AreEqual(1, model.Statuses.Length);
            Assert.AreEqual("e.ignite", model.Statuses[0].EffectId);
            Assert.AreEqual(2U, model.Statuses[0].Stacks);
            Assert.AreEqual("buff", model.Statuses[0].StatusKind);

            // 205 teaches the kind the wire puts on the icon.
            feed.RaiseStatus(new S2CStatusEvent
            {
                EffectId = "e.ignite",
                StatusKind = "debuff",
            });
            tracker.Update();
            Assert.AreEqual("debuff", model.Statuses[0].StatusKind);
            tracker.Dispose();
        }

        // --- §1 UI FSM -----------------------------------------------------

        [Test]
        public void TestUiFsmStatesAndTransitions()
        {
            var fsm = new SessionStateMachine();

            // Every §1 edge lands literally (post-#185 edge set).
            fsm.TransitionUi(ClientUiState.PatchingUpdate);
            fsm.TransitionUi(ClientUiState.AuthTitle);
            fsm.TransitionUi(ClientUiState.LoginQueued);
            fsm.TransitionUi(ClientUiState.CharacterSelect);
            fsm.TransitionUi(ClientUiState.AuthTitle);
            fsm.TransitionUi(ClientUiState.CharacterSelect);
            fsm.TransitionUi(ClientUiState.TransferringMap);
            fsm.TransitionUi(ClientUiState.InWorld);
            fsm.TransitionUi(ClientUiState.CharacterSelect);
            fsm.TransitionUi(ClientUiState.TransferringMap);
            fsm.TransitionUi(ClientUiState.InWorld);
            fsm.TransitionUi(ClientUiState.TransferringMap);
            fsm.TransitionUi(ClientUiState.Disconnected);
            fsm.TransitionUi(ClientUiState.InWorld);
            fsm.TransitionUi(ClientUiState.Disconnected);
            fsm.TransitionUi(ClientUiState.TransferringMap);
            fsm.TransitionUi(ClientUiState.Disconnected);
            fsm.TransitionUi(ClientUiState.CharacterSelect);
            fsm.TransitionUi(ClientUiState.Disconnected);
            fsm.TransitionUi(ClientUiState.AuthTitle);
            Assert.AreEqual(ClientUiState.AuthTitle, fsm.UiState);

            // Edges outside the §1 set throw.
            var fresh = new SessionStateMachine();
            Assert.Throws<InvalidOperationException>(
                () => fresh.TransitionUi(ClientUiState.InWorld));
            fresh.TransitionUi(ClientUiState.AuthTitle);
            fresh.TransitionUi(ClientUiState.CharacterSelect);
            fresh.TransitionUi(ClientUiState.TransferringMap);
            Assert.Throws<InvalidOperationException>(
                () => fresh.TransitionUi(ClientUiState.CharacterSelect));
            Assert.Throws<InvalidOperationException>(
                () => fresh.TransitionUi(ClientUiState.AuthTitle));
            Assert.Throws<InvalidOperationException>(
                () => fresh.TransitionUi(ClientUiState.LoginQueued));

            // any → AUTH_TITLE on CLIENT_UPDATE_REQUIRED / SESSION_REPLACED
            // are flag paths, not table edges.
            var forced = new SessionStateMachine();
            forced.TransitionUi(ClientUiState.AuthTitle);
            forced.ForceUpdateRequired();
            Assert.AreEqual(ClientUiState.AuthTitle, forced.UiState);
            Assert.IsTrue(forced.ClientUpdateRequired);

            var replaced = new SessionStateMachine();
            replaced.TransitionUi(ClientUiState.AuthTitle);
            replaced.TransitionUi(ClientUiState.CharacterSelect);
            replaced.TransitionUi(ClientUiState.TransferringMap);
            replaced.TransitionUi(ClientUiState.InWorld);
            replaced.SetSessionReplaced();
            Assert.AreEqual(ClientUiState.AuthTitle, replaced.UiState);
            Assert.IsTrue(replaced.SessionReplaced);
        }

        [Test]
        public void TestNoDuplicateBindingPerContext()
        {
            var state = new InputSemanticState();
            using var maps = new ClientInputMaps(state);

            var seen = new HashSet<string>(StringComparer.Ordinal);
            foreach (UnityEngine.InputSystem.InputAction action in
                maps.Map.actions)
            {
                foreach (UnityEngine.InputSystem.InputBinding binding in
                    action.bindings)
                {
                    if (binding.isComposite ||
                        string.IsNullOrEmpty(binding.path))
                    {
                        continue;
                    }

                    string key = binding.path + "@" + binding.groups;
                    Assert.IsTrue(
                        seen.Add(key),
                        "duplicate binding " + key + " on " + action.name);
                }
            }
        }

        [Test]
        public void TestContextInteractPortalVsInteract()
        {
            // Real resolver: portal anchor beats an NPC at the same distance.
            var store = new EntityViewStore();
            store.Upsert(new EntityState
            {
                EntityId = 5,
                EntityKind = EntityKind.Npc,
                ContentId = "npc.elder",
                XMm = 11000,
                Flags = UiEntityFlags.Interactable,
            });
            var geometry = new GeometryData(
                "map_test_flat", "map", "flat", "test-revision",
                200000, 50000,
                new GeometryData.Segment[0],
                new GeometryData.CameraRegion[0],
                new[]
                {
                    new GeometryData.Anchor("portal.gate_a", 11000, 0),
                });
            var resolver = new NearestInteractResolver(store, geometry);
            Assert.IsTrue(
                resolver.TryResolve(10000, 0, out InteractTarget portal));
            Assert.IsTrue(portal.IsPortal);
            Assert.AreEqual("portal.gate_a", portal.PortalId);

            // Dispatch split: portal → 104, anything else → 103.
            var rig = new InputRig();
            rig.Interact.Next = portal;
            rig.State.PushEdge(SemanticEdge.ContextInteract);
            rig.Actions.Dispatch(Frame(0.0), rig.Source);
            Assert.AreEqual(1, rig.Sender.Sent.Count);
            Assert.AreEqual(104U, rig.Sender.Sent[0].Id);
            var portalUse = (C2SPortalUse)rig.Sender.Sent[0].Msg;
            Assert.AreEqual("portal.gate_a", portalUse.PortalId);
            Assert.AreEqual(16, portalUse.OperationId.Length);

            rig.Interact.Next = new InteractTarget(
                false, null, InteractKind.Talk, "npc.elder");
            rig.State.PushEdge(SemanticEdge.ContextInteract);
            rig.Actions.Dispatch(Frame(0.0), rig.Source);
            Assert.AreEqual(2, rig.Sender.Sent.Count);
            Assert.AreEqual(103U, rig.Sender.Sent[1].Id);
            var interact = (C2SInteract)rig.Sender.Sent[1].Msg;
            Assert.AreEqual(InteractKind.Talk, interact.InteractKind);
            Assert.AreEqual("npc.elder", interact.TargetId);
            Assert.AreEqual(16, interact.OperationId.Length);
        }

        [Test]
        public void TestTargetIntentOnlyPath()
        {
            var rig = new InputRig();
            rig.Targets.Next = 99UL;

            rig.State.PushEdge(SemanticEdge.TargetCycle);
            rig.Actions.Dispatch(Frame(0.0), rig.Source);
            Assert.AreEqual(1, rig.Sender.Sent.Count);
            Assert.AreEqual(202U, rig.Sender.Sent[0].Id);
            Assert.AreEqual(
                99UL, ((C2STargetIntent)rig.Sender.Sent[0].Msg)
                    .TargetEntityId);

            // Esc / empty-ground clears: same wire id, zero target.
            rig.State.PushEdge(SemanticEdge.TargetClear);
            rig.Actions.Dispatch(Frame(0.0), rig.Source);
            Assert.AreEqual(202U, rig.Sender.Sent[1].Id);
            Assert.AreEqual(
                0UL, ((C2STargetIntent)rig.Sender.Sent[1].Msg)
                    .TargetEntityId);

            // Displayed/combat target is the server-accepted one only.
            FeedBaseline(
                rig.Replication, new EntityState { EntityId = 7 },
                baselineId: 2,
                selfPrivate: new SelfPrivateState
                {
                    AcceptedTargetEntityId = 77,
                });
            rig.State.PushEdge(SemanticEdge.BasicAttack);
            rig.Actions.Dispatch(Frame(0.0), rig.Source);
            var basic = (C2SBasicAttack)rig.Sender.Sent[2].Msg;
            Assert.AreEqual(77UL, basic.TargetEntityId);

            for (int i = 0; i < rig.Sender.Sent.Count; i++)
            {
                Assert.IsTrue(
                    rig.Sender.Sent[i].Id == 202U ||
                    rig.Sender.Sent[i].Id == 201U,
                    "unexpected wire id " + rig.Sender.Sent[i].Id);
            }
        }

        [Test]
        public void TestInputStateSendRate()
        {
            var state = new InputSemanticState();
            var source = new GameInputSource(state);
            var sender = new FakeSender();
            var dispatch = new MovementInputDispatch(
                source, sender, new InputHistory(), new FakeSink());

            // Direction edge is immediate (108); first flag change sends 100.
            state.SetDirection(1);
            dispatch.Dispatch(Frame(0.0));
            Assert.AreEqual(2, sender.Sent.Count);
            Assert.AreEqual(
                MovementConstants.WireC2SMovementEdge, sender.Sent[0].Id);
            Assert.AreEqual(
                MovementConstants.WireC2SInputState, sender.Sent[1].Id);
            Assert.AreEqual(
                InputFlagBits.MoveRight,
                ((C2SInputState)sender.Sent[1].Msg).InputFlags);

            // No edge and <250 ms elapsed: silent.
            dispatch.Dispatch(Frame(0.03));
            Assert.AreEqual(2, sender.Sent.Count);

            // Held flag resend cadence: ≥250 ms while ≥1 flag held.
            dispatch.Dispatch(Frame(0.30));
            Assert.AreEqual(3, sender.Sent.Count);
            Assert.AreEqual(
                MovementConstants.WireC2SInputState, sender.Sent[2].Id);

            // Flag changes send at most once per 50 ms — the flip edge
            // still goes out immediately.
            state.SetDirection(-1);
            dispatch.Dispatch(Frame(0.31));
            Assert.AreEqual(4, sender.Sent.Count);
            Assert.AreEqual(
                MovementConstants.WireC2SMovementEdge, sender.Sent[3].Id);
            var flip = (C2SMovementEdge)sender.Sent[3].Msg;
            Assert.AreEqual(MovementEdgeType.Flip, flip.EdgeType);
            Assert.AreEqual(Facing.Left, flip.Direction);

            dispatch.Dispatch(Frame(0.36));
            Assert.AreEqual(5, sender.Sent.Count);
            Assert.AreEqual(
                MovementConstants.WireC2SInputState, sender.Sent[4].Id);
            Assert.AreEqual(
                InputFlagBits.MoveLeft,
                ((C2SInputState)sender.Sent[4].Msg).InputFlags);

            // Release to neutral: edge immediate, flags=0 goes out on the
            // next due slot, then no resend while nothing is held.
            state.SetDirection(0);
            dispatch.Dispatch(Frame(0.40));
            Assert.AreEqual(6, sender.Sent.Count);
            Assert.AreEqual(
                MovementConstants.WireC2SMovementEdge, sender.Sent[5].Id);
            dispatch.Dispatch(Frame(0.46));
            Assert.AreEqual(7, sender.Sent.Count);
            Assert.AreEqual(
                0U, ((C2SInputState)sender.Sent[6].Msg).InputFlags);
            dispatch.Dispatch(Frame(1.20));
            Assert.AreEqual(7, sender.Sent.Count);
            dispatch.Dispose();
        }

        // --- Wired flows (10/11/15/206/207 on the real FakeServer) ---------

        private static int FreePort()
        {
            var listener = new TcpListener(IPAddress.Loopback, 0);
            listener.Start();
            int port = ((IPEndPoint)listener.LocalEndpoint).Port;
            listener.Stop();
            return port;
        }

        private sealed class WorldRig : IDisposable
        {
            public readonly SessionStateMachine Fsm = new SessionStateMachine();
            public readonly SessionOrchestrator Orchestrator;
            public readonly ReplicationApplier Replication =
                new ReplicationApplier();
            public readonly SessionUiPresenter Presenter;
            public readonly UiScreenSet Screens = new UiScreenSet();
            public readonly UiFsmDriver Driver;
            public readonly CharacterSessionController Controller;

            public WorldRig(FakeServer server)
            {
                var credentials = new SessionCredentials
                {
                    DeviceId = Guid.NewGuid().ToByteArray(),
                    AccessToken = "test-access",
                    ClientBuild = 1,
                    ContentRevision = "test",
                };
                var store = new SessionStore(new InMemorySessionStorage());
                store.Save(credentials);
                Orchestrator = new SessionOrchestrator(
                    Fsm, credentials, store,
                    new AuthClient(server.ControlUrl),
                    delay: (ms, cancel) =>
                        Task.Delay(Math.Min(ms, 25), cancel));
                Orchestrator.Replication = Replication;
                Presenter = new SessionUiPresenter(Orchestrator, Fsm);
                Driver = new UiFsmDriver(
                    Fsm, Orchestrator, Replication, Presenter, Screens);
                Controller = new CharacterSessionController(Orchestrator);
            }

            public void Dispose()
            {
                Driver.Dispose();
            }
        }

        private static async Task PumpUntil(
            WorldRig rig, Func<bool> condition, int timeoutMs)
        {
            var time = new FrameTime(0.016f, 0.0, 0);
            long end = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() +
                timeoutMs;
            while (!condition() &&
                DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() < end)
            {
                rig.Orchestrator.Tick(in time);
                rig.Driver.Tick(in time);
                await Task.Delay(10).ConfigureAwait(false);
            }
        }

        private static async Task<WorldRig> EnterWorld(FakeServer server)
        {
            var rig = new WorldRig(server);
            Result<bool> connected = await rig.Orchestrator
                .ConnectWithTicketAsync(CancellationToken.None)
                .ConfigureAwait(false);
            Assert.IsTrue(connected.Ok, connected.ErrorCode);
            await PumpUntil(
                rig, () => rig.Fsm.UiState == ClientUiState.CharacterSelect,
                5000);
            Assert.AreEqual(
                ClientUiState.CharacterSelect, rig.Fsm.UiState);

            Result<S2CCharacterCreateResult> created =
                await rig.Controller.CreateAsync(
                    "nhan_vat", "class.kim", CancellationToken.None)
                    .ConfigureAwait(false);
            Assert.IsTrue(created.Ok);
            Result<S2CCharacterAttachOk> attached =
                await rig.Controller.AttachAsync(
                    created.Value.Character.CharacterId.ToByteArray(),
                    CancellationToken.None).ConfigureAwait(false);
            Assert.IsTrue(attached.Ok);
            // attach_ok → TRANSFERRING_MAP; baseline (300) lands IN_WORLD.
            await PumpUntil(
                rig, () => rig.Fsm.UiState == ClientUiState.InWorld, 5000);
            Assert.AreEqual(ClientUiState.InWorld, rig.Fsm.UiState);
            Assert.AreEqual(
                ClientUiState.InWorld, rig.Screens.ActiveScreen);
            return rig;
        }

        [Test]
        public async Task TestDetachToCharacterSelect()
        {
            using var server = new FakeServer(FreePort());
            server.Start();
            using WorldRig rig = await EnterWorld(server);

            Result<S2CCharacterDetachOk> detached =
                await rig.Controller.DetachAsync(CancellationToken.None)
                    .ConfigureAwait(false);
            Assert.IsTrue(detached.Ok);
            await PumpUntil(
                rig,
                () => rig.Fsm.UiState == ClientUiState.CharacterSelect,
                5000);
            Assert.AreEqual(
                ClientUiState.CharacterSelect, rig.Fsm.UiState);
            Assert.AreEqual(
                ClientUiState.CharacterSelect, rig.Screens.ActiveScreen);
            await rig.Orchestrator.ShutdownAsync();
        }

        [Test]
        public async Task TestPlacementPendingNoTimeout()
        {
            using var server = new FakeServer(FreePort());
            server.Start();
            using WorldRig rig = await EnterWorld(server);

            FakeServerSocket? socket = server.Latest;
            Assert.IsNotNull(socket);
            await socket!.SendAsync(15, new S2CPlacementPending
            {
                RequestMessageId = 207,
                Reason = PlacementReason.Respawn,
            });
            await PumpUntil(
                rig, () => rig.Fsm.PlacementPending, 5000);
            Assert.IsTrue(rig.Fsm.PlacementPending);
            Assert.IsTrue(rig.Screens.PlacementPending);

            // §1: the respawn overlay carries no timeout — the flag must
            // still be set after a real wait.
            await PumpUntil(
                rig, () => false, 600);
            Assert.IsTrue(rig.Fsm.PlacementPending);
            Assert.AreEqual(ClientUiState.InWorld, rig.Fsm.UiState);
            Assert.IsFalse(rig.Fsm.DeadOverlay);
            await rig.Orchestrator.ShutdownAsync();
        }

        [Test]
        public async Task TestRespawnPendingOverlay()
        {
            using var server = new FakeServer(FreePort());
            server.Start();
            using WorldRig rig = await EnterWorld(server);

            FakeServerSocket? socket = server.Latest;
            Assert.IsNotNull(socket);
            // Self state lands via a refreshed baseline carrying entity 7.
            await socket!.SendAsync(300, new S2CWorldBaseline
            {
                BaselineId = 2,
                ServerTick = 20,
                MapId = "map_test_flat",
                Self = new EntityState
                {
                    EntityId = 7,
                    Flags = UiEntityFlags.Eligible,
                    Hp = 100,
                    MaxHp = 100,
                },
            });
            await PumpUntil(
                rig, () => rig.Replication.SelfState != null, 5000);

            // Death → DEAD flag; placement pending → respawn overlay.
            await socket.SendAsync(206, new S2CDeath
            {
                EntityId = 7,
                ServerTick = 21,
            });
            await socket.SendAsync(15, new S2CPlacementPending
            {
                RequestMessageId = 207,
                Reason = PlacementReason.Respawn,
            });
            await PumpUntil(
                rig, () => rig.Fsm.DeadOverlay, 5000);
            Assert.IsTrue(rig.Fsm.DeadOverlay);
            Assert.IsTrue(rig.Screens.DeadOverlay);

            // Respawn clears the flag → the overlay drops on the next tick.
            await socket.SendAsync(207, new S2CRespawn
            {
                EntityId = 7,
                MapId = "map_test_flat",
                XMm = 10000,
                HpAfter = 100,
            });
            await socket.SendAsync(300, new S2CWorldBaseline
            {
                BaselineId = 3,
                ServerTick = 30,
                MapId = "map_test_flat",
                Self = new EntityState
                {
                    EntityId = 7,
                    Flags = UiEntityFlags.Eligible,
                    Hp = 100,
                    MaxHp = 100,
                },
            });
            await PumpUntil(
                rig, () => !rig.Fsm.DeadOverlay, 5000);
            Assert.IsFalse(rig.Fsm.DeadOverlay);
            await rig.Orchestrator.ShutdownAsync();
        }
    }
}
