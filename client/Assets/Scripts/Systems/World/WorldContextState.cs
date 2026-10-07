using System;
using ThinhThan.Core.Session;

namespace ThinhThan.Systems.World
{
    /// <summary>
    /// The client's current world context (map / channel / content
    /// revision): fed from <c>PendingTransfer</c> while TRANSFERRING_MAP and
    /// confirmed on the baseline attach. Composition binds this state to
    /// <c>IWorldContextFeed</c> (UI asmdef — this package cannot reference it).
    /// </summary>
    public sealed class WorldContextState
    {
        private readonly Func<string, string?>? _localize;

        /// <summary>localize maps a loc key to display text; null = fallback to MapId.</summary>
        public WorldContextState(Func<string, string?>? localize = null)
        {
            _localize = localize;
        }

        /// <summary>Current map_id ("" when unknown).</summary>
        public string MapId
        {
            get;
            private set;
        } = "";

        /// <summary>Current channel_index (0 = instance/none).</summary>
        public uint ChannelIndex
        {
            get;
            private set;
        }

        /// <summary>content_revision of the active world.</summary>
        public string ContentRevision
        {
            get;
            private set;
        } = "";

        /// <summary>Localization key of the current map's display name.</summary>
        public string DisplayNameKey
        {
            get;
            private set;
        } = "";

        /// <summary>Localized display name (falls back to MapId).</summary>
        public string DisplayName
        {
            get
            {
                if (DisplayNameKey.Length > 0)
                {
                    var localized = _localize?.Invoke(DisplayNameKey);
                    if (!string.IsNullOrEmpty(localized))
                    {
                        return localized!;
                    }
                }
                return MapId;
            }
        }

        /// <summary>Whether a real map is bound.</summary>
        public bool HasMap
        {
            get
            {
                return MapId.Length > 0;
            }
        }

        /// <summary>
        /// Adopts the destination of a 105 transfer (or the baseline
        /// attach_ok): updates map/channel/revision. Unknown map_ids keep the
        /// raw id — the server is authoritative.
        /// </summary>
        public void Apply(in TransferDestination destination)
        {
            MapId = destination.MapId ?? "";
            ChannelIndex = destination.ChannelIndex;
            ContentRevision = destination.ContentRevision ?? "";
            DisplayNameKey = WorldMapRegistry.TryGet(MapId, out var rec) ? rec.DisplayNameKey : "";
        }

        /// <summary>Clears the context (disconnect / world detach).</summary>
        public void Clear()
        {
            MapId = "";
            ChannelIndex = 0;
            ContentRevision = "";
            DisplayNameKey = "";
        }
    }
}
