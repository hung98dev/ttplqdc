using ThinhThan.Protocol.V1;

namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Resolved context-interact target (client_experience_contract.md §3):
    /// nearest interactable by distance to the character anchor — a portal
    /// resolves to <c>C2S_PORTAL_USE</c> and wins ties, anything else to
    /// <c>C2S_INTERACT</c>. String ids are object/entity ids per
    /// maps_zones.md patterns.
    /// </summary>
    public readonly struct InteractTarget
    {
        public InteractTarget(
            bool isPortal, string? portalId, InteractKind kind,
            string? targetId)
        {
            IsPortal = isPortal;
            PortalId = portalId;
            Kind = kind;
            TargetId = targetId;
        }

        /// <summary>True when the wire shape is C2S_PORTAL_USE (104).</summary>
        public bool IsPortal
        {
            get;
        }

        /// <summary>portal_id for 104; null otherwise.</summary>
        public string? PortalId
        {
            get;
        }

        /// <summary>interact_kind for 103; UNSPECIFIED for portals.</summary>
        public InteractKind Kind
        {
            get;
        }

        /// <summary>target_id for 103; null otherwise.</summary>
        public string? TargetId
        {
            get;
        }
    }
}
