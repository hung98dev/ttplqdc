using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Rewards;
using ThinhThan.UI.Rewards;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.Tests.PlayMode.RewardClaimUi
{
    /// <summary>
    /// IMP-010 PlayMode: 434 claim list render, row Claim retry intent,
    /// 441 delta row removal (overflow claim materialized → leaves
    /// pending list), 409 success with granted lines, and authoritative
    /// rejection leaving state untouched.
    /// </summary>
    public sealed class RewardClaimUiTests
    {
        private sealed class FakeIntents : IRewardClaimIntents
        {
            public byte[]? LastClaimId;
            public (uint offset, uint limit)? LastList;

            public UnityEngine.Awaitable<byte[]> RequestClaim(
                byte[] rewardClaimId,
                System.Threading.CancellationToken cancel)
            {
                LastClaimId = rewardClaimId;
                var src = new UnityEngine.AwaitableCompletionSource<byte[]>();
                src.SetResult(new byte[16]);
                return src.Awaitable;
            }

            public UnityEngine.Awaitable<byte[]> RequestList(
                uint offset, uint limit,
                System.Threading.CancellationToken cancel)
            {
                LastList = (offset, limit);
                var src = new UnityEngine.AwaitableCompletionSource<byte[]>();
                src.SetResult(new byte[16]);
                return src.Awaitable;
            }
        }

        private static DecodedFrame Frame(uint id, IMessage payload)
        {
            return new DecodedFrame
            {
                MessageId = id,
                Payload = payload,
            };
        }

        private static byte[] Id16(byte seed)
        {
            var b = new byte[16];
            b[15] = seed;
            return b;
        }

        private static RewardClaimView Claim(byte seed, string source)
        {
            return new RewardClaimView
            {
                RewardClaimId = ByteString.CopyFrom(Id16(seed)),
                SourceType = source,
                SourceReference = "ref-" + seed,
                RewardSlot = "slot.a",
                State = RewardClaimState.Pending,
                Lines =
                {
                    new RewardClaimLine
                    {
                        ItemId = "item.mat.ore",
                        Quantity = 3,
                    },
                },
            };
        }

        private static RewardClaimPanel NewPanel()
        {
            var go = new GameObject("panel", typeof(RectTransform),
                typeof(RewardClaimPanel));
            var root = new GameObject("grid", typeof(RectTransform));
            root.transform.SetParent(go.transform, false);
            var panel = go.GetComponent<RewardClaimPanel>();
            panel.GridRoot = root.GetComponent<RectTransform>();
            return panel;
        }

        private static RewardClaimRowView RowWithButton(
            RewardClaimPanel panel, int i)
        {
            RewardClaimRowView row = panel.CellAt(i);
            if (row.ClaimButton == null)
            {
                var btn = new GameObject("claim", typeof(Button),
                    typeof(RectTransform));
                btn.transform.SetParent(row.transform, false);
                row.ClaimButton = btn.GetComponent<Button>();
            }
            return row;
        }

        [Test]
        public void ClaimListRendersOldestClaims()
        {
            var applier = new RewardClaimApplier();
            applier.Apply(Frame(WireIds.S2CRewardClaimsState,
                new S2CRewardClaimsState
                {
                    ClaimsRevision = 5,
                    TotalCount = 2,
                    Cap = 100,
                    Claims =
                    {
                        Claim(1, "LEVEL_MILESTONE"),
                        Claim(2, "DUNGEON_CLEAR"),
                    },
                }));

            Assert.AreEqual(5UL, applier.State.ClaimsRevision);
            Assert.AreEqual(2, applier.State.Claims.Count);
            Assert.AreEqual("LEVEL_MILESTONE",
                applier.State.Claims[0].SourceType);

            RewardClaimPanel panel = NewPanel();
            panel.Apply(applier.State);
            Assert.AreEqual(2, panel.LiveCount);
            Assert.AreEqual(1, panel.ApplyCount);
        }

        [Test]
        public void ClaimRowRetryFiresIntent()
        {
            var applier = new RewardClaimApplier();
            applier.Apply(Frame(WireIds.S2CRewardClaimsState,
                new S2CRewardClaimsState
                {
                    ClaimsRevision = 1,
                    TotalCount = 1,
                    Cap = 100,
                    Claims = { Claim(7, "COMPENSATION") },
                }));
            var intents = new FakeIntents();
            RewardClaimPanel panel = NewPanel();
            panel.Intents = intents;
            panel.Apply(applier.State);
            RewardClaimRowView row = RowWithButton(panel, 0);
            // Rewire: the listener was bound in Apply before the
            // button existed — re-apply so the listener attaches.
            panel.Apply(applier.State);

            row.ClaimButton!.onClick.Invoke();

            Assert.IsNotNull(intents.LastClaimId);
            CollectionAssert.AreEqual(Id16(7), intents.LastClaimId);
            Assert.AreEqual(2, panel.ApplyCount);
        }

        [Test]
        public void DeltaRemovesMaterializedClaimRow()
        {
            var applier = new RewardClaimApplier();
            applier.Apply(Frame(WireIds.S2CRewardClaimsState,
                new S2CRewardClaimsState
                {
                    ClaimsRevision = 2,
                    TotalCount = 2,
                    Cap = 100,
                    Claims =
                    {
                        Claim(1, "LEVEL_MILESTONE"),
                        Claim(2, "OVERFLOW_INVENTORY"),
                    },
                }));

            // A successful claim materializes the overflow grant; the
            // delta removes the row (overflow → inventory delivery).
            applier.Apply(Frame(WireIds.S2CRewardClaimDelta,
                new S2CRewardClaimDelta
                {
                    ClaimsRevision = 3,
                    TotalCount = 1,
                    Removed =
                    {
                        ByteString.CopyFrom(Id16(2)),
                    },
                }));

            Assert.AreEqual(3UL, applier.State.ClaimsRevision);
            Assert.AreEqual(1, applier.State.Claims.Count);
            CollectionAssert.AreEqual(Id16(1),
                applier.State.Claims[0].RewardClaimId);

            RewardClaimPanel panel = NewPanel();
            panel.Apply(applier.State);
            panel.Apply(applier.State);
            Assert.AreEqual(1, panel.LiveCount);
            Assert.IsTrue(panel.CellAt(0).gameObject.activeSelf);
        }

        [Test]
        public void ClaimSuccessCarriesGrantedLines()
        {
            var applier = new RewardClaimApplier();
            var grant = new ItemGrant
            {
                ItemInstanceId = ByteString.CopyFrom(Id16(9)),
                ItemId = "item.mat.ore",
            };
            applier.Apply(Frame(WireIds.S2CRewardClaimResult,
                new S2CRewardClaimResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Success,
                    },
                    RewardClaimId = ByteString.CopyFrom(Id16(2)),
                    Granted = { grant },
                    ClaimState = RewardClaimState.Claimed,
                }));

            Assert.IsNotNull(applier.LastClaimResult);
            Assert.AreEqual(ResultStatus.Success,
                applier.LastClaimResult!.Result.Status);
            Assert.AreEqual(1, applier.LastClaimResult.Granted.Count);
            Assert.AreEqual(RewardClaimState.Claimed,
                applier.LastClaimResult.ClaimState);
        }

        [Test]
        public void RejectVerdictLeavesStateUntouched()
        {
            var applier = new RewardClaimApplier();
            applier.Apply(Frame(WireIds.S2CRewardClaimsState,
                new S2CRewardClaimsState
                {
                    ClaimsRevision = 4,
                    TotalCount = 1,
                    Cap = 100,
                    Claims = { Claim(3, "LEVEL_MILESTONE") },
                }));
            ulong before = applier.State.ClaimsRevision;

            applier.Apply(Frame(WireIds.S2CRewardClaimResult,
                new S2CRewardClaimResult
                {
                    Result = new OperationResult
                    {
                        Status = ResultStatus.Error,
                        ErrorCode = ErrorCode.ClaimCapReached,
                    },
                    RewardClaimId = ByteString.CopyFrom(Id16(3)),
                }));

            Assert.AreEqual(before, applier.State.ClaimsRevision);
            Assert.AreEqual(1, applier.State.Claims.Count);
            Assert.AreEqual(ErrorCode.ClaimCapReached,
                applier.LastClaimResult!.Result.ErrorCode);
        }
    }
}
