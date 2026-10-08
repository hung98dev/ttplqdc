using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Equipment
{
    /// <summary>
    /// `IEquipmentSink` applier (seam landed #225): receives the 403
    /// S2C_LOADOUT_RESULT frames the orchestrator forwards under lease
    /// and folds them into <see cref="State"/>. Changed slots apply
    /// verbatim — equip fills a slot, unequip clears it, switch renames
    /// the active loadout; `Version` bumps once per applied frame.
    /// </summary>
    public sealed class EquipmentApplier : IEquipmentSink
    {
        /// <summary>messages.md C2S id this system sends (402).</summary>
        public const uint C2SLoadoutChange = 402;

        private readonly EquipmentState _state = new EquipmentState();

        /// <summary>Monotonic apply counter (dirty flag source).</summary>
        public ulong Version
        {
            get;
            private set;
        }

        public EquipmentState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Latest 403 result for the UI to display.</summary>
        public S2CLoadoutResult? LastResult
        {
            get;
            private set;
        }

        public void Apply(DecodedFrame frame)
        {
            if (frame.MessageId != WireIds.S2CLoadoutResult)
            {
                return;
            }
            LastResult = (S2CLoadoutResult)frame.Payload!;
            if (LastResult.Result != null &&
                LastResult.Result.Status == ResultStatus.Success)
            {
                _state.ApplyResult(LastResult);
            }
            Version++;
        }
    }
}
