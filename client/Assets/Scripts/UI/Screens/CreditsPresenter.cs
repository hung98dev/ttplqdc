using System;
using System.Threading;
using UnityEngine;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// Credits presenter (packet IMP-099 § Change): resolves
    /// <c>asset.ui.credits.text</c> — the generated third-party notice
    /// TextAsset in <c>shared.local</c> (client_assets.md §118, ADR-0071)
    /// — through an injected loader, so composition binds Addressables and
    /// tests fake it. A failed load shows the unavailable key.
    /// </summary>
    public sealed class CreditsPresenter
    {
        /// <summary>Canonical addressables key (client_assets.md §118).</summary>
        public const string CreditsKey = "asset.ui.credits.text";

        private readonly Func<string, CancellationToken, Awaitable<string>> _loadText;
        private bool _busy;

        /// <param name="loadText">(addressablesKey, cancel) → asset text.</param>
        public CreditsPresenter(
            Func<string, CancellationToken, Awaitable<string>> loadText)
        {
            _loadText = loadText ?? throw new ArgumentNullException(nameof(loadText));
            Text = string.Empty;
        }

        /// <summary>Raised when Text or the error state changes.</summary>
        public event Action? Changed;

        /// <summary>Resolved credits body ("" until loaded).</summary>
        public string Text
        {
            get;
            private set;
        }

        /// <summary>Load failed — show loc.screens.credits.unavailable.</summary>
        public bool Unavailable
        {
            get;
            private set;
        }

        /// <summary>Resolves the credits TextAsset.</summary>
        public async Awaitable LoadAsync(CancellationToken cancel)
        {
            if (_busy)
            {
                return;
            }

            _busy = true;
            try
            {
                Text = await _loadText(CreditsKey, cancel);
                Unavailable = string.IsNullOrEmpty(Text);
            }
            catch (OperationCanceledException)
            {
                Unavailable = true;
            }
            catch (Exception)
            {
                Unavailable = true;
            }
            finally
            {
                _busy = false;
                if (Changed != null)
                {
                    Changed();
                }
            }
        }
    }
}
