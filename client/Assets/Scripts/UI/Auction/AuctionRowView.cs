using System;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Auction;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Auction
{
    /// <summary>
    /// One auction row: item id, price, state label and the actions the
    /// row type allows (buy for search rows, cancel for ACTIVE own
    /// listings, reclaim for escrow assets, claim for PENDING
    /// proceeds).
    /// </summary>
    public sealed class AuctionRowView : MonoBehaviour
    {
        public TMP_Text? ItemText;
        public TMP_Text? PriceText;
        public TMP_Text? StateText;
        public Button? ActionButton;
        public TMP_Text? ActionText;

        private byte[] _id = Array.Empty<byte>();
        private long _price;

        /// <summary>Row action intents, wired by the panel.</summary>
        public IAuctionIntents? Intents
        {
            get;
            set;
        }

        /// <summary>Wire result codes stay on the panel; the row only
        /// renders the projection it is fed.</summary>
        public void Apply(AuctionListingModel m, bool own)
        {
            _id = m.ListingId;
            _price = m.PriceCommon;
            Set(ItemText, m.ItemId);
            Set(PriceText, "{0}", m.PriceCommon);
            Set(StateText, m.State.ToString());
            Bind(own ? "CANCEL" : "BUY",
                own && m.State == AuctionListingState.Active ||
                !own && m.State == AuctionListingState.Active);
        }

        public void ApplyEscrow(AuctionEscrowModel m)
        {
            _id = m.EscrowAssetId;
            _price = 0;
            Set(ItemText, m.ItemId);
            Set(PriceText, "");
            Set(StateText, m.Reason.ToString());
            Bind("RECLAIM", true);
        }

        public void ApplyProceeds(AuctionProceedsModel m)
        {
            _id = m.ProceedsId;
            _price = m.AmountCommon;
            Set(ItemText, "");
            Set(PriceText, "{0}", m.AmountCommon);
            Set(StateText, m.State.ToString());
            Bind("CLAIM", m.State == AuctionProceedsState.Pending);
        }

        private void Bind(string label, bool enabled)
        {
            Set(ActionText, label);
            if (ActionButton != null)
            {
                ActionButton.interactable = enabled;
                ActionButton.onClick.RemoveAllListeners();
                if (enabled)
                {
                    ActionButton.onClick.AddListener(OnAction);
                }
            }
        }

        private void OnAction()
        {
            if (Intents == null || ActionText == null)
            {
                return;
            }
            switch (ActionText.text)
            {
                case "BUY":
                    _ = Intents.RequestBuy(_id, _price,
                        System.Threading.CancellationToken.None);
                    break;
                case "CANCEL":
                    _ = Intents.RequestCancelListing(_id,
                        System.Threading.CancellationToken.None);
                    break;
                case "RECLAIM":
                    _ = Intents.RequestReclaim(_id,
                        System.Threading.CancellationToken.None);
                    break;
                case "CLAIM":
                    _ = Intents.RequestProceedsClaim(_id,
                        System.Threading.CancellationToken.None);
                    break;
            }
        }

        private static void Set(TMP_Text? t, string v)
        {
            if (t != null)
            {
                t.SetText(v);
            }
        }

        private static void Set(TMP_Text? t, string fmt, long v)
        {
            if (t != null)
            {
                t.SetText(fmt, v);
            }
        }
    }
}
