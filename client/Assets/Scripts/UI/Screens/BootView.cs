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

    /// <summary>BOOT view: a single checking label.</summary>
    public sealed class BootView : ScreenView
    {
        [SerializeField] private TMP_Text? _checking;

        private void Awake()
        {
            SetLabel(_checking, ScreensLoc.BootChecking);
        }
    }
}
