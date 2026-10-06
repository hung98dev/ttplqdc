namespace ThinhThan.Systems.Movement
{
    /// <summary>Semantic discrete edge the client sent (wire-agnostic).</summary>
    public enum LocalEdge
    {
        None = 0,
        PressLeft,
        PressRight,
        ReleaseLeft,
        ReleaseRight,
        FlipLeft,
        FlipRight,
        Jump,
        Drop,
    }
}
