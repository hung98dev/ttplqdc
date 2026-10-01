using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Google.Protobuf;
using NUnit.Framework;
using UnityEngine;

namespace ThinhThan.Tests.EditMode.ProtocolParity
{
    /// <summary>
    /// Wire parity between the generated C# (ThinhThan.Protocol.V1) and the
    /// canonical Go fixtures (proto/testdata/golden): every registered network
    /// message ID resolves to exactly one generated message type, every golden
    /// binary decodes and re-encodes byte-identically, and journal fixtures are
    /// confirmed absent from the client assembly (Go-internal schema only).
    /// </summary>
    public class ProtocolParityTests
    {
        private sealed class RegistryRow
        {
            public readonly int Id;
            public readonly string TypeName;

            public RegistryRow(int id, string typeName)
            {
                Id = id;
                TypeName = typeName;
            }
        }

        [Serializable]
        private sealed class GoldenEntry
        {
            public string file = string.Empty;
            public string type = string.Empty;
            public bool journal;
        }

        [Serializable]
        private sealed class GoldenIndex
        {
            public List<GoldenEntry> fixtures = new List<GoldenEntry>();
        }

        private static readonly RegistryRow[] NetworkRegistry =
        {
            new RegistryRow(1, "C2SHello"),
            new RegistryRow(2, "S2CHelloOk"),
            new RegistryRow(3, "S2CError"),
            new RegistryRow(4, "C2SHeartbeat"),
            new RegistryRow(5, "S2CHeartbeat"),
            new RegistryRow(6, "C2SCharacterAttach"),
            new RegistryRow(7, "S2CCharacterAttachOk"),
            new RegistryRow(8, "S2CSessionReplaced"),
            new RegistryRow(9, "S2CServerDraining"),
            new RegistryRow(10, "C2SCharacterDetach"),
            new RegistryRow(11, "S2CCharacterDetachOk"),
            new RegistryRow(12, "C2SCharacterCreate"),
            new RegistryRow(13, "S2CCharacterCreateResult"),
            new RegistryRow(14, "S2CCharacterList"),
            new RegistryRow(15, "S2CPlacementPending"),
            new RegistryRow(16, "S2CResumeCredential"),
            new RegistryRow(100, "C2SInputState"),
            new RegistryRow(101, "C2SJump"),
            new RegistryRow(102, "C2SDropThrough"),
            new RegistryRow(103, "C2SInteract"),
            new RegistryRow(104, "C2SPortalUse"),
            new RegistryRow(105, "S2CTransferPrepare"),
            new RegistryRow(106, "C2SPresentationReady"),
            new RegistryRow(107, "S2CMovementCorrection"),
            new RegistryRow(108, "C2SMovementEdge"),
            new RegistryRow(109, "C2SChannelSwitch"),
            new RegistryRow(110, "S2CChannelSwitchResult"),
            new RegistryRow(111, "C2SDungeonEnterRequest"),
            new RegistryRow(112, "S2CDungeonEntryState"),
            new RegistryRow(113, "C2SDungeonEntryRespond"),
            new RegistryRow(114, "C2SDungeonLeave"),
            new RegistryRow(115, "S2CDungeonLeaveResult"),
            new RegistryRow(116, "S2CInteractResult"),
            new RegistryRow(117, "C2SDungeonEntryCancel"),
            new RegistryRow(200, "C2SSkillUse"),
            new RegistryRow(201, "C2SBasicAttack"),
            new RegistryRow(202, "C2STargetIntent"),
            new RegistryRow(203, "S2CActionStarted"),
            new RegistryRow(204, "S2CActionRejected"),
            new RegistryRow(205, "S2CStatusEvent"),
            new RegistryRow(206, "S2CDeath"),
            new RegistryRow(207, "S2CRespawn"),
            new RegistryRow(208, "C2SRespawnRequest"),
            new RegistryRow(300, "S2CWorldBaseline"),
            new RegistryRow(301, "S2CEntitySpawn"),
            new RegistryRow(302, "S2CEntityDespawn"),
            new RegistryRow(303, "S2CStateDelta"),
            new RegistryRow(304, "S2CCombatEvent"),
            new RegistryRow(305, "S2CEncounterEvent"),
            new RegistryRow(306, "C2SBaselineAck"),
            new RegistryRow(307, "C2SBaselineResyncRequest"),
            new RegistryRow(308, "S2CBaselineResyncResult"),
            new RegistryRow(400, "C2SInventoryMutate"),
            new RegistryRow(401, "S2CInventoryResult"),
            new RegistryRow(402, "C2SLoadoutChange"),
            new RegistryRow(403, "S2CLoadoutResult"),
            new RegistryRow(404, "C2SCraft"),
            new RegistryRow(405, "S2CCraftResult"),
            new RegistryRow(406, "C2SEnhance"),
            new RegistryRow(407, "S2CEnhanceResult"),
            new RegistryRow(408, "C2SRewardClaim"),
            new RegistryRow(409, "S2CRewardClaimResult"),
            new RegistryRow(410, "C2SBeastSetActive"),
            new RegistryRow(411, "S2CBeastSetActiveResult"),
            new RegistryRow(412, "C2SBeastEquip"),
            new RegistryRow(413, "S2CBeastEquipResult"),
            new RegistryRow(414, "C2SBeastUnequip"),
            new RegistryRow(415, "S2CBeastUnequipResult"),
            new RegistryRow(416, "C2SBeastFeed"),
            new RegistryRow(417, "S2CBeastFeedResult"),
            new RegistryRow(418, "C2SEntitlementClaim"),
            new RegistryRow(419, "S2CEntitlementClaimResult"),
            new RegistryRow(420, "C2SNpcShopBuy"),
            new RegistryRow(421, "S2CNpcShopBuyResult"),
            new RegistryRow(422, "C2SCosmeticRedeem"),
            new RegistryRow(423, "S2CCosmeticRedeemResult"),
            new RegistryRow(424, "C2SCosmeticEquip"),
            new RegistryRow(425, "S2CCosmeticEquipResult"),
            new RegistryRow(426, "C2SNpcShopSell"),
            new RegistryRow(427, "S2CNpcShopSellResult"),
            new RegistryRow(428, "C2SInventoryExpand"),
            new RegistryRow(429, "S2CInventoryExpandResult"),
            new RegistryRow(430, "C2SBeastLevelUp"),
            new RegistryRow(431, "S2CBeastLevelUpResult"),
            new RegistryRow(432, "S2CWalletState"),
            new RegistryRow(433, "S2CInventoryState"),
            new RegistryRow(434, "S2CRewardClaimsState"),
            new RegistryRow(435, "S2CEntitlementPanelState"),
            new RegistryRow(436, "S2CBeastState"),
            new RegistryRow(437, "S2CSoulState"),
            new RegistryRow(438, "S2CCosmeticState"),
            new RegistryRow(439, "C2SRewardClaimListRequest"),
            new RegistryRow(440, "S2CRewardClaimListResult"),
            new RegistryRow(441, "S2CRewardClaimDelta"),
            new RegistryRow(442, "C2SSoulListRequest"),
            new RegistryRow(443, "S2CSoulListResult"),
            new RegistryRow(500, "C2SQuestAccept"),
            new RegistryRow(501, "S2CQuestAcceptResult"),
            new RegistryRow(502, "C2SQuestTurnIn"),
            new RegistryRow(503, "S2CQuestUpdate"),
            new RegistryRow(504, "C2SAtlasClaim"),
            new RegistryRow(505, "S2CAtlasClaimResult"),
            new RegistryRow(506, "S2CProgressionEvent"),
            new RegistryRow(507, "C2SQuestAbandon"),
            new RegistryRow(508, "S2CQuestAbandonResult"),
            new RegistryRow(509, "C2SStoryBranchChoose"),
            new RegistryRow(510, "S2CStoryBranchResult"),
            new RegistryRow(511, "C2SSkillUpgrade"),
            new RegistryRow(512, "C2SPotentialAllocate"),
            new RegistryRow(513, "C2SRespec"),
            new RegistryRow(514, "S2CProgressionMutateResult"),
            new RegistryRow(515, "S2CProgressionState"),
            new RegistryRow(516, "C2SDailyBoardRequest"),
            new RegistryRow(517, "S2CDailyBoardState"),
            new RegistryRow(518, "S2CAtlasState"),
            new RegistryRow(600, "C2SChatSend"),
            new RegistryRow(601, "S2CChatMessage"),
            new RegistryRow(602, "C2SPartyInvite"),
            new RegistryRow(603, "S2CPartyInvite"),
            new RegistryRow(604, "C2SPartyAccept"),
            new RegistryRow(605, "C2SPartyLeave"),
            new RegistryRow(606, "C2SPartyKick"),
            new RegistryRow(607, "S2CPartyState"),
            new RegistryRow(608, "C2SGuildInvite"),
            new RegistryRow(609, "S2CGuildInvite"),
            new RegistryRow(610, "C2SGuildAccept"),
            new RegistryRow(611, "C2SFriendRequest"),
            new RegistryRow(612, "S2CFriendRequest"),
            new RegistryRow(613, "C2SFriendAccept"),
            new RegistryRow(614, "C2SFriendDecline"),
            new RegistryRow(615, "C2SFriendRemove"),
            new RegistryRow(616, "S2CFriendState"),
            new RegistryRow(617, "C2SBlockAdd"),
            new RegistryRow(618, "C2SBlockRemove"),
            new RegistryRow(619, "S2CBlockState"),
            new RegistryRow(620, "C2SPartyDecline"),
            new RegistryRow(621, "C2SPartyInviteCancel"),
            new RegistryRow(622, "C2SPartyLeaderTransfer"),
            new RegistryRow(623, "C2SGuildDecline"),
            new RegistryRow(624, "C2SGuildLeave"),
            new RegistryRow(625, "C2SGuildKick"),
            new RegistryRow(626, "C2SGuildRoleUpdate"),
            new RegistryRow(627, "C2SGuildLeaderTransfer"),
            new RegistryRow(628, "S2CGuildState"),
            new RegistryRow(629, "C2SGuildStorageDeposit"),
            new RegistryRow(630, "C2SGuildStorageWithdraw"),
            new RegistryRow(631, "S2CGuildStorageState"),
            new RegistryRow(632, "C2SReportPlayer"),
            new RegistryRow(633, "S2CReportPlayerResult"),
            new RegistryRow(634, "C2SPartyBoardPost"),
            new RegistryRow(635, "C2SPartyBoardCancel"),
            new RegistryRow(636, "S2CPartyBoardState"),
            new RegistryRow(637, "C2SGuildCreate"),
            new RegistryRow(638, "C2SGuildDisband"),
            new RegistryRow(639, "C2SGuildApply"),
            new RegistryRow(640, "C2SGuildApplicationDecide"),
            new RegistryRow(641, "S2CGuildApplications"),
            new RegistryRow(642, "C2SGuildMotdSet"),
            new RegistryRow(643, "C2SGuildLeadershipClaim"),
            new RegistryRow(644, "C2SGuildStorageMove"),
            new RegistryRow(645, "C2SGuildStorageClaimRequest"),
            new RegistryRow(646, "C2SGuildStorageClaimDecide"),
            new RegistryRow(647, "S2CGuildStorageClaims"),
            new RegistryRow(648, "C2SGuildBlessingVote"),
            new RegistryRow(649, "S2CGuildResult"),
            new RegistryRow(650, "C2SGuildSettingsSet"),
            new RegistryRow(651, "C2SGuildInviteCancel"),
            new RegistryRow(652, "C2SGuildApplicationCancel"),
            new RegistryRow(653, "S2CPartyResult"),
            new RegistryRow(654, "S2CSocialResult"),
            new RegistryRow(655, "S2CChatSendResult"),
            new RegistryRow(656, "C2SGuildCosmeticEquip"),
            new RegistryRow(700, "C2STradeInvite"),
            new RegistryRow(701, "S2CTradeInvite"),
            new RegistryRow(702, "C2STradeAccept"),
            new RegistryRow(703, "C2STradeCancel"),
            new RegistryRow(704, "S2CTradeCancelled"),
            new RegistryRow(705, "C2STradeOfferUpdate"),
            new RegistryRow(706, "S2CTradeOfferState"),
            new RegistryRow(707, "C2STradeConfirm"),
            new RegistryRow(708, "C2STradeFinalise"),
            new RegistryRow(709, "S2CTradeResult"),
            new RegistryRow(710, "S2CTradeRequestResult"),
            new RegistryRow(730, "C2SAuctionList"),
            new RegistryRow(731, "S2CAuctionListResult"),
            new RegistryRow(732, "C2SAuctionBuy"),
            new RegistryRow(733, "S2CAuctionBuyResult"),
            new RegistryRow(734, "C2SAuctionCancelListing"),
            new RegistryRow(735, "S2CAuctionCancelResult"),
            new RegistryRow(736, "S2CAuctionSold"),
            new RegistryRow(738, "C2SAuctionSearch"),
            new RegistryRow(739, "S2CAuctionSearchResult"),
            new RegistryRow(740, "C2SAuctionReclaim"),
            new RegistryRow(741, "S2CAuctionReclaimResult"),
            new RegistryRow(742, "C2SAuctionProceedsClaim"),
            new RegistryRow(743, "S2CAuctionProceedsResult"),
            new RegistryRow(744, "S2CAuctionMyState"),
            new RegistryRow(800, "C2SSparringRequest"),
            new RegistryRow(801, "C2SSparringAccept"),
            new RegistryRow(802, "C2SRankedQueueJoin"),
            new RegistryRow(803, "C2SRankedQueueLeave"),
            new RegistryRow(804, "S2CRankedQueueUpdate"),
            new RegistryRow(805, "C2SMatchReady"),
            new RegistryRow(806, "S2CMatchState"),
            new RegistryRow(807, "C2SMatchSurrender"),
            new RegistryRow(808, "C2SGuildWarQueueJoin"),
            new RegistryRow(809, "C2SGuildWarQueueLeave"),
            new RegistryRow(810, "S2CGuildWarState"),
            new RegistryRow(811, "S2CSparringChallenge"),
            new RegistryRow(812, "C2SSparringDecline"),
            new RegistryRow(813, "S2CSparringOutcome"),
            new RegistryRow(814, "C2SDuelChallenge"),
            new RegistryRow(815, "S2CDuelChallenge"),
            new RegistryRow(816, "C2SDuelRespond"),
            new RegistryRow(817, "C2SDuelCancel"),
            new RegistryRow(818, "S2CDuelOutcome"),
            new RegistryRow(819, "S2CPvpResult"),
        };

        private static string RepoRoot()
        {
            var dir = new DirectoryInfo(Application.dataPath);
            while (dir != null && !Directory.Exists(Path.Combine(dir.FullName, "docs")))
            {
                dir = dir.Parent;
            }
            if (dir == null)
            {
                Assert.Fail("repo root not found above Application.dataPath");
            }
            return dir!.FullName;
        }

        private static IEnumerable<Type> ProtocolMessageTypes()
        {
            var asm = typeof(ThinhThan.Protocol.V1.Envelope).Assembly;
            return asm.GetTypes()
                .Where(t => t.Namespace == "ThinhThan.Protocol.V1"
                    && typeof(IMessage).IsAssignableFrom(t)
                    && !t.IsAbstract);
        }

        [Test]
        public void EveryRegisteredNetworkIdHasOneCSharpType()
        {
            var byName = ProtocolMessageTypes()
                .GroupBy(t => t.Name)
                .ToDictionary(g => g.Key, g => g.Count());
            var missing = new List<string>();
            var duplicate = new List<string>();
            var ids = new HashSet<int>();
            foreach (var row in NetworkRegistry)
            {
                if (!ids.Add(row.Id))
                {
                    Assert.Fail($"duplicate registry id {row.Id} in test table");
                }
                if (!byName.TryGetValue(row.TypeName, out var count))
                {
                    missing.Add($"{row.Id}:{row.TypeName}");
                }
                else if (count != 1)
                {
                    duplicate.Add($"{row.Id}:{row.TypeName}x{count}");
                }
            }
            Assert.IsEmpty(missing, "registry IDs with no generated C# type");
            Assert.IsEmpty(duplicate, "registry names resolving to >1 type");
            Assert.AreEqual(217, NetworkRegistry.Length,
                "messages.md registry must allocate exactly 217 IDs");
        }

        [Test]
        public void GoldenFixturesDecodeAndReencodeByteIdentical()
        {
            var goldenDir = Path.Combine(RepoRoot(), "proto", "testdata", "golden");
            var indexPath = Path.Combine(goldenDir, "index.json");
            Assert.IsTrue(File.Exists(indexPath),
                $"golden index missing at {indexPath}");
            var index = JsonUtility.FromJson<GoldenIndex>(File.ReadAllText(indexPath));
            Assert.IsNotNull(index);
            Assert.Greater(index!.fixtures.Count, 0, "golden index is empty");

            var typesByFullName = ProtocolMessageTypes()
                .ToDictionary(t => "thinhthan.v1." + t.Name, t => t);
            var failures = new List<string>();
            var journalSeen = new List<string>();
            var decoded = 0;
            foreach (var entry in index.fixtures)
            {
                if (entry.journal)
                {
                    // Go-internal journal schema: must NOT exist in the client
                    // assembly — asserting absence is the parity check here.
                    var shortName = entry.type.Substring("thinhthan.internal.v1.".Length);
                    if (ProtocolMessageTypes().Any(t => t.Name == shortName))
                    {
                        journalSeen.Add($"{entry.file}:{entry.type} leaked into client");
                    }
                    continue;
                }
                var path = Path.Combine(goldenDir, entry.file);
                var expected = File.ReadAllBytes(path);
                if (!typesByFullName.TryGetValue(entry.type, out var t))
                {
                    failures.Add($"{entry.file}:{entry.type} no generated type");
                    continue;
                }
                var parserProp = t.GetProperty("Parser");
                var parser = (MessageParser)parserProp!.GetValue(null);
                var msg = parser.ParseFrom(expected);
                var again = msg.ToByteArray();
                if (!expected.SequenceEqual(again))
                {
                    failures.Add($"{entry.file}:{entry.type} re-encode differs");
                    continue;
                }
                decoded++;
            }
            Assert.IsEmpty(journalSeen, "journal types must stay server-side");
            Assert.IsEmpty(failures, "golden parity failures");
            Assert.AreEqual(index.fixtures.Count(e => !e.journal), decoded,
                "every non-journal fixture must decode");
        }
    }
}
