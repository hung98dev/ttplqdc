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

    /// <summary>DISCONNECTED modal view: attempt line + retry/exit buttons.</summary>
    public sealed class DisconnectedModalView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _attempt;
        [SerializeField] private Button? _retryButton;
        [SerializeField] private Button? _exitButton;
        [SerializeField] private TMP_Text? _retryLabel;
        [SerializeField] private TMP_Text? _exitLabel;

        private DisconnectedModalPresenter? _presenter;

        /// <summary>Binds the presenter.</summary>
        public void Bind(DisconnectedModalPresenter presenter)
        {
            _presenter = presenter;
            if (_retryButton != null)
            {
                _retryButton.onClick.AddListener(delegate()
                {
                    _presenter.RetryNow();
                });
            }

            if (_exitButton != null)
            {
                _exitButton.onClick.AddListener(delegate()
                {
                    _presenter.ExitToTitle();
                });
            }

            SetLabel(_title, ScreensLoc.DisconnectedTitle);
            SetLabel(_retryLabel, ScreensLoc.DisconnectedRetry);
            SetLabel(_exitLabel, ScreensLoc.DisconnectedExit);
            Refresh();
        }

        /// <summary>UI-phase refresh of the attempt counter.</summary>
        public void Refresh()
        {
            if (_presenter != null)
            {
                SetText(_attempt, _presenter.AttemptLine);
            }
        }
    }
}
