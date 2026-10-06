using System;
using ThinhThan.Protocol.V1;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Dirty sections of <see cref="HudDataModel"/> — PERF-022: widgets
    /// only re-draw sections whose bit was marked this frame.
    /// </summary>
    [Flags]
    public enum HudDirty : uint
    {
        None = 0U,
        Vitals = 0x1U,
        Target = 0x2U,
        Statuses = 0x4U,
        Cooldowns = 0x8U,
        WorldContext = 0x10U,
        QuestDock = 0x20U,
        ChatDock = 0x40U,
        Interact = 0x80U,
    }

    /// <summary>
    /// One buff/debuff row on the HUD (messages.md EntityStatus +
    /// S2C_STATUS_EVENT): effect id, shape-differentiated kind, stacks,
    /// expiry in server ticks.
    /// </summary>
    public readonly struct HudStatusSlot
    {
        public readonly string EffectId;
        public readonly string StatusKind;
        public readonly uint Stacks;
        public readonly ulong ExpiresAtTick;

        public HudStatusSlot(
            string effectId, string statusKind, uint stacks,
            ulong expiresAtTick)
        {
            EffectId = effectId;
            StatusKind = statusKind;
            Stacks = stacks;
            ExpiresAtTick = expiresAtTick;
        }
    }

    /// <summary>
    /// One hotbar cooldown slot: skill id plus absolute end tick
    /// (S2C_ACTION_STARTED.cooldown_ends_at_tick).
    /// </summary>
    public readonly struct HudCooldownSlot
    {
        public readonly int Slot;
        public readonly string SkillId;
        public readonly ulong CooldownEndsAtTick;
        public readonly ulong ServerTick;

        public HudCooldownSlot(
            int slot, string skillId, ulong cooldownEndsAtTick,
            ulong serverTick)
        {
            Slot = slot;
            SkillId = skillId;
            CooldownEndsAtTick = cooldownEndsAtTick;
            ServerTick = serverTick;
        }

        /// <summary>0..1 fill remaining at <paramref name="nowTick"/>.</summary>
        public float Fill(ulong nowTick)
        {
            if (CooldownEndsAtTick <= ServerTick || nowTick >= CooldownEndsAtTick)
            {
                return 0f;
            }

            return (float)(
                (CooldownEndsAtTick - nowTick) /
                (double)(CooldownEndsAtTick - ServerTick));
        }
    }

    /// <summary>
    /// Dirty-flagged projection the four trackers write and
    /// <see cref="CoreHudView"/> reads — PERF-022: a value change marks its
    /// section dirty; the view applies dirty sections at most once per
    /// frame at the UI phase. Fixed buffers — no per-frame allocation.
    /// </summary>
    public sealed class HudDataModel
    {
        public const int StatusCapacity = 16;
        public const int CooldownCapacity = 5;

        private readonly HudStatusSlot[] _statuses =
            new HudStatusSlot[StatusCapacity];
        private int _statusCount;
        private readonly HudCooldownSlot[] _cooldowns =
            new HudCooldownSlot[CooldownCapacity];
        private int _cooldownCount;

        /// <summary>Sections dirty this frame (cleared by the view apply).</summary>
        public HudDirty Dirty
        {
            get;
            private set;
        }

        // ---- vitals (self) ----
        public long Hp;
        public long MaxHp;
        public long Shield;
        public long Mp;
        public long MaxMp;
        public bool Dead;

        // ---- target frame ----
        public bool HasTarget;
        public ulong TargetEntityId;
        public string? TargetName;
        public uint TargetLevel;
        public EntityKind TargetKind;
        public long TargetHp;
        public long TargetMaxHp;
        public bool TargetDead;

        // ---- world context (top-right) ----
        public string? MapDisplay;
        public uint ChannelIndex;
        public int PingMs;

        // ---- docks ----
        public string? QuestDockText;
        public string? ChatDockText;

        // ---- context prompt ----
        public bool HasInteract;
        public string? InteractLabel;

        public void Mark(HudDirty section)
        {
            Dirty |= section;
        }

        /// <summary>Clears every dirty bit (called after the view apply).</summary>
        public void ClearDirty()
        {
            Dirty = HudDirty.None;
        }

        /// <summary>Status rows as a read-only view (no copy).</summary>
        public ReadOnlySpan<HudStatusSlot> Statuses
        {
            get
            {
                return new ReadOnlySpan<HudStatusSlot>(_statuses, 0, _statusCount);
            }
        }

        /// <summary>Cooldown slots as a read-only view (no copy).</summary>
        public ReadOnlySpan<HudCooldownSlot> Cooldowns
        {
            get
            {
                return new ReadOnlySpan<HudCooldownSlot>(_cooldowns, 0, _cooldownCount);
            }
        }

        /// <summary>Overwrites the status rows (authoritative or feed-driven).</summary>
        public void SetStatuses(ReadOnlySpan<HudStatusSlot> rows)
        {
            int n = Math.Min(rows.Length, StatusCapacity);
            for (int i = 0; i < n; i++)
            {
                _statuses[i] = rows[i];
            }

            _statusCount = n;
            Mark(HudDirty.Statuses);
        }

        /// <summary>Overwrites the cooldown slots.</summary>
        public void SetCooldowns(ReadOnlySpan<HudCooldownSlot> rows)
        {
            int n = Math.Min(rows.Length, CooldownCapacity);
            for (int i = 0; i < n; i++)
            {
                _cooldowns[i] = rows[i];
            }

            _cooldownCount = n;
            Mark(HudDirty.Cooldowns);
        }

        /// <summary>World-context readout (map name, channel, ping).</summary>
        public void SetWorldContext(string? mapDisplay, uint channel, int pingMs)
        {
            if (mapDisplay == MapDisplay && channel == ChannelIndex &&
                pingMs == PingMs)
            {
                return;
            }

            MapDisplay = mapDisplay;
            ChannelIndex = channel;
            PingMs = pingMs;
            Mark(HudDirty.WorldContext);
        }

        /// <summary>Quest dock body (composition-sourced).</summary>
        public void SetQuestDock(string? text)
        {
            if (text == QuestDockText)
            {
                return;
            }

            QuestDockText = text;
            Mark(HudDirty.QuestDock);
        }

        /// <summary>Chat dock body (composition-sourced).</summary>
        public void SetChatDock(string? text)
        {
            if (text == ChatDockText)
            {
                return;
            }

            ChatDockText = text;
            Mark(HudDirty.ChatDock);
        }

        /// <summary>Context-interact prompt (resolver-sourced).</summary>
        public void SetInteract(bool hasInteract, string? label)
        {
            if (hasInteract == HasInteract && label == InteractLabel)
            {
                return;
            }

            HasInteract = hasInteract;
            InteractLabel = label;
            Mark(HudDirty.Interact);
        }
    }
}
