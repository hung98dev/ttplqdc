using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Combat
{
    /// <summary>
    /// ICombatSink applier: routes authoritative combat frames into
    /// CombatState and bumps <see cref="Version"/> so consumers re-present.
    /// 204 reaches both this sink and IWorldSink; this sink only consumes
    /// frames whose request_message_id belongs to combat (200/201/202/208).
    /// </summary>
    public sealed class CombatApplier : ICombatSink
    {
        public const uint C2SSkillUse = 200;
        public const uint C2SBasicAttack = 201;
        public const uint C2STargetIntent = 202;
        public const uint S2CActionStarted = 203;
        public const uint S2CActionRejected = 204;
        public const uint S2CStatusEvent = 205;
        public const uint S2CCombatEvent = 304;
        private const uint C2SRespawnRequest = 208;

        private readonly CombatState _state = new CombatState();
        private readonly ulong _selfEntityId;
        private ulong _version;

        public CombatApplier(ulong selfEntityId)
        {
            _selfEntityId = selfEntityId;
        }

        public CombatState State
        {
            get
            {
                return _state;
            }
        }

        public ulong Version
        {
            get
            {
                return _version;
            }
        }

        public void Apply(DecodedFrame frame)
        {
            switch (frame.MessageId)
            {
                case S2CActionStarted:
                    if (frame.Payload is S2CActionStarted started &&
                        started.SourceEntityId == _selfEntityId)
                    {
                        _state.OnStarted(started);
                    }
                    break;
                case S2CActionRejected:
                    if (frame.Payload is S2CActionRejected rejected &&
                        IsCombatRequest(rejected.RequestMessageId))
                    {
                        _state.OnRejected(rejected);
                    }
                    break;
                case S2CStatusEvent:
                    break; // status presentation lane: IMP-016.
                case S2CCombatEvent:
                    if (frame.Payload is S2CCombatEvent ev)
                    {
                        _state.OnCombatEvent(ev, _selfEntityId);
                    }
                    break;
                default:
                    return;
            }
            _version++;
        }

        private static bool IsCombatRequest(uint requestMessageId)
        {
            return requestMessageId == C2SSkillUse
                || requestMessageId == C2SBasicAttack
                || requestMessageId == C2STargetIntent
                || requestMessageId == C2SRespawnRequest;
        }
    }
}
