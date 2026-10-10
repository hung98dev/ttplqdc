// Package guild_storage implements the durable guild-storage domain
// (guild_storage.md): two vault sections (COMMON unlocked at guild Lv1,
// RESERVE at Lv10), role-gated deposit/withdraw/move, per-day withdraw
// quotas, the Reserve-claim lifecycle (PENDING -> APPROVED/REJECTED/
// CANCELLED/EXPIRED -> COMPLETED on delivery), the STORAGE_CLAIM_EXPIRE
// job sweep, depositor-identity enforcement (ADR-0049 same-account
// prohibition + 72h membership gate + item_partner_counts signal),
// full-column guild_storage_audit rows, and the monotonic
// guilds.guild_storage_revision.
//
// Membership revalidation (plan r4): every claim transition re-checks
// the requester's guild_memberships row; the expiry sweep cancels
// PENDING/APPROVED claims of non-members; CancelClaimsForMember is the
// erasure-detach hook (data_model.md § Account Erasure step 6). The
// disband gate is IMP-036's — guild.Store.StorageEmpty /
// ApprovedClaimsOpen — and is not duplicated here.
//
// Wire: client.<id> executors 629/630/644/645/646 carry a
// S2C_GUILD_RESULT (649) verdict; the STORAGE_CLAIM_EXPIRE sweep runs
// under the JOB / job.<target> = guild family (save_rules.md), exported
// as StorageClaimExpireJob for the composition's job.guild mux.
package guild_storage
