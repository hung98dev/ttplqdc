using ThinhThan.Core.Geometry;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Per-tick parity port of server/internal/sim/movement/integrate.go
    /// onto <see cref="GeometryMath.GeometryWorld"/> (contract §4.2):
    /// intent merge → vx → vy → X sweep → Y sweep → one-way → grounded
    /// recompute → state transitions. All math is integer millimetres.
    /// <para>
    /// Prediction-side only: the outcome updates the display shadow
    /// (<see cref="PredictedState"/>); the server stays authoritative —
    /// illegal/OOB outcomes here only flag a pending correction
    /// (PredictedState keeps last safe position until the real 107 lands).
    /// </para>
    /// </summary>
    public static class MovementIntegrator
    {
        /// <summary>
        /// Fallback effective parameters for a prediction that has never
        /// seen a checkpoint (synchronization.md § Local Reconciliation —
        /// replay uses checkpoint effective parameters; before the first
        /// one lands the canonical baseline stands in).
        /// </summary>
        public static readonly EffectiveMovementParameters DefaultParameters =
            new EffectiveMovementParameters
            {
                RunSpeedMmS = unchecked((uint)MovementConstants.RunSpeedMmS),
                FirstJumpMmS = unchecked((uint)MovementConstants.FirstJumpMmS),
                SecondJumpMmS = unchecked((uint)MovementConstants.SecondJumpMmS),
                GravityMmS2 = unchecked((uint)MovementConstants.GravityMmS2),
                MaxFallMmS = unchecked((uint)MovementConstants.MaxFallMmS),
                AirControlBp = unchecked((uint)MovementConstants.AirControlBp),
                MaxStepHeightMm = unchecked((uint)MovementConstants.MaxStepHeightMm),
            };

        /// <summary>Applies one semantic edge to the intent accumulator.</summary>
        public static void ApplyEdge(ref PredictedState st, LocalEdge edge, ulong tick)
        {
            EffectiveMovementParameters p =
                st.EffectiveParameters ?? DefaultParameters;
            switch (edge)
            {
                case LocalEdge.PressLeft:
                case LocalEdge.FlipLeft:
                    st.HeldHorizontalIntent = HeldHorizontalIntent.Left;
                    st.Facing = Facing.Left;
                    break;
                case LocalEdge.PressRight:
                case LocalEdge.FlipRight:
                    st.HeldHorizontalIntent = HeldHorizontalIntent.Right;
                    st.Facing = Facing.Right;
                    break;
                case LocalEdge.ReleaseLeft:
                    if (st.HeldHorizontalIntent == HeldHorizontalIntent.Left)
                    {
                        st.HeldHorizontalIntent = HeldHorizontalIntent.None;
                    }
                    break;
                case LocalEdge.ReleaseRight:
                    if (st.HeldHorizontalIntent == HeldHorizontalIntent.Right)
                    {
                        st.HeldHorizontalIntent = HeldHorizontalIntent.None;
                    }
                    break;
                case LocalEdge.Jump:
                    if (st.IsGrounded)
                    {
                        st.VyMmS = p.FirstJumpMmS;
                        st.JumpCount = 1;
                        st.IsGrounded = false;
                    }
                    else if (st.JumpCount < MovementConstants.MaxJumpCount)
                    {
                        st.VyMmS = p.SecondJumpMmS;
                        st.JumpCount++;
                    }
                    break;
                case LocalEdge.Drop:
                    if (st.IsGrounded && st.PlatformId != 0)
                    {
                        st.DropIgnorePlatformId = st.PlatformId;
                        st.DropIgnoreUntilTick = tick + unchecked((ulong)MovementConstants.DropIgnoreTicks);
                        st.IsGrounded = false;
                    }
                    break;
            }
        }

        /// <summary>
        /// One 50 ms integration step: <paramref name="heldDirection"/> is
        /// -1/0/+1 (F-04 — semantic, never packed bits); jump/drop edges
        /// arrive via <see cref="ApplyEdge"/> beforehand.
        /// </summary>
        /// <returns>
        /// True when the move resolved legally; false on illegal/OOB — the
        /// caller freezes the prediction until the authoritative 107.
        /// </returns>
        public static bool Step(
            ref PredictedState st, GeometryMath.GeometryWorld world,
            int heldDirection, ulong tick)
        {
            EffectiveMovementParameters p =
                st.EffectiveParameters ?? DefaultParameters;
            int dir = st.HeldHorizontalIntent == HeldHorizontalIntent.Left
                ? -1
                : st.HeldHorizontalIntent == HeldHorizontalIntent.Right
                    ? 1
                    : heldDirection;

            long vx;
            if (st.IsGrounded)
            {
                vx = (long)dir * p.RunSpeedMmS;
            }
            else
            {
                vx = GeometryMath.RoundDiv(
                    (long)dir * p.RunSpeedMmS *
                    p.AirControlBp,
                    10000);
            }

            long vy = st.VyMmS;
            if (st.IsGrounded)
            {
                vy = 0;
            }
            else
            {
                vy -= GeometryMath.RoundDiv(p.GravityMmS2, 20);
                if (vy < -(long)p.MaxFallMmS)
                {
                    vy = -(long)p.MaxFallMmS;
                }
            }

            long dx = GeometryMath.RoundDiv(vx, 20);
            long dy = GeometryMath.RoundDiv(vy, 20);
            GeometryMath.Aabb box =
                CharacterColliderProfile.FeetBox(st.XMm, st.YMm);
            GeometryMath.Result res = world.ResolveMove(
                box,
                dx,
                dy,
                new GeometryMath.MoveOpts(
                    p.MaxStepHeightMm,
                    st.DropIgnorePlatformId,
                    st.DropIgnoreUntilTick,
                    tick));
            if (res.Illegal)
            {
                return false;
            }

            long finalX = res.Final.MinX + CharacterColliderProfile.HalfWidthMm;
            long finalY = res.Final.MinY;
            if (finalX < 0 || finalY < 0 ||
                finalX > world.Geometry.BoundsMaxX ||
                finalY > world.Geometry.BoundsMaxY)
            {
                return false;
            }

            if (res.VxZeroed)
            {
                vx = 0;
            }

            if (res.VyZeroed)
            {
                vy = 0;
            }

            bool grounded = res.Grounded;
            if (grounded && res.OnOneWay &&
                st.DropIgnorePlatformId != 0 &&
                res.GroundSegment == st.DropIgnorePlatformId &&
                tick < st.DropIgnoreUntilTick)
            {
                grounded = false;
            }

            if (grounded)
            {
                vy = 0;
                st.JumpCount = 0;
            }

            if (grounded && res.GroundSegment >= 0)
            {
                st.PlatformId = res.GroundSegment;
            }
            else if (!grounded)
            {
                st.PlatformId = 0;
            }

            if (st.DropIgnoreUntilTick != 0 && tick >= st.DropIgnoreUntilTick)
            {
                st.DropIgnorePlatformId = 0;
                st.DropIgnoreUntilTick = 0;
            }

            st.XMm = (int)finalX;
            st.YMm = (int)finalY;
            st.VxMmS = (int)vx;
            st.VyMmS = (int)vy;
            st.IsGrounded = grounded;
            st.MovementState = grounded
                ? vx != 0 ? MovementState.Run : MovementState.Idle
                : vy > 0 ? MovementState.Jump : MovementState.Fall;
            return true;
        }
    }
}
