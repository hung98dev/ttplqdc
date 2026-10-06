using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// One sent input retained for self-ack replay (synchronization.md §
    /// Local Reconciliation): the stamped client_seq plus exactly the data
    /// needed to re-apply the input at the predicted tick it was issued on.
    /// </summary>
    public struct InputRecord
    {
        public InputRecordKind Kind;
        public ulong Seq;
        /// <summary>Predicted tick the input was applied on locally.</summary>
        public ulong Tick;
        /// <summary>Semantic held direction after this record: -1/0/+1.</summary>
        public int HeldDirection;
        /// <summary>Opaque input_flags bits sent on the wire (F-04: pack owned by the input producer).</summary>
        public uint Flags;
        /// <summary>Discrete edge for <see cref="InputRecordKind.Edge"/>.</summary>
        public LocalEdge Edge;

        /// <summary>Wire edge type + direction for an edge record.</summary>
        public bool TryWireEdge(out MovementEdgeType type, out Facing direction)
        {
            switch (Edge)
            {
                case LocalEdge.PressLeft:
                    type = MovementEdgeType.Press;
                    direction = Facing.Left;
                    return true;
                case LocalEdge.PressRight:
                    type = MovementEdgeType.Press;
                    direction = Facing.Right;
                    return true;
                case LocalEdge.ReleaseLeft:
                    type = MovementEdgeType.Release;
                    direction = Facing.Left;
                    return true;
                case LocalEdge.ReleaseRight:
                    type = MovementEdgeType.Release;
                    direction = Facing.Right;
                    return true;
                case LocalEdge.FlipLeft:
                    type = MovementEdgeType.Flip;
                    direction = Facing.Left;
                    return true;
                case LocalEdge.FlipRight:
                    type = MovementEdgeType.Flip;
                    direction = Facing.Right;
                    return true;
                default:
                    type = MovementEdgeType.Unspecified;
                    direction = Facing.Unspecified;
                    return false;
            }
        }
    }
}
