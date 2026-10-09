using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Crafting
{
    /// <summary>
    /// Client-side projection of the crafting consult: the bound
    /// station context plus the latest recorded results (405/407).
    /// Catalog data lives in <see cref="CraftingRecipes"/>; inventory
    /// deltas the server reports ride the existing inventory/wallet
    /// projections — this state is the authoritative outcome surface
    /// for the crafting UI only.
    /// </summary>
    public sealed class CraftingState
    {
        /// <summary>Station kind bound to the open consult.</summary>
        public enum Station
        {
            None = 0,
            Crafting = 1,
            Enhancement = 2,
        }

        /// <summary>NPC id of the bound station (empty when None).</summary>
        public string NpcId = "";

        /// <summary>Which consult the panel is bound to.</summary>
        public Station BoundStation = Station.None;

        /// <summary>Currently selected recipe row (craft consult).</summary>
        public string SelectedRecipeId = "";

        /// <summary>Current batch selection (1..99).</summary>
        public uint Batch = 1;

        /// <summary>Enhance target item instance id (16 bytes).</summary>
        public byte[] EnhanceItemInstanceId = new byte[0];

        /// <summary>Enhance target level selection.</summary>
        public uint EnhanceTargetLevel;

        /// <summary>Selected lucky charm instance (empty = none).</summary>
        public byte[] LuckyCharmInstanceId = new byte[0];

        /// <summary>Selected insurance instance (empty = none).</summary>
        public byte[] InsuranceInstanceId = new byte[0];

        /// <summary>Latest recorded craft result (405).</summary>
        public S2CCraftResult? LastCraftResult;

        /// <summary>Latest recorded enhance result (407).</summary>
        public S2CEnhanceResult? LastEnhanceResult;

        /// <summary>Bind the consult to a station NPC.</summary>
        public void BindStation(string npcId, Station station)
        {
            NpcId = npcId ?? "";
            BoundStation = npcId != null && npcId.Length != 0
                ? station
                : Station.None;
        }

        /// <summary>Apply one recorded 405.</summary>
        public void ApplyCraft(S2CCraftResult result)
        {
            if (result == null)
            {
                return;
            }
            LastCraftResult = result;
        }

        /// <summary>Apply one recorded 407.</summary>
        public void ApplyEnhance(S2CEnhanceResult result)
        {
            if (result == null)
            {
                return;
            }
            LastEnhanceResult = result;
        }
    }
}
