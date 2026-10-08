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

    /// <summary>TRANSFERRING_MAP view: progress bar + status + folk art backdrop.</summary>
    public sealed class TransferView : ScreenView
    {
        [SerializeField] private TMP_Text? _status;
        [SerializeField] private TMP_Text? _percent;
        [SerializeField] private Image? _bar;
        [SerializeField] private GameObject? _progressRoot;

        private TransferPresenter? _presenter;

        /// <summary>Binds the presenter.</summary>
        public void Bind(TransferPresenter presenter)
        {
            _presenter = presenter;
            Refresh();
        }

        /// <summary>UI-phase refresh of progress + status.</summary>
        public void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            SetLabel(_status, _presenter.StatusKey);
            if (_progressRoot != null)
            {
                _progressRoot.SetActive(
                    _presenter.Progress.Visible && !_presenter.PlacementPending);
            }

            SetText(
                _percent,
                ((int)(_presenter.Progress.Progress * 100f)).ToString() + "%");
            if (_bar != null)
            {
                _bar.fillAmount = _presenter.Progress.Progress;
            }
        }
    }
}
