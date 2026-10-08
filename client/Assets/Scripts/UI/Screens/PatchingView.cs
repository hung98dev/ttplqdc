using System;
using System.Collections.Generic;
using System.Threading;
using ThinhThan.Core.Localization;
using ThinhThan.Protocol.V1;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Screens
{

    /// <summary>PATCHING_UPDATE view: progress percent + bar.</summary>
    public sealed class PatchingView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _percent;
        [SerializeField] private Image? _bar;

        private PatchingPresenter? _presenter;

        /// <summary>Binds the presenter.</summary>
        public void Bind(PatchingPresenter presenter)
        {
            _presenter = presenter;
            SetLabel(_title, ScreensLoc.PatchingTitle);
            Refresh();
        }

        /// <summary>Progress refresh driven by the presenter.</summary>
        public void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            SetText(_percent, ((int)(_presenter.Percent * 100f)).ToString() + "%");
            if (_bar != null)
            {
                _bar.fillAmount = _presenter.Percent;
            }
        }
    }
}
