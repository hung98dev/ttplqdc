using System.Collections.Generic;

namespace ThinhThan.Systems.Inventory
{
    /// <summary>One of the exactly-three character loadouts (433).</summary>
    public sealed class LoadoutModel
    {
        public string LoadoutId = "";
        public bool IsActive;
        public readonly List<LoadoutSlotModel> Slots =
            new List<LoadoutSlotModel>();
    }

}
