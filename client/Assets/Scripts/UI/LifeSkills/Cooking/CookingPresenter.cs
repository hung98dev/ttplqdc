using System;
using System.Collections.Generic;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.LifeSkills.Cooking;

namespace ThinhThan.UI.LifeSkills.Cooking
{
    /// <summary>
    /// Binds <see cref="CookingPresentation"/> + the buff tracker to
    /// <see cref="CookingPanel"/>: derives each recipe row's craftable
    /// flag from the bound inventory count lookup (server-authoritative
    /// counts — this is a presentation projection only), mirrors the
    /// bonfire/rest/buff section state.
    /// </summary>
    public sealed class CookingPresenter
    {
        private readonly CookingPanel _panel;
        private readonly CookingPresentation _presentation;
        private readonly CookingBuffTracker _buff;
        private readonly Func<string, uint> _countOf;
        private ulong _serverTick;

        /// <summary>
        /// countOf — inventory quantity lookup by item_id (bind the
        /// inventory system's read surface; presentation-only).
        /// </summary>
        public CookingPresenter(
            CookingPanel panel,
            CookingPresentation presentation,
            CookingBuffTracker buff,
            Func<string, uint> countOf)
        {
            _panel = panel ?? throw new ArgumentNullException(nameof(panel));
            _presentation = presentation ??
                throw new ArgumentNullException(nameof(presentation));
            _buff = buff ?? throw new ArgumentNullException(nameof(buff));
            _countOf = countOf ?? throw new ArgumentNullException(nameof(countOf));
        }

        /// <summary>Latest server tick for buff-remaining display.</summary>
        public ulong ServerTick
        {
            set
            {
                _serverTick = value;
            }
        }

        /// <summary>Rebuilds the panel model and renders it.</summary>
        public void Apply()
        {
            var rows = new List<CookingPanelModel.RecipeRow>(
                CookingRecipes.All.Length);
            foreach (CookingRecipes.Row r in CookingRecipes.All)
            {
                uint missing = 0;
                foreach ((string itemId, uint qty) in r.Inputs)
                {
                    uint have = _countOf(itemId);
                    if (have < qty)
                    {
                        missing += qty - have;
                    }
                }
                rows.Add(new CookingPanelModel.RecipeRow(
                    r.RecipeId, r.OutputItemId, missing, missing == 0));
            }
            _panel.Render(new CookingPanelModel
            {
                Rows = rows,
                BonfireActive = _presentation.BonfireActive,
                Resting = _presentation.Resting,
                BuffActive = _buff.Active,
                BuffRemainingTicks = _buff.RemainingTicks(_serverTick),
                ErrorKey = ErrorKeyOf(_presentation.FailureCode),
            });
        }

        /// <summary>Maps a failed interact error code to a loc key.</summary>
        public static string ErrorKeyOf(ErrorCode code)
        {
            switch (code)
            {
                case ErrorCode.Unspecified:
                    return "";
                case ErrorCode.ItemNotFound:
                    return "loc.cooking.error.insufficient";
                case ErrorCode.InventoryFull:
                    return "loc.cooking.error.inventory_full";
                case ErrorCode.TargetInvalid:
                    return "loc.cooking.error.target_invalid";
                case ErrorCode.StateConflict:
                    return "loc.cooking.error.state_conflict";
                default:
                    return "loc.cooking.error.generic";
            }
        }
    }
}
