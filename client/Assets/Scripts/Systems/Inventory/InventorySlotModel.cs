namespace ThinhThan.Systems.Inventory
{
    /// <summary>One inventory grid slot (433 InventorySlotView).</summary>
    public sealed class InventorySlotModel
    {
        public uint Slot;
        public InventoryItemModel? Item;
        public uint LockedQuantity;
    }

}
