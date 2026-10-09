using System.Collections.Generic;
using System.Threading;
using TMPro;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Cosmetics;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Cosmetics
{
    /// <summary>
    /// Wardrobe panel root (client_experience_contract.md cosmetics
    /// screen): owned cosmetics list with scope badges (CHARACTER /
    /// ACCOUNT), equipped-slot readout, equip preview text, the
    /// redemption route picker (material vs special) and the guild
    /// cosmetic slots. <see cref="Apply"/> runs at most once per
    /// frame from the UI stage and only when the presenter reports a
    /// dirty model.
    /// </summary>
    public sealed class CosmeticsWardrobePanel : MonoBehaviour
    {
        /// <summary>One instantiated wardrobe row.</summary>
        public sealed class CosmeticRowView : MonoBehaviour
        {
            public TMP_Text? NameText;
            public TMP_Text? ScopeBadge;
            public TMP_Text? EquippedText;
            public Button? EquipButton;
            public Button? RedeemMaterialButton;
            public Button? RedeemSpecialButton;
            public string CosmeticId = "";
        }

        /// <summary>Guild slot row (SHRINE / BANNER / CREST).</summary>
        public sealed class GuildSlotRowView : MonoBehaviour
        {
            public TMP_Text? SlotLabel;
            public TMP_Text? CosmeticText;
            public Button? UnequipButton;
            public GuildCosmeticSlot Slot;
        }

        /// <summary>Vertical container for wardrobe rows.</summary>
        public RectTransform? RowContainer;

        /// <summary>Row prefab (CosmeticRowView).</summary>
        public CosmeticRowView? RowPrefab;

        /// <summary>Equip preview: selected cosmetic readout.</summary>
        public TMP_Text? PreviewText;

        /// <summary>Latest verdict line (423/425/649 result).</summary>
        public TMP_Text? VerdictText;

        /// <summary>Three guild slot rows in fixed order.</summary>
        public GuildSlotRowView?[] GuildRows =
            new GuildSlotRowView?[3];

        private readonly List<CosmeticRowView> _rows =
            new List<CosmeticRowView>();
        private CosmeticsWardrobePresenter? _presenter;
        private CancellationTokenSource _cancel =
            new CancellationTokenSource();
        private string _selectedId = "";
        private CosmeticSlot _selectedSlot = CosmeticSlot.Title;

        /// <summary>Binds the presenter (session driver calls once).</summary>
        public void Bind(CosmeticsWardrobePresenter presenter)
        {
            _presenter = presenter;
        }

        /// <summary>Applies the latest model when dirty (UI stage).</summary>
        public void Apply()
        {
            if (_presenter == null || !_presenter.Dirty)
            {
                return;
            }
            RebuildRows();
            RebuildGuild();
            _presenter.MarkClean();
        }

        /// <summary>Selects a wardrobe row for equip preview.</summary>
        public void Select(string cosmeticId, CosmeticSlot slot)
        {
            _selectedId = cosmeticId;
            _selectedSlot = slot;
            if (PreviewText != null)
            {
                PreviewText.text = cosmeticId;
            }
        }

        /// <summary>Equip button → 424.</summary>
        public void OnEquipClicked()
        {
            if (_presenter == null || _selectedId.Length == 0)
            {
                return;
            }
            _ = _presenter.Equip(_selectedSlot, _selectedId,
                _cancel.Token);
        }

        /// <summary>Unequip button → 424 with empty id.</summary>
        public void OnUnequipClicked()
        {
            if (_presenter == null)
            {
                return;
            }
            _ = _presenter.Equip(_selectedSlot, "", _cancel.Token);
        }

        /// <summary>Redeem via the material route → 422.</summary>
        public void OnRedeemMaterialClicked()
        {
            if (_presenter == null || _selectedId.Length == 0)
            {
                return;
            }
            _ = _presenter.Redeem(_selectedId,
                CosmeticRoute.Material, _cancel.Token);
        }

        /// <summary>Redeem via the special-currency route → 422.</summary>
        public void OnRedeemSpecialClicked()
        {
            if (_presenter == null || _selectedId.Length == 0)
            {
                return;
            }
            _ = _presenter.Redeem(_selectedId,
                CosmeticRoute.CurrencySpecial, _cancel.Token);
        }

        private void RebuildRows()
        {
            if (RowContainer == null || RowPrefab == null)
            {
                return;
            }
            for (int i = 0; i < _rows.Count; i++)
            {
                if (_rows[i] != null)
                {
                    Destroy(_rows[i].gameObject);
                }
            }
            _rows.Clear();
            IReadOnlyList<CosmeticsWardrobePresenter.CosmeticRow> rows =
                _presenter!.WardrobeRows();
            for (int i = 0; i < rows.Count; i++)
            {
                CosmeticRowView row =
                    Instantiate(RowPrefab, RowContainer);
                CosmeticRowView rowView = row;
                rowView.CosmeticId = rows[i].CosmeticId;
                if (rowView.NameText != null)
                {
                    rowView.NameText.text = rows[i].CosmeticId;
                }
                if (rowView.ScopeBadge != null)
                {
                    rowView.ScopeBadge.text =
                        rows[i].Scope == CosmeticScope.Account
                            ? "ACCOUNT"
                            : "CHARACTER";
                }
                if (rowView.EquippedText != null)
                {
                    rowView.EquippedText.text =
                        rows[i].Equipped ? rows[i].EquippedSlot : "";
                }
                _rows.Add(rowView);
            }
        }

        private void RebuildGuild()
        {
            IReadOnlyList<CosmeticsWardrobePresenter.GuildSlotRow> rows =
                _presenter!.GuildRows();
            for (int i = 0; i < rows.Count && i < GuildRows.Length; i++)
            {
                GuildSlotRowView? view = GuildRows[i];
                if (view == null)
                {
                    continue;
                }
                view.Slot = rows[i].Slot;
                if (view.CosmeticText != null)
                {
                    view.CosmeticText.text = rows[i].CosmeticId;
                }
                if (view.SlotLabel != null)
                {
                    view.SlotLabel.text = rows[i].Slot.ToString();
                }
            }
        }

        private void OnDisable()
        {
            _cancel.Cancel();
            _cancel = new CancellationTokenSource();
        }
    }
}
