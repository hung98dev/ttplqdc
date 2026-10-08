using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Combat
{
    /// <summary>
    /// Client-side mirror of the authoritative combat action lifecycle
    /// consumed by presentation/UI. Everything here is display-only — the
    /// server owns resolution (combat.md).
    /// </summary>
    public sealed class CombatState
    {
        private CombatActionState _state = CombatActionState.Idle;
        private ulong _actionInstanceId;
        private ulong _recoveryEndsAtTick;
        private ulong _activeEndsAtTick;
        private ulong _motionEndsAtTick;
        private ulong _cooldownEndsAtTick;
        private CombatActionState _resumeState = CombatActionState.Idle;
        private ErrorCode _lastReject = ErrorCode.Unspecified;
        private ulong _lastRejectSeq;
        private bool _inCombat;
        private bool _justGuardWindow;
        private bool _justGuardTriggered;
        private ulong _respawnAvailableAtTick;
        private long _selfHp;
        private long _selfShield;

        public CombatActionState State
        {
            get
            {
                return _state;
            }
        }

        public ulong ActionInstanceId
        {
            get
            {
                return _actionInstanceId;
            }
        }

        public ulong RecoveryEndsAtTick
        {
            get
            {
                return _recoveryEndsAtTick;
            }
        }

        public ulong ActiveEndsAtTick
        {
            get
            {
                return _activeEndsAtTick;
            }
        }

        public ulong CooldownEndsAtTick
        {
            get
            {
                return _cooldownEndsAtTick;
            }
        }

        public ulong MotionEndsAtTick
        {
            get
            {
                return _motionEndsAtTick;
            }
        }

        public ErrorCode LastReject
        {
            get
            {
                return _lastReject;
            }
        }

        public ulong LastRejectSeq
        {
            get
            {
                return _lastRejectSeq;
            }
        }

        public bool InCombat
        {
            get
            {
                return _inCombat;
            }
        }

        public bool JustGuardWindow
        {
            get
            {
                return _justGuardWindow;
            }
        }

        public bool JustGuardTriggered
        {
            get
            {
                return _justGuardTriggered;
            }
        }

        public ulong RespawnAvailableAtTick
        {
            get
            {
                return _respawnAvailableAtTick;
            }
        }

        public long SelfHp
        {
            get
            {
                return _selfHp;
            }
        }

        public long SelfShield
        {
            get
            {
                return _selfShield;
            }
        }

        internal void OnStarted(S2CActionStarted m)
        {
            _state = CombatActionState.Startup;
            _actionInstanceId = m.ActionInstanceId;
            _cooldownEndsAtTick = m.CooldownEndsAtTick;
            _activeEndsAtTick = m.ActiveEndsAtTick;
            _recoveryEndsAtTick = m.RecoveryEndsAtTick;
            _motionEndsAtTick = m.MotionEndsAtTick;
            _justGuardWindow = false;
            _justGuardTriggered = false;
        }

        internal void OnRejected(S2CActionRejected m)
        {
            _lastReject = m.ErrorCode;
            _lastRejectSeq = m.ClientSeq;
        }

        internal void OnCombatEvent(S2CCombatEvent m, ulong selfEntityId)
        {
            if (m.TargetEntityId == selfEntityId)
            {
                _selfHp = m.TargetHpAfter;
                _selfShield = m.TargetShieldAfter;
                if (m.HpDamage > 0)
                {
                    _inCombat = true;
                }
                if (m.Killed)
                {
                    _state = CombatActionState.Dead;
                    return;
                }
                if (m.HpDamage > 0)
                {
                    if (_state != CombatActionState.HitReaction)
                    {
                        _resumeState = _state;
                    }
                    _state = CombatActionState.HitReaction;
                }
            }
            if (m.SourceEntityId == selfEntityId && m.HpDamage > 0)
            {
                _inCombat = true;
            }
            if (m.JustGuardWindow && m.TargetEntityId == selfEntityId)
            {
                _justGuardWindow = true;
            }
            if (m.JustGuardTriggered && m.TargetEntityId == selfEntityId)
            {
                _justGuardTriggered = true;
            }
        }

        internal void OnDeath(ulong respawnAvailableAtTick)
        {
            _state = CombatActionState.Dead;
            _respawnAvailableAtTick = respawnAvailableAtTick;
            _inCombat = false;
        }

        internal void ClearHitReaction()
        {
            if (_state == CombatActionState.HitReaction)
            {
                _state = _resumeState;
            }
        }

        internal void ClearJustGuard()
        {
            _justGuardWindow = false;
            _justGuardTriggered = false;
        }
    }
}
