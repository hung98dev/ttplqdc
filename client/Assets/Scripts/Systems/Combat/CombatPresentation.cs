using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Combat
{
    /// <summary>
    /// Presentation model over CombatState: translates authoritative
    /// state transitions into display cues (action anim phase, reject
    /// flash, hitstop on Just Guard success, death overlay). Pure C# —
    /// PlayMode tests drive it without a scene.
    /// </summary>
    public sealed class CombatPresentation
    {
        private readonly CombatState _state;
        private CombatActionState _lastState = CombatActionState.Idle;
        private ulong _lastActionId;
        private ulong _hitReactUntilTick;
        private CombatPresentationCue _cue = CombatPresentationCue.None;
        private ulong _rejectSeenSeq;
        private bool _rejectShown;

        public CombatPresentation(CombatState state)
        {
            _state = state;
        }

        public CombatPresentationCue Cue
        {
            get
            {
                return _cue;
            }
        }

        public ulong HitReactUntilTick
        {
            get
            {
                return _hitReactUntilTick;
            }
        }

        /// <summary>Advance the presentation model; returns the cue for
        /// this evaluation tick.</summary>
        public CombatPresentationCue Evaluate(ulong tick)
        {
            _cue = CombatPresentationCue.None;
            // Reject flash: new reject since last shown.
            if (_state.LastRejectSeq != _rejectSeenSeq)
            {
                _rejectSeenSeq = _state.LastRejectSeq;
                _rejectShown = false;
            }
            if (!_rejectShown && _state.LastReject != ErrorCode.Unspecified)
            {
                _rejectShown = true;
                _cue = CombatPresentationCue.Rejected;
            }
            // JG success hitstop takes priority over reject flash.
            if (_state.JustGuardTriggered)
            {
                _cue = CombatPresentationCue.JustGuardSuccess;
                _state.ClearJustGuard();
            }
            var st = _state.State;
            if (st != _lastState)
            {
                switch (st)
                {
                    case CombatActionState.Startup:
                    case CombatActionState.Active:
                        _cue = _state.ActionInstanceId != _lastActionId
                            ? CombatPresentationCue.ActionStart : _cue;
                        _lastActionId = _state.ActionInstanceId;
                        break;
                    case CombatActionState.HitReaction:
                        _hitReactUntilTick = tick + 3; // 120 ms = 3 sim ticks (20 Hz)
                        _cue = CombatPresentationCue.HitReaction;
                        break;
                    case CombatActionState.Dead:
                        _cue = CombatPresentationCue.Death;
                        break;
                    case CombatActionState.Idle:
                        if (_lastState == CombatActionState.Recovery)
                        {
                            _cue = CombatPresentationCue.ActionEnd;
                        }
                        break;
                }
                _lastState = st;
            }
            if (st == CombatActionState.HitReaction &&
                tick >= _hitReactUntilTick && _hitReactUntilTick != 0)
            {
                _state.ClearHitReaction();
            }
            return _cue;
        }
    }
}
