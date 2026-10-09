using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Guild;
using ThinhThan.UI.Guild;
using UnityEngine;
using GuildSys = ThinhThan.Systems.Guild;

namespace ThinhThan.Tests.PlayMode.GuildUi
{
    /// <summary>
    /// GuildUi tests (packet § Tests): the applier folds 609/628/641/649
    /// and the panel renders membership, role, progression and
    /// permission states once per version.
    /// </summary>
    public sealed class GuildUiTests
    {
        private static byte[] Id16(int seed)
        {
            var b = new byte[16];
            b[15] = (byte)seed;
            return b;
        }

        private static S2CGuildState StateFrame(
            byte[] guildId, ulong revision, string receiverRole,
            params (byte[] id, string name, string role, bool online)[] members)
        {
            var s = new S2CGuildState
            {
                GuildId = Google.Protobuf.ByteString.CopyFrom(guildId),
                GuildRevision = revision,
                GuildName = "Thien Ha",
                Role = receiverRole,
                Level = 3,
                MembersCount = (uint)members.Length,
                MaxMembers = 30,
                Motd = "glory",
                RecruitmentMode =
                    GuildRecruitmentMode.Applications,
            };
            foreach ((byte[] id, string name, string role, bool online) m
                in members)
            {
                s.Members.Add(new GuildMemberView
                {
                    CharacterId =
                        Google.Protobuf.ByteString.CopyFrom(m.id),
                    DisplayName = m.name,
                    ClassId = "class.kim",
                    Level = 12,
                    Role = m.role,
                    OnlineState = m.online
                        ? OnlineState.Online
                        : OnlineState.Offline,
                    LastOnlineAt = m.online ? 0 : 1700000000000,
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
            var a = new GuildApplier();
            var invite = new S2CGuildInvite
            {
                GuildId = Google.Protobuf.ByteString.CopyFrom(Id16(9)),
                GuildName = "Thien Ha",
                InviterName = "Bao",
                ExpiresInSeconds = 600,
            };
            a.Apply(Frame(GuildApplier.S2CGuildInvite, invite));
            Assert.AreEqual(1UL, a.Version);
            Assert.AreSame(invite, a.LastInvite);
        }

        [Test]
        public void MembershipStateRendersRosterOncePerVersion()
        {
            var a = new GuildApplier();
            a.Apply(Frame(GuildApplier.S2CGuildState, StateFrame(
                Id16(9), 3, "guild.role.leader",
                (Id16(1), "An", "guild.role.leader", true),
                (Id16(2), "Bao", "guild.role.member", false))));

            var go = new GameObject(
                "GuildPanel", typeof(RectTransform), typeof(GuildPanel));
            var panel = go.GetComponent<GuildPanel>();
            panel.Applier = a;
            panel.RowContainer = go.transform;

            panel.RenderIfDirty();
            Assert.AreEqual(2, panel.RowCount);
            Assert.AreEqual("Thien Ha", a.State.GuildName);
            Assert.AreEqual(2U, a.State.MembersCount);
            Assert.AreEqual("guild.role.leader", a.State.Role);
            // No re-render without a version bump.
            panel.RenderIfDirty();
            Assert.AreEqual(2, panel.RowCount);

            a.Apply(Frame(GuildApplier.S2CGuildState, StateFrame(
                Id16(9), 4, "guild.role.leader",
                (Id16(2), "Bao", "guild.role.member", false))));
            panel.RenderIfDirty();
            Assert.AreEqual(1, panel.RowCount);
            Object.Destroy(go);
        }

        [Test]
        public void RoleAndPermissionStatesDriveRowAffordances()
        {
            var a = new GuildApplier();
            a.Apply(Frame(GuildApplier.S2CGuildState, StateFrame(
                Id16(9), 1, "guild.role.member",
                (Id16(1), "An", "guild.role.leader", true),
                (Id16(2), "Bao", "guild.role.member", true))));

            var go = new GameObject(
                "Panel", typeof(RectTransform), typeof(GuildPanel));
            var panel = go.GetComponent<GuildPanel>();
            panel.Applier = a;
            panel.RowContainer = go.transform;
            panel.FlushRendered();

            GuildMemberRowView? row0 = panel.RowAt(0);
            GuildMemberRowView? row1 = panel.RowAt(1);
            Assert.IsNotNull(row0);
            Assert.IsNotNull(row1);
            // Leader badge marks the leader row only.
            Assert.IsTrue(row0!.LeaderMark!.activeSelf);
            Assert.IsFalse(row1!.LeaderMark!.activeSelf);
            // MEMBER receiver: no kick affordance on any row.
            Assert.IsFalse(
                row0!.KickButton == null
                    ? false : row0!.KickButton!.gameObject.activeSelf);
            Assert.IsFalse(
                row1!.KickButton == null
                    ? false : row1!.KickButton!.gameObject.activeSelf);

            // LEADER receiver: kick affordance on non-leader rows.
            a.Apply(Frame(GuildApplier.S2CGuildState, StateFrame(
                Id16(9), 2, "guild.role.leader",
                (Id16(1), "An", "guild.role.leader", true),
                (Id16(2), "Bao", "guild.role.member", true))));
            panel.FlushRendered();
            row0 = panel.RowAt(0);
            row1 = panel.RowAt(1);
            if (row1!.KickButton != null)
            {
                Assert.IsTrue(row1!.KickButton!.gameObject.activeSelf);
            }
            if (row0!.KickButton != null)
            {
                Assert.IsFalse(row0!.KickButton!.gameObject.activeSelf);
            }
            Object.Destroy(go);
        }

        [Test]
        public void ProgressionProjectionFoldsCycleAndVoteFields()
        {
            var a = new GuildApplier();
            var s = StateFrame(Id16(9), 7, "guild.role.officer",
                (Id16(1), "An", "guild.role.leader", true));
            s.Progression = new GuildProgressionView
            {
                GuildExp = 1520,
                RitualStreak = 4,
                CycleId = "2026-09-28",
                MEffective = 12,
                RequiredPointsPerElement = 264,
                CompletedAtMs = 0,
                VoteClosesAtMs = 1700000100000,
                ReceiverVote = "blessing.hunt",
                ActiveBlessingId = "blessing.advancement",
                BlessingExpiresAtMs = 1700060000000,
                ReceiverLifetimeContribution = 900,
                ReceiverCycleContribution = 45,
            };
            s.Progression.Points.Add(new GuildProgressionPoint
            {
                Element = Element.Kim, Current = 264,
            });
            s.Progression.Points.Add(new GuildProgressionPoint
            {
                Element = Element.Moc, Current = 100,
            });
            s.Progression.CandidateBlessingIds.Add("blessing.hunt");
            s.Progression.CandidateBlessingIds.Add("blessing.craft");
            s.Progression.CandidateBlessingIds.Add("blessing.activity");
            s.Progression.VoteCounts.Add(3);
            s.Progression.VoteCounts.Add(1);
            s.Progression.VoteCounts.Add(0);
            a.Apply(Frame(GuildApplier.S2CGuildState, s));

            GuildProgressionModel p = a.State.Progression;
            Assert.AreEqual(1520UL, p.GuildExp);
            Assert.AreEqual(4U, p.RitualStreak);
            Assert.AreEqual("2026-09-28", p.CycleId);
            Assert.AreEqual(12U, p.MEffective);
            Assert.AreEqual(264U, p.RequiredPerElement);
            Assert.AreEqual(2, p.Points.Count);
            Assert.AreEqual(Element.Kim, p.Points[0].Element);
            Assert.AreEqual(264U, p.Points[0].Current);
            Assert.AreEqual(3, p.Candidates.Count);
            Assert.AreEqual(3, p.VoteCounts.Count);
            Assert.AreEqual("blessing.hunt", p.ReceiverVote);
            Assert.AreEqual("blessing.advancement", p.ActiveBlessingId);
            Assert.AreEqual(900UL, p.ReceiverLifetimeContribution);
            Assert.AreEqual(45UL, p.ReceiverCycleContribution);
        }

        [Test]
        public void ResultAndApplicationsRecorded()
        {
            var a = new GuildApplier();
            var res = new S2CGuildResult
            {
                RequestMessageId = GuildSys.GuildIntents.C2SGuildInvite,
                GuildId = Google.Protobuf.ByteString.CopyFrom(Id16(9)),
                Result = new OperationResult
                {
                    OperationId =
                        Google.Protobuf.ByteString.CopyFrom(Id16(1)),
                    Status = ResultStatus.Success,
                },
            };
            a.Apply(Frame(GuildApplier.S2CGuildResult, res));
            Assert.AreSame(res, a.LastResult);
            Assert.AreEqual(GuildSys.GuildIntents.C2SGuildInvite,
                a.LastResult!.RequestMessageId);

            var apps = new S2CGuildApplications();
            apps.Applications.Add(new GuildApplicationView
            {
                CharacterId =
                    Google.Protobuf.ByteString.CopyFrom(Id16(3)),
                DisplayName = "Cu",
                ClassId = "class.moc",
                Level = 11,
                AppliedAt = 1700000000000,
            });
            a.Apply(Frame(GuildApplier.S2CGuildApplications, apps));
            Assert.AreEqual(1, a.LastApplications!.Applications.Count);
        }

        [Test]
        public void UnknownFrameIgnored()
        {
            var a = new GuildApplier();
            a.Apply(Frame(9999, new S2CGuildState()));
            Assert.AreEqual(0UL, a.Version);
        }
    }
}
