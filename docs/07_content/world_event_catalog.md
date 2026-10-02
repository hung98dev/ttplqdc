# Launch World Event Catalog
status: LOCKED

## Scope
Concrete launch definition for the recurring `SPIRIT_SURGE` event owned generically by `../02_world/world_rules.md`.

The event is optional social/open-world content. It must not become a mandatory hourly power appointment.

## Compiler Source Schema

The source below uses the registered Markdown grammar in `../06_data/content_authoring_contract.md`. All `text` fences are registered assignment/pattern grammars: `field = value`, `index <id>` ordered lists, `ELEMENT -> <payload>` dispatch lines, and `N. <step>` ordered chain steps.

| source_section | output / key | typed inputs | defaults / finite rule |
|---|---|---|---|
| `Event Identity` / `text` fences | event / `event.spirit_surge` | assignment fence: `event_id`,`schedule`,`duration` + `H = floor(unix_seconds / 3600)` expr; region-order + element-order indexed lists; slot-selection assignments (`region_slot_0`,`pair_index`,`pairs` tuple list, `field_slot_*`,`element_slot_*` mod exprs) | Hourly 15m event; all selection derived from `H` alone (no persisted state); 3 concurrent regions (region 0 always + pair from 5-pair cycle); field/element slots per region from `(H + i) mod` formulas. |
| `Event Identity` / access + recommendation paragraph + `H mod 5` table | access/recommendation rule | table `H mod 5, active region indices, Act-I eligible/recommended index`; filter rule `<= highest_unlocked_region` | Normal region/map access applies; recommended = highest-index active region unlocked; per-act eligibility guaranteed by the table. |
| `Eligible Field Order` / table `region, field 0, field 1, field 2` | event field candidates / `(region, slot)` | region:id; field 0..2:id | 6 regions × 3 fields; safe/social anchors excluded; `field_slot_N` picks column index mod 3. |
| `Surge Variant` / `text` fences | surge variant rules | multiplier fence (`MAX_HP`,`ATTACK`,`DEFENSE`,`base_exp`,`normal drop table`); cooldown/telegraph mins; element payload grammar `ELEMENT -> <payload desc + coefficient>` | Variant inherits base family; exactly one element attack; cooldown ≥8s, telegraph ≥0.80s; payloads per element with typed coefficients (0.50-0.65 ATTACK + status). |
| `Temporary Spawn Groups` / `text` fence | spawn-group budget | `2 temporary event groups`, `max_alive per group = 4` | Max 2 temp groups/channel using the field's normal pool + event anchors; separate from the 54 persistent groups. |
| `Elite Event Chain` / numbered `event.spirit_surge.wave.*` list | event chain steps / `event.spirit_surge.wave.0N` | ordered chain: `event.<id>` + `defeat <n> <kind> variants`/`activate <n> markers` steps | 3 waves; one chain per map channel; 90s local cooldown; `chain_id` = durable UUID v4 per ADR-0062; no major boss required. |
| `Participation` / `text` fences | contribution rules | points grammar `damage\|healing\|protection\|marker interaction: <expr>`; eligibility fence `contribution_points >= 10 AND ...` | Per-enemy cap 100 points; eligibility ≥10 + presence at wave.03 commit; presence alone = 0. |
| `Rewards` / `text` fences | reward grants / `drop.event.spirit_surge.*` | grant grammar `drop.event.spirit_surge.<tier>.completion\|daily_first  key <idempotency_key>`; EXP fence `Act <roman> <int>` | Completion ≤1/character/UTC hour (`surge.completion.drop.<utc_hour>.<character_id>`); daily_first 1/day; WORLD_EVENT EXP per character_act (6 values) keyed `surge.completion.exp.<utc_hour>.<character_id>`. |
| `Restart / Idempotency` / `text` fence | instance key | `spirit_surge.<UTC-hour-start>.<map_id>.<channel_id>` | Restart recomputes selection from UTC; committed keys persist; same hour = same region/element. |
| `Validation` + `Invariants` | validation rules | enumerated reject rules + invariant counts | Reference only; counted invariants (1 event, 3 regions, coverage 100%/40%). |

# Event Identity
```text
event_id = event.spirit_surge
schedule = every whole UTC hour
duration = 15m
```

The event does not use server RNG to choose its regions/elements. Selection is deterministic from server UTC time so reconnect/restart cannot reroll the active event.

Let:
```text
H = floor(unix_seconds / 3600)
```

Region order:
```text
0 zone.lang_da
1 zone.rung_u_minh
2 zone.ben_nuoc_den
3 zone.deo_may
4 zone.thanh_co
5 zone.nui_thieng
```

Element order:
```text
0 KIM
1 MOC
2 THUY
3 HOA
4 THO
```

Three concurrent active regions per hour (ADR-0079, access-respecting coverage):
```text
region_slot_0 = 0
pair_index = H mod 5
region_slot_1, region_slot_2 = pairs[pair_index]
pairs = [(1,3),(2,4),(3,5),(4,1),(5,2)]
```

Region 0 is active every hour; regions 1..5 each appear twice in every five hours (40%). At 15 minutes per event this gives 15 active minutes/hour in Làng Đa and 6 minutes/hour on average elsewhere. These are world schedule slots, not per-character unlock grants.

Field and element assignment per slot:
```text
field_slot_0  = floor(H / 6) mod 3
field_slot_1  = (floor(H / 6) + 1) mod 3
field_slot_2  = (floor(H / 6) + 2) mod 3

element_slot_0 = (H + floor(H / 6)) mod 5
element_slot_1 = (H + floor(H / 6) + 1) mod 5
element_slot_2 = (H + floor(H / 6) + 2) mod 5
```

The three concurrent events each host a distinct field and a distinct element where possible.
All three selections are derivable from H alone; no state must be persisted across restart.

Participation eligibility first applies normal region/map access (`world_route_catalog.md`); an active locked region is never enterable merely because a Surge is running. The deterministic recommended region is the highest-index active region the character has unlocked (ties cannot occur), with region 0 always available. All six acts therefore have an accessible hourly event and receive their own character-act WORLD_EVENT EXP on eligible completion; travel to a higher locked act is not required.

| H mod 5 | active region indices | Act-I eligible/recommended index |
|---|---|---|
| 0 | `0,1,3` | `0` |
| 1 | `0,2,4` | `0` |
| 2 | `0,3,5` | `0` |
| 3 | `0,4,1` | `0` |
| 4 | `0,5,2` | `0` |

For unlocked acts I..VI (highest region index 0..5), filter each row to indices `<= highest_unlocked_region` and select its maximum. The result exists in every row, never exceeds access, and includes region 0 for Act I in all five cases.

# Eligible Field Order
| region | field 0 | field 1 | field 2 |
|---|---|---|---|
| `zone.lang_da` | `map.lang_da.bo_ruong` | `map.lang_da.ben_da` | `map.lang_da.go_ma` |
| `zone.rung_u_minh` | `map.rung_u_minh.loi_tram` | `map.rung_u_minh.rung_sau` | `map.rung_u_minh.mieu_bo_hoang` |
| `zone.ben_nuoc_den` | `map.ben_nuoc_den.bai_lau` | `map.ben_nuoc_den.duong_ngap` | `map.ben_nuoc_den.ben_do_cu` |
| `zone.deo_may` | `map.deo_may.duong_rung` | `map.deo_may.khe_da` | `map.deo_may.rung_cam` |
| `zone.thanh_co` | `map.thanh_co.duong_da` | `map.thanh_co.hao_can` | `map.thanh_co.den_tran` |
| `zone.nui_thieng` | `map.nui_thieng.rung_may` | `map.nui_thieng.suon_da` | `map.nui_thieng.cong_co` |

Safe/social anchors are never selected.

# Surge Variant
The event reuses the selected field's ordinary regional monster families. It does not create a second permanent monster roster.

For event-spawned variants only:
```text
MAX_HP multiplier = 1.15
ATTACK multiplier = 1.05
DEFENSE multiplier = 1.00
base_exp multiplier = 1.00
normal drop table = unchanged
```

Each variant gains exactly one event attack from the selected event element. Base monster primary element remains unchanged.

Event-attack cooldown per entity:
```text
>= 8s
```
Telegraph startup:
```text
>= 0.80s
```

Element attack payloads:
```text
KIM  -> narrow marked line, 0.65 ATTACK
MOC  -> marked root patch, 0.50 ATTACK + ROOT 0.75s on center hit
THUY -> directional wave, 0.55 ATTACK + SLOW 15% for 2s
HOA  -> marked burst, 0.60 ATTACK + BURN total 0.15 ATTACK over 3s
THO  -> falling-stone marker, 0.65 ATTACK + KNOCKBACK 1m
```
No event attack may chain hard control into another event attack without a normal player reaction window.

# Temporary Spawn Groups
The selected map may activate at most:
```text
2 temporary event groups
max_alive per group = 4
```

Each group selects from the selected field's normal regional pool and uses logical event anchors from the map asset.

Event spawns are separate from the 54 persistent launch groups and clean up under `spawning.md` when the event ends.

# Elite Event Chain
One chain is available in the selected map:

1. `event.spirit_surge.wave.01`
   - defeat `4` Surge NORMAL variants
2. `event.spirit_surge.wave.02`
   - defeat `4` Surge NORMAL variants
   - activate `2` visible quest/event markers
3. `event.spirit_surge.wave.03`
   - defeat `1` regional ELITE Surge variant + `2` NORMAL variants

Only one chain instance may be active per map channel at a time. Completion starts a `90s` local cooldown before another chain may begin while the global event is still active. Chain identity: `chain_id` = durable UUID v4 (`crypto/rand`) generated by the server when the chain starts (`../06_data/ids.md`), never reused, so it stays unique across restarts (ADR-0062; replaces the ADR-0061 per-hour `chain_seq`). A chain interrupted by a restart is not resumed; the next chain gets a new `chain_id`.

No PUBLIC major boss is required for Spirit Surge completion.

# Participation
Each event enemy tracks normalized contribution.

Contribution points:
```text
damage:     1 point per 1% enemy MAX_HP-equivalent valid damage
healing:    1 point per 1% enemy MAX_HP-equivalent effective healing to an eligible participant, weighted x0.50
protection: 1 point per 1% enemy MAX_HP-equivalent prevented damage to an eligible participant, weighted x0.50
marker interaction: 3 points per authored event marker, once per marker per character
```

Per enemy, one character's damage/support contribution is capped at `100` points for eligibility accounting; overkill/overheal gives no points.

Completion reward eligibility requires:
```text
contribution_points >= 10
AND character was in the same map_instance_id when wave.03 completed
```
Presence alone gives zero points.

# Rewards
Eligible completion settles, at most once per character per UTC hour (later chains in the same hour grant contribution credit only):
```text
drop.event.spirit_surge.<tier>.completion      key surge.completion.drop.<utc_hour>.<character_id>
```

If the character has not yet received the UTC-day event bonus, also settle:
```text
drop.event.spirit_surge.<tier>.daily_first     key surge.daily_first.<utc_date>.<character_id>
```

Daily-first is an accelerator/cosmetic-material opportunity only; baseline completion remains repeatable during later Surges.

`quest.event.spirit_surge.contribute` observes the same authoritative completion and does not duplicate rewards.

WORLD_EVENT character EXP (not inventory) on eligible completion, keyed `surge.completion.exp.<utc_hour>.<character_id>`:
```text
Act I 256667
Act II 331333
Act III 253269
Act IV 282111
Act V 377909
Act VI 419769
```
EXP act = the eligible character's current `character_act` at completion (ADR-0079), independent of selected region tier. Access and contribution requirements still apply; no region unlock, monster EXP scaling, or additional hourly grant is implied. This is a WORLD_EVENT side grant, not a drop-table slot.

# Restart / Idempotency
Event instance key:
```text
spirit_surge.<UTC-hour-start>.<map_id>.<channel_id>   (each chain instance is identified by its chain_id)
```

On restart, active event selection is recomputed from UTC time. Already committed completion/daily-first reward keys remain committed.

A restart never changes region/element for the same UTC hour.

# Validation
Reject:
- selected map outside the region's three adventure fields
- safe/social map event spawn
- event attack with startup <0.80s
- more than two temporary groups or max_alive >4
- exclusive equipment/Soul/permanent stat in daily-first reward
- reward without contribution threshold
- event variant changing base monster drop table or Soul odds invisibly

# Invariants
```text
Spirit Surge count = 1 recurring event definition
three regions active per UTC hour (region_slot_0/1/2)
one field per active region slot
safe anchors never selected
presence != reward
no exclusive permanent power
selection deterministic from server UTC time
coverage region0 = 100% of hours; each region1..5 = 40% of hours (2 of 5)
```