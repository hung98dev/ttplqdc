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

    /// <summary>LOGIN_QUEUED view: position line + cancel button.</summary>
    public sealed class LoginQueueView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _position;
        [SerializeField] private TMP_Text? _retrying;
        [SerializeField] private Button? _cancelButton;
        [SerializeField] private TMP_Text? _cancelLabel;

        private LoginQueuePresenter? _presenter;

        /// <summary>Binds the presenter.</summary>
        public void Bind(LoginQueuePresenter presenter)
        {
            _presenter = presenter;
            if (_cancelButton != null)
            {
                _cancelButton.onClick.AddListener(OnCancel);
            }

            SetLabel(_title, ScreensLoc.QueueTitle);
            SetLabel(_retrying, ScreensLoc.QueueRetrying);
            SetLabel(_cancelLabel, ScreensLoc.QueueCancel);
            Refresh();
        }

        /// <summary>UI-phase refresh of the live position.</summary>
        public void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            SetLabel(
                _position, _presenter.PositionKey,
                new Dictionary<string, object>
                {
                    { "position", _presenter.Position },
                });
        }

        private void OnCancel()
        {
            if (_presenter != null)
            {
                _presenter.Cancel();
            }
        }
    }
}
