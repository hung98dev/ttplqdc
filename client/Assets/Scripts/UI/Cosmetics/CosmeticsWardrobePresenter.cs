using System;
using System.Collections.Generic;
using System.Threading;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Cosmetics;
using UnityEngine;

namespace ThinhThan.UI.Cosmetics
{
    /// <summary>
    /// Bridge between the authoritative <see cref="CosmeticsState"/>
    /// projection and the wardrobe panel: reads state (never
    /// mutates), exposes a view model with a dirty flag, and turns
    /// panel gestures into <see cref="ICosmeticsIntents"/> calls.
    /// Route choice and scope badges are presentation-only — the
    /// server re-validates every mutation (client authority
    /// boundary, cosmetics.md §2).
    /// </summary>
    public sealed class CosmeticsWardrobePresenter
    {
        /// <summary>One wardrobe row shown to the player.</summary>
        public readonly struct CosmeticRow
        {
            public readonly string CosmeticId;
            public readonly CosmeticScope Scope;
            public readonly string EquippedSlot;
            public readonly bool Equipped;

            public CosmeticRow(string cosmeticId, CosmeticScope scope,
                string equippedSlot, bool equipped)
            {
                CosmeticId = cosmeticId;
                Scope = scope;
                EquippedSlot = equippedSlot;
                Equipped = equipped;
            }
        }

        /// <summary>Guild panel row (one slot per slot id).</summary>
        public readonly struct GuildSlotRow
        {
            public readonly GuildCosmeticSlot Slot;
            public readonly string CosmeticId;

            public GuildSlotRow(GuildCosmeticSlot slot, string cosmeticId)
            {
                Slot = slot;
                CosmeticId = cosmeticId;
            }
        }

        private readonly CosmeticsApplier _applier;
        private readonly ICosmeticsIntents _intents;
        private ulong _seenVersion;
        private bool _dirty = true;

        public CosmeticsWardrobePresenter(CosmeticsApplier applier,
            ICosmeticsIntents intents)
        {
            _applier = applier ?? throw new ArgumentNullException(
                nameof(applier));
            _intents = intents ?? throw new ArgumentNullException(
                nameof(intents));
        }

        /// <summary>True when the last applied frame changed state.</summary>
        public bool Dirty
        {
            get
            {
                if (_seenVersion != _applier.Version)
                {
                    _seenVersion = _applier.Version;
                    _dirty = true;
                }
                return _dirty;
            }
        }

        /// <summary>Reads the wardrobe rows (scope badge + equipped
        /// slot label per owned cosmetic).</summary>
        public IReadOnlyList<CosmeticRow> WardrobeRows()
        {
            CosmeticsState s = _applier.State;
            var rows = new List<CosmeticRow>(s.Owned.Count);
            for (int i = 0; i < s.Owned.Count; i++)
            {
                CosmeticsState.OwnedRow o = s.Owned[i];
                string slotLabel = "";
                bool equipped = false;
                foreach (CosmeticSlot slot in SlotOrder)
                {
                    if (s.EquippedAt(slot) == o.CosmeticId)
                    {
                        slotLabel = slot.ToString();
                        equipped = true;
                        break;
                    }
                }
                rows.Add(new CosmeticRow(o.CosmeticId, o.Scope,
                    slotLabel, equipped));
            }
            return rows;
        }

        /// <summary>Reads the three guild slot rows.</summary>
        public IReadOnlyList<GuildSlotRow> GuildRows()
        {
            CosmeticsState s = _applier.State;
            return new[]
            {
                new GuildSlotRow(
                    GuildCosmeticSlot.Shrine, s.GuildShrine),
                new GuildSlotRow(
                    GuildCosmeticSlot.Banner, s.GuildBanner),
                new GuildSlotRow(
                    GuildCosmeticSlot.Crest, s.GuildCrest),
            };
        }

        /// <summary>Guild cosmetic revision for a 656 request.</summary>
        public ulong GuildRevision
        {
            get
            {
                return _applier.State.GuildCosmeticRevision;
            }
        }

        /// <summary>Equip gesture → 424 (empty id = unequip).</summary>
        public Awaitable<byte[]> Equip(CosmeticSlot slot,
            string cosmeticId, CancellationToken cancel)
        {
            return _intents.RequestEquip(slot, cosmeticId, cancel);
        }

        /// <summary>Redeem gesture → 422 with the player's chosen
        /// route (dual-route cosmetics expose material + special; the
        /// server consumes exactly one).</summary>
        public Awaitable<byte[]> Redeem(string cosmeticId,
            CosmeticRoute route, CancellationToken cancel)
        {
            return _intents.RequestRedeem(cosmeticId, route, cancel);
        }

        /// <summary>Guild equip gesture → 656.</summary>
        public Awaitable<byte[]> GuildEquip(byte[] guildId,
            GuildCosmeticSlot slot, string cosmeticId,
            CancellationToken cancel)
        {
            return _intents.RequestGuildEquip(guildId, slot, cosmeticId,
                _applier.State.GuildCosmeticRevision, cancel);
        }

        /// <summary>Marks the panel consumed the current model.</summary>
        public void MarkClean()
        {
            _dirty = false;
        }

        private static readonly CosmeticSlot[] SlotOrder =
        {
            CosmeticSlot.Title,
            CosmeticSlot.TitleGlow,
            CosmeticSlot.Frame,
            CosmeticSlot.Nameplate,
            CosmeticSlot.Appearance,
            CosmeticSlot.WeaponTrail,
            CosmeticSlot.Aura,
            CosmeticSlot.CharacterShrine,
            CosmeticSlot.GuildStoneInscription,
        };
    }
}
