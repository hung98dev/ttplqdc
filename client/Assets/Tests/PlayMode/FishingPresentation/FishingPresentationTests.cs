using System.Collections.Generic;
using System.Threading;
using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.LifeSkills.Fishing;
using ThinhThan.UI.LifeSkills.Fishing;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.FishingPresentation
{
    /// <summary>
    /// IMP-058 client facet: CAST / HOOK intents mint UUIDv7 operation
    /// ids and ship the right 103 fields; recorded 116 results drive
    /// the cast/window/result projection and the daily counter; a rare
    /// catch raises the one merged PHAT_HIEN peak; the panel projects
    /// every row without scene objects.
    /// </summary>
    public sealed class FishingPresentationTests
    {
        private sealed class FakeSender : IFishingSender
        {
            public readonly List<(uint MessageId, IMessage Payload)> Sent =
                new List<(uint, IMessage)>();

            public Awaitable<ulong> SendAsync(
                uint messageId, IMessage payload, CancellationToken cancel)
            {
                Sent.Add((messageId, payload));
                var src = new AwaitableCompletionSource<ulong>();
                src.SetResult((ulong)Sent.Count);
                return src.Awaitable;
            }
        }

        private static S2CInteractResult Result(
            InteractKind kind, ResultStatus status, ErrorCode code,
            params string[] granted)
        {
            var r = new S2CInteractResult
            {
                Result = new OperationResult
                {
                    Status = status,
                    ErrorCode = code,
                    OperationId = ByteString.CopyFrom(new byte[16]),
                },
                InteractKind = kind,
                TargetId = "fishing_spot.map.lang_da.ben_da.01",
            };
            foreach (var id in granted)
            {
                r.Granted.Add(new ItemQuantity { ItemId = id, Quantity = 1 });
            }
            return r;
        }

        [Test]
        public void CastIntentShips103Cast()
        {
            var sender = new FakeSender();
            var intents = new FishingIntents(sender);
            _ = intents.RequestCast(
                "fishing_spot.map.lang_da.ben_da.01",
                CancellationToken.None);
            Assert.AreEqual(1, sender.Sent.Count);
            Assert.AreEqual(103u, sender.Sent[0].MessageId);
            var req = (C2SInteract)sender.Sent[0].Payload;
            Assert.AreEqual(InteractKind.Cast, req.InteractKind);
            Assert.AreEqual(
                "fishing_spot.map.lang_da.ben_da.01", req.TargetId);
            Assert.AreEqual(16, req.OperationId.Length);
            var op = req.OperationId.ToByteArray();
            // UUIDv7 version nibble.
            Assert.AreEqual(0x70, op[6] & 0xF0);
        }

        [Test]
        public void HookIntentShips103Hook()
        {
            var sender = new FakeSender();
            var intents = new FishingIntents(sender);
            _ = intents.RequestHook(
                "fishing_spot.map.lang_da.ben_da.01",
                CancellationToken.None);
            Assert.AreEqual(1, sender.Sent.Count);
            var req = (C2SInteract)sender.Sent[0].Payload;
            Assert.AreEqual(InteractKind.Hook, req.InteractKind);
            Assert.AreEqual(16, req.OperationId.Length);
        }

        [Test]
        public void SinkRoutes116AndIgnoresForeignKinds()
        {
            var p = new global::ThinhThan.Systems.LifeSkills.Fishing
                .FishingPresentation();
            var sink = new FishingSinkApplier(p);
            sink.Apply(new DecodedFrame
            {
                MessageId = 116,
                Payload = Result(InteractKind.Talk,
                    ResultStatus.Success, ErrorCode.Unspecified),
            });
            Assert.AreEqual(
                global::ThinhThan.Systems.LifeSkills.Fishing
                    .FishingPresentation.ActionState.Idle, p.Cast);
            sink.Apply(new DecodedFrame
            {
                MessageId = 116,
                Payload = Result(InteractKind.Cast,
                    ResultStatus.Success, ErrorCode.Unspecified),
            });
            Assert.AreEqual(
                global::ThinhThan.Systems.LifeSkills.Fishing
                    .FishingPresentation.CastState.Casting, p.CastFlow);
        }

        [Test]
        public void CastSuccessOpensThenResolvesWindow()
        {
            var p = new global::ThinhThan.Systems.LifeSkills.Fishing
                .FishingPresentation();
            p.BeginRequest(InteractKind.Cast);
            Assert.AreEqual(
                global::ThinhThan.Systems.LifeSkills.Fishing
                    .FishingPresentation.ActionState.Pending, p.Cast);
            p.Apply(Result(InteractKind.Cast, ResultStatus.Success,
                ErrorCode.Unspecified));
            Assert.AreEqual(
                global::ThinhThan.Systems.LifeSkills.Fishing
                    .FishingPresentation.CastState.Casting, p.CastFlow);
            // Timing hint opens the window; the server owns timing.
            p.OpenHookWindow();
            Assert.AreEqual(
                global::ThinhThan.Systems.LifeSkills.Fishing
                    .FishingPresentation.CastState.HookWindow, p.CastFlow);
            p.Apply(Result(InteractKind.Hook, ResultStatus.Success,
                ErrorCode.Unspecified, "item.material.ca_bong"));
            Assert.AreEqual(
                global::ThinhThan.Systems.LifeSkills.Fishing
                    .FishingPresentation.CastState.Resolved, p.CastFlow);
            Assert.AreEqual(1, p.Granted.Count);
            Assert.AreEqual("item.material.ca_bong", p.Granted[0].ItemId);
            Assert.AreEqual(1, p.DailyCatchCount);
        }

        [Test]
        public void HookFailureResolvesWithoutCatch()
        {
            var p = new global::ThinhThan.Systems.LifeSkills.Fishing
                .FishingPresentation();
            p.Apply(Result(InteractKind.Cast, ResultStatus.Success,
                ErrorCode.Unspecified));
            p.Apply(Result(InteractKind.Hook, ResultStatus.Error,
                ErrorCode.StateConflict));
            Assert.AreEqual(
                global::ThinhThan.Systems.LifeSkills.Fishing
                    .FishingPresentation.ActionState.Failed, p.Hook);
            Assert.AreEqual(ErrorCode.StateConflict, p.FailureCode);
            Assert.AreEqual(0, p.Granted.Count);
            Assert.AreEqual(0, p.DailyCatchCount);
        }

        [Test]
        public void DailyCounterCapsAtFifty()
        {
            var p = new global::ThinhThan.Systems.LifeSkills.Fishing
                .FishingPresentation();
            for (int i = 0; i < 55; i++)
            {
                p.Apply(Result(InteractKind.Hook, ResultStatus.Success,
                    ErrorCode.Unspecified, "item.material.ca_bong"));
            }
            Assert.AreEqual(
                global::ThinhThan.Systems.LifeSkills.Fishing
                    .FishingPresentation.DailyCatchCap, p.DailyCatchCount);
        }

        [Test]
        public void RareCatchRaisesSinglePeak()
        {
            var p = new global::ThinhThan.Systems.LifeSkills.Fishing
                .FishingPresentation();
            p.Apply(Result(InteractKind.Hook, ResultStatus.Success,
                ErrorCode.Unspecified,
                "item.material.ca_chep_hoa_rong"));
            Assert.IsTrue(p.RarePeak);
            p.ConsumeRarePeak();
            Assert.IsFalse(p.RarePeak);
        }

        [Test]
        public void SpectatorSplashFlag()
        {
            var p = new global::ThinhThan.Systems.LifeSkills.Fishing
                .FishingPresentation();
            p.RaiseSpectatorSplash();
            Assert.IsTrue(p.SpectatorSplash);
            p.ConsumeSpectatorSplash();
            Assert.IsFalse(p.SpectatorSplash);
        }

        [Test]
        public void PanelProjectsWindowAndCounter()
        {
            var go = new GameObject("panel");
            try
            {
                var panel = go.AddComponent<FishingPanel>();
                var p = new global::ThinhThan.Systems.LifeSkills.Fishing
                    .FishingPresentation();
                var presenter = new FishingPresenter(panel, p);
                p.Apply(Result(InteractKind.Cast, ResultStatus.Success,
                    ErrorCode.Unspecified));
                p.OpenHookWindow();
                presenter.WindowElapsedSeconds = 0.40f;
                presenter.Apply();
                Assert.IsTrue(panel.LastModel.HookWindowOpen);
                Assert.AreEqual(0.40f,
                    panel.LastModel.WindowElapsedSeconds);
                p.Apply(Result(InteractKind.Hook, ResultStatus.Success,
                    ErrorCode.Unspecified, "item.material.tom_song"));
                presenter.Apply();
                Assert.AreEqual(1, panel.LastModel.DailyCatchCount);
                Assert.AreEqual(50, panel.LastModel.DailyCatchCap);
                Assert.AreEqual(1, panel.LastModel.Catches.Count);
                Assert.AreEqual("item.material.tom_song",
                    panel.LastModel.Catches[0].ItemId);
            }
            finally
            {
                Object.Destroy(go);
            }
        }
    }
}
