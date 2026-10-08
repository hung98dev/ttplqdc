using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Equipment;
using ThinhThan.UI.Equipment;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.EquipmentUi
{
    /// <summary>
    /// IMP-012 PlayMode: 403 apply -> authoritative slot/revision/active
    /// state; error verdicts display without mutating state; soul-return
    /// bindings surface on UNEQUIP results.
    /// </summary>
    public sealed class EquipmentUiTests
    {
        private static DecodedFrame Frame403(S2CLoadoutResult r)
        {
            return new DecodedFrame
            {
                MessageId = WireIds.S2CLoadoutResult,
                Payload = r,
            };
        }

        private static byte[] Id16(byte seed)
        {
            var b = new byte[16];
            b[15] = seed;
            return b;
        }

        private static S2CLoadoutResult Ok(
            LoadoutChangeKind kind, ulong revision, string active,
            params LoadoutChangedSlot[] changed)
        {
            var r = new S2CLoadoutResult
            {
                Result = new OperationResult
                {
                    Status = ResultStatus.Success,
                },
                Kind = kind,
                LoadoutRevision = revision,
                ActiveLoadoutId = active,
            };
            r.ChangedSlots.AddRange(changed);
            return r;
        }

        [Test]
        public void EquipResultFillsSlotAndRevision()
        {
            var applier = new EquipmentApplier();
            applier.Apply(Frame403(Ok(LoadoutChangeKind.Equip, 7,
                "loadout.primary",
                new LoadoutChangedSlot
                {
                    LoadoutId = "loadout.primary",
                    SlotId = "weapon",
                    ItemInstanceId = ByteString.CopyFrom(Id16(9)),
                })));
            Assert.AreEqual(1UL, applier.Version);
            Assert.AreEqual(7UL, applier.State.LoadoutRevision);
            Assert.AreEqual("loadout.primary",
                applier.State.ActiveLoadoutId);
            CollectionAssert.AreEqual(Id16(9),
                applier.State.ItemAt("loadout.primary", "weapon"));
        }

        [Test]
        public void UnequipResultClearsSlotAndCarriesSoul()
        {
            var applier = new EquipmentApplier();
            applier.Apply(Frame403(Ok(LoadoutChangeKind.Equip, 1,
                "loadout.primary",
                new LoadoutChangedSlot
                {
                    LoadoutId = "loadout.primary",
                    SlotId = "head",
                    ItemInstanceId = ByteString.CopyFrom(Id16(4)),
                })));
            var unequip = Ok(LoadoutChangeKind.Unequip, 2,
                "loadout.primary",
                new LoadoutChangedSlot
                {
                    LoadoutId = "loadout.primary",
                    SlotId = "head",
                });
            unequip.SoulContracts.Add(new SoulContractBinding
            {
                SoulInstanceId = ByteString.CopyFrom(Id16(77)),
            });
            applier.Apply(Frame403(unequip));
            Assert.IsNull(applier.State.ItemAt("loadout.primary", "head"));
            Assert.AreEqual(1, applier.LastResult!.SoulContracts.Count);
            // Empty item_instance_id = Soul returned to Collection.
            Assert.AreEqual(0,
                applier.LastResult.SoulContracts[0].ItemInstanceId.Length);
        }

        [Test]
        public void SwitchResultMovesActiveBadgeOnly()
        {
            var applier = new EquipmentApplier();
            applier.Apply(Frame403(Ok(LoadoutChangeKind.SwitchActive, 4,
                "loadout.secondary_1")));
            Assert.AreEqual("loadout.secondary_1",
                applier.State.ActiveLoadoutId);
            Assert.AreEqual(4UL, applier.State.LoadoutRevision);
        }

        [Test]
        public void ErrorVerdictLeavesStateUntouched()
        {
            var applier = new EquipmentApplier();
            applier.Apply(Frame403(new S2CLoadoutResult
            {
                Result = new OperationResult
                {
                    Status = ResultStatus.Error,
                    ErrorCode = ErrorCode.InventoryFull,
                },
                Kind = LoadoutChangeKind.Unequip,
                LoadoutRevision = 3,
                ActiveLoadoutId = "loadout.secondary_2",
            }));
            // Error results still publish to LastResult but never apply.
            Assert.AreEqual(1UL, applier.Version);
            Assert.AreEqual(0UL, applier.State.LoadoutRevision);
            Assert.AreEqual("loadout.primary",
                applier.State.ActiveLoadoutId);
            Assert.AreEqual(ErrorCode.InventoryFull,
                applier.LastResult!.Result.ErrorCode);
        }

        [Test]
        public void PanelRendersSlotsOncePerApply()
        {
            var go = new GameObject("panel",
                typeof(RectTransform), typeof(EquipmentPanel));
            var panel = go.GetComponent<EquipmentPanel>();
            var state = new EquipmentState();
            panel.Apply(state);
            Assert.AreEqual(EquipmentPanel.SlotIds.Length,
                panel.LiveCount);
            panel.Apply(state);
            Assert.AreEqual(2, panel.ApplyCount);
            // Cells are reused, never re-instantiated.
            Assert.AreEqual(EquipmentPanel.SlotIds.Length,
                panel.LiveCount);
            Object.DestroyImmediate(go);
        }
    }
}
