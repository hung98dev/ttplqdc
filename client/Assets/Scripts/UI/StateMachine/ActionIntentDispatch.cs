using System;
using Google.Protobuf;
using ThinhThan.Core.Input;
using ThinhThan.Core.Runtime;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Movement;
using ThinhThan.Systems.Replication;
using UnityEngine;

namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Gameplay-action edge → wire bridge (client_experience_contract.md §3,
    /// ADR-0071): context interact resolves to C2S_PORTAL_USE (104) or
    /// C2S_INTERACT (103) with a minted operation_id; skill slots send
    /// C2S_SKILL_USE (200); basic sends C2S_BASIC_ATTACK (201); target
    /// cycle/clear send C2S_TARGET_INTENT (202) — the ONLY targeting path,
    /// so the client never displays a locally-chosen target.
    /// <para>
    /// client_mono_ms is <c>FrameTime.NowSeconds * 1000</c> (single clock);
    /// facing/position come from the predicted state; target_entity_id is
    /// always the server-accepted target — intents only.
    /// </para>
    /// </summary>
    public sealed class ActionIntentDispatch
    {
        private const int EdgeBufferSize = 16;

        private readonly IMovementSender _sender;
        private readonly IInteractResolver _interact;
        private readonly ITargetSelector _targets;
        private readonly ISkillLoadout _loadout;
        private readonly MovementPredictionSystem _prediction;
        private readonly ReplicationApplier _replication;
        private readonly Func<byte[]> _mintOperationId;
        private readonly SemanticEdge[] _edges =
            new SemanticEdge[EdgeBufferSize];

        /// <summary>
        /// <paramref name="mintOperationId"/> defaults to UUIDv7 minting;
        /// tests inject a deterministic counter.
        /// </summary>
        public ActionIntentDispatch(
            IMovementSender sender,
            IInteractResolver interact,
            ITargetSelector targets,
            ISkillLoadout loadout,
            MovementPredictionSystem prediction,
            ReplicationApplier replication,
            Func<byte[]>? mintOperationId = null)
        {
            _sender = sender ?? throw new ArgumentNullException(nameof(sender));
            _interact = interact ??
                throw new ArgumentNullException(nameof(interact));
            _targets = targets ??
                throw new ArgumentNullException(nameof(targets));
            _loadout = loadout ??
                throw new ArgumentNullException(nameof(loadout));
            _prediction = prediction ??
                throw new ArgumentNullException(nameof(prediction));
            _replication = replication ??
                throw new ArgumentNullException(nameof(replication));
            _mintOperationId = mintOperationId ??
                new OperationIdMinter().Mint;
        }

        /// <summary>
        /// Drains gameplay-action edges queued on the shared semantic state
        /// and issues one discrete send each. Called once per frame from
        /// <see cref="ClientInputPhase"/> while UI is IN_WORLD.
        /// </summary>
        public void Dispatch(in FrameTime time, GameInputSource input)
        {
            int n = input.SampleActionEdges(_edges);
            for (int i = 0; i < n; i++)
            {
                DispatchEdge(_edges[i], in time);
            }
        }

        private void DispatchEdge(SemanticEdge edge, in FrameTime time)
        {
            PredictedState predicted = _prediction.State;
            switch (edge)
            {
                case SemanticEdge.BasicAttack:
                    Send(UiWireIds.C2SBasicAttack, new C2SBasicAttack
                    {
                        ClientMonoMs = MonoMs(in time),
                        Facing = predicted.Facing,
                        TargetEntityId = AcceptedTarget(),
                    });
                    break;

                case SemanticEdge.ContextInteract:
                    DispatchInteract(in predicted);
                    break;

                case SemanticEdge.TargetCycle:
                    Send(UiWireIds.C2STargetIntent, new C2STargetIntent
                    {
                        TargetEntityId = _targets.NextHostile(
                            predicted.XMm, predicted.YMm, AcceptedTarget()),
                    });
                    break;

                case SemanticEdge.TargetClear:
                    Send(UiWireIds.C2STargetIntent, new C2STargetIntent
                    {
                        TargetEntityId = 0UL,
                    });
                    break;

                default:
                    DispatchSkill(edge, in predicted, in time);
                    break;
            }
        }

        private void DispatchSkill(
            SemanticEdge edge, in PredictedState predicted, in FrameTime time)
        {
            int slot = (int)edge - (int)SemanticEdge.Skill1;
            string? skillId = _loadout.SkillIdAt(slot + 1);
            if (skillId == null)
            {
                return;
            }

            Send(UiWireIds.C2SSkillUse, new C2SSkillUse
            {
                ClientMonoMs = MonoMs(in time),
                SkillId = skillId,
                Facing = predicted.Facing,
                TargetEntityId = AcceptedTarget(),
            });
        }

        /// <summary>
        /// F/LT/context → nearest interactable by distance to the character
        /// anchor; a portal wins ties → 104, anything else → 103. No other
        /// key may send 104.
        /// </summary>
        private void DispatchInteract(in PredictedState predicted)
        {
            if (!_interact.TryResolve(
                predicted.XMm, predicted.YMm, out InteractTarget target))
            {
                return;
            }

            if (target.IsPortal)
            {
                if (target.PortalId == null)
                {
                    return;
                }

                Send(UiWireIds.C2SPortalUse, new C2SPortalUse
                {
                    OperationId = ByteString.CopyFrom(_mintOperationId()),
                    PortalId = target.PortalId,
                });
                return;
            }

            if (target.TargetId == null)
            {
                return;
            }

            Send(UiWireIds.C2SInteract, new C2SInteract
            {
                InteractKind = target.Kind,
                TargetId = target.TargetId,
                OperationId = ByteString.CopyFrom(_mintOperationId()),
            });
        }

        /// <summary>Server-accepted target id (0 = none).</summary>
        private ulong AcceptedTarget()
        {
            SelfPrivateState? selfPrivate = _replication.SelfPrivate;
            return selfPrivate != null
                ? selfPrivate.AcceptedTargetEntityId
                : 0UL;
        }

        private void Send(uint id, IMessage payload)
        {
            // Discrete intents: fire and forget on the shared seq-stamping
            // seam; rejects come back as 204/116 for the HUD feed.
            _ = _sender.SendAsync(id, payload, default);
        }

        private static ulong MonoMs(in FrameTime time)
        {
            return (ulong)(time.NowSeconds * 1000.0);
        }
    }
}
