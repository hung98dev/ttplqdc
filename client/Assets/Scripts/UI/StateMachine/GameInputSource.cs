using System;
using ThinhThan.Core.Input;
using ThinhThan.Systems.Movement;

namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Bridges <see cref="InputSemanticState"/> (Core) to the gameplay
    /// consumers: it is the <see cref="IInputSource"/> that
    /// <see cref="MovementInputDispatch"/> samples, and it exposes the
    /// action/UI drains for <see cref="ActionIntentDispatch"/> and the shell.
    /// One semantic state — three class-filtered drains, so a drain for one
    /// consumer never eats another's edges.
    /// </summary>
    public sealed class GameInputSource : IInputSource
    {
        private readonly InputSemanticState _state;
        private readonly SemanticEdge[] _movementScratch =
            new SemanticEdge[InputSemanticState.EdgeCapacity];

        public GameInputSource(InputSemanticState state)
        {
            _state = state ?? throw new ArgumentNullException(nameof(state));
        }

        /// <summary>The shared semantic state (producers push into it).</summary>
        public InputSemanticState State
        {
            get
            {
                return _state;
            }
        }

        public int HeldDirection
        {
            get
            {
                return _state.HeldDirection;
            }
        }

        public uint HeldFlags
        {
            get
            {
                return _state.HeldFlags;
            }
        }

        /// <summary>
        /// <see cref="IInputSource"/> drain: movement-class semantic edges
        /// translated to <see cref="LocalEdge"/> for the wire. Action and
        /// UI-local edges stay queued for their own drains.
        /// </summary>
        public int SampleEdges(LocalEdge[] edges)
        {
            int n = _state.DrainMovement(_movementScratch, edges.Length);
            for (int i = 0; i < n; i++)
            {
                edges[i] = ToLocal(_movementScratch[i]);
            }

            return n;
        }

        /// <summary>Drains gameplay-action edges for the intent dispatcher.</summary>
        public int SampleActionEdges(SemanticEdge[] dst)
        {
            return _state.DrainActions(dst);
        }

        /// <summary>Drains UI-local edges for the shell (chat/nav).</summary>
        public int SampleUiEdges(SemanticEdge[] dst)
        {
            return _state.DrainUi(dst);
        }

        /// <summary>§5.1 lock: drops queued gameplay edges.</summary>
        public void DiscardGameplay()
        {
            _state.DiscardGameplay();
        }

        private static LocalEdge ToLocal(SemanticEdge edge)
        {
            switch (edge)
            {
                case SemanticEdge.PressLeft: return LocalEdge.PressLeft;
                case SemanticEdge.PressRight: return LocalEdge.PressRight;
                case SemanticEdge.ReleaseLeft: return LocalEdge.ReleaseLeft;
                case SemanticEdge.ReleaseRight: return LocalEdge.ReleaseRight;
                case SemanticEdge.FlipLeft: return LocalEdge.FlipLeft;
                case SemanticEdge.FlipRight: return LocalEdge.FlipRight;
                case SemanticEdge.Jump: return LocalEdge.Jump;
                default: return LocalEdge.Drop;
            }
        }
    }
}
