# Specification Traceability
status: LOCKED

## Scope
Generated consumer index from every specification file, ADR and requirement ID to the task packets that consume it. Packets in `task_queue.md` remain the owner of exact `specs:`, `adrs:`, paths, acceptance and tests; this index is regenerated whenever a packet's `specs:`/`adrs:` change and Q0 fails on any mismatch.

Every implementation change must additionally grep the exact symbol/ID/constant it changes; this index does not replace the contract-change rule in `AGENTS.md`.

## Global Inputs
All tasks consume `../README.md`, all files in `../00_context/`, accepted ADRs whose Consequences name a consumed spec, and this directory's `repository_layout.md`, `architecture_conformance.md`, `engineering_conventions.md`, `agent_execution_protocol.md` and `definition_of_done.md`.

## Spec → Tasks

| Spec (relative to `docs/`) | Tasks |
|---|---|
| `00_context/constraints.md` | IMP-000, IMP-075 |
| `00_context/glossary.md` | IMP-000 |
| `00_context/non_goals.md` | IMP-000 |
| `00_context/technology_versions.md` | IMP-000, IMP-001, IMP-048, IMP-067, IMP-068, IMP-081, IMP-096, IMP-098 |
| `00_context/vision.md` | IMP-000 |
| `01_gameplay/README.md` | IMP-020 |
| `01_gameplay/character.md` | IMP-100 |
| `01_gameplay/classes.md` | IMP-017, IMP-071 |
| `01_gameplay/combat.md` | IMP-014, IMP-016, IMP-039, IMP-049, IMP-084, IMP-092 |
| `01_gameplay/core_loop.md` | IMP-020 |
| `01_gameplay/death_respawn.md` | IMP-084 |
| `01_gameplay/movement.md` | IMP-013, IMP-062, IMP-066, IMP-078 |
| `01_gameplay/progression.md` | IMP-011, IMP-090 |
| `01_gameplay/skills.md` | IMP-003, IMP-004, IMP-014, IMP-015, IMP-017, IMP-049 |
| `01_gameplay/stats.md` | IMP-011 |
| `01_gameplay/status_effects.md` | IMP-016, IMP-084, IMP-092 |
| `02_world/README.md` | IMP-018 |
| `02_world/bosses.md` | IMP-022, IMP-052, IMP-091 |
| `02_world/dungeons.md` | IMP-023, IMP-024 |
| `02_world/maps_zones.md` | IMP-018, IMP-055, IMP-072, IMP-084 |
| `02_world/monsters.md` | IMP-019 |
| `02_world/npcs.md` | IMP-020, IMP-028 |
| `02_world/quests.md` | IMP-021, IMP-089, IMP-090 |
| `02_world/spawning.md` | IMP-019 |
| `02_world/world_rules.md` | IMP-018, IMP-023, IMP-025, IMP-055, IMP-058, IMP-059, IMP-063, IMP-087, IMP-101 |
| `03_systems/README.md` | IMP-007 |
| `03_systems/account_storage.md` | IMP-009, IMP-053, IMP-102 |
| `03_systems/atlas.md` | IMP-052, IMP-060 |
| `03_systems/cosmetics.md` | IMP-038, IMP-053, IMP-074, IMP-085, IMP-102 |
| `03_systems/crafting.md` | IMP-027, IMP-088 |
| `03_systems/economy.md` | IMP-007 |
| `03_systems/equipment.md` | IMP-012, IMP-026 |
| `03_systems/formations.md` | IMP-033 |
| `03_systems/guild.md` | IMP-036, IMP-093 |
| `03_systems/guild_progression.md` | IMP-036 |
| `03_systems/guild_storage.md` | IMP-037 |
| `03_systems/guild_war.md` | IMP-042, IMP-062, IMP-067, IMP-105 |
| `03_systems/inventory.md` | IMP-009 |
| `03_systems/items.md` | IMP-008, IMP-090 |
| `03_systems/monetization.md` | IMP-053, IMP-102 |
| `03_systems/party.md` | IMP-035 |
| `03_systems/pvp.md` | IMP-039, IMP-040, IMP-041, IMP-062, IMP-067, IMP-087, IMP-105 |
| `03_systems/reward_claims.md` | IMP-010 |
| `03_systems/seasons.md` | IMP-052, IMP-093, IMP-102 |
| `03_systems/social.md` | IMP-034, IMP-086, IMP-094 |
| `03_systems/soul_contracts.md` | IMP-031 |
| `03_systems/spirit_beasts.md` | IMP-050, IMP-057, IMP-092 |
| `03_systems/spirit_meridian.md` | IMP-032 |
| `03_systems/trading_auction.md` | IMP-029, IMP-030, IMP-054 |
| `04_architecture/authority.md` | IMP-006, IMP-069, IMP-077, IMP-079 |
| `04_architecture/backend.md` | IMP-000, IMP-069, IMP-080, IMP-098 |
| `04_architecture/client.md` | IMP-063, IMP-065, IMP-066, IMP-067, IMP-095, IMP-099, IMP-101 |
| `04_architecture/client_assets.md` | IMP-063, IMP-067, IMP-070, IMP-075, IMP-076, IMP-088, IMP-101 |
| `04_architecture/client_experience_contract.md` | IMP-063, IMP-065, IMP-066, IMP-067, IMP-099, IMP-103 |
| `04_architecture/client_localization.md` | IMP-064, IMP-067, IMP-073, IMP-099 |
| `04_architecture/client_performance.md` | IMP-000, IMP-018, IMP-048, IMP-063, IMP-065, IMP-066, IMP-067, IMP-083, IMP-088, IMP-095, IMP-096, IMP-099, IMP-101 |
| `04_architecture/concurrency.md` | IMP-002, IMP-079, IMP-080, IMP-082 |
| `04_architecture/physics_geometry_contract.md` | IMP-004, IMP-013, IMP-015, IMP-018, IMP-019, IMP-022, IMP-023, IMP-024, IMP-040, IMP-041, IMP-042, IMP-062, IMP-063, IMP-066, IMP-067, IMP-070, IMP-071, IMP-072, IMP-078, IMP-104, IMP-105 |
| `04_architecture/realtime_loop.md` | IMP-013, IMP-014, IMP-055, IMP-062, IMP-069, IMP-078, IMP-079 |
| `04_architecture/service_boundaries.md` | IMP-022, IMP-025, IMP-069, IMP-080, IMP-081 |
| `04_architecture/system_overview.md` | IMP-000, IMP-069, IMP-080 |
| `05_network/errors.md` | IMP-006, IMP-061, IMP-065, IMP-081, IMP-100 |
| `05_network/messages.md` | IMP-009, IMP-010, IMP-013, IMP-014, IMP-018, IMP-020, IMP-029, IMP-034, IMP-035, IMP-036, IMP-037, IMP-040, IMP-041, IMP-042, IMP-061, IMP-062, IMP-066, IMP-078, IMP-079, IMP-087, IMP-100, IMP-103 |
| `05_network/protobuf_conventions.md` | IMP-061 |
| `05_network/protocol.md` | IMP-006, IMP-014, IMP-061, IMP-065, IMP-081 |
| `05_network/reconnect.md` | IMP-065, IMP-069 |
| `05_network/synchronization.md` | IMP-013, IMP-055, IMP-061, IMP-062, IMP-065, IMP-066, IMP-078, IMP-079 |
| `05_network/versioning.md` | IMP-061, IMP-065, IMP-067, IMP-081 |
| `06_data/config.md` | IMP-001, IMP-002, IMP-003, IMP-004, IMP-078 |
| `06_data/content_authoring_contract.md` | IMP-003, IMP-004 |
| `06_data/data_model.md` | IMP-005, IMP-006, IMP-007, IMP-008, IMP-010, IMP-022, IMP-029, IMP-030, IMP-034, IMP-036, IMP-037, IMP-040, IMP-041, IMP-042, IMP-053, IMP-054, IMP-056, IMP-077, IMP-086, IMP-091, IMP-094, IMP-097, IMP-100 |
| `06_data/database.md` | IMP-005, IMP-022, IMP-056, IMP-082, IMP-097 |
| `06_data/ids.md` | IMP-001 |
| `06_data/migrations.md` | IMP-005, IMP-047 |
| `06_data/physical_schema_contract.md` | IMP-005, IMP-006, IMP-007, IMP-008, IMP-009, IMP-010, IMP-030, IMP-036, IMP-053, IMP-100 |
| `06_data/save_rules.md` | IMP-005, IMP-082, IMP-091 |
| `06_data/text.md` | IMP-064, IMP-100 |
| `07_content/README.md` | IMP-003 |
| `07_content/atlas_catalog.md` | IMP-060 |
| `07_content/balance_validation.md` | IMP-004, IMP-049 |
| `07_content/boss_catalog.md` | IMP-003, IMP-022, IMP-092, IMP-104 |
| `07_content/build_catalog.md` | IMP-032, IMP-033 |
| `07_content/class_skill_catalog.md` | IMP-003, IMP-004, IMP-015, IMP-016, IMP-017, IMP-049, IMP-073 |
| `07_content/cosmetic_catalog.md` | IMP-038, IMP-074, IMP-085, IMP-086 |
| `07_content/crafting_catalog.md` | IMP-027, IMP-059 |
| `07_content/drop_tables.md` | IMP-051 |
| `07_content/dungeon_catalog.md` | IMP-003, IMP-022, IMP-023, IMP-024, IMP-062, IMP-067, IMP-105 |
| `07_content/economy_catalog.md` | IMP-058 |
| `07_content/encounter_catalog.md` | IMP-023 |
| `07_content/equipment_catalog.md` | IMP-012, IMP-026, IMP-073 |
| `07_content/integration_validation.md` | IMP-004, IMP-021 |
| `07_content/item_catalog.md` | IMP-003, IMP-057, IMP-073, IMP-090 |
| `07_content/map_spawn_catalog.md` | IMP-019 |
| `07_content/monster_catalog.md` | IMP-003, IMP-019, IMP-051, IMP-104 |
| `07_content/npc_shop_catalog.md` | IMP-020, IMP-028 |
| `07_content/presentation_asset_manifest.md` | IMP-063, IMP-067, IMP-070, IMP-071, IMP-072, IMP-073, IMP-074, IMP-075, IMP-076, IMP-088, IMP-095, IMP-101, IMP-104, IMP-105 |
| `07_content/progression_route.md` | IMP-003, IMP-004 |
| `07_content/quest_catalog.md` | IMP-021, IMP-089 |
| `07_content/soul_catalog.md` | IMP-031 |
| `07_content/spirit_beast_catalog.md` | IMP-050, IMP-057, IMP-104 |
| `07_content/world_event_catalog.md` | IMP-025 |
| `07_content/world_route_catalog.md` | IMP-003, IMP-018, IMP-020, IMP-022, IMP-023, IMP-062, IMP-067, IMP-072 |
| `07_security/anti_cheat.md` | IMP-037, IMP-045, IMP-054 |
| `07_security/auth.md` | IMP-006, IMP-045, IMP-065, IMP-077, IMP-099, IMP-103 |
| `07_security/data_protection.md` | IMP-056, IMP-094, IMP-103 |
| `07_security/external_integrations.md` | IMP-006, IMP-045, IMP-053, IMP-068 |
| `07_security/personal_data_register.md` | IMP-056, IMP-094 |
| `07_security/rate_limits.md` | IMP-006, IMP-045, IMP-081 |
| `07_security/session.md` | IMP-006, IMP-045, IMP-065, IMP-099 |
| `07_security/validation.md` | IMP-045, IMP-053, IMP-077, IMP-091 |
| `08_scale_ops/backup_recovery.md` | IMP-047, IMP-056, IMP-103 |
| `08_scale_ops/caching.md` | IMP-043 |
| `08_scale_ops/capacity.md` | IMP-000, IMP-046, IMP-055, IMP-079, IMP-081, IMP-082 |
| `08_scale_ops/deployment.md` | IMP-046, IMP-047, IMP-048, IMP-067, IMP-068, IMP-069 |
| `08_scale_ops/observability.md` | IMP-043, IMP-048, IMP-077, IMP-098 |
| `08_scale_ops/sharding.md` | IMP-018, IMP-022, IMP-048 |
| `09_testing/backend.md` | IMP-043, IMP-044 |
| `09_testing/gameplay.md` | IMP-002, IMP-014, IMP-016 |
| `09_testing/load.md` | IMP-046, IMP-095 |
| `09_testing/network.md` | IMP-013, IMP-045, IMP-062 |
| `09_testing/strategy.md` | IMP-044, IMP-048 |
| `09_testing/test_and_release_evidence.md` | IMP-000, IMP-048, IMP-065, IMP-068, IMP-079, IMP-096 |
| `10_implementation/README.md` | IMP-083 |
| `10_implementation/agent_execution_protocol.md` | IMP-000, IMP-068 |
| `10_implementation/architecture_conformance.md` | IMP-000, IMP-069, IMP-083 |
| `10_implementation/audit_gates.md` | IMP-000, IMP-048, IMP-068, IMP-083, IMP-095, IMP-096 |
| `10_implementation/definition_of_done.md` | IMP-048, IMP-076 |
| `10_implementation/dependency_graph.md` | IMP-083 |
| `10_implementation/engineering_conventions.md` | IMP-000, IMP-065, IMP-066, IMP-079, IMP-081, IMP-083, IMP-095 |
| `10_implementation/known_blockers.md` | IMP-068 |
| `10_implementation/milestones.md` | IMP-048 |
| `10_implementation/repository_layout.md` | IMP-000, IMP-061, IMP-063, IMP-064, IMP-070, IMP-083, IMP-101 |
| `10_implementation/spec_traceability.md` | IMP-083 |
| `10_implementation/task_queue.md` | IMP-083 |
| `10_implementation/wave_execution_prompts.md` | IMP-083 |
| `README.md` | IMP-083 |
| `templates/adr.md` | IMP-083 |
| `templates/spec.md` | IMP-083 |
| `templates/task.md` | IMP-083 |

## ADR → Tasks

| ADR | Tasks |
|---|---|
| `0001-content-revision-contract.md` | IMP-001, IMP-003, IMP-004 |
| `0002-effect-value-shield-contract.md` | IMP-016 |
| `0003-spawn-selector-anchor-contract.md` | IMP-019 |
| `0004-party-dungeon-scaling-reward-slots.md` | IMP-023, IMP-024, IMP-035 |
| `0005-skill-action-timing-geometry.md` | IMP-015, IMP-062, IMP-078 |
| `0006-unity-go-postgresql-stack.md` | IMP-000, IMP-067 |
| `0007-single-owner-fixed-step-simulation.md` | IMP-002, IMP-069, IMP-079, IMP-084 |
| `0008-client-network-transport-protocol.md` | IMP-040, IMP-041, IMP-042, IMP-045, IMP-061, IMP-065, IMP-081 |
| `0009-account-session-credentials.md` | IMP-006, IMP-077 |
| `0010-exact-technology-version-pinning.md` | IMP-000, IMP-043, IMP-067, IMP-068, IMP-098 |
| `0011-postgresql-relational-persistence.md` | IMP-005, IMP-044, IMP-047, IMP-056, IMP-082 |
| `0012-reward-claim-item-materialization.md` | IMP-010, IMP-031 |
| `0013-canonical-unicode-text-normalization.md` | IMP-034, IMP-094, IMP-100 |
| `0014-unity-addressables-asset-delivery.md` | IMP-063, IMP-070, IMP-075, IMP-076 |
| `0015-unity-localization.md` | IMP-064, IMP-099 |
| `0016-twelve-skill-pool-upgradeable-basics.md` | IMP-003, IMP-004, IMP-015, IMP-017, IMP-032, IMP-033, IMP-049, IMP-073 |
| `0017-percentage-dodge-stat-no-active-dodge.md` | IMP-039 |
| `0018-combat-target-caps-and-skill-scaling.md` | IMP-014 |
| `0019-spirit-beast-companion-system.md` | IMP-050, IMP-057 |
| `0020-map-channel-capacity-contract.md` | IMP-018, IMP-046, IMP-055 |
| `0021-hardcore-enhancement-rate-curve.md` | IMP-012, IMP-026 |
| `0022-four-tier-lucky-charm-system.md` | IMP-027 |
| `0023-engagement-loops-weapon-glow-bonfire-chivalry-chests-sparring.md` | IMP-059, IMP-086, IMP-087, IMP-088 |
| `0024-fishing-cooking-feats-titles-boss-chest-ceremony.md` | IMP-058, IMP-059, IMP-085, IMP-091 |
| `0025-peak-moments-and-progression-books.md` | IMP-020, IMP-021, IMP-090 |
| `0026-just-guard-and-ma-am-status.md` | IMP-014, IMP-092 |
| `0027-world-liveliness-mystery-bounty-capacity.md` | IMP-025, IMP-089 |
| `0028-atlas-soft-pity-guild-stone-morning-market.md` | IMP-028, IMP-030, IMP-036, IMP-060, IMP-093 |
| `0029-character-resource-isolation.md` | IMP-007, IMP-008, IMP-009, IMP-037, IMP-053, IMP-086, IMP-100, IMP-102 |
| `0030-one-account-one-live-session.md` | IMP-006, IMP-065, IMP-069, IMP-100 |
| `0031-exp-scale-x100-and-corrected-act-budgets.md` | IMP-003, IMP-004, IMP-011, IMP-019, IMP-021, IMP-022, IMP-049, IMP-051, IMP-089, IMP-092, IMP-104 |
| `0032-seven-channel-exp-source-portfolio.md` | IMP-003, IMP-004, IMP-011, IMP-049 |
| `0033-skill-unlock-schedule-remap.md` | IMP-003, IMP-004, IMP-011, IMP-017, IMP-090 |
| `0034-just-guard-edge-trigger-streak.md` | IMP-011, IMP-014, IMP-016, IMP-039, IMP-049, IMP-084, IMP-092 |
| `0035-spawn-density-increase.md` | IMP-018, IMP-019, IMP-025, IMP-046, IMP-051, IMP-055, IMP-058, IMP-059, IMP-063, IMP-087, IMP-101 |
| `0036-seasons-as-launch-infrastructure.md` | IMP-023, IMP-039, IMP-040, IMP-041, IMP-052, IMP-062, IMP-067, IMP-087, IMP-093, IMP-102, IMP-105 |
| `0037-reflect-lifesteal-absorb-heal-reduction-stats.md` | IMP-002, IMP-004, IMP-011, IMP-012, IMP-014, IMP-016, IMP-026, IMP-032, IMP-033, IMP-039, IMP-040, IMP-041, IMP-049, IMP-050, IMP-057, IMP-062, IMP-067, IMP-073, IMP-084, IMP-087, IMP-092, IMP-105 |
| `0038-discrete-movement-edge-input-message.md` | IMP-013, IMP-014, IMP-016, IMP-039, IMP-049, IMP-055, IMP-061, IMP-062, IMP-065, IMP-066, IMP-069, IMP-078, IMP-079, IMP-081, IMP-084, IMP-092 |
| `0039-entity-capacity-model-and-ai-budget-classes.md` | IMP-013, IMP-014, IMP-019, IMP-046, IMP-055, IMP-061, IMP-062, IMP-065, IMP-066, IMP-069, IMP-078, IMP-079, IMP-082 |
| `0040-world-consequence-durable-aggregate.md` | IMP-000, IMP-005, IMP-013, IMP-014, IMP-022, IMP-052, IMP-055, IMP-062, IMP-069, IMP-078, IMP-079, IMP-080, IMP-082, IMP-091, IMP-097, IMP-098 |
| `0041-anti-rmt-trade-gates-and-iap-entitlement-integrity.md` | IMP-009, IMP-029, IMP-030, IMP-037, IMP-038, IMP-045, IMP-053, IMP-054, IMP-077, IMP-091, IMP-102 |
| `0042-atlas-roster-expansion-104-pages.md` | IMP-052, IMP-060 |
| `0043-spirit-beast-instance-identity.md` | IMP-001, IMP-008, IMP-050, IMP-057, IMP-090, IMP-092 |
| `0044-launch-topology-single-binary-role-modes.md` | IMP-069, IMP-080, IMP-081, IMP-083 |
| `0045-ci-evidence-without-self-referential-sha.md` | IMP-068, IMP-076 |
| `0046-reference-viewport-entity-scale-and-map-geometry.md` | IMP-003, IMP-004, IMP-013, IMP-015, IMP-018, IMP-019, IMP-022, IMP-023, IMP-024, IMP-040, IMP-041, IMP-042, IMP-052, IMP-062, IMP-063, IMP-065, IMP-066, IMP-067, IMP-070, IMP-071, IMP-072, IMP-073, IMP-078, IMP-091, IMP-092, IMP-104, IMP-105 |
| `0047-skill-reach-budget-and-collider-aware-resolution.md` | IMP-003, IMP-004, IMP-014, IMP-015, IMP-049 |
| `0048-character-update-timestamp.md` | IMP-005, IMP-047, IMP-100 |
| `0049-guild-storage-same-account-transfer-prohibition.md` | IMP-037 |
| `0050-windows-only-ci-and-auto-merge.md` | IMP-000, IMP-048, IMP-061, IMP-063, IMP-067, IMP-068, IMP-070, IMP-083, IMP-095, IMP-096 |
| `0051-first-party-username-password-login.md` | IMP-006, IMP-045, IMP-053, IMP-056, IMP-065, IMP-068, IMP-077, IMP-081, IMP-083, IMP-094, IMP-099, IMP-103 |
| `0052-single-launch-world.md` | IMP-000, IMP-006, IMP-045, IMP-046, IMP-047, IMP-048, IMP-055, IMP-065, IMP-067, IMP-068, IMP-069, IMP-079, IMP-080, IMP-082, IMP-083, IMP-095, IMP-099 |
| `0053-durable-contract-reconciliation.md` | IMP-002, IMP-005, IMP-009, IMP-022, IMP-037, IMP-038, IMP-045, IMP-052, IMP-053, IMP-054, IMP-065, IMP-069, IMP-074, IMP-079, IMP-080, IMP-081, IMP-082, IMP-085, IMP-091, IMP-097, IMP-102 |
| `0054-wire-message-completion.md` | IMP-006, IMP-014, IMP-016, IMP-039, IMP-045, IMP-049, IMP-061, IMP-065, IMP-081, IMP-084, IMP-092 |
| `0055-2x-texture-authoring-and-cutout-quality-gate.md` | IMP-004, IMP-013, IMP-015, IMP-018, IMP-019, IMP-022, IMP-023, IMP-024, IMP-040, IMP-041, IMP-042, IMP-048, IMP-062, IMP-063, IMP-066, IMP-067, IMP-070, IMP-071, IMP-072, IMP-073, IMP-074, IMP-075, IMP-076, IMP-078, IMP-088, IMP-095, IMP-101, IMP-104, IMP-105 |
| `0056-volumetric-art-direction-and-2d-lighting.md` | IMP-018, IMP-025, IMP-055, IMP-058, IMP-059, IMP-063, IMP-065, IMP-066, IMP-067, IMP-070, IMP-071, IMP-072, IMP-073, IMP-074, IMP-075, IMP-076, IMP-087, IMP-088, IMP-095, IMP-099, IMP-101, IMP-104, IMP-105 |
| `0057-bootstrap-trusted-ci-evidence-identity-and-merge-mechanics.md` | IMP-000, IMP-048, IMP-068, IMP-076, IMP-083, IMP-095, IMP-096 |
| `0058-public-repo-github-hosted-linux-and-windows-runners.md` | IMP-000, IMP-005, IMP-048, IMP-067, IMP-068, IMP-070, IMP-095, IMP-096 |
| `0059-client-smoothness-by-construction-and-machine-enforced-code-quality.md` | IMP-000, IMP-018, IMP-061, IMP-063, IMP-065, IMP-066, IMP-067, IMP-079, IMP-081, IMP-083, IMP-095, IMP-099, IMP-101 |
| `0060-wire-and-durable-contract-completion.md` | IMP-005, IMP-006, IMP-007, IMP-008, IMP-009, IMP-010, IMP-011, IMP-012, IMP-013, IMP-014, IMP-017, IMP-018, IMP-020, IMP-021, IMP-022, IMP-023, IMP-027, IMP-028, IMP-029, IMP-030, IMP-031, IMP-034, IMP-035, IMP-036, IMP-037, IMP-038, IMP-040, IMP-041, IMP-042, IMP-043, IMP-044, IMP-045, IMP-047, IMP-053, IMP-054, IMP-056, IMP-057, IMP-061, IMP-062, IMP-065, IMP-066, IMP-068, IMP-077, IMP-079, IMP-081, IMP-082, IMP-083, IMP-086, IMP-087, IMP-091, IMP-094, IMP-097, IMP-100, IMP-103 |
| `0061-world-lifecycle-and-content-reconciliation.md` | IMP-002, IMP-003, IMP-004, IMP-005, IMP-006, IMP-007, IMP-008, IMP-009, IMP-010, IMP-011, IMP-014, IMP-016, IMP-018, IMP-019, IMP-020, IMP-021, IMP-022, IMP-023, IMP-024, IMP-025, IMP-028, IMP-029, IMP-030, IMP-036, IMP-037, IMP-046, IMP-051, IMP-052, IMP-053, IMP-054, IMP-055, IMP-056, IMP-058, IMP-059, IMP-061, IMP-062, IMP-063, IMP-065, IMP-067, IMP-069, IMP-072, IMP-077, IMP-080, IMP-081, IMP-084, IMP-086, IMP-087, IMP-089, IMP-090, IMP-091, IMP-092, IMP-094, IMP-097, IMP-100, IMP-101, IMP-104, IMP-105 |
| `0062-world-and-systems-regression-fixes.md` | IMP-002, IMP-004, IMP-008, IMP-009, IMP-010, IMP-013, IMP-014, IMP-016, IMP-018, IMP-019, IMP-020, IMP-021, IMP-022, IMP-023, IMP-024, IMP-025, IMP-029, IMP-030, IMP-034, IMP-035, IMP-036, IMP-037, IMP-040, IMP-041, IMP-042, IMP-051, IMP-052, IMP-054, IMP-055, IMP-058, IMP-059, IMP-061, IMP-062, IMP-063, IMP-066, IMP-079, IMP-087, IMP-089, IMP-090, IMP-091, IMP-100, IMP-101 |
| `0063-economy-contract-reconciliation.md` | IMP-005, IMP-006, IMP-007, IMP-008, IMP-009, IMP-010, IMP-012, IMP-022, IMP-023, IMP-026, IMP-027, IMP-028, IMP-029, IMP-030, IMP-036, IMP-037, IMP-038, IMP-051, IMP-052, IMP-053, IMP-054, IMP-056, IMP-058, IMP-061, IMP-065, IMP-073, IMP-074, IMP-077, IMP-081, IMP-085, IMP-086, IMP-088, IMP-090, IMP-091, IMP-093, IMP-094, IMP-097, IMP-100, IMP-102 |
| `0064-session-handshake-wire-types-and-result-contract.md` | IMP-001, IMP-002, IMP-005, IMP-006, IMP-007, IMP-008, IMP-009, IMP-010, IMP-013, IMP-014, IMP-018, IMP-020, IMP-029, IMP-030, IMP-034, IMP-035, IMP-036, IMP-037, IMP-040, IMP-041, IMP-042, IMP-045, IMP-053, IMP-054, IMP-061, IMP-062, IMP-063, IMP-065, IMP-066, IMP-067, IMP-068, IMP-069, IMP-077, IMP-079, IMP-080, IMP-081, IMP-082, IMP-083, IMP-087, IMP-091, IMP-099, IMP-100, IMP-103 |
| `0065-data-schema-completion-and-erasure-retention.md` | IMP-001, IMP-005, IMP-006, IMP-007, IMP-008, IMP-009, IMP-010, IMP-022, IMP-029, IMP-030, IMP-034, IMP-036, IMP-037, IMP-040, IMP-041, IMP-042, IMP-043, IMP-044, IMP-045, IMP-047, IMP-048, IMP-052, IMP-053, IMP-054, IMP-056, IMP-064, IMP-068, IMP-077, IMP-082, IMP-086, IMP-091, IMP-094, IMP-097, IMP-098, IMP-100, IMP-103 |
| `0066-measurable-client-gates-forced-cap-worst-case-drain-and-ops-stack.md` | IMP-000, IMP-002, IMP-013, IMP-014, IMP-016, IMP-018, IMP-022, IMP-025, IMP-043, IMP-045, IMP-046, IMP-047, IMP-048, IMP-055, IMP-062, IMP-063, IMP-065, IMP-066, IMP-067, IMP-068, IMP-069, IMP-075, IMP-077, IMP-078, IMP-079, IMP-080, IMP-081, IMP-082, IMP-083, IMP-088, IMP-095, IMP-096, IMP-098, IMP-099, IMP-101, IMP-103 |
| `0068-implementation-packet-readiness-corrections.md` | IMP-000, IMP-004, IMP-009, IMP-013, IMP-015, IMP-018, IMP-019, IMP-020, IMP-022, IMP-023, IMP-024, IMP-036, IMP-040, IMP-041, IMP-042, IMP-048, IMP-061, IMP-062, IMP-063, IMP-064, IMP-065, IMP-067, IMP-068, IMP-069, IMP-070, IMP-071, IMP-072, IMP-078, IMP-079, IMP-081, IMP-083, IMP-095, IMP-096, IMP-098, IMP-101, IMP-104, IMP-105 |
| `0069-session-continuity-auth-hardening-and-wire-corrections.md` | IMP-004, IMP-005, IMP-006, IMP-007, IMP-008, IMP-009, IMP-010, IMP-013, IMP-014, IMP-015, IMP-018, IMP-019, IMP-020, IMP-022, IMP-023, IMP-024, IMP-029, IMP-030, IMP-034, IMP-035, IMP-036, IMP-037, IMP-040, IMP-041, IMP-042, IMP-045, IMP-053, IMP-054, IMP-055, IMP-056, IMP-061, IMP-062, IMP-063, IMP-065, IMP-066, IMP-067, IMP-068, IMP-069, IMP-070, IMP-071, IMP-072, IMP-077, IMP-078, IMP-079, IMP-081, IMP-083, IMP-086, IMP-087, IMP-091, IMP-094, IMP-097, IMP-099, IMP-100, IMP-103, IMP-104, IMP-105 |
| `0070-durable-restart-relic-expiry-erasure-ledger-and-entity-budgets.md` | IMP-000, IMP-001, IMP-003, IMP-005, IMP-006, IMP-007, IMP-008, IMP-010, IMP-013, IMP-014, IMP-018, IMP-022, IMP-023, IMP-025, IMP-029, IMP-030, IMP-036, IMP-037, IMP-043, IMP-044, IMP-046, IMP-047, IMP-048, IMP-052, IMP-053, IMP-054, IMP-055, IMP-056, IMP-058, IMP-059, IMP-062, IMP-063, IMP-065, IMP-066, IMP-067, IMP-068, IMP-069, IMP-077, IMP-078, IMP-079, IMP-080, IMP-081, IMP-082, IMP-083, IMP-086, IMP-087, IMP-088, IMP-091, IMP-092, IMP-094, IMP-095, IMP-096, IMP-097, IMP-098, IMP-099, IMP-100, IMP-101, IMP-103, IMP-104 |
| `0071-client-presentation-contract-reconciliation.md` | IMP-000, IMP-004, IMP-013, IMP-015, IMP-018, IMP-019, IMP-021, IMP-022, IMP-023, IMP-024, IMP-032, IMP-033, IMP-040, IMP-041, IMP-042, IMP-048, IMP-055, IMP-061, IMP-062, IMP-063, IMP-065, IMP-066, IMP-067, IMP-070, IMP-071, IMP-072, IMP-073, IMP-074, IMP-075, IMP-076, IMP-078, IMP-079, IMP-083, IMP-088, IMP-089, IMP-095, IMP-096, IMP-099, IMP-101, IMP-103, IMP-104, IMP-105 |
| `0072-executable-merge-pipeline-for-ai-agents.md` | IMP-000, IMP-001, IMP-048, IMP-061, IMP-063, IMP-064, IMP-065, IMP-067, IMP-068, IMP-070, IMP-071, IMP-072, IMP-073, IMP-074, IMP-075, IMP-076, IMP-079, IMP-081, IMP-083, IMP-088, IMP-095, IMP-096, IMP-098, IMP-101, IMP-104, IMP-105 |

## Requirement ID → Task

| ID | Spec | Tasks (named in `## Acceptance` and `## Tests`) |
|---|---|---|
| `PERF-001` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-002` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-003` | `04_architecture/client_performance.md` | IMP-048, IMP-096 |
| `PERF-004` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-005` | `04_architecture/client_performance.md` | IMP-095, IMP-096 |
| `PERF-006` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-007` | `04_architecture/client_performance.md` | IMP-067, IMP-096 |
| `PERF-008` | `04_architecture/client_performance.md` | IMP-099 |
| `PERF-009` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-010` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-011` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-012` | `04_architecture/client_performance.md` | IMP-048, IMP-096 |
| `PERF-013` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-014` | `04_architecture/client_performance.md` | IMP-065 |
| `PERF-015` | `04_architecture/client_performance.md` | IMP-065, IMP-095, IMP-099 |
| `PERF-016` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-017` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-018` | `04_architecture/client_performance.md` | IMP-018, IMP-095 |
| `PERF-019` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-020` | `04_architecture/client_performance.md` | IMP-083 |
| `PERF-021` | `04_architecture/client_performance.md` | IMP-095 |
| `PERF-022` | `04_architecture/client_performance.md` | IMP-066 |
| `PERF-023` | `04_architecture/client_performance.md` | IMP-066 |
| `PERF-024` | `04_architecture/client_performance.md` | IMP-065 |
| `CODE-001` | `10_implementation/engineering_conventions.md` | IMP-000 |
| `CODE-002` | `10_implementation/engineering_conventions.md` | IMP-000 |
| `CODE-003` | `10_implementation/engineering_conventions.md` | IMP-000 |
| `CODE-004` | `10_implementation/engineering_conventions.md` | IMP-061 |
| `CODE-005` | `10_implementation/engineering_conventions.md` | IMP-083 |
| `CODE-006` | `10_implementation/engineering_conventions.md` | IMP-083 |
| `HOT-001` | `08_scale_ops/capacity.md` | IMP-079 |
| `HOT-002` | `08_scale_ops/capacity.md` | IMP-079 |
| `HOT-003` | `08_scale_ops/capacity.md` | IMP-081 |

## Coverage Gate
Q0 must fail when:

- a spec file under `docs/` (except `11_decisions/`) or an ADR has no consuming task;
- a `specs:`/`adrs:` path does not exist, or this index differs from the packets;
- a requirement ID has no packet naming it in both `## Acceptance` and `## Tests`;
- a task changes a contract but omits a grep-derived consumer from `consumers_checked:`;
- a canonical spec names an `IMP-*` ID that has no packet;
- an implementation-owned path has no task owner, or two tasks own overlapping paths without a `depends_on` order.

## Invariants

```text
one canonical concept owner
every spec file, ADR and requirement ID maps to at least one task
every implementation path has one task owner
dangling task IDs and spec paths fail Q0
```
