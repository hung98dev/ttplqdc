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

    /// <summary>Non-dismissible SESSION_REPLACED modal (§5): message + Đồng ý.</summary>
    public sealed class SessionReplacedModalView : ScreenView
    {
        [SerializeField] private TMP_Text? _message;
        [SerializeField] private Button? _okButton;
        [SerializeField] private TMP_Text? _okLabel;

        private Action? _acknowledge;

        /// <summary>Binds the acknowledge intent (UiFsmDriver.AcknowledgeSessionReplaced).</summary>
        public void Bind(Action acknowledge)
        {
            _acknowledge = acknowledge;
            if (_okButton != null)
            {
                _okButton.onClick.AddListener(delegate()
                {
                    if (_acknowledge != null)
                    {
                        _acknowledge();
                    }
                });
            }

            SetLabel(_message, ScreensLoc.SessionReplaced);
            SetLabel(_okLabel, ScreensLoc.ModalOk);
        }
    }
}
