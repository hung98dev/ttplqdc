using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.LifeSkills.Cooking
{
    /// <summary>
    /// Tracks the Rượu Nếp buff (<c>buff.ruou_nep_am_long</c>, +5%
    /// ATTACK for 30 minutes — world_rules.md § Village Wine) from
    /// authoritative S2C_STATUS_EVENT (205) frames: APPLIED arms the
    /// buff at the reported expiry tick; EXPIRED / DISPELLED / CONSUMED
    /// clear it. Composition forwards the self-target status stream
    /// here; the buff does not stack or refresh server-side.
    /// </summary>
    public sealed class CookingBuffTracker
    {
        /// <summary>Authored effect id of the Rượu Nếp buff.</summary>
        public const string RuouNepEffectId = "buff.ruou_nep_am_long";

        private readonly ulong _selfEntityId;

        public CookingBuffTracker(ulong selfEntityId)
        {
            _selfEntityId = selfEntityId;
        }

        /// <summary>Buff currently active on self.</summary>
        public bool Active
        {
            get;
            private set;
        }

        /// <summary>Server tick at which the buff expires.</summary>
        public ulong ExpiresAtTick
        {
            get;
            private set;
        }

        /// <summary>
        /// Remaining ticks at the given server tick (0 when inactive or
        /// past expiry).
        /// </summary>
        public ulong RemainingTicks(ulong serverTick)
        {
            if (!Active || serverTick >= ExpiresAtTick)
            {
                return 0UL;
            }
            return ExpiresAtTick - serverTick;
        }

        /// <summary>Applies one decoded 205 status event.</summary>
        public void Apply(S2CStatusEvent evt)
        {
            if (evt == null ||
                evt.TargetEntityId != _selfEntityId ||
                evt.EffectId != RuouNepEffectId)
            {
                return;
            }
            switch (evt.Event)
            {
                case StatusEventKind.Applied:
                case StatusEventKind.Refreshed:
                case StatusEventKind.StackChanged:
                    Active = true;
                    ExpiresAtTick = evt.ExpiresAtTick;
                    break;
                case StatusEventKind.Expired:
                case StatusEventKind.Dispelled:
                case StatusEventKind.Consumed:
                    Active = false;
                    ExpiresAtTick = 0UL;
                    break;
            }
        }
    }
}
