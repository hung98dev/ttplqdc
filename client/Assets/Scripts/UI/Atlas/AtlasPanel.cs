using System.Collections.Generic;
using System.Threading;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Atlas;
using TMPro;
using UnityEngine;

namespace ThinhThan.UI.Atlas
{
    /// <summary>
    /// Atlas journal panel: grid of all authored pages (locked pages
    /// rendered dimmed from the roster), a lore/title detail view and
    /// the acknowledge affordance per reached tier — the marker clears
    /// only on the 505 verdict (or the next 518 with acknowledged_at).
    /// Rows re-render only on a new authoritative frame (PERF-022).
    /// </summary>
    public sealed class AtlasPanel : MonoBehaviour
    {
        public RectTransform? GridRoot;
        public AtlasPageCellView? CellPrefab;
        public TMP_Text? HeaderText;
        public TMP_Text? DetailTitleText;
        public TMP_Text? DetailLoreText;
        public AtlasApplier? Applier
        {
            get;
            set;
        }
        public IAtlasIntents? Intents
        {
            get;
            set;
        }

        private Pool<AtlasPageCellView>? _pool;
        private readonly List<AtlasPageCellView> _live =
            new List<AtlasPageCellView>();
        private ulong _appliedVersion;
        private string _selectedPageId = "";

        /// <summary>Apply-call count (once-per-version proof).</summary>
        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>Live cell count (test introspection).</summary>
        public int LiveCount
        {
            get
            {
                return _live.Count;
            }
        }

        /// <summary>Cell i (test introspection).</summary>
        public AtlasPageCellView CellAt(int i)
        {
            return _live[i];
        }

        /// <summary>Currently selected page id.</summary>
        public string SelectedPageId
        {
            get
            {
                return _selectedPageId;
            }
        }

        /// <summary>
        /// Folds the applier's state into cells. Applies once per
        /// applier version — repeated calls with no new frame are no-ops.
        /// </summary>
        public void Apply()
        {
            if (Applier == null || GridRoot == null || CellPrefab == null)
            {
                return;
            }
            if (Applier.Version == _appliedVersion)
            {
                return;
            }
            _appliedVersion = Applier.Version;
            ApplyCount++;

            if (_pool == null)
            {
                _pool = new Pool<AtlasPageCellView>(
                    () => Instantiate(CellPrefab, GridRoot));
            }
            for (int i = 0; i < _live.Count; i++)
            {
                _live[i].gameObject.SetActive(false);
                _pool.Return(_live[i]);
            }
            _live.Clear();

            var ids = new List<string>(Applier.State.Pages.Keys);
            ids.Sort();
            foreach (string id in ids)
            {
                AtlasPageCellView cell = _pool.Rent();
                cell.transform.SetParent(GridRoot, false);
                cell.gameObject.SetActive(true);
                cell.Bind(Applier.State.Pages[id]);
                _live.Add(cell);
            }
            if (HeaderText != null)
            {
                HeaderText.text = "Atlas — rev " + Applier.State.Revision;
            }
            if (_selectedPageId.Length > 0)
            {
                RenderDetail(_selectedPageId);
            }
        }

        /// <summary>Opens the lore/title detail for one page.</summary>
        public void Select(string pageId)
        {
            _selectedPageId = pageId;
            RenderDetail(pageId);
        }

        private void RenderDetail(string pageId)
        {
            if (!Applier!.State.Pages.TryGetValue(pageId,
                    out AtlasPageModel m))
            {
                return;
            }
            if (DetailTitleText != null)
            {
                DetailTitleText.text = m.TitleCosmeticId.Length > 0
                    ? m.TitleCosmeticId : pageId;
            }
            if (DetailLoreText != null)
            {
                DetailLoreText.text = m.Family + " — " +
                    m.ProgressText();
            }
        }

        /// <summary>
        /// Acknowledges the page's latest reached tier (504): the
        /// marker clears on the 505 verdict — never on send.
        /// </summary>
        public async Awaitable AcknowledgeSelected(
            CancellationToken cancel)
        {
            if (Intents == null || Applier == null ||
                _selectedPageId.Length == 0 ||
                !Applier.State.Pages.TryGetValue(_selectedPageId,
                    out AtlasPageModel m) || m.ReachedTier == 0)
            {
                return;
            }
            await Intents.Acknowledge(
                _selectedPageId, m.ReachedTier, cancel);
        }
    }
}
