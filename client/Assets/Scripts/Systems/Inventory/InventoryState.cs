using System.Collections.Generic;
using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Inventory
{
    /// <summary>
    /// Client-side inventory/wallet projection (REPLACEABLE_STATE): the
    /// 433 full snapshot swaps wholesale; the 401 result merges
    /// changed_slots onto it; 429 reports capacity_after. Item identity
    /// is the 16-byte instance id.
    /// </summary>
    public sealed class InventoryState
    {
        private readonly Dictionary<uint, InventorySlotModel> _slots =
            new Dictionary<uint, InventorySlotModel>();
        private readonly List<InventorySlotModel> _ordered =
            new List<InventorySlotModel>();
        private readonly Dictionary<string, WalletBalanceModel> _balances =
            new Dictionary<string, WalletBalanceModel>();
        private readonly List<WalletBalanceModel> _orderedBalances =
            new List<WalletBalanceModel>();

        public int Capacity;
        public ulong Revision;
        public ulong LoadoutRevision;
        public readonly List<LoadoutModel> Loadouts = new List<LoadoutModel>();
        public ulong WalletRevision;

        public IReadOnlyList<InventorySlotModel> Slots
        {
            get
            {
                return _ordered;
            }
        }

        public IReadOnlyList<WalletBalanceModel> Balances
        {
            get
            {
                return _orderedBalances;
            }
        }

        /// <summary>Slot lookup by wire slot number; null when empty.</summary>
        public InventorySlotModel? SlotAt(uint slot)
        {
            return _slots.TryGetValue(slot, out InventorySlotModel m)
                ? m
                : null;
        }

        /// <summary>432 replace: balances swap wholesale.</summary>
        internal void ReplaceWallet(S2CWalletState s)
        {
            _balances.Clear();
            _orderedBalances.Clear();
            foreach (WalletBalance b in s.Balances)
            {
                var m = new WalletBalanceModel
                {
                    CurrencyId = b.CurrencyId,
                    Amount = b.Amount,
                    Cap = b.Cap,
                };
                _balances[b.CurrencyId] = m;
                _orderedBalances.Add(m);
            }
            WalletRevision = s.WalletRevision;
        }

        /// <summary>433 replace: capacity/revision/slots/loadouts swap.</summary>
        internal void ReplaceInventory(S2CInventoryState s)
        {
            Capacity = (int)s.Capacity;
            Revision = s.InventoryRevision;
            LoadoutRevision = s.LoadoutRevision;
            _slots.Clear();
            _ordered.Clear();
            foreach (InventorySlotView v in s.Slots)
            {
                var m = new InventorySlotModel
                {
                    Slot = v.Slot,
                    LockedQuantity = v.LockedQuantity,
                };
                if (v.Item != null)
                {
                    m.Item = new InventoryItemModel();
                    m.Item.Read(v.Item);
                }
                _slots[v.Slot] = m;
                _ordered.Add(m);
            }
            _ordered.Sort(CompareSlots);

            Loadouts.Clear();
            foreach (LoadoutView lv in s.Loadouts)
            {
                var lm = new LoadoutModel
                {
                    LoadoutId = lv.LoadoutId,
                    IsActive = lv.IsActive,
                };
                foreach (LoadoutSlotView sv in lv.Slots)
                {
                    var sm = new LoadoutSlotModel { SlotId = sv.SlotId };
                    if (sv.Item != null)
                    {
                        sm.Item = new InventoryItemModel();
                        sm.Item.Read(sv.Item);
                    }
                    lm.Slots.Add(sm);
                }
                Loadouts.Add(lm);
            }
        }

        /// <summary>
        /// 401 merge: each changed_slots row rewrites its slot —
        /// item_instance_id present = put, empty = clear. A fresh
        /// instance id replaces the model (rolled stats come back on the
        /// next 433 snapshot). Revision follows the result.
        /// </summary>
        internal void ApplyResult(S2CInventoryResult r)
        {
            foreach (InventoryChangedSlot c in r.ChangedSlots)
            {
                InventorySlotModel m;
                if (!_slots.TryGetValue(c.Slot, out m))
                {
                    m = new InventorySlotModel { Slot = c.Slot };
                    _slots[c.Slot] = m;
                    _ordered.Add(m);
                }
                if (c.ItemInstanceId.Length == 0)
                {
                    m.Item = null;
                    m.LockedQuantity = 0U;
                    continue;
                }
                InventoryItemModel? item = m.Item;
                if (item == null ||
                    !SameInstance(item.InstanceId, c.ItemInstanceId))
                {
                    item = new InventoryItemModel
                    {
                        InstanceId = c.ItemInstanceId.ToByteArray(),
                    };
                    m.LockedQuantity = 0U;
                }
                item.ItemId = c.ItemId;
                item.Quantity = c.Quantity;
                m.Item = item;
            }
            _ordered.Sort(CompareSlots);
            Revision = r.InventoryRevision;
        }

        /// <summary>429 merge: capacity adopts capacity_after.</summary>
        internal void ApplyExpandResult(S2CInventoryExpandResult r)
        {
            Capacity = (int)r.CapacityAfter;
            foreach (CurrencyDelta d in r.CurrencyDelta)
            {
                if (_balances.TryGetValue(d.CurrencyId,
                    out WalletBalanceModel m))
                {
                    m.Amount += d.Amount;
                }
            }
        }

        private static int CompareSlots(
            InventorySlotModel a, InventorySlotModel b)
        {
            return a.Slot.CompareTo(b.Slot);
        }

        private static bool SameInstance(byte[] a, ByteString b)
        {
            if (a.Length != b.Length)
            {
                return false;
            }
            for (int i = 0; i < a.Length; i++)
            {
                if (a[i] != b[i])
                {
                    return false;
                }
            }
            return true;
        }
    }

}
