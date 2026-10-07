using System;
using ThinhThan.Core.Geometry;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;

namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Default context-interact resolver: nearest interactable to the
    /// character anchor among (a) INTERACTABLE replicated entities and
    /// (b) <c>portal.*</c> anchors in the active geometry
    /// (maps_zones.md portal convention, spawn anchors land in
    /// <see cref="GeometryData.Anchors"/>). A portal resolves to the 104
    /// path and wins distance ties; NPC → TALK, anything else →
    /// QUEST_OBJECT.
    /// </summary>
    public sealed class NearestInteractResolver : IInteractResolver
    {
        private const string PortalIdPrefix = "portal.";

        private readonly EntityViewStore _store;
        private readonly GeometryData? _geometry;

        public NearestInteractResolver(
            EntityViewStore store, GeometryData? geometry = null)
        {
            _store = store ?? throw new ArgumentNullException(nameof(store));
            _geometry = geometry;
        }

        public bool TryResolve(
            long selfXMm, long selfYMm, out InteractTarget target)
        {
            target = default;
            long bestDistSq = long.MaxValue;
            bool found = false;

            ReadOnlySpan<EntityState?> states = _store.States;
            for (int i = 0; i < states.Length; i++)
            {
                EntityState? e = states[i];
                if (e == null ||
                    (e.Flags & UiEntityFlags.Interactable) == 0U)
                {
                    continue;
                }

                bool portal = e.EntityKind == EntityKind.Object &&
                    e.ContentId.StartsWith(
                        PortalIdPrefix, StringComparison.Ordinal);
                long dx = e.XMm - selfXMm;
                long dy = e.YMm - selfYMm;
                if (!Consider(
                    dx * dx + dy * dy, portal, ref bestDistSq, ref found))
                {
                    continue;
                }

                target = portal
                    ? new InteractTarget(
                        true, e.ContentId, InteractKind.Unspecified, null)
                    : new InteractTarget(
                        false, null, KindOf(e), e.ContentId);
            }

            if (_geometry != null)
            {
                for (int i = 0; i < _geometry.Anchors.Length; i++)
                {
                    GeometryData.Anchor anchor = _geometry.Anchors[i];
                    if (!anchor.Id.StartsWith(
                        PortalIdPrefix, StringComparison.Ordinal))
                    {
                        continue;
                    }

                    long dx = anchor.X - selfXMm;
                    long dy = anchor.Y - selfYMm;
                    if (!Consider(
                        dx * dx + dy * dy, true, ref bestDistSq, ref found))
                    {
                        continue;
                    }

                    target = new InteractTarget(
                        true, anchor.Id, InteractKind.Unspecified, null);
                }
            }

            return found;
        }

        /// <summary>
        /// Keeps the closest candidate; a portal wins an exact distance tie
        /// over a non-portal. Updates <paramref name="bestDistSq"/> and
        /// returns true when the candidate took the slot.
        /// </summary>
        private static bool Consider(
            long distSq, bool portal, ref long bestDistSq, ref bool found)
        {
            if (found &&
                (distSq > bestDistSq || (distSq == bestDistSq && !portal)))
            {
                return false;
            }

            bestDistSq = distSq;
            found = true;
            return true;
        }

        private static InteractKind KindOf(EntityState e)
        {
            return e.EntityKind == EntityKind.Npc
                ? InteractKind.Talk
                : InteractKind.QuestObject;
        }
    }
}
