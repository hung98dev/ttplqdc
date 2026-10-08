using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Social;
using ThinhThan.UI.Social;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.SocialChatUi
{
    /// <summary>
    /// IMP-034 PlayMode: the social projection over S2C frames —
    /// 616 full snapshot then deltas (UPSERT/REMOVED + request lists),
    /// 619 block replace, 601 chat feed cap, 612 pending push, 654/655/633
    /// results, and the once-per-frame presenter dirty flag.
    /// </summary>
    public sealed class SocialChatUiTests
    {
        private static DecodedFrame Frame(uint id, IMessage payload)
        {
            return new DecodedFrame
            {
                MessageId = id,
                Payload = payload,
            };
        }

        private static ByteString Id16(byte seed)
        {
            var b = new byte[16];
            b[15] = seed;
            return ByteString.CopyFrom(b);
        }

        [Test]
        public void FriendStateSnapshotThenDeltas()
        {
            var a = new SocialApplier();
            var entry1 = new FriendEntry
            {
                FriendCharacterId = Id16(1),
                DisplayName = "alpha",
                Change = FriendChange.Upsert,
                OnlineState = OnlineState.Online,
            };
            var entry2 = new FriendEntry
            {
                FriendCharacterId = Id16(2),
                DisplayName = "beta",
                Change = FriendChange.Upsert,
                OnlineState = OnlineState.Offline,
            };
            a.Apply(Frame(616, new S2CFriendState
            {
                FullSnapshot = true,
                Entries = { entry1, entry2 },
            }));
            Assert.AreEqual(2, a.State.Friends.Count);

            // Delta: remove one, upsert a new one, and a pending list.
            a.Apply(Frame(616, new S2CFriendState
            {
                Entries =
                {
                    new FriendEntry
                    {
                        FriendCharacterId = Id16(1),
                        Change = FriendChange.Removed,
                    },
                    new FriendEntry
                    {
                        FriendCharacterId = Id16(3),
                        DisplayName = "gamma",
                        Change = FriendChange.Upsert,
                    },
                },
                IncomingRequests =
                {
                    new FriendRequestView
                    {
                        RequesterCharacterId = Id16(9),
                        DisplayName = "iota",
                        ExpiresAtMs = 100,
                    },
                },
            }));
            Assert.AreEqual(2, a.State.Friends.Count);
            Assert.IsFalse(a.State.Friends.ContainsKey(
                SocialState.Key(Id16(1))));
            Assert.AreEqual(1, a.State.Incoming.Count);
            Assert.AreEqual("iota", a.State.Incoming[0].DisplayName);
        }

        [Test]
        public void BlockStateReplacesList()
        {
            var a = new SocialApplier();
            a.Apply(Frame(619, new S2CBlockState
            {
                Blocked =
                {
                    new BlockEntry
                    {
                        BlockedCharacterId = Id16(4),
                        DisplayName = "x",
                    },
                },
            }));
            Assert.AreEqual(1, a.State.Blocks.Count);
            a.Apply(Frame(619, new S2CBlockState()));
            Assert.AreEqual(0, a.State.Blocks.Count);
        }

        [Test]
        public void ChatFeedCapsAndResults()
        {
            var a = new SocialApplier();
            for (int i = 0; i < SocialState.MaxChatMessages + 5; i++)
            {
                a.Apply(Frame(601, new S2CChatMessage
                {
                    ChatMessageId = Id16((byte)(i % 200)),
                    Channel = ChatChannel.World,
                    MessageText = "m" + i,
                }));
            }
            Assert.AreEqual(SocialState.MaxChatMessages,
                a.State.Chat.Count);
            Assert.AreEqual("m5", a.State.Chat[0].MessageText);

            a.Apply(Frame(655, new S2CChatSendResult
            {
                Result = new OperationResult
                {
                    Status = ResultStatus.Success,
                },
                ChatMessageId = Id16(7),
            }));
            Assert.NotNull(a.State.LastChatResult);
            Assert.AreEqual(ResultStatus.Success,
                a.State.LastChatResult.Result.Status);

            a.Apply(Frame(633, new S2CReportPlayerResult
            {
                Result = new OperationResult
                {
                    Status = ResultStatus.Success,
                },
                ReportId = Id16(8),
            }));
            Assert.NotNull(a.State.LastReportResult);
        }

        [Test]
        public void FriendRequestPushAndSocialResult()
        {
            var a = new SocialApplier();
            a.Apply(Frame(612, new S2CFriendRequest
            {
                RequesterCharacterId = Id16(9),
                DisplayName = "req",
                ExpiresAtMs = 42,
            }));
            Assert.NotNull(a.State.PendingPush);
            Assert.AreEqual("req", a.State.PendingPush.DisplayName);

            a.Apply(Frame(654, new S2CSocialResult
            {
                Result = new OperationResult
                {
                    Status = ResultStatus.Error,
                    ErrorCode = ErrorCode.CapacityFull,
                },
                RequestMessageId = 611,
            }));
            Assert.NotNull(a.State.LastSocialResult);
            Assert.AreEqual(ErrorCode.CapacityFull,
                a.State.LastSocialResult.Result.ErrorCode);
        }

        [Test]
        public void PresenterAppliesOncePerVersion()
        {
            var a = new SocialApplier();
            var go = new GameObject("panel", typeof(RectTransform),
                typeof(SocialPanel), typeof(SocialPresenter));
            var panel = go.GetComponent<SocialPanel>();
            var presenter = go.GetComponent<SocialPresenter>();
            presenter.Applier = a;
            presenter.Panel = panel;

            presenter.ApplyIfDirty();
            Assert.AreEqual(0, panel.ApplyCount);

            a.Apply(Frame(619, new S2CBlockState
            {
                Blocked =
                {
                    new BlockEntry { DisplayName = "b" },
                },
            }));
            presenter.ApplyIfDirty();
            Assert.AreEqual(1, panel.ApplyCount);
            Assert.AreEqual(1, panel.BlockCount);
            presenter.ApplyIfDirty();
            Assert.AreEqual(1, panel.ApplyCount);
            Object.DestroyImmediate(go);
        }
    }
}
