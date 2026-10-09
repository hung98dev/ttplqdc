using System;
using System.Collections.Generic;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.LifeSkills.Fishing;

namespace ThinhThan.UI.LifeSkills.Fishing
{
    /// <summary>
    /// Binds <see cref="FishingPresentation"/> to
    /// <see cref="FishingPanel"/>: projects the cast/hook-window strip
    /// (timing hint only — the server owns acceptance), the n/50 daily
    /// counter, the latest catch rows and the single rare peak.
    /// </summary>
    public sealed class FishingPresenter
    {
        private readonly FishingPanel _panel;
        private readonly FishingPresentation _presentation;

        /// <summary>
        /// seconds — window elapsed hint advanced by the composition
        /// frame clock between window open and hook/expiry.
        /// </summary>
        public float WindowElapsedSeconds
        {
            get;
            set;
        }

        public FishingPresenter(
            FishingPanel panel,
            FishingPresentation presentation)
        {
            _panel = panel ?? throw new ArgumentNullException(nameof(panel));
            _presentation = presentation ??
                throw new ArgumentNullException(nameof(presentation));
        }

        /// <summary>Rebuilds the panel model and renders it.</summary>
        public void Apply()
        {
            var model = new FishingPanelModel
            {
                Casting = _presentation.CastFlow ==
                    FishingPresentation.CastState.Casting,
                HookWindowOpen = _presentation.CastFlow ==
                    FishingPresentation.CastState.HookWindow,
                WindowElapsedSeconds = WindowElapsedSeconds,
                DailyCatchCount = _presentation.DailyCatchCount,
                DailyCatchCap = FishingPresentation.DailyCatchCap,
                RarePeak = _presentation.RarePeak,
            };
            var rows = new List<FishingPanelModel.CatchRow>(
                _presentation.Granted.Count);
            foreach (var g in _presentation.Granted)
            {
                rows.Add(new FishingPanelModel.CatchRow(g.ItemId, g.Quantity));
            }
            model.Catches = rows;
            if (_presentation.FailureCode != ErrorCode.Unspecified)
            {
                model.ErrorKey = _presentation.FailureCode.ToString();
            }
            _panel.Render(model);
        }
    }
}
