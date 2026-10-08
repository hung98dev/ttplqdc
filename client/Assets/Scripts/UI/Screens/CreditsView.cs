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

    /// <summary>Credits view: resolves the credits TextAsset body.</summary>
    public sealed class CreditsView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _body;

        private CreditsPresenter? _presenter;

        /// <summary>Binds the presenter; composition then calls LoadAsync.</summary>
        public void Bind(CreditsPresenter presenter)
        {
            _presenter = presenter;
            SetLabel(_title, ScreensLoc.CreditsTitle);
            presenter.Changed += Refresh;
            Refresh();
        }

        private void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            if (_presenter.Unavailable)
            {
                SetLabel(_body, ScreensLoc.CreditsUnavailable);
            }
            else
            {
                SetText(_body, _presenter.Text);
            }
        }
    }
}
