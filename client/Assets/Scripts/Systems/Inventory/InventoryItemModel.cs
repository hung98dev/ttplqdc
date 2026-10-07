using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Inventory
{
    /// <summary>
    /// One item instance inside an inventory slot (messages.md 433
    /// ItemInstanceView). Rolled stats ride the proto row — the panel
    /// renders them read-only.
    /// </summary>
    public sealed class InventoryItemModel
    {
        public byte[] InstanceId = new byte[16];
        public string ItemId = "";
        public uint Quantity;
        public ItemBinding EffectiveBinding;
        public uint EnhancementLevel;
        public readonly List<StatValue> RolledBaseStats = new List<StatValue>();
        public readonly List<StatValue> RolledSecondaryStats =
            new List<StatValue>();
        public readonly List<StatValue> EffectiveStats = new List<StatValue>();

        internal void Read(ItemInstanceView v)
        {
            InstanceId = v.ItemInstanceId.ToByteArray();
            ItemId = v.ItemId;
            Quantity = v.Quantity;
            EffectiveBinding = v.EffectiveBinding;
            EnhancementLevel = v.EnhancementLevel;
            CopyStats(RolledBaseStats, v.RolledBaseStats);
            CopyStats(RolledSecondaryStats, v.RolledSecondaryStats);
            CopyStats(EffectiveStats, v.EffectiveStats);
        }

        private static void CopyStats(
            List<StatValue> dst, IEnumerable<StatValue> src)
        {
            dst.Clear();
            foreach (StatValue s in src)
            {
                dst.Add(s);
            }
        }
    }

}
