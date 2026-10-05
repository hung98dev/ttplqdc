using UnityEngine;

namespace ThinhThan.Core.Geometry
{
    /// <summary>
    /// Per-scene metadata marker on the `meta` GameObject of a collision
    /// scene (physics_geometry_contract.md §7.2): identifies the playable
    /// space and declares its bounds in world meters. The GeometryExporter
    /// reads this component; it carries no runtime behavior.
    /// </summary>
    public sealed class GeometryMeta : MonoBehaviour
    {
        /// <summary>Playable space id this scene collides for.</summary>
        public string spaceId = "";

        /// <summary>FIELD_OR_TOWN | DUNGEON | FINALE | PVP | GUILD_WAR.</summary>
        public string spaceKind = "";

        /// <summary>Layout profile enum token from the space record.</summary>
        public string layoutProfile = "";

        /// <summary>Bounds right edge in world meters (bounds_mm.max_x).</summary>
        public float boundsMaxX;

        /// <summary>Bounds top edge in world meters (bounds_mm.max_y).</summary>
        public float boundsMaxY;
    }
}
