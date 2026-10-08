using System;
using System.Collections.Generic;
using NUnit.Framework;
using ThinhThan.Core.Assets;
using ThinhThan.Core.Geometry;
using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.World;

namespace ThinhThan.Tests.PlayMode.WorldTransferPresentation
{
    /// <summary>
    /// IMP-018 client facet: 24-map registry, PERF-018 prewarm + region
    /// group load, transfer presentation (pending-excluded timeout),
    /// channel-switch result state, world sink dispatch, respawn intent.
    /// </summary>
    public sealed class WorldTransferPresentationTests
    {
        private sealed class ManualClock : IClock
        {
            public double Now
            {
                get;
                set;
            }

            public double NowSeconds
            {
                get
                {
                    return Now;
                }
            }

            public float UnscaledDeltaSeconds
            {
                get
                {
                    return 0.016f;
                }
            }
        }

        private sealed class FakeGroupLoad : IGroupLoad
        {
            public bool Done
            {
                get;
                set;
            }

            public bool Succeeded
            {
                get;
                set;
            }
        }

        private static TransferDestination Destination(
            string mapId = "map.lang_da.bo_ruong",
            uint channel = 3,
            string revision = "rev1")
        {
            return new TransferDestination(mapId, channel, null, revision);
        }

        private static DecodedFrame Frame(uint messageId, Google.Protobuf.IMessage payload)
        {
            var f = new DecodedFrame();
            f.MessageId = messageId;
            f.Payload = payload;
            return f;
        }

        // ---------------------------------------------------------------
        // Registry

        [Test]
        public void TestRegistryCoversAllTwentyFourWorldMaps()
        {
            Assert.AreEqual(24, WorldMapRegistry.Count);
            Assert.AreEqual(24, WorldMapRegistry.All.Count);

            var zones = new HashSet<string>();
            foreach (var rec in WorldMapRegistry.All)
            {
                Assert.IsTrue(rec.MapId.StartsWith("map.", StringComparison.Ordinal), rec.MapId);
                Assert.IsTrue(AddressableGroups.IsZoneKey(rec.ZoneKey), rec.ZoneKey);
                Assert.AreEqual("region." + rec.ZoneKey, rec.GroupKey);
                Assert.IsTrue(AddressableGroups.IsCanonicalName(rec.GroupKey), rec.GroupKey);
                Assert.AreEqual(rec.GroupKey, AddressableGroups.SpaceGroup(rec.MapId));
                Assert.IsTrue(rec.DisplayNameKey.StartsWith("loc.", StringComparison.Ordinal));
                Assert.IsTrue(rec.GeometryAssetPath.EndsWith(".geom.json", StringComparison.Ordinal));
                zones.Add(rec.ZoneKey);
            }
            Assert.AreEqual(6, zones.Count);
        }

        [Test]
        public void TestRegistryRejectsNonWorldIds()
        {
            Assert.IsFalse(WorldMapRegistry.TryGet("dungeon.dinh_lang_bo_hoang", out _));
            Assert.IsFalse(WorldMapRegistry.TryGet("map.pvp.duel_court", out _));
            Assert.IsFalse(WorldMapRegistry.TryGet("", out _));
            Assert.IsFalse(WorldMapRegistry.TryGet(null!, out _));
        }

        [Test]
        public void TestCameraBoundsUnionClampedToMapBounds()
        {
            var regions = new[]
            {
                new GeometryData.CameraRegion(1, -100, 0, 5000, 4000),
                new GeometryData.CameraRegion(2, 4000, 1000, 9000, 6000),
            };
            var geom = new GeometryData(
                "map.lang_da.bo_ruong", "WORLD_MAP", "IRRIGATION_BRAID", "rev1",
                76800, 18000,
                Array.Empty<GeometryData.Segment>(),
                regions,
                Array.Empty<GeometryData.Anchor>());

            WorldMapRegistry.CameraBounds(geom, out var minX, out var minY, out var maxX, out var maxY);
            Assert.AreEqual(0, minX);
            Assert.AreEqual(0, minY);
            Assert.AreEqual(9000, maxX);
            Assert.AreEqual(6000, maxY);

            var empty = new GeometryData(
                "map.lang_da.bo_ruong", "WORLD_MAP", "IRRIGATION_BRAID", "rev1",
                76800, 18000,
                Array.Empty<GeometryData.Segment>(),
                Array.Empty<GeometryData.CameraRegion>(),
                Array.Empty<GeometryData.Anchor>());
            WorldMapRegistry.CameraBounds(empty, out _, out _, out maxX, out maxY);
            Assert.AreEqual(76800, maxX);
            Assert.AreEqual(18000, maxY);
        }

        // ---------------------------------------------------------------
        // Context state

        [Test]
        public void TestWorldContextStateFeedsFromTransferDestination()
        {
            var ctx = new WorldContextState();
            Assert.IsFalse(ctx.HasMap);

            ctx.Apply(Destination());
            Assert.AreEqual("map.lang_da.bo_ruong", ctx.MapId);
            Assert.AreEqual(3u, ctx.ChannelIndex);
            Assert.AreEqual("rev1", ctx.ContentRevision);
            Assert.AreEqual("loc.world_map.lang_da.bo_ruong", ctx.DisplayNameKey);
            Assert.AreEqual("map.lang_da.bo_ruong", ctx.DisplayName);

            var localized = new WorldContextState(_ => "Bờ Ruộng");
            localized.Apply(Destination());
            Assert.AreEqual("Bờ Ruộng", localized.DisplayName);

            ctx.Clear();
            Assert.IsFalse(ctx.HasMap);
            Assert.AreEqual("", ctx.MapId);
            Assert.AreEqual(0u, ctx.ChannelIndex);
        }

        // ---------------------------------------------------------------
        // Map load pipeline (PERF-018)

        [Test]
        public void TestMapLoadPrewarmsPoolsAndGroup()
        {
            var loaded = new List<string>();
            var pipeline = new MapLoadPipeline(group =>
            {
                loaded.Add(group);
                return new FakeGroupLoad { Done = true, Succeeded = true };
            });

            var actors = new Pool<object>(() => new object());
            var projectiles = new Pool<object>(() => new object());
            var vfx = new Pool<object>(() => new object());
            var text = new Pool<object>(() => new object());
            var uiRows = new Pool<object>(() => new object());
            pipeline.AddPool(actors, 24);
            pipeline.AddPool(projectiles, 48);
            pipeline.AddPool(vfx, 16);
            pipeline.AddPool(text, 32);
            pipeline.AddPool(uiRows, 12);

            Assert.IsTrue(pipeline.Begin("map.lang_da.bo_ruong"));
            Assert.IsTrue(pipeline.Ready);
            Assert.AreEqual(MapLoadPipeline.Status.Ready, pipeline.Current);
            Assert.AreEqual("region.lang_da", pipeline.ActiveGroupKey);
            Assert.AreEqual(new[] { "region.lang_da" }, loaded);

            Assert.AreEqual(24, actors.IdleCount);
            Assert.AreEqual(48, projectiles.IdleCount);
            Assert.AreEqual(16, vfx.IdleCount);
            Assert.AreEqual(32, text.IdleCount);
            Assert.AreEqual(12, uiRows.IdleCount);
        }

        [Test]
        public void TestMapLoadFailureIsFailedNotFallback()
        {
            var pipeline = new MapLoadPipeline(_ => new FakeGroupLoad { Done = true, Succeeded = false });
            Assert.IsFalse(pipeline.Begin("map.lang_da.bo_ruong"));
            Assert.AreEqual(MapLoadPipeline.Status.Failed, pipeline.Current);
            Assert.IsFalse(pipeline.Ready);
        }

        [Test]
        public void TestMapLoadUnknownMapFailsWithoutMutation()
        {
            var loaded = new List<string>();
            var pipeline = new MapLoadPipeline(group =>
            {
                loaded.Add(group);
                return new FakeGroupLoad { Done = true, Succeeded = true };
            });
            var pool = new Pool<object>(() => new object());
            pipeline.AddPool(pool, 8);

            Assert.IsFalse(pipeline.Begin("map.unknown.nowhere"));
            Assert.AreEqual(MapLoadPipeline.Status.Failed, pipeline.Current);
            Assert.AreEqual(0, loaded.Count);
            Assert.AreEqual(0, pool.IdleCount);
        }

        [Test]
        public void TestMapLoadReadyGateResetOnNewTransfer()
        {
            var pipeline = new MapLoadPipeline(_ => new FakeGroupLoad { Done = true, Succeeded = true });
            Assert.IsTrue(pipeline.Begin("map.lang_da.bo_ruong"));
            Assert.IsTrue(pipeline.Ready);
            pipeline.Reset();
            Assert.AreEqual(MapLoadPipeline.Status.Idle, pipeline.Current);
            Assert.IsFalse(pipeline.Ready);
            Assert.AreEqual("", pipeline.ActiveGroupKey);
        }

        [Test]
        public void TestMapLoadAsyncHandleSettlesViaPoll()
        {
            var handle = new FakeGroupLoad();
            var pipeline = new MapLoadPipeline(_ => handle);
            Assert.IsTrue(pipeline.Begin("map.lang_da.bo_ruong"));
            Assert.AreEqual(MapLoadPipeline.Status.Loading, pipeline.Current);
            Assert.IsFalse(pipeline.Ready);

            handle.Succeeded = true;
            handle.Done = true;
            pipeline.Poll();
            Assert.AreEqual(MapLoadPipeline.Status.Ready, pipeline.Current);
            Assert.IsTrue(pipeline.Ready);

            var failed = new FakeGroupLoad();
            var second = new MapLoadPipeline(_ => failed);
            Assert.IsTrue(second.Begin("map.lang_da.bo_ruong"));
            failed.Done = true;
            second.Poll();
            Assert.AreEqual(MapLoadPipeline.Status.Failed, second.Current);

            var nullLoader = new MapLoadPipeline(_ => null);
            Assert.IsFalse(nullLoader.Begin("map.lang_da.bo_ruong"));
            Assert.AreEqual(MapLoadPipeline.Status.Failed, nullLoader.Current);
        }

        // ---------------------------------------------------------------
        // Transfer presentation

        [Test]
        public void TestTransferTimeoutExcludesPlacementPending()
        {
            var clock = new ManualClock();
            var t = new TransferPresentation(clock);
            t.Begin(Destination(), budgetSeconds: 30.0);

            clock.Now = 10.0;
            t.SetPlacementPending(true);
            clock.Now = 60.0; // 50 s pending
            Assert.IsTrue(t.PlacementPending);
            Assert.AreEqual(TransferPresentation.State.Loading, t.Current);
            Assert.AreEqual(10.0, t.ElapsedSeconds, 1e-6);
            Assert.IsFalse(t.Expired);

            t.SetPlacementPending(false);
            clock.Now = 79.0; // 29 s active — still inside the 30 s budget
            Assert.IsFalse(t.Expired);
            clock.Now = 80.0; // 30 s active — budget spent
            Assert.IsTrue(t.Expired);
        }

        [Test]
        public void TestTransferFailRaisesNoticeState()
        {
            var clock = new ManualClock();
            var t = new TransferPresentation(clock);
            t.Begin(Destination(), 30.0);
            t.Fail();
            Assert.AreEqual(TransferPresentation.State.Failed, t.Current);
            Assert.IsFalse(t.Expired);
            t.Clear();
            Assert.AreEqual(TransferPresentation.State.None, t.Current);
        }

        [Test]
        public void TestTransferSettlingTransition()
        {
            var clock = new ManualClock();
            var t = new TransferPresentation(clock);
            t.Begin(Destination(), 30.0);
            t.BeginSettling();
            Assert.AreEqual(TransferPresentation.State.Settling, t.Current);
            t.Clear();
            Assert.AreEqual(TransferPresentation.State.None, t.Current);
        }

        // ---------------------------------------------------------------
        // Channel switch

        [Test]
        public void TestChannelSwitchResultStates()
        {
            var cs = new ChannelSwitchPresentation();
            cs.BeginRequest(4);
            Assert.AreEqual(ChannelSwitchPresentation.State.Pending, cs.Current);
            Assert.AreEqual(4u, cs.TargetChannelIndex);

            cs.Apply(new S2CChannelSwitchResult
            {
                Result = new OperationResult { Status = ResultStatus.Success },
                TargetChannelIndex = 4,
            });
            Assert.AreEqual(ChannelSwitchPresentation.State.Succeeded, cs.Current);
            Assert.AreEqual(4u, cs.TargetChannelIndex);

            cs.BeginRequest(5);
            cs.Apply(new S2CChannelSwitchResult
            {
                Result = new OperationResult
                {
                    Status = ResultStatus.Error,
                    ErrorCode = ErrorCode.CooldownActive,
                },
                RetryAfterMs = 5000,
            });
            Assert.AreEqual(ChannelSwitchPresentation.State.Failed, cs.Current);
            Assert.AreEqual(5000u, cs.RetryAfterMs);
            Assert.AreEqual(ErrorCode.CooldownActive, cs.FailureCode);
        }

        // ---------------------------------------------------------------
        // World sink dispatch (110 / 116 / 204)

        [Test]
        public void TestWorldSinkApplierDispatch()
        {
            var cs = new ChannelSwitchPresentation();
            S2CInteractResult? interact = null;
            S2CActionRejected? rejected = null;
            var applier = new WorldSinkApplier(
                cs, r => interact = r, r => rejected = r);

            cs.BeginRequest(2);
            applier.Apply(Frame(110, new S2CChannelSwitchResult
            {
                Result = new OperationResult { Status = ResultStatus.Success },
                TargetChannelIndex = 2,
            }));
            Assert.AreEqual(ChannelSwitchPresentation.State.Succeeded, cs.Current);

            var ir = new S2CInteractResult
            {
                Result = new OperationResult { Status = ResultStatus.Success },
                InteractKind = InteractKind.NpcService,
                TargetId = "npc.test",
            };
            applier.Apply(Frame(116, ir));
            Assert.AreSame(ir, interact);

            var ar = new S2CActionRejected
            {
                ClientSeq = 7,
                RequestMessageId = 208,
                ErrorCode = ErrorCode.InvalidState,
            };
            applier.Apply(Frame(204, ar));
            Assert.AreSame(ar, rejected);

            // Unknown ids and empty frames are ignored.
            applier.Apply(Frame(105, new S2CTransferPrepare()));
            applier.Apply(new DecodedFrame());
            Assert.AreSame(ar, rejected);
        }

        // ---------------------------------------------------------------
        // Respawn intent

        [Test]
        public void TestRespawnPresentationGatesAndReArms()
        {
            var sent = 0;
            var r = new RespawnPresentation(() => sent++);

            Assert.IsFalse(r.RequestRespawn());
            Assert.AreEqual(0, sent);

            r.OnDead();
            Assert.IsTrue(r.CanRequest);
            Assert.IsTrue(r.RequestRespawn());
            Assert.AreEqual(1, sent);
            Assert.IsFalse(r.RequestRespawn());
            Assert.AreEqual(1, sent);

            r.OnRejected();
            Assert.IsTrue(r.CanRequest);
            Assert.IsTrue(r.RequestRespawn());
            Assert.AreEqual(2, sent);

            r.OnAlive();
            Assert.IsFalse(r.CanRequest);
            Assert.IsFalse(r.RequestRespawn());
        }
    }
}
