using System;
using UnityEngine.InputSystem;

namespace ThinhThan.Core.Input
{
    /// <summary>
    /// Client_experience_contract.md §3 bindings for keyboard+mouse and
    /// gamepad, built as a programmatic Input System action map
    /// (com.unity.inputsystem 1.20.0). Touch input never enters this map —
    /// the virtual joystick and HUD buttons push the same
    /// <see cref="InputSemanticState"/> directly.
    /// <para>
    /// Binding groups are named <c>kbmouse</c>/<c>gamepad</c> so the
    /// per-context uniqueness test can verify each physical control drives
    /// exactly one action inside one context.
    /// </para>
    /// </summary>
    public sealed class ClientInputMaps : IDisposable
    {
        public const string KeyboardGroup = "kbmouse";
        public const string GamepadGroup = "gamepad";

        private readonly InputSemanticState _state;
        private readonly InputActionMap _map;
        private readonly InputAction[] _skills = new InputAction[5];

        public ClientInputMaps(InputSemanticState state)
        {
            _state = state ?? throw new ArgumentNullException(nameof(state));
            _map = new InputActionMap("client");

            Move = _map.AddAction("Move", InputActionType.Value);
            Move.expectedControlType = "Axis";
            Move.AddCompositeBinding("1DAxis")
                .With("negative", "<Keyboard>/a", groups: KeyboardGroup)
                .With("positive", "<Keyboard>/d", groups: KeyboardGroup);
            Move.AddCompositeBinding("1DAxis")
                .With("negative", "<Keyboard>/leftArrow",
                    groups: KeyboardGroup)
                .With("positive", "<Keyboard>/rightArrow",
                    groups: KeyboardGroup);
            Move.AddBinding("<Gamepad>/leftStick/x",
                groups: GamepadGroup);
            Move.performed += OnMove;
            Move.canceled += OnMove;

            Down = _map.AddAction("Down", InputActionType.Button);
            Down.AddBinding("<Keyboard>/s", groups: KeyboardGroup);
            Down.AddBinding("<Gamepad>/leftStick/down",
                groups: GamepadGroup);
            Down.performed += _ => _state.SetDown(true);
            Down.canceled += _ => _state.SetDown(false);

            Jump = _map.AddAction("Jump", InputActionType.Button);
            Jump.AddBinding("<Keyboard>/space", groups: KeyboardGroup);
            Jump.AddBinding("<Keyboard>/w", groups: KeyboardGroup);
            Jump.AddBinding("<Gamepad>/buttonSouth",
                groups: GamepadGroup);
            Jump.performed += OnJump;

            Basic = _map.AddAction("Basic", InputActionType.Button);
            Basic.AddBinding("<Keyboard>/j", groups: KeyboardGroup);
            Basic.AddBinding("<Mouse>/leftButton", groups: KeyboardGroup);
            Basic.AddBinding("<Gamepad>/buttonWest",
                groups: GamepadGroup);
            Basic.performed += _ => _state.PushEdge(SemanticEdge.BasicAttack);

            string[] skillKeys = { "k", "l", "u", "i", "o" };
            string[] skillPads =
            {
                "buttonNorth", "buttonEast", "rightShoulder",
                "rightTrigger", "leftShoulder",
            };
            for (int i = 0; i < 5; i++)
            {
                int slot = i;
                InputAction skill = _map.AddAction(
                    "Skill" + (i + 1), InputActionType.Button);
                skill.AddBinding(
                    "<Keyboard>/" + skillKeys[i], groups: KeyboardGroup);
                skill.AddBinding(
                    "<Gamepad>/" + skillPads[i], groups: GamepadGroup);
                skill.performed += _ => _state.PushEdge(
                    (SemanticEdge)((int)SemanticEdge.Skill1 + slot));
                _skills[i] = skill;
            }

            Interact = _map.AddAction("Interact", InputActionType.Button);
            Interact.AddBinding("<Keyboard>/f", groups: KeyboardGroup);
            Interact.AddBinding("<Gamepad>/leftTrigger",
                groups: GamepadGroup);
            Interact.performed +=
                _ => _state.PushEdge(SemanticEdge.ContextInteract);

            TargetCycle = _map.AddAction(
                "TargetCycle", InputActionType.Button);
            TargetCycle.AddBinding(
                "<Keyboard>/tab", groups: KeyboardGroup);
            TargetCycle.AddBinding(
                "<Gamepad>/rightStickPress", groups: GamepadGroup);
            TargetCycle.performed +=
                _ => _state.PushEdge(SemanticEdge.TargetCycle);

            TargetClear = _map.AddAction(
                "TargetClear", InputActionType.Button);
            TargetClear.AddBinding(
                "<Keyboard>/escape", groups: KeyboardGroup);
            TargetClear.performed +=
                _ => _state.PushEdge(SemanticEdge.TargetClear);

            ChatOpen = _map.AddAction("ChatOpen", InputActionType.Button);
            ChatOpen.AddBinding("<Keyboard>/enter", groups: KeyboardGroup);
            ChatOpen.AddBinding("<Gamepad>/select", groups: GamepadGroup);
            ChatOpen.performed +=
                _ => _state.PushEdge(SemanticEdge.ChatOpen);

            UiNavigate = _map.AddAction(
                "UiNavigate", InputActionType.Button);
            UiNavigate.AddBinding(
                "<Gamepad>/dpad/up", groups: GamepadGroup);
            UiNavigate.AddBinding(
                "<Gamepad>/dpad/down", groups: GamepadGroup);
            UiNavigate.AddBinding(
                "<Gamepad>/dpad/left", groups: GamepadGroup);
            UiNavigate.AddBinding(
                "<Gamepad>/dpad/right", groups: GamepadGroup);
            UiNavigate.performed +=
                _ => _state.PushEdge(SemanticEdge.UiNavigate);
        }

        public InputActionMap Map
        {
            get
            {
                return _map;
            }
        }

        public InputAction Move
        {
            get;
            private set;
        }
        public InputAction Down
        {
            get;
            private set;
        }
        public InputAction Jump
        {
            get;
            private set;
        }
        public InputAction Basic
        {
            get;
            private set;
        }
        public InputAction Interact
        {
            get;
            private set;
        }
        public InputAction TargetCycle
        {
            get;
            private set;
        }
        public InputAction TargetClear
        {
            get;
            private set;
        }
        public InputAction ChatOpen
        {
            get;
            private set;
        }
        public InputAction UiNavigate
        {
            get;
            private set;
        }

        /// <summary>Skill actions 1..5 in slot order.</summary>
        public InputAction SkillAt(int slot)
        {
            return _skills[slot];
        }

        public void Enable()
        {
            _map.Enable();
        }

        public void Disable()
        {
            _map.Disable();
        }

        public void Dispose()
        {
            _map.Dispose();
        }

        private void OnMove(InputAction.CallbackContext context)
        {
            float axis = context.ReadValue<float>();
            _state.SetDirection(
                axis < -0.0001f ? -1 : axis > 0.0001f ? 1 : 0);
        }

        private void OnJump(InputAction.CallbackContext context)
        {
            // S + Space / Down + A is the drop-through chord (§3).
            _state.PushEdge(
                _state.DownHeld ? SemanticEdge.Drop : SemanticEdge.Jump);
        }
    }
}
