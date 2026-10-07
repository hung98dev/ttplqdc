namespace ThinhThan.Core.Input
{
    /// <summary>
    /// Edge classification by ordinal: the movement octet ends at
    /// <see cref="SemanticEdge.Drop"/>, gameplay-action edges end at
    /// <see cref="SemanticEdge.TargetClear"/>, and everything from
    /// <see cref="SemanticEdge.ChatOpen"/> is UI-local. The §5.1 input lock
    /// drops gameplay classes (movement + action) and keeps UI-local ones.
    /// </summary>
    public static class SemanticEdgeClass
    {
        public static bool IsMovement(SemanticEdge edge)
        {
            return edge <= SemanticEdge.Drop;
        }

        public static bool IsAction(SemanticEdge edge)
        {
            return edge > SemanticEdge.Drop && edge <= SemanticEdge.TargetClear;
        }

        public static bool IsGameplay(SemanticEdge edge)
        {
            return edge <= SemanticEdge.TargetClear;
        }

        public static bool IsUiLocal(SemanticEdge edge)
        {
            return edge > SemanticEdge.TargetClear;
        }
    }
}
