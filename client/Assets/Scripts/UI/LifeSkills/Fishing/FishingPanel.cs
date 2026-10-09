using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.LifeSkills.Fishing
{
    /// <summary>
    /// Fishing panel view (PERF-022 pooled rows): renders the cast
    /// prompt, the hook-window strip, the n/50 daily counter and the
    /// latest catch rows from <see cref="FishingPanelModel"/>. View
    /// only — every field arrives pre-computed by the presenter.
    /// </summary>
    public sealed class FishingPanel : MonoBehaviour
    {
        [SerializeField] private Text? _promptText;
        [SerializeField] private Text? _windowText;
        [SerializeField] private Text? _counterText;
        [SerializeField] private Text? _errorText;
        [SerializeField] private RectTransform? _catchRoot;
        [SerializeField] private Text? _catchRowPrefab;
        [SerializeField] private GameObject? _peakBanner;

        private readonly List<Text> _rows = new List<Text>();
        private int _liveRows;

        /// <summary>Latest rendered model (test-visible).</summary>
        public FishingPanelModel LastModel
        {
            get;
            private set;
        }
        = new FishingPanelModel();

        /// <summary>Renders one panel model.</summary>
        public void Render(FishingPanelModel model)
        {
            LastModel = model;
            if (_promptText != null)
            {
                _promptText.text = model.HookWindowOpen
                    ? "Hook!"
                    : (model.Casting ? "…" : "Cast");
            }
            if (_windowText != null)
            {
                _windowText.text = model.HookWindowOpen
                    ? $"T+{model.WindowElapsedSeconds:0.00}"
                    : "";
            }
            if (_counterText != null)
            {
                _counterText.text =
                    $"{model.DailyCatchCount}/{model.DailyCatchCap}";
            }
            if (_errorText != null)
            {
                _errorText.text = model.ErrorKey;
            }
            if (_peakBanner != null)
            {
                _peakBanner.SetActive(model.RarePeak);
            }
            RenderRows(model.Catches);
        }

        private void RenderRows(IReadOnlyList<FishingPanelModel.CatchRow> rows)
        {
            if (rows == null)
            {
                return;
            }
            int want = rows.Count;
            while (_rows.Count < want)
            {
                if (_catchRowPrefab == null || _catchRoot == null)
                {
                    break;
                }
                var row = Instantiate(_catchRowPrefab, _catchRoot);
                _rows.Add(row);
            }
            for (int i = 0; i < _rows.Count; i++)
            {
                bool live = i < want;
                if (_rows[i].gameObject.activeSelf != live)
                {
                    _rows[i].gameObject.SetActive(live);
                }
                if (live)
                {
                    _rows[i].text = $"{rows[i].ItemId} x{rows[i].Quantity}";
                }
            }
            _liveRows = want;
        }
    }
}
