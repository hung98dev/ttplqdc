using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Party;
using ThinhThan.UI.Party;
using UnityEngine;
using PartySys = ThinhThan.Systems.Party;

namespace ThinhThan.Tests.PlayMode.PartyUi
{
    /// <summary>
    /// PartyUi tests (packet § Tests): applier folds 603/607/636/653 and
    /// the panel renders roster/invite/result states once per version —
    /// invite, membership, leader, restart dissolution.
    /// </summary>
    public sealed class PartyUiTests
    {
        private static byte[] Id16(int seed)
        {
            var b = new byte[16];
            b[15] = (byte)seed;
            return b;
        }

        private static S2CPartyState StateFrame(
            byte[] partyId, ulong revision, byte[] leader,
            params (byte[] id, string name, bool online)[] members)
        {
            var s = new S2CPartyState
            {
                PartyId = Google.Protobuf.ByteString.CopyFrom(partyId),
                PartyRevision = revision,
                LeaderCharacterId =
                    Google.Protobuf.ByteString.CopyFrom(leader),
            };
            foreach ((byte[] id, string name, bool online) m in members)
            {
                s.Members.Add(new PartyMemberView
                {
                    CharacterId =
                        Google.Protobuf.ByteString.CopyFrom(m.id),
                    DisplayName = m.name,
                    ClassId = "warrior",
                    Level = 12,
                    OnlineState = m.online
                        ? OnlineState.Online
                        : OnlineState.Offline,
                    ZoneId = "hometown",
                });
            }
            return s;
        }

        private static DecodedFrame Frame(uint id, object payload)
        {
            return new DecodedFrame
            {
                MessageId = id,
                Payload = (Google.Protobuf.IMessage)payload,
            };
        }

        [Test]
        public void InviteApplyBumpsVersionAndShowsBanner()
        {
            var a = new PartyApplier();
            var invite = new S2CPartyInvite
            {
                PartyId = Google.Protobuf.ByteString.CopyFrom(Id16(9)),
                InviterCharacterId =
                    Google.Protobuf.ByteString.CopyFrom(Id16(2)),
                InviterName = "Bao",
                ExpiresInSeconds = 60,
            };
            a.Apply(Frame(WireIds.S2CPartyInvite, invite));
            Assert.AreEqual(1UL, a.Version);
            Assert.AreSame(invite, a.LastInvite);
        }

        [Test]
        public void MembershipStateRendersRowsOncePerVersion()
        {
            var a = new PartyApplier();
            a.Apply(Frame(WireIds.S2CPartyState, StateFrame(
                Id16(9), 3, Id16(1),
                (Id16(1), "An", true), (Id16(2), "Bao", false))));

            var go = new GameObject(
                "PartyPanel", typeof(RectTransform), typeof(PartyPanel));
            var panel = go.GetComponent<PartyPanel>();
            panel.Applier = a;
            panel.RowContainer = go.transform;

            panel.RenderIfDirty();
            Assert.AreEqual(2, panel.RowCount);
            panel.RenderIfDirty();
            Assert.AreEqual(2, panel.RowCount);

            a.Apply(Frame(WireIds.S2CPartyState, StateFrame(
                Id16(9), 4, Id16(2), (Id16(2), "Bao", false))));
            panel.RenderIfDirty();
            Assert.AreEqual(1, panel.RowCount);
            Object.Destroy(go);
        }

        [Test]
        public void LeaderMarkFollowsLeaderId()
        {
            var a = new PartyApplier();
            a.Apply(Frame(WireIds.S2CPartyState, StateFrame(
                Id16(9), 1, Id16(2),
                (Id16(1), "An", true), (Id16(2), "Bao", true))));

            var go = new GameObject(
                "Panel", typeof(RectTransform), typeof(PartyPanel));
            var panel = go.GetComponent<PartyPanel>();
            panel.Applier = a;
            panel.RowContainer = go.transform;
            panel.FlushRendered();

            PartyMemberRowView? row0 = panel.RowAt(0);
            PartyMemberRowView? row1 = panel.RowAt(1);
            Assert.IsNotNull(row0);
            Assert.IsNotNull(row1);
            Assert.IsFalse(row0!.LeaderMark!.activeSelf);
            Assert.IsTrue(row1!.LeaderMark!.activeSelf);
            Object.Destroy(go);
        }

        [Test]
        public void BoardReplaceSwapsEntries()
        {
            var a = new PartyApplier();
            var board = new S2CPartyBoardState();
            board.Entries.Add(new PartyBoardEntry
            {
                PostId = Google.Protobuf.ByteString.CopyFrom(Id16(7)),
                PosterCharacterId =
                    Google.Protobuf.ByteString.CopyFrom(Id16(1)),
                DungeonId = "grotto",
                DesiredSize = 3,
                ExpiresInSeconds = 120,
            });
            a.Apply(Frame(WireIds.S2CPartyBoardState, board));
            Assert.AreEqual(1, a.Board.Entries.Count);
            a.Apply(Frame(WireIds.S2CPartyBoardState,
                new S2CPartyBoardState()));
            Assert.AreEqual(0, a.Board.Entries.Count);
        }

        [Test]
        public void ResultVerdictRecorded()
        {
            var a = new PartyApplier();
            var res = new S2CPartyResult
            {
                RequestMessageId = PartySys.PartyIntents.C2SPartyInvite,
                Result = new OperationResult
                {
                    OperationId =
                        Google.Protobuf.ByteString.CopyFrom(Id16(5)),
                    Status = ResultStatus.Success,
                },
                PartyId = Google.Protobuf.ByteString.CopyFrom(Id16(9)),
            };
            a.Apply(Frame(WireIds.S2CPartyResult, res));
            Assert.AreSame(res, a.LastResult);
        }

        // Restart dissolution: a fresh applier (new session) holds no
        // stale party — ephemeral state never survives restart.
        [Test]
        public void RestartLeavesNoStaleParty()
        {
            var a = new PartyApplier();
            a.Apply(Frame(WireIds.S2CPartyState, StateFrame(
                Id16(9), 2, Id16(1), (Id16(1), "An", true))));
            var fresh = new PartyApplier();
            Assert.IsFalse(fresh.State.InParty);
            Assert.AreEqual(0, fresh.State.Members.Count);
        }
    }
}
