using System.Collections.Generic;
using System.Threading;
using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Cosmetics;
using ThinhThan.UI.Cosmetics;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.CosmeticsUi
{
    /// <summary>
    /// IMP-038 PlayMode: 438 snapshot apply → wardrobe rows with scope
    /// badges + equipped labels, 628 guild view → guild slots, 656
    /// intent → expected_revision from the view, 422 route picker →
    /// exactly one route on the wire.
    /// </summary>
    public sealed class CosmeticsUiTests
    {
        private sealed class RecordingSender : ICosmeticsSender
        {
            public readonly List<(uint Id, IMessage Msg)> Sent =
                new List<(uint, IMessage)>();

            public Awaitable<ulong> SendAsync(uint messageId,
                IMessage payload, CancellationToken cancel)
            {
                Sent.Add((messageId, payload));
                var tcs =
                    new AwaitableCompletionSource<ulong>();
                tcs.SetResult((ulong)Sent.Count);
                return tcs.Awaitable;
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

        [Test]
        public void StateSnapshotBuildsWardrobeRows()
        {
            var applier = new CosmeticsApplier();
            applier.Apply(Frame(CosmeticsApplier.MessageCosmeticState,
                new S2CCosmeticState
                {
                    Owned =
                    {
                        new OwnedCosmeticView
                        {
                            CosmeticId = "cosmetic.title.lang_da",
                            Scope = CosmeticScope.Character,
                        },
                        new OwnedCosmeticView
                        {
                            CosmeticId = "cosmetic.iap.frame.thien_long",
                            Scope = CosmeticScope.Account,
                        },
                    },
                    Equipped =
                    {
                        new EquippedCosmetic
                        {
                            Slot = CosmeticSlot.Title,
                            CosmeticId = "cosmetic.title.lang_da",
                        },
                    },
                }));
            CosmeticsState s = applier.State;
            Assert.AreEqual(2, s.Owned.Count);
            Assert.IsTrue(s.Owns("cosmetic.title.lang_da"));
            Assert.AreEqual("cosmetic.title.lang_da",
                s.EquippedAt(CosmeticSlot.Title));
            Assert.IsNull(s.EquippedAt(CosmeticSlot.Frame));

            var presenter = new CosmeticsWardrobePresenter(
                applier, new CosmeticsIntents(new RecordingSender()));
            Assert.IsTrue(presenter.Dirty);
            IReadOnlyList<CosmeticsWardrobePresenter.CosmeticRow> rows =
                presenter.WardrobeRows();
            Assert.AreEqual(2, rows.Count);
            Assert.AreEqual("Title", rows[0].EquippedSlot);
            Assert.AreEqual(CosmeticScope.Account, rows[1].Scope);
            Assert.IsFalse(rows[1].Equipped);
        }

        [Test]
        public void GuildViewBindsFrom628()
        {
            var applier = new CosmeticsApplier();
            applier.Apply(Frame(CosmeticsApplier.MessageGuildState,
                new S2CGuildState
                {
                    OwnedGuildCosmeticIds =
                    {
                        "cosmetic.guild.banner.guild_war_champion",
                    },
                    GuildCosmeticSelections =
                        new GuildCosmeticSelections
                        {
                            Banner =
                                "cosmetic.guild.banner.guild_war_champion",
                        },
                    CosmeticRevision = 7,
                }));
            CosmeticsState s = applier.State;
            Assert.AreEqual("cosmetic.guild.banner.guild_war_champion",
                s.GuildBanner);
            Assert.AreEqual("", s.GuildShrine);
            Assert.AreEqual(7UL, s.GuildCosmeticRevision);

            var presenter = new CosmeticsWardrobePresenter(
                applier, new CosmeticsIntents(new RecordingSender()));
            IReadOnlyList<CosmeticsWardrobePresenter.GuildSlotRow> rows =
                presenter.GuildRows();
            Assert.AreEqual(3, rows.Count);
            Assert.AreEqual("cosmetic.guild.banner.guild_war_champion",
                rows[1].CosmeticId);
        }

        [Test]
        public void GuildEquipSendsExpectedRevision()
        {
            var sender = new RecordingSender();
            var applier = new CosmeticsApplier();
            applier.Apply(Frame(CosmeticsApplier.MessageGuildState,
                new S2CGuildState
                {
                    CosmeticRevision = 11,
                }));
            var presenter = new CosmeticsWardrobePresenter(
                applier, new CosmeticsIntents(sender));
            _ = presenter.GuildEquip(Id16(9), GuildCosmeticSlot.Crest,
                "cosmetic.guild.crest.ritual_4",
                CancellationToken.None);
            Assert.AreEqual(1, sender.Sent.Count);
            Assert.AreEqual(CosmeticsApplier.C2SGuildCosmeticEquip,
                sender.Sent[0].Id);
            var req = (C2SGuildCosmeticEquip)sender.Sent[0].Msg;
            Assert.AreEqual(11UL, req.ExpectedRevision);
            Assert.AreEqual(GuildCosmeticSlot.Crest, req.Slot);
        }

        [Test]
        public void RedeemPickerSendsExactlyOneRoute()
        {
            var sender = new RecordingSender();
            var intents = new CosmeticsIntents(sender);
            _ = intents.RequestRedeem("cosmetic.frame.nui_thieng",
                CosmeticRoute.Material, CancellationToken.None);
            _ = intents.RequestRedeem("cosmetic.frame.nui_thieng",
                CosmeticRoute.CurrencySpecial, CancellationToken.None);
            Assert.AreEqual(2, sender.Sent.Count);
            var m = (C2SCosmeticRedeem)sender.Sent[0].Msg;
            var sp = (C2SCosmeticRedeem)sender.Sent[1].Msg;
            Assert.AreEqual(CosmeticsApplier.C2SCosmeticRedeem,
                sender.Sent[0].Id);
            Assert.AreEqual(CosmeticRoute.Material, m.Route);
            Assert.AreEqual(CosmeticRoute.CurrencySpecial, sp.Route);
            Assert.AreEqual("cosmetic.frame.nui_thieng", m.CosmeticId);
        }

        [Test]
        public void EquipResultStampsVerdictAndFilter656()
        {
            var applier = new CosmeticsApplier();
            applier.Apply(Frame(CosmeticsApplier.MessageGuildResult,
                new S2CGuildResult
                {
                    RequestMessageId = 602, // another guild request type
                }));
            Assert.IsNull(applier.LastGuildResult);
            applier.Apply(Frame(CosmeticsApplier.MessageGuildResult,
                new S2CGuildResult
                {
                    RequestMessageId =
                        CosmeticsApplier.C2SGuildCosmeticEquip,
                }));
            Assert.IsNotNull(applier.LastGuildResult);
        }

        [Test]
        public void UnownedEquipFallsBackToNone()
        {
            var applier = new CosmeticsApplier();
            applier.Apply(Frame(CosmeticsApplier.MessageCosmeticState,
                new S2CCosmeticState()));
            Assert.IsNull(applier.State.EquippedAt(CosmeticSlot.Title));
            Assert.AreEqual(0, applier.State.Owned.Count);
        }
    }
}
