using System;
using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;
using ThinhThan.Systems.Movement;

namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Sole <see cref="FramePhase.Input"/> occupant (F-08): runs IMP-013's
    /// <see cref="MovementInputDispatch"/> (edges 101/102/108 + held 100)
    /// then <see cref="ActionIntentDispatch"/> (103/104/200/201/202) — one
    /// semantic input source feeding both.
    /// <para>
    /// §5.1 lock: while UI state is not IN_WORLD every queued gameplay
    /// edge is dropped — a lock drops input rather than deferring it;
    /// UI-local edges (chat/nav) stay queued for the shell.
    /// </para>
    /// </summary>
    public sealed class ClientInputPhase : IFrameSystem
    {
        private readonly GameInputSource _input;
        private readonly MovementInputDispatch _movement;
        private readonly ActionIntentDispatch _actions;
        private readonly Func<ClientUiState> _uiState;

        public ClientInputPhase(
            GameInputSource input,
            MovementInputDispatch movement,
            ActionIntentDispatch actions,
            Func<ClientUiState> uiState)
        {
            _input = input ?? throw new ArgumentNullException(nameof(input));
            _movement = movement ??
                throw new ArgumentNullException(nameof(movement));
            _actions = actions ??
                throw new ArgumentNullException(nameof(actions));
            _uiState = uiState ??
                throw new ArgumentNullException(nameof(uiState));
        }

        public void Tick(in FrameTime time)
        {
            if (_uiState() != ClientUiState.InWorld)
            {
                _input.DiscardGameplay();
                return;
            }

            _movement.Dispatch(in time);
            _actions.Dispatch(in time, _input);
        }
    }
}
