# Guild
status: LOCKED

## Scope
Defines guild identity, lifecycle, membership, roles, permissions, recruitment, leadership, chat integration, and persistence.

Guild progression/blessings belong in `guild_progression.md`; shared items belong in `guild_storage.md`; competitive Guild War belongs in `guild_war.md`.

## Identity
Immutable:
```text
guild_id
```

Guild name rules:
```text
1..24 visible Unicode grapheme clusters and <= 96 UTF-8 bytes (../06_data/text.md § Name Limits)
canonical trim + NFC
must contain at least one non-whitespace grapheme
globally unique by canonical server-computed name_key
```

Canonical normalization, Unicode case folding, grapheme counting, and the PostgreSQL uniqueness-key contract are owned by `../06_data/text.md`.

Profanity/reserved-name validation is server-controlled.

`guild_id` is never reused.

## Membership
Membership is character-scoped:
```text
character_id -> 0 or 1 guild_id
```

Guild join eligibility:
```text
character level >= 10
```

This matches the Guild Join feature milestone in `../01_gameplay/progression.md`.

Different characters on one account may independently belong to guilds.

Member capacity is derived from Guild Level in `guild_progression.md` (30..60).

## Roles
Authority order:
```text
LEADER > VICE_LEADER > OFFICER > MEMBER
```

Stable IDs:
```text
guild.role.leader
guild.role.vice_leader
guild.role.officer
guild.role.member
```

Exactly one LEADER exists in every ACTIVE guild.

Vice Leader capacity is derived from Guild Level.

Custom roles are not enabled initially.

## Permissions
### LEADER
May:
- invite/process applications
- kick any lower role
- promote/demote OFFICER
- assign/remove VICE_LEADER
- transfer leadership
- modify guild settings
- manage all storage permissions granted by `guild_storage.md`
- submit/cancel Guild War registration
- disband

### VICE_LEADER
May:
- invite/process applications
- kick OFFICER/MEMBER
- promote MEMBER -> OFFICER
- demote OFFICER -> MEMBER
- use Vice-Leader storage permissions
- submit/cancel Guild War registration

Cannot:
- modify LEADER
- assign/remove another VICE_LEADER
- transfer guild leadership
- disband

### OFFICER
May:
- invite/process applications
- kick MEMBER
- use Officer storage permissions

Cannot modify equal/higher roles or submit rated Guild War registration.

### MEMBER
May:
- view guild information
- use guild chat
- use Member storage permissions
- contribute to guild progression
- participate in Guild War when eligible and selected
- leave guild

## Creation
Requirements:
```text
character level >= 20
character currently has no guild
currency.common cost = 10,000
valid unique guild name
```

The previous `100,000` launch cost was inconsistent with the authored Level-20 common-currency faucet and made the feature technically unlocked but economically unavailable for a first character. Creation remains a meaningful sink without requiring late-game farming.

Creation is atomic:
```text
debit cost
+ create guild
+ add creator
+ creator -> LEADER
```

Failure leaves currency and membership unchanged.

New guild:
```text
state = ACTIVE
guild_level = 1
member_count = 1
```

## States
```text
ACTIVE
DISBANDING
DISBANDED
```

`DISBANDED` is terminal.

## Recruitment Mode
Guild setting:
```text
CLOSED
APPLICATIONS
```

Direct leader/officer invites are allowed in either mode.

`APPLICATIONS` allows eligible unguilded characters to submit applications.

## Invitations
States:
```text
PENDING
ACCEPTED
DECLINED
CANCELLED
EXPIRED
```

Lifetime:
```text
10 minutes
```

Invite validates:
- inviter permission
- target level >= 10
- target no guild
- capacity available at acceptance
- `social.md` direct-interaction gate

Pending invites do not reserve capacity.

## Applications
States:
```text
PENDING
ACCEPTED
REJECTED
CANCELLED
EXPIRED
```

Lifetime:
```text
7 days
```

A character may have at most:
```text
5 PENDING guild applications
```

Application requires character level >= 10.

Acceptance revalidates level, no current guild, and capacity.

Successful invite/application join role:
```text
MEMBER
```

Concurrent joins preserve one-character-one-guild.

## Leave
MEMBER/OFFICER/VICE_LEADER may leave voluntarily.

Leaving does not affect party, friends, dungeon membership, inventory, or character progression.

Leaving an already ACTIVE Guild War does not rewrite the match roster/result; Guild War disconnect/abandon handling follows `guild_war.md`.

LEADER cannot leave while another member remains. Leader must transfer leadership first.

If LEADER is the only member, leaving is a disband request and must meet the Disband preconditions; otherwise it is rejected with `GUILD_DISBAND_BLOCKED` and nothing changes. The sole leader can always empty storage (unlimited withdraw) and cancel claims first, so a guild never becomes ownerless with locked items.

## Kick
An actor may kick only a strictly lower role.

Cannot kick:
- self through kick operation
- non-member
- equal/higher role
- LEADER

Kick removes guild membership and active Guild Blessing eligibility immediately.

A kick cannot erase or reassign an already ACTIVE Guild War result/reward record.

## Promotion / Demotion
LEADER:
```text
MEMBER <-> OFFICER
MEMBER/OFFICER -> VICE_LEADER
VICE_LEADER -> OFFICER
```

VICE_LEADER:
```text
MEMBER <-> OFFICER
```

VICE_LEADER assignment must not exceed current level-derived capacity.

## Leadership Transfer
Only LEADER may transfer to another current member.

Atomic result:
```text
old LEADER -> VICE_LEADER
new target -> LEADER
```

If this would exceed Vice-Leader capacity, transfer is rejected until a Vice Leader is demoted or capacity increases.

Exactly one LEADER exists before and after commit.

## Leader Inactivity
If the LEADER has not attached any character session for `30 days` (server time since last detach), one eligible member may claim leadership:
```text
eligible claimant = highest role present (VICE_LEADER > OFFICER > MEMBER), ties -> longest guild tenure
claimant must have been a member >= 14 days and attached within the last 7 days
claim            -> claimant becomes LEADER; old LEADER becomes MEMBER; audited; one claim per guild per 30 days
```
Any later attach by the old leader does not revert the claim. Shorter offline time never changes ownership, preventing hostile takeovers.

## Chat
Guild chat uses `GUILD` from `social.md`.

Blocking affects message delivery but never membership/role.

## Presence UI
Guild roster may show:
```text
character display name
class
level
role
ONLINE/OFFLINE
last_online_at rounded to minute/day for offline members
```

Exact coordinates and private instance IDs are not exposed.

## Progression and Benefits
Guild Level, EXP, Contribution, weekly Five-Element Ritual, Blessing vote, member/Vice capacity, and prestige streak belong in `guild_progression.md`.

Guild membership alone grants no hidden combat multiplier beyond an explicitly active Guild Blessing.

## Storage
Guild Storage belongs in `guild_storage.md`.

There is no guild-owned currency treasury initially.

## Guild War
Rated Guild War is an optional Guild Level 10+ activity defined in `guild_war.md`.

Only LEADER/VICE_LEADER may register a roster.

Guild War:
- reuses ranked PvP combat transformation
- does not create permanent territory ownership
- does not grant exclusive permanent combat power
- does not allow active Guild Blessings to modify rated combat initially

Guild War participation is not required for normal Guild Level progression because PvE Guild EXP sources remain available.

## Guild Stone — Bia Da Danh Vong
Every Safe Anchor hosts a communal **Guild Stone** (`object.guild_stone.<map_id>`) in the central square:

- **Display**: Top 3 Guild War weekly ranking (by Guild War rating) + first +16 achiever of the week + most Atlas completions. Display is per logical world, same across all 30 channels, updated every Monday 00:00 UTC.
- **Inscription entries** (server-authoritative):
  ```
  rank 1..3: guild_name, guild_level, leader_name, war_rating
  first_plus16: character_name, class_id, item_id, timestamp
  atlas_champion: character_name, pages_completed
  ```
- **Prestige only**: Stone grants no stats, currency, or combat buff. It is a screenshot/social proof surface.
- **Interaction**: Inspect stone to view guild profiles and teleport to guild recruitment UI.
- **Persistence**: the weekly display is derived every Monday from Guild War and progression tables; seasonal category completions are persisted in `guild_stone_category_completions` (`../06_data/data_model.md`); no separate guild currency.
- **Inscription styles**: a character may buy a cosmetic style for how its own name renders on the Stone (`cosmetic.guild_stone.inscription.*`, prices in `../07_content/economy_catalog.md`). Styles are the only purchasable part of the Stone; they change presentation only.
- **Seasonal category** (`guild_stone.season.<season_region_index>.<region>`, IDs in `seasons.md`): during a season, a guild completes the category when its members' seasonal Atlas T3 masteries earned during that season sum to `>= 30` (each character-page counted once; members counted while in the guild at mastery time). Completion permanently inscribes the guild under that category and adds `+1` to the guild's Guild Stone count; it grants no currency, stats or character cosmetic. Repeat cycles of the region can be completed again and add another inscription line.

This section is the single owner of Guild Stone rules.

## Disband
Only LEADER may request disband.

Preconditions:
```text
Guild Storage empty
no APPROVED storage claims
no QUEUED/MATCHED/ACCEPTING/PREPARING/ACTIVE/RESOLVING Guild War registration or match
```

Flow:
```text
ACTIVE -> DISBANDING
-> invalidate pending invites/applications
-> remove memberships
-> finalize progression/blessing state
-> DISBANDED
```

Operation is idempotent.

## Persistence
Guilds persist across disconnect and full server restart.

Persist:
```text
guild_id
guild_name
state
leader_character_id
guild_revision
created_at
members + roles
recruitment_mode
pending invites/applications
```

Guild War competitive state is persisted by `guild_war.md`.

## Revision / Audit
Every persistent guild mutation increments `guild_revision`.

Audit:
```text
CREATE
JOIN
LEAVE
KICK
ROLE_CHANGE
VICE_ASSIGN/REMOVE
LEADERSHIP_TRANSFER
SETTINGS_CHANGE
GUILD_WAR_REGISTER/CANCEL
DISBAND
```

## Design Guardrails
- Four roles only.
- No custom ACL tree.
- No guild currency/research tree.
- Benefits come from simple progression + one weekly Blessing.
- Guild War is one reusable objective mode, not a territory-management subsystem.
- Guild supports social retention without making guild membership mandatory for solo viability.

## Invariants
```text
one character -> max one guild
guild join level >= 10
one ACTIVE guild -> exactly one LEADER
role order = LEADER > VICE_LEADER > OFFICER > MEMBER
new member -> MEMBER
guild creation level >= 20
creation cost = 10,000 currency.common
guild persists across restart
non-empty Guild Storage -> cannot disband
active Guild War -> cannot disband
```
