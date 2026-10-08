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

    /// <summary>CHARACTER_SELECT view: slot rows + create + logout.</summary>
    public sealed class CharacterSelectView : ScreenView
    {
        [SerializeField] private TMP_Text? _title;
        [SerializeField] private TMP_Text? _emptyLabel;
        [SerializeField] private RectTransform? _slotRoot;
        [SerializeField] private Button? _createButton;
        [SerializeField] private Button? _logoutButton;
        [SerializeField] private TMP_Text? _message;
        [SerializeField] private TMP_InputField? _nameInput;
        [SerializeField] private TMP_InputField? _classInput;

        private CharacterSelectPresenter? _presenter;
        private CancellationToken _cancel;

        /// <summary>Binds the presenter.</summary>
        public void Bind(CharacterSelectPresenter presenter, CancellationToken cancel)
        {
            _presenter = presenter;
            _cancel = cancel;
            if (_createButton != null)
            {
                _createButton.onClick.AddListener(OnCreate);
            }

            if (_logoutButton != null)
            {
                _logoutButton.onClick.AddListener(OnLogout);
            }

            SetLabel(_title, ScreensLoc.SelectTitle);
            SetLabel(_emptyLabel, ScreensLoc.SelectEmpty);
            presenter.Changed += Refresh;
            Refresh();
        }

        /// <summary>Presenter-driven refresh of slots + busy state.</summary>
        public void Refresh()
        {
            if (_presenter == null)
            {
                return;
            }

            if (_emptyLabel != null)
            {
                _emptyLabel.gameObject.SetActive(_presenter.Slots.Count == 0);
            }

            SetLabel(_message, _presenter.MessageKey);
            if (_createButton != null)
            {
                _createButton.interactable =
                    !_presenter.Busy && _presenter.Slots.Count < CharacterSelectPresenter.MaxSlots;
            }
        }

        private void OnCreate()
        {
            if (_presenter == null)
            {
                return;
            }

            _ = _presenter.CreateAsync(
                _nameInput != null ? _nameInput.text : string.Empty,
                _classInput != null ? _classInput.text : string.Empty,
                _cancel);
        }

        private void OnLogout()
        {
            if (_presenter != null)
            {
                _ = _presenter.LogoutAsync(_cancel);
            }
        }
    }
}
