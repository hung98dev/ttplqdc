using System;
using ThinhThan.Core.Input;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Context-interact button (§3 F/LT surface on touch): prompt label +
    /// press emits a <see cref="SemanticEdge.ContextInteract"/> edge — the
    /// same path as the keyboard binding, so the §5.1 lock applies
    /// identically.
    /// </summary>
    public sealed class ContextButtonWidget : MonoBehaviour
    {
        public GameObject? Root;
        public TMP_Text? Label;

        private InputSemanticState? _input;
        private bool _shown = true;

        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>Composition wires the semantic input sink once.</summary>
        public void BindInput(InputSemanticState input)
        {
            _input = input ?? throw new ArgumentNullException(nameof(input));
        }

        /// <summary>uGUI Button.onClick hook.</summary>
        public void OnPressed()
        {
            _input?.PushEdge(SemanticEdge.ContextInteract);
        }

        public void Apply(in HudDataModel m)
        {
            ApplyCount++;
            if (Root != null && _shown != m.HasInteract)
            {
                _shown = m.HasInteract;
                Root.SetActive(_shown);
            }

            if (m.HasInteract)
            {
                HudText.Set(Label, m.InteractLabel);
            }
        }
    }
}
