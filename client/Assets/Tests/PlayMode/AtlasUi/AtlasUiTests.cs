using System.Threading;
using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Atlas;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.AtlasUi
{
    /// <summary>
    /// AtlasUi tests (packet § Tests): 518 snapshot folds into
    /// Locked/Seen/Studied/Mastered page states, 505 acknowledges the
    /// reached tier and clears the journal marker, unreached-tier
    /// verdicts leave state untouched, and Apply runs once per version.
    /// </summary>
    public sealed class AtlasUiTests
    {
        private const string PageA = "atlas.page.quai_dam.lang_da.dom_dom_ma";
        private const string PageB = "atlas.page.co_vat.ca_bong";
        private const string PageC = "atlas.page.di_tich.quy_nhap_trang";
        private const string PageD = "atlas.page.hon_giam.dom_dom_ma";

        private static DecodedFrame Frame(uint id, object payload)
        {
            return new DecodedFrame
            {
                MessageId = id,
                Payload = (IMessage)payload,
            };
        }

        private static S2CAtlasState Snapshot(ulong revision,
            params AtlasPageView[] pages)
        {
            var s = new S2CAtlasState { AtlasRevision = revision };
            foreach (AtlasPageView p in pages)
            {
                s.Pages.Add(p);
            }
            return s;
        }

        private static AtlasPageView Page(string id, ulong counter,
            uint reached, params uint[] acked)
        {
            var v = new AtlasPageView
            {
                AtlasPageId = id,
                Counter = counter,
                ReachedTier = reached,
            };
            for (uint t = 1; t <= reached; t++)
            {
                v.Tiers.Add(new AtlasTierView
                {
                    Tier = t,
                    AcknowledgedAtMs =
                        Contains(acked, t) ? 1700000000000L : 0L,
                });
            }
            return v;
        }

        private static bool Contains(uint[] xs, uint v)
        {
            foreach (uint x in xs)
            {
                if (x == v)
                {
                    return true;
                }
            }
            return false;
        }

        [Test]
        public void SnapshotFoldsSeenStudiedMasteredStates()
        {
            var a = new AtlasApplier();
            a.Apply(Frame(AtlasApplier.S2CAtlasStateId, Snapshot(7,
                Page(PageA, 1, 1),      // Seen, unacknowledged
                Page(PageB, 10, 2, 1),  // Studied, T1 acked
                Page(PageC, 10, 3, 1, 2, 3),  // Mastered, all acked
                Page(PageD, 0, 0))));   // Locked

            Assert.AreEqual(7UL, a.State.Revision);
            Assert.AreEqual(4, a.State.Pages.Count);
            Assert.AreEqual(AtlasTierName.Seen,
                a.State.Pages[PageA].TierName);
            Assert.AreEqual(AtlasTierName.Studied,
                a.State.Pages[PageB].TierName);
            Assert.AreEqual(AtlasTierName.Mastered,
                a.State.Pages[PageC].TierName);
            Assert.AreEqual(AtlasTierName.Locked,
                a.State.Pages[PageD].TierName);
            Assert.IsTrue(a.State.Pages[PageA].HasUnacknowledgedTier());
            Assert.IsTrue(a.State.Pages[PageB].HasUnacknowledgedTier());
            Assert.IsFalse(a.State.Pages[PageC].HasUnacknowledgedTier());
            Assert.AreEqual("1 / 10",
                a.State.Pages[PageA].ProgressText());
            Assert.AreEqual("10 / 50",
                a.State.Pages[PageB].ProgressText());
            Assert.AreEqual("Mastered",
                a.State.Pages[PageC].ProgressText());
            Assert.AreEqual("0 / 1",
                a.State.Pages[PageD].ProgressText());
        }

        [Test]
        public void ClaimAcknowledgeClearsMarkerOn505()
        {
            var a = new AtlasApplier();
            a.Apply(Frame(AtlasApplier.S2CAtlasStateId, Snapshot(1,
                Page(PageA, 1, 1))));
            Assert.IsTrue(a.State.Pages[PageA].HasUnacknowledgedTier());

            a.Apply(Frame(AtlasApplier.S2CAtlasClaimResultId,
                new S2CAtlasClaimResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Success,
                    },
                    AtlasPageId = PageA,
                    Tier = 1,
                }));

            Assert.IsFalse(a.State.Pages[PageA].HasUnacknowledgedTier());
            Assert.IsNotNull(a.LastClaimResult);
            Assert.AreEqual(PageA, a.LastClaimResult!.AtlasPageId);
            Assert.AreEqual(2UL, a.Version);
        }

        [Test]
        public void UnreachedTierVerdictLeavesStateUntouched()
        {
            var a = new AtlasApplier();
            a.Apply(Frame(AtlasApplier.S2CAtlasStateId, Snapshot(1,
                Page(PageA, 1, 1))));
            a.Apply(Frame(AtlasApplier.S2CAtlasClaimResultId,
                new S2CAtlasClaimResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Error,
                        ErrorCode = ErrorCode.AtlasTierNotReached,
                    },
                    AtlasPageId = PageA,
                    Tier = 3,
                }));

            Assert.IsTrue(a.State.Pages[PageA].HasUnacknowledgedTier());
            Assert.AreEqual(1u, a.State.Pages[PageA].ReachedTier);
            Assert.AreEqual(ErrorCode.AtlasTierNotReached,
                a.LastClaimResult!.Result.ErrorCode);
        }

        [Test]
        public void ApplyRunsOncePerVersion()
        {
            var a = new AtlasApplier();
            a.Apply(Frame(AtlasApplier.S2CAtlasStateId, Snapshot(3,
                Page(PageA, 1, 1))));
            Assert.AreEqual(1UL, a.Version);
            a.Apply(Frame(AtlasApplier.S2CAtlasClaimResultId,
                new S2CAtlasClaimResult
                {
                    Result = new OperationResult
                    { Status = ResultStatus.Success },
                    AtlasPageId = PageA,
                    Tier = 1,
                }));
            Assert.AreEqual(2UL, a.Version);
        }

        [Test]
        public void AcknowledgeIntentSendsAtlasClaim()
        {
            var sender = new FakeSender();
            var intents = new AtlasIntents(sender,
                () => new byte[] {1, 2, 3});
            _ = intents.Acknowledge(PageA, 2,
                CancellationToken.None);
            // Unity Awaitable runs synchronously on the main thread
            // until first suspension; FakeSender completes inline.
            Assert.AreEqual(AtlasApplier.C2SAtlasClaim,
                sender.LastMessageId);
            var req = (C2SAtlasClaim)sender.LastPayload!;
            Assert.AreEqual(PageA, req.AtlasPageId);
            Assert.AreEqual(2u, req.Tier);
            Assert.AreEqual(
                new byte[] {1, 2, 3},
                req.OperationId.ToByteArray());
        }

        private sealed class FakeSender : IAtlasSender
        {
            public uint LastMessageId;
            public IMessage? LastPayload;

            public Awaitable<ulong> SendAsync(uint messageId,
                IMessage payload, CancellationToken cancel)
            {
                LastMessageId = messageId;
                LastPayload = payload;
                var src = new AwaitableCompletionSource<ulong>();
                src.SetResult(1UL);
                return src.Awaitable;
            }
        }
    }
}
