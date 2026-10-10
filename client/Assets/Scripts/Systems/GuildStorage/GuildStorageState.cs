using System.Collections.Generic;
using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.GuildStorage
{
    /// <summary>
    /// Authoritative guild-storage mirror (631 contents + 647 claims):
    /// full snapshot replace per frame, keyed by storage_revision.
    /// </summary>
    public sealed class GuildStorageState
    {
        private readonly List<GuildStorageItemView> _items =
            new List<GuildStorageItemView>();
        private readonly List<GuildStorageClaimView> _claims =
            new List<GuildStorageClaimView>();
        private byte[] _guildId = new byte[0];

        public byte[] GuildId
        {
            get
            {
                return _guildId;
            }
        }

        public ulong StorageRevision
        {
            get;
            private set;
        }

        public IReadOnlyList<GuildStorageItemView> Items
        {
            get
            {
                return _items;
            }
        }

        public IReadOnlyList<GuildStorageClaimView> Claims
        {
            get
            {
                return _claims;
            }
        }

        /// <summary>631 snapshot: replaces the whole item list.</summary>
        public void ReplaceItems(S2CGuildStorageState frame)
        {
            _guildId = frame.GuildId.ToByteArray();
            StorageRevision = frame.StorageRevision;
            _items.Clear();
            _items.AddRange(frame.Items);
        }

        /// <summary>647 snapshot: replaces the whole claim list.</summary>
        public void ReplaceClaims(S2CGuildStorageClaims frame)
        {
            _guildId = frame.GuildId.ToByteArray();
            StorageRevision = frame.StorageRevision;
            _claims.Clear();
            _claims.AddRange(frame.Claims);
        }

        /// <summary>Item count in one section (capacity surface).</summary>
        public int SectionCount(GuildStorageSection section)
        {
            var n = 0;
            foreach (var item in _items)
            {
                if (item.Section == section)
                {
                    n++;
                }
            }
            return n;
        }
    }
}
