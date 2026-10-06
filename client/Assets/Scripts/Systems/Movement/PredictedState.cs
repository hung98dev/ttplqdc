using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Display-only prediction shadow of the authoritative
    /// <see cref="MovementCheckpoint"/> (client never authoritative —
    /// movement.md L87–91). A mutable struct so per-tick prediction and
    /// reconciliation replay allocate nothing.
    /// </summary>
    public struct PredictedState
    {
        public int XMm;
        public int YMm;
        public int VxMmS;
        public int VyMmS;
        public Facing Facing;
        public MovementState MovementState;
        public long PlatformId;
        public bool IsGrounded;
        public int JumpCount;
        public long DropIgnorePlatformId;
        public ulong DropIgnoreUntilTick;
        public HeldHorizontalIntent HeldHorizontalIntent;
        public EffectiveMovementParameters EffectiveParameters;

        /// <summary>Copies every checkpoint field into this state.</summary>
        public void RestoreFrom(MovementCheckpoint cp)
        {
            XMm = cp.XMm;
            YMm = cp.YMm;
            VxMmS = cp.VxMmS;
            VyMmS = cp.VyMmS;
            Facing = cp.Facing;
            MovementState = cp.MovementState;
            PlatformId = cp.PlatformId;
            IsGrounded = cp.IsGrounded;
            JumpCount = cp.JumpCount;
            DropIgnorePlatformId = cp.DropIgnorePlatformId;
            DropIgnoreUntilTick = cp.DropIgnoreUntilTick;
            HeldHorizontalIntent = cp.HeldHorizontalIntent;
            EffectiveParameters = cp.EffectiveParameters;
        }
    }
}
