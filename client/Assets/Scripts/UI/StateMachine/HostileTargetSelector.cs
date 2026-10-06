using System;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;

namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Default Tab/R3 hostile cycling over the replicated entity view:
    /// hostiles = live MONSTER or BEAST entities carrying ELIGIBLE.
    /// Ordering is ascending squared distance to the character anchor,
    /// tie → lower entity_id; the next key after the current accepted
    /// target wraps to the head. Returns 0 with no hostile visible.
    /// </summary>
    public sealed class HostileTargetSelector : ITargetSelector
    {
        private readonly EntityViewStore _store;

        public HostileTargetSelector(EntityViewStore store)
        {
            _store = store ?? throw new ArgumentNullException(nameof(store));
        }

        public ulong NextHostile(
            long selfXMm, long selfYMm, ulong currentEntityId)
        {
            ReadOnlySpan<ulong> ids = _store.Ids;
            ReadOnlySpan<EntityState?> states = _store.States;

            long currentDistSq = 0L;
            bool hasCurrent = false;
            for (int i = 0; i < states.Length; i++)
            {
                EntityState? e = states[i];
                if (e != null && IsHostile(e) && ids[i] == currentEntityId)
                {
                    long dx = e.XMm - selfXMm;
                    long dy = e.YMm - selfYMm;
                    currentDistSq = dx * dx + dy * dy;
                    hasCurrent = true;
                    break;
                }
            }

            // Smallest key overall (wrap target) and smallest key strictly
            // after the current one — the current entity's key is known
            // before ranking others.
            ulong minId = 0UL;
            long minDistSq = 0L;
            ulong nextId = 0UL;
            long nextDistSq = 0L;
            for (int i = 0; i < states.Length; i++)
            {
                EntityState? e = states[i];
                if (e == null || !IsHostile(e))
                {
                    continue;
                }

                ulong id = ids[i];
                long dx = e.XMm - selfXMm;
                long dy = e.YMm - selfYMm;
                long distSq = dx * dx + dy * dy;

                if (minId == 0UL ||
                    Before(distSq, id, minDistSq, minId))
                {
                    minDistSq = distSq;
                    minId = id;
                }

                if (hasCurrent && id != currentEntityId &&
                    Before(currentDistSq, currentEntityId, distSq, id) &&
                    (nextId == 0UL ||
                        Before(distSq, id, nextDistSq, nextId)))
                {
                    nextDistSq = distSq;
                    nextId = id;
                }
            }

            return nextId != 0UL ? nextId : minId;
        }

        /// <summary>
        /// (dist, id) ordering: a is "before" b when it is strictly closer,
        /// or equal-distance with the lower entity_id.
        /// </summary>
        private static bool Before(long aDist, ulong aId, long bDist, ulong bId)
        {
            return aDist < bDist || (aDist == bDist && aId < bId);
        }

        private static bool IsHostile(EntityState e)
        {
            if ((e.Flags & UiEntityFlags.Eligible) == 0U ||
                (e.Flags & UiEntityFlags.Dead) != 0U)
            {
                return false;
            }

            return e.EntityKind == EntityKind.Monster ||
                e.EntityKind == EntityKind.Beast;
        }
    }
}
