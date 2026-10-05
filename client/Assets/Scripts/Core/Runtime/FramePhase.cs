namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// Fixed frame pipeline order (client_performance.md § Smoothness by
    /// Construction): Input -> NetReceive -> Prediction -> Interpolation ->
    /// Presentation -> UI run from Update; Camera runs from LateUpdate.
    /// </summary>
    public enum FramePhase
    {
        Input = 0,
        NetReceive = 1,
        Prediction = 2,
        Interpolation = 3,
        Presentation = 4,
        UI = 5,
        Camera = 6,
    }
}
