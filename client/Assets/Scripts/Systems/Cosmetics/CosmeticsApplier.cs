using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Cosmetics
{
    /// <summary>
    /// Sink applier for the cosmetics family: receives 438 state and
    /// 628 guild frames the orchestrator forwards under lease and
    /// folds them into <see cref="State"/>. 423/425/649 results are
    /// exposed as latest-verdict properties for UI notification
    /// surfaces. `Version` bumps once per applied frame (PERF-022
    /// dirty flag).
    /// </summary>
    public sealed class CosmeticsApplier
    {
        /// <summary>messages.md C2S ids this system sends.</summary>
        public const uint C2SCosmeticRedeem = 422;
        public const uint C2SCosmeticEquip = 424;
        public const uint C2SGuildCosmeticEquip = 656;

        private readonly CosmeticsState _state = new CosmeticsState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public CosmeticsState State
        {
            get
            {
                return _state;
            }
        }

        public S2CCosmeticRedeemResult? LastRedeemResult
        {
            get;
            private set;
        }

        public S2CCosmeticEquipResult? LastEquipResult
        {
            get;
            private set;
        }

        public S2CGuildResult? LastGuildResult
        {
            get;
            private set;
        }

        /// <summary>S2C_COSMETIC_STATE (438).</summary>
        public const uint MessageCosmeticState = 438;

        /// <summary>S2C_GUILD_STATE (628) — carries the guild
        /// cosmetic view fields.</summary>
        public const uint MessageGuildState = 628;

        /// <summary>S2C_COSMETIC_REDEEM_RESULT (423).</summary>
        public const uint MessageRedeemResult = 423;

        /// <summary>S2C_COSMETIC_EQUIP_RESULT (425).</summary>
        public const uint MessageEquipResult = 425;

        /// <summary>S2C_GUILD_RESULT (649).</summary>
        public const uint MessageGuildResult = 649;

        /// <summary>Dispatches a decoded frame (sink seam shape).</summary>
        public void Apply(DecodedFrame frame)
        {
            if (frame?.Payload == null)
            {
                return;
            }
            switch (frame.MessageId)
            {
                case MessageCosmeticState:
                    ApplyCosmeticState((S2CCosmeticState)frame.Payload);
                    break;
                case MessageGuildState:
                    ApplyGuildState((S2CGuildState)frame.Payload);
                    break;
                case MessageRedeemResult:
                    ApplyRedeemResult(
                        (S2CCosmeticRedeemResult)frame.Payload);
                    break;
                case MessageEquipResult:
                    ApplyEquipResult(
                        (S2CCosmeticEquipResult)frame.Payload);
                    break;
                case MessageGuildResult:
                    ApplyGuildResult((S2CGuildResult)frame.Payload);
                    break;
            }
        }

        /// <summary>438 — wholesale cosmetics snapshot.</summary>
        public void ApplyCosmeticState(S2CCosmeticState state)
        {
            _state.ApplyState(state);
            Version++;
        }

        /// <summary>628 — guild frame carrying the cosmetic view.</summary>
        public void ApplyGuildState(S2CGuildState guild)
        {
            _state.ApplyGuildView(guild);
            Version++;
        }

        /// <summary>423 — redeem verdict (no state change).</summary>
        public void ApplyRedeemResult(S2CCosmeticRedeemResult result)
        {
            LastRedeemResult = result;
            Version++;
        }

        /// <summary>425 — equip verdict (no state change).</summary>
        public void ApplyEquipResult(S2CCosmeticEquipResult result)
        {
            LastEquipResult = result;
            Version++;
        }

        /// <summary>649 — guild verdict for a 656 request.</summary>
        public void ApplyGuildResult(S2CGuildResult result)
        {
            if (result.RequestMessageId == C2SGuildCosmeticEquip)
            {
                LastGuildResult = result;
                Version++;
            }
        }
    }
}
