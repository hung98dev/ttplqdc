# ADR-0029: Character Resource Isolation
status: ACCEPTED
amended_by: [ADR-0041]

> **AMENDMENT NOTICE**: Anti-RMT trade restrictions and direct IAP cosmetic wardrobe access were amended by **ADR-0041** (`docs/03_systems/monetization.md`).

## Context
An account may own three permanent characters. Previous launch text allowed ACCOUNT_BOUND items to move through a 40-slot account vault and credited `currency.special` once per account. That made alts a mule for materials, Linh Đan, charms, and cosmetic currency.

Player rule: characters on one account share **no** gameplay resources. The only shared pool is deposited real-money / IAP balance (`tiền nạp`).

## Decision
1. **Isolation.** Each character owns its own inventory, equipment, Souls, Linh Thú, quests, atlas progress, EXP/level, `currency.common`, `currency.bound`, `currency.special`, enhancement state, and gameplay cosmetic entitlements. No deposit, withdraw, trade, auction, or mail path moves those between same-account characters.
2. **Nạp only.** Account-shared value is payment/IAP entitlements owned by a future payment spec. Claiming an IAP grant creates CHARACTER_BOUND items or character-scoped cosmetics on the **currently selected character**. After claim, that grant cannot be moved to another character.
3. **Account vault.** Launch `account_storage.md` does not accept gameplay items. `service.storage.account` opens the IAP entitlement panel, not a bank.
4. **ACCOUNT_BOUND.** Binding still forbids player trade/auction. It does **not** permit same-account transfer. Treat transfer like CHARACTER_BOUND.
5. **`currency.special`.** Character-scoped. Atlas and PvE special grants key on `character_id`. A second character earns its own special by playing.
6. **Gameplay cosmetics.** Story/feat/atlas/chivalry/bond titles and appearance overlays are character-scoped. Guild cosmetics stay guild-scoped. IAP cosmetics are account-entitled (nạp).

## Consequences
- Alts are separate lives (class reroll), not storage.
- Economy/atlas special caps apply per character.
- Payment spec must not grant tradable power or a fourth gameplay currency.

## Amendment — ADR-0041

> **Superseded in part**: §2 states that claiming an IAP grant creates CHARACTER_BOUND items on the currently selected character and cannot be moved to another character. **ADR-0041** (Anti-RMT Trade Gates and IAP Entitlement Integrity) introduces a narrow carve-out for the paid season track: that purchase creates an `ACCOUNT_SCOPED_ACCESS` entitlement (not a CHARACTER_BOUND item), covering all current and future characters on the account for the season. Each character claims their own cosmetic rewards independently via a composite idempotency key `(account_entitlement_id + character_id + reward_tier_id)`. The entitlement is not consumed by the first character to interact with it.
>
> This carve-out is limited to the season-track cosmetic entitlement. ADR-0029's core rule is unchanged: no same-account sharing of gameplay resources (`currency.special`, inventory, equipment, progression state). CHARACTER_BOUND items remain character-scoped; only the season-track access token is account-scoped.
