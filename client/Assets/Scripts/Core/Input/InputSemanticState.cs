using System;
using ThinhThan.Core.Runtime;

namespace ThinhThan.Core.Input
{
    /// <summary>
    /// Semantic input state (client_experience_contract.md §3): the held
    /// direction/flags that become <c>C2S_INPUT_STATE.input_flags</c>, plus a
    /// bounded FIFO of semantic edges. Direction changes derive their
    /// movement edges here — the only place press/release/flip semantics
    /// exist, so held flags and queued edges can never disagree.
    /// <para>
    /// §5.1 lock: <see cref="DiscardGameplay"/> drops queued gameplay edges
    /// (movement + action) instead of deferring them; UI-local edges
    /// (chat/nav) are kept. Held flags keep tracking while locked so the
    /// first INPUT_STATE after unlock reflects the current hold.
    /// </para>
    /// </summary>
    public sealed class InputSemanticState
    {
        /// <summary>Bounded edge ring capacity (dispatch drains every tick).</summary>
        public const int EdgeCapacity = 16;

        private readonly SemanticEdge[] _edges = new SemanticEdge[EdgeCapacity];
        private int _head;
        private int _count;
        private uint _heldFlags;
        private int _heldDirection;
        private bool _downHeld;

        /// <summary>input_flags packing for the next INPUT_STATE.</summary>
        public uint HeldFlags
        {
            get
            {
                return _heldFlags;
            }
        }

        /// <summary>Held horizontal direction: -1 / 0 / +1.</summary>
        public int HeldDirection
        {
            get
            {
                return _heldDirection;
            }
        }

        /// <summary>Down held (S key / stick-down / joystick-down).</summary>
        public bool DownHeld
        {
            get
            {
                return _downHeld;
            }
        }

        /// <summary>Queued edge count (for tests and drop detection).</summary>
        public int EdgeCount
        {
            get
            {
                return _count;
            }
        }

        /// <summary>
        /// Sets the held horizontal direction, queuing the corresponding
        /// movement edge on every transition: 0 -> dir is PRESS, dir -> 0 is
        /// RELEASE, dir -> -dir is FLIP.
        /// </summary>
        public void SetDirection(int direction)
        {
            direction = direction < 0 ? -1 : direction > 0 ? 1 : 0;
            if (direction == _heldDirection)
            {
                return;
            }

            switch (_heldDirection)
            {
                case 0:
                    PushEdge(direction < 0
                        ? SemanticEdge.PressLeft
                        : SemanticEdge.PressRight);
                    break;
                case -1:
                    PushEdge(direction == 0
                        ? SemanticEdge.ReleaseLeft
                        : SemanticEdge.FlipRight);
                    break;
                default:
                    PushEdge(direction == 0
                        ? SemanticEdge.ReleaseRight
                        : SemanticEdge.FlipLeft);
                    break;
            }

            _heldDirection = direction;
            RecomputeFlags();
        }

        /// <summary>Down held flag — no wire edge exists for it.</summary>
        public void SetDown(bool down)
        {
            _downHeld = down;
            RecomputeFlags();
        }

        /// <summary>
        /// Queues one semantic edge. A full ring drops the oldest queued edge
        /// (it can never be re-ordered ahead of newer input).
        /// </summary>
        public void PushEdge(SemanticEdge edge)
        {
            if (_count == EdgeCapacity)
            {
                Log.Warn("input semantic edge ring full; dropping oldest edge");
                _head = (_head + 1) % EdgeCapacity;
                _count--;
            }

            _edges[(_head + _count) % EdgeCapacity] = edge;
            _count++;
        }

        private static readonly EdgePredicate TakeMovement =
            SemanticEdgeClass.IsMovement;
        private static readonly EdgePredicate TakeAction =
            SemanticEdgeClass.IsAction;
        private static readonly EdgePredicate TakeUi =
            SemanticEdgeClass.IsUiLocal;

        /// <summary>Drains movement-class edges in FIFO order.</summary>
        public int DrainMovement(SemanticEdge[] dst)
        {
            return DrainMovement(dst, dst.Length);
        }

        /// <summary>Movement drain capped at <paramref name="max"/>.</summary>
        public int DrainMovement(SemanticEdge[] dst, int max)
        {
            return DrainWhere(dst, Math.Min(max, dst.Length), TakeMovement);
        }

        /// <summary>Drains gameplay-action edges in FIFO order.</summary>
        public int DrainActions(SemanticEdge[] dst)
        {
            return DrainWhere(dst, dst.Length, TakeAction);
        }

        /// <summary>Drains UI-local edges in FIFO order.</summary>
        public int DrainUi(SemanticEdge[] dst)
        {
            return DrainWhere(dst, dst.Length, TakeUi);
        }

        /// <summary>
        /// §5.1 lock: drops queued gameplay edges (movement + action) — a
        /// drop, not a defer. UI-local edges stay queued for the shell.
        /// </summary>
        public void DiscardGameplay()
        {
            int kept = 0;
            for (int i = 0; i < _count; i++)
            {
                SemanticEdge edge = _edges[(_head + i) % EdgeCapacity];
                if (SemanticEdgeClass.IsUiLocal(edge))
                {
                    _edges[kept++] = edge;
                }
            }

            _head = 0;
            _count = kept;
        }

        /// <summary>Clears held state and every queued edge.</summary>
        public void Reset()
        {
            _head = 0;
            _count = 0;
            _heldDirection = 0;
            _downHeld = false;
            RecomputeFlags();
        }

        private int DrainWhere(
            SemanticEdge[] dst, int max, EdgePredicate take)
        {
            int written = 0;
            int kept = 0;
            for (int i = 0; i < _count; i++)
            {
                SemanticEdge edge = _edges[(_head + i) % EdgeCapacity];
                if (written < max && take(edge))
                {
                    dst[written++] = edge;
                }
                else
                {
                    _edges[kept++] = edge;
                }
            }

            _head = 0;
            _count = kept;
            return written;
        }

        private void RecomputeFlags()
        {
            uint flags = 0;
            if (_heldDirection < 0)
            {
                flags |= InputFlagBits.MoveLeft;
            }
            else if (_heldDirection > 0)
            {
                flags |= InputFlagBits.MoveRight;
            }

            if (_downHeld)
            {
                flags |= InputFlagBits.DownHeld;
            }

            _heldFlags = flags;
        }

        private delegate bool EdgePredicate(SemanticEdge edge);
    }
}
