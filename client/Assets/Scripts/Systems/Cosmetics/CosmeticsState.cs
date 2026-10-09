using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Cosmetics
{
    /// <summary>
    /// Authoritative cosmetic projection: S2C_COSMETIC_STATE (438)
    /// replaces the whole snapshot (owned ids + equipped slots); the
    /// guild cosmetic view arrives inside S2C_GUILD_STATE (628)
    /// fields 12-14. Presentation reads the snapshot; intents go
    /// through <see cref="ICosmeticsIntents"/>.
    /// </summary>
    public sealed class CosmeticsState
    {
        public readonly struct OwnedRow
        {
            public readonly string CosmeticId;
            public readonly CosmeticScope Scope;

            public OwnedRow(string cosmeticId, CosmeticScope scope)
            {
                CosmeticId = cosmeticId;
                Scope = scope;
            }
        }

        private readonly List<OwnedRow> _owned = new List<OwnedRow>();
        private readonly Dictionary<int, string> _equipped =
            new Dictionary<int, string>();
        private readonly List<string> _guildOwned = new List<string>();

        public IReadOnlyList<OwnedRow> Owned
        {
            get
            {
                return _owned;
            }
        }

        public string? EquippedAt(CosmeticSlot slot)
        {
            return _equipped.TryGetValue((int)slot, out string? id)
                ? id
                : null;
        }

        public bool Owns(string cosmeticId)
        {
            for (int i = 0; i < _owned.Count; i++)
            {
                if (_owned[i].CosmeticId == cosmeticId)
                {
                    return true;
                }
            }
            return false;
        }

        public IReadOnlyList<string> GuildOwned
        {
            get
            {
                return _guildOwned;
            }
        }

        public string GuildShrine
        {
            get;
            private set;
        } = "";

        public string GuildBanner
        {
            get;
            private set;
        } = "";

        public string GuildCrest
        {
            get;
            private set;
        } = "";

        public ulong GuildCosmeticRevision
        {
            get;
            private set;
        }

        internal void ApplyState(S2CCosmeticState state)
        {
            _owned.Clear();
            foreach (OwnedCosmeticView row in state.Owned)
            {
                _owned.Add(new OwnedRow(row.CosmeticId, row.Scope));
            }
            _equipped.Clear();
            foreach (EquippedCosmetic eq in state.Equipped)
            {
                _equipped[(int)eq.Slot] = eq.CosmeticId;
            }
        }

        internal void ApplyGuildView(S2CGuildState guild)
        {
            _guildOwned.Clear();
            _guildOwned.AddRange(guild.OwnedGuildCosmeticIds);
            GuildCosmeticSelections? sel = guild.GuildCosmeticSelections;
            if (sel != null)
            {
                GuildShrine = sel.Shrine;
                GuildBanner = sel.Banner;
                GuildCrest = sel.Crest;
            }
            else
            {
                GuildShrine = "";
                GuildBanner = "";
                GuildCrest = "";
            }
            GuildCosmeticRevision = guild.CosmeticRevision;
        }
    }
}
