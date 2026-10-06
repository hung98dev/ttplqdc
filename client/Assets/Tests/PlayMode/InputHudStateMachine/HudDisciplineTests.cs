using NUnit.Framework;
using ThinhThan.Core.Runtime;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;
using ThinhThan.UI.CoreHud;
using ThinhThan.UI.StateMachine;
using TMPro;
using UnityEngine;
using UnityEngine.TestTools.Constraints;
using UnityEngine.UI;

namespace ThinhThan.Tests.PlayMode.InputHudStateMachine
{
    /// <summary>
    /// PERF-022 discipline: one HUD apply per frame, nested static/dynamic
    /// canvas split, zero-alloc value updates, non-interactive surfaces
    /// never raycast.
    /// </summary>
    public sealed class HudDisciplineTests
    {
        private sealed class FakeFeed : ICombatEventFeed
        {
            private System.Action<S2CActionStarted>? _actionStarted;
            private System.Action<S2CActionRejected>? _actionRejected;
            private System.Action<S2CStatusEvent>? _statusEvent;
            private System.Action<S2CCombatEvent>? _combatEvent;
            private System.Action<S2CInteractResult>? _interactResult;

            public event System.Action<S2CActionStarted> ActionStarted
            {
                add => _actionStarted += value;
                remove => _actionStarted -= value;
            }

            public event System.Action<S2CActionRejected> ActionRejected
            {
                add => _actionRejected += value;
                remove => _actionRejected -= value;
            }

            public event System.Action<S2CStatusEvent> StatusEvent
            {
                add => _statusEvent += value;
                remove => _statusEvent -= value;
            }

            public event System.Action<S2CCombatEvent> CombatEvent
            {
                add => _combatEvent += value;
                remove => _combatEvent -= value;
            }

            public event System.Action<S2CInteractResult> InteractResult
            {
                add => _interactResult += value;
                remove => _interactResult -= value;
            }
        }

        private sealed class FakeWorld : IWorldContextFeed
        {
            public string MapId
            {
                get;
                set;
            }
            public uint ChannelIndex
            {
                get;
                set;
            }
            public string ContentRevision
            {
                get;
                set;
            }
            public string DisplayName
            {
                get;
                set;
            }

            public FakeWorld()
            {
                MapId = "map_test_flat";
                ChannelIndex = 3U;
                ContentRevision = "rev";
                DisplayName = "Test Map";
            }
        }

        private sealed class FakeLatency : ILatencyProbe
        {
            public int RttMs
            {
                get;
                set;
            }

            public FakeLatency()
            {
                RttMs = 42;
            }
        }

        private static void FeedBaseline(
            ReplicationApplier applier, EntityState self,
            ulong baselineId = 1)
        {
            applier.Apply(new DecodedFrame
            {
                MessageId = 300,
                BaselineId = baselineId,
                ServerSeq = baselineId,
                Payload = new S2CWorldBaseline
                {
                    BaselineId = baselineId,
                    ServerTick = 10,
                    MapId = "map_test_flat",
                    Self = self,
                },
            });
        }

        /// <summary>
        /// Builds a HUD hierarchy root with dynamic + static canvases and
        /// one live widget per section.
        /// </summary>
        private sealed class HudRig : System.IDisposable
        {
            public readonly GameObject Root;
            public readonly CoreHudView View;
            public readonly VitalsBarWidget Vitals;
            public readonly QuestTrackerWidget QuestDock;
            public readonly MenuButtonWidget Menu;
            public readonly HudDataModel Model = new HudDataModel();
            public readonly ReplicationApplier Replication =
                new ReplicationApplier();
            public readonly FakeFeed Feed = new FakeFeed();
            public readonly CoreHudSystem System;

            public HudRig()
            {
                Root = new GameObject("hud");
                var dynamicGo = new GameObject("dynamic");
                dynamicGo.transform.SetParent(Root.transform, false);
                Canvas dynamicCanvas = dynamicGo.AddComponent<Canvas>();
                var staticGo = new GameObject("static");
                staticGo.transform.SetParent(Root.transform, false);
                Canvas staticCanvas = staticGo.AddComponent<Canvas>();

                var vitalsGo = new GameObject("vitals");
                vitalsGo.transform.SetParent(dynamicGo.transform, false);
                vitalsGo.AddComponent<Image>();
                Vitals = vitalsGo.AddComponent<VitalsBarWidget>();
                Vitals.HpText = new GameObject("hp")
                    .AddComponent<TextMeshProUGUI>();
                Vitals.HpText.transform.SetParent(
                    vitalsGo.transform, false);

                var dockGo = new GameObject("questDock");
                dockGo.transform.SetParent(staticGo.transform, false);
                QuestDock = dockGo.AddComponent<QuestTrackerWidget>();
                QuestDock.Body = dockGo.AddComponent<TextMeshProUGUI>();

                var menuGo = new GameObject("menu");
                menuGo.transform.SetParent(staticGo.transform, false);
                Menu = menuGo.AddComponent<MenuButtonWidget>();
                Menu.MapText = menuGo.AddComponent<TextMeshProUGUI>();
                Menu.PingText = new GameObject("ping")
                    .AddComponent<TextMeshProUGUI>();

                View = Root.AddComponent<CoreHudView>();
                View.DynamicCanvas = dynamicCanvas;
                View.StaticCanvas = staticCanvas;
                View.Vitals = Vitals;
                View.QuestDock = QuestDock;
                View.MenuCluster = Menu;

                FeedBaseline(
                    Replication,
                    new EntityState
                    {
                        EntityId = 7,
                        Hp = 80,
                        MaxHp = 100,
                    });

                var vitals = new VitalsTracker(Replication, Model);
                var target = new TargetTracker(Replication, Model);
                var status = new StatusTracker(Replication, Model, Feed);
                var cooldown = new CooldownTracker(
                    Replication,
                    new FixedSkillLoadout(
                        new string?[] { "skill.a" }),
                    Model, Feed);
                System = new CoreHudSystem(
                    vitals, target, status, cooldown, Model, View,
                    new FakeWorld(), new FakeLatency(), () => 10UL);
            }

            public void Tick()
            {
                var time = new FrameTime(0.016f, 1.0, 0);
                System.Tick(in time);
            }

            public void Dispose()
            {
                if (Root != null)
                {
                    Object.Destroy(Root);
                }
            }
        }

        [Test]
        public void TestHudRebuildOncePerFrame()
        {
            using var rig = new HudRig();

            // First tick: baseline vitals + world context + (status rows
            // unchanged → no Statuses mark) collapse into ONE apply.
            rig.Tick();
            Assert.AreEqual(1, rig.View.ApplyCount);
            Assert.IsTrue(rig.System.AppliedThisTick);
            Assert.AreEqual(HudDirty.None, rig.Model.Dirty);

            // A tick with no state change performs no apply at all.
            rig.Tick();
            Assert.AreEqual(1, rig.View.ApplyCount);
            Assert.IsFalse(rig.System.AppliedThisTick);

            // Multiple dirty sections still land in one apply.
            FeedBaseline(
                rig.Replication,
                new EntityState
                {
                    EntityId = 7,
                    Hp = 40,
                    MaxHp = 100,
                },
                baselineId: 2);
            rig.Model.SetQuestDock("quest text");
            rig.Tick();
            Assert.AreEqual(2, rig.View.ApplyCount);
        }

        [Test]
        public void TestStaticDynamicCanvasSplit()
        {
            using var rig = new HudRig();
            rig.Tick(); // drains the baseline-init dirty flags
            Assert.AreEqual(1, rig.Vitals.ApplyCount);
            Assert.AreEqual(1, rig.QuestDock.ApplyCount);
            Assert.AreEqual(1, rig.Menu.ApplyCount);

            // A vitals-only change touches only the dynamic canvas widgets.
            FeedBaseline(
                rig.Replication,
                new EntityState
                {
                    EntityId = 7,
                    Hp = 10,
                    MaxHp = 100,
                },
                baselineId: 2);
            rig.Tick();
            Assert.AreEqual(2, rig.Vitals.ApplyCount);
            Assert.AreEqual(1, rig.QuestDock.ApplyCount);
            Assert.AreEqual(1, rig.Menu.ApplyCount);

            // A dock-only change touches only the static canvas widgets.
            rig.Model.SetQuestDock("dock");
            rig.Tick();
            Assert.AreEqual(2, rig.Vitals.ApplyCount);
            Assert.AreEqual(2, rig.QuestDock.ApplyCount);
            Assert.AreEqual(1, rig.Menu.ApplyCount);
        }

        [Test]
        public void TestHudValueUpdateZeroAlloc()
        {
            using var rig = new HudRig();
            rig.Tick(); // warmup: TMP buffers, graphic dirty state

            // The hot path: dirty vitals → single view apply, zero GC.
            FeedBaseline(
                rig.Replication,
                new EntityState
                {
                    EntityId = 7,
                    Hp = 77,
                    MaxHp = 100,
                },
                baselineId: 2);
            rig.Tick();
            Assert.AreEqual(2, rig.View.ApplyCount);

            FeedBaseline(
                rig.Replication,
                new EntityState
                {
                    EntityId = 7,
                    Hp = 76,
                    MaxHp = 100,
                },
                baselineId: 3);
            Assert.That(
                () => rig.Tick(),
                Is.Not.AllocatingGCMemory());
        }

        [Test]
        public void TestNonInteractiveRaycastOff()
        {
            var root = new GameObject("hudRaycast");
            try
            {
                var vitalsGo = new GameObject("vitals");
                vitalsGo.transform.SetParent(root.transform, false);
                Image image = vitalsGo.AddComponent<Image>();
                var textGo = new GameObject("t");
                textGo.transform.SetParent(vitalsGo.transform, false);
                TextMeshProUGUI text =
                    textGo.AddComponent<TextMeshProUGUI>();
                image.raycastTarget = true;
                text.raycastTarget = true;

                // Awake runs on AddComponent — display-only graphics flip off.
                vitalsGo.AddComponent<VitalsBarWidget>();
                Assert.IsFalse(image.raycastTarget);
                Assert.IsFalse(text.raycastTarget);

                var frameGo = new GameObject("target");
                frameGo.transform.SetParent(root.transform, false);
                Image frameBg = frameGo.AddComponent<Image>();
                frameBg.raycastTarget = true;
                frameGo.AddComponent<TargetFrameWidget>();
                Assert.IsFalse(frameBg.raycastTarget);

                var dockGo = new GameObject("dock");
                dockGo.transform.SetParent(root.transform, false);
                TextMeshProUGUI dockText =
                    dockGo.AddComponent<TextMeshProUGUI>();
                dockText.raycastTarget = true;
                var dock = dockGo.AddComponent<QuestTrackerWidget>();
                dock.Body = dockText;
                dock.Apply(new HudDataModel());
                Assert.IsFalse(dockText.raycastTarget);
            }
            finally
            {
                Object.Destroy(root);
            }
        }
    }
}
