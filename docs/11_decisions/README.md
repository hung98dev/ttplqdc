# Architecture Decision Records

Use ADRs only for decisions that materially affect architecture, data contracts, operations, or irreversible implementation cost.

File: `NNNN-short-name.md`.

ADR numbers are unique and monotonic. Before creating a new ADR, list this directory and use the next number after the highest existing one (skipped numbers such as 0067 are never assigned); never create a second file with an existing ADR number.

Use `docs/templates/adr.md`.

## Index

| # | Title | Status |
|---|-------|--------|
| 0001 | Content Revision Contract | ACCEPTED |
| 0002 | Effect Value and Shield Contract | ACCEPTED |
| 0003 | Spawn Selector Anchor Contract | ACCEPTED |
| 0004 | Party Dungeon Scaling and Reward Slots | ACCEPTED |
| 0005 | Skill Action Timing and Geometry _(amended by ADR-0047)_ | ACCEPTED |
| 0006 | Unity / Go / PostgreSQL Stack | ACCEPTED |
| 0007 | Single-Owner Fixed-Step Realtime Simulation | ACCEPTED |
| 0008 | Client Network Transport Protocol | ACCEPTED |
| 0009 | Account Session Credentials | ACCEPTED |
| 0010 | Exact Technology Version Pinning | ACCEPTED |
| 0011 | PostgreSQL Relational Persistence | ACCEPTED |
| 0012 | Reward Claim and Item Materialization | ACCEPTED |
| 0013 | Canonical Unicode Text Normalization | ACCEPTED |
| 0014 | Unity Addressables Asset Delivery | ACCEPTED |
| 0015 | Unity Localization | ACCEPTED |
| 0016 | Twelve-Skill Pool and Upgradeable Basic Attacks _(amended by ADR-0033)_ | ACCEPTED |
| 0017 | Percentage Dodge Stat — No Active Dodge | ACCEPTED |
| 0018 | Combat Target Caps and Skill Level Target Scaling _(amended by ADR-0033)_ | ACCEPTED |
| 0019 | Spirit Beast Companion System | ACCEPTED |
| 0020 | Map Channel Count and Channel Capacity Contract _(amended: routing rule; amended by ADR-0035)_ | ACCEPTED |
| 0021 | Hardcore Enhancement Rate Curve | ACCEPTED |
| 0022 | Four-Tier Lucky Charm System | ACCEPTED |
| 0023 | Engagement Loops — Weapon Glow, Bonfire, Chivalry, Chests, Sparring | ACCEPTED |
| 0024 | Fishing, Cooking, Feats, Titles, Boss Chest Ceremony | ACCEPTED |
| 0025 | Peak Moments and Bonus Progression Books _(amended by ADR-0033)_ | ACCEPTED |
| 0026 | Just Guard Mitigation and Ma Am Folklore Status _(amended by ADR-0034, ADR-0038)_ | ACCEPTED |
| 0027 | Boss Aftermath, Mystery Bounty, and Capacity Unification _(amended by ADR-0040)_ | ACCEPTED |
| 0028 | Atlas Collection, Soft Pity, Guild Stone, and Morning Market _(amended by ADR-0042)_ | ACCEPTED |
| 0029 | Character Resource Isolation _(amended by ADR-0041)_ | ACCEPTED |
| 0030 | One Account One Live Session | ACCEPTED |
| 0031 | EXP Scale ×100 and Corrected Act Budgets | ACCEPTED |
| 0032 | Seven-Channel EXP Source Portfolio | ACCEPTED |
| 0033 | Skill Unlock Schedule Remap and Lv55/Lv60 Bonus Skill Points | ACCEPTED |
| 0034 | Just Guard Edge-Trigger, Streak Mechanic, and Revised Mitigation Budget | ACCEPTED |
| 0035 | Spawn Density Increase and Channel-Capacity Implications _(amended: channel cap 20→18)_ _(amended by ADR-0061, ADR-0062)_ | ACCEPTED |
| 0036 | Seasons as Launch Infrastructure | ACCEPTED |
| 0037 | Reflect, Lifesteal, Absorb, Heal-Reduction Stats | ACCEPTED |
| 0038 | Discrete Movement-Edge Input Message | ACCEPTED |
| 0039 | Entity Capacity Model and AI Budget Classes _(amended by ADR-0066, ADR-0070)_ | ACCEPTED |
| 0040 | WorldConsequence Durable Aggregate _(amended by ADR-0053)_ | ACCEPTED |
| 0041 | Anti-RMT Trade Gates and IAP Entitlement Integrity _(amended by ADR-0060, ADR-0063)_ | ACCEPTED |
| 0042 | Atlas Roster Expansion to 104 Launch Pages | ACCEPTED |
| 0043 | Spirit Beast Instance Identity | ACCEPTED |
| 0044 | Launch Topology — One Process Hosting Edge, Sim, Durable and Global _(amended by ADR-0052)_ | ACCEPTED |
| 0045 | CI Evidence Without Self-Referential Commit SHA _(amended by ADR-0057)_ | ACCEPTED |
| 0046 | Reference Viewport, Entity Scale, and Map Geometry _(amended by ADR-0055, ADR-0068)_ | ACCEPTED |
| 0047 | Skill Reach Budget and Collider-Aware Resolution | ACCEPTED |
| 0048 | Character Row Update Timestamp _(amended: included in baseline 000001)_ | ACCEPTED |
| 0049 | Guild Storage Same-Account Transfer Prohibition | ACCEPTED |
| 0050 | Windows-Only CI and Auto-Merge on Green _(amended by ADR-0057, ADR-0058)_ | ACCEPTED |
| 0051 | First-Party Username/Password Login | ACCEPTED |
| 0052 | Single Launch World | ACCEPTED |
| 0053 | Durable Data Contract Reconciliation | ACCEPTED |
| 0054 | Wire Message Completion | ACCEPTED |
| 0055 | 2x Texture Authoring and Cutout Quality Gate _(amended by ADR-0071)_ | ACCEPTED |
| 0056 | Volumetric Art Direction and URP 2D Lighting _(amended by ADR-0071)_ | ACCEPTED |
| 0057 | Bootstrap, Trusted CI, Evidence Identity and Merge Mechanics _(amended by ADR-0058, ADR-0068, ADR-0072)_ | ACCEPTED |
| 0058 | Public Repository on GitHub-Hosted Linux and Windows Runners _(amended by ADR-0072)_ | ACCEPTED |
| 0059 | Client Smoothness by Construction and Machine-Enforced Code Quality | ACCEPTED |
| 0060 | Wire and Durable Contract Completion for Gameplay, World and Systems _(amended by ADR-0062)_ | ACCEPTED |
| 0061 | World Lifecycle and Content Reconciliation _(amended by ADR-0062)_ | ACCEPTED |
| 0062 | World and Systems Regression Fixes | ACCEPTED |
| 0063 | Economy Contract Reconciliation | ACCEPTED |
| 0064 | Session Handshake, Wire Scalar Types and Result Contract | ACCEPTED |
| 0065 | Data Schema Completion, Erasure and Retention _(amended by ADR-0070)_ | ACCEPTED |
| 0066 | Measurable Client Gates, Forced-Cap Worst Case, Drain and Operations Stack _(amended by ADR-0070)_ | ACCEPTED |
| 0067 | _Number not used; never assign_ | — |
| 0068 | Implementation Packet Readiness Corrections _(amended by ADR-0072)_ | ACCEPTED |
| 0069 | Session Continuity, Auth Hardening and Wire Corrections | ACCEPTED |
| 0070 | Durable Restart Safety, Relic Expiry, Erasure Ledger and Entity Class Budgets | ACCEPTED |
| 0071 | Client Presentation Contract Reconciliation | ACCEPTED |
| 0072 | Executable Merge Pipeline for AI Agents | ACCEPTED |
