# ADR-0042: Atlas Roster Expansion to 104 Launch Pages
status: ACCEPTED

## Context
ADR-0028 recorded an 81-page launch atlas (35 quai_dam + 25 hon_giam + 8 di_tich + 13 co_vat). When the monster roster expanded to 46 NORMAL + 12 ELITE field monsters, the quai_dam tier grew from 35 to 58 pages, taking the launch total to 104. The hon_giam, di_tich, and co_vat counts are unchanged.

This is a data-contract change. `07_content/atlas_catalog.md` defines the canonical page list, and every additional page shifts the `currency.special` faucet that the economy is balanced against. `00_context/constraints.md` requires an ADR for data-contract changes.

## Decision
1. **Launch page total**: 104 pages.
   ```text
   quai_dam  = 58  (46 NORMAL + 12 ELITE monsters from monster_catalog.md)
   hon_giam  = 25  (unchanged)
   di_tich   = 8   (unchanged)
   co_vat    = 13  (unchanged)
   TOTAL     = 104
   ```

2. **`currency.special` faucet (Atlas, per character, all tiers completed)**:
   ```text
   T1 (Seen):     104 × 1 = 104 special
   T2 (Studied):  104 × 2 = 208 special
   T3 (Mastered): 104 × 2 = 208 special   (T3 capped at 2, not 3, to stay within sink)
   Atlas subtotal            = 520 special
   Total faucet (+ base PvE) = 540 special per character
   ```
   The total faucet (540) does not exceed the ~545 sink surface documented by `07_content/economy_catalog.md`, leaving a 5-unit margin. This margin must be preserved: any future atlas page addition must be evaluated against the faucet ceiling before the page is added.

3. **Authoritative roster**: `07_content/atlas_catalog.md` is the canonical page list. ADR-0028 §1's 81-page figure is historical.

## Consequences
- **Specs changed**: `07_content/atlas_catalog.md` (58-page quai_dam roster, 104-page total, T3 capped at 2 to preserve economy margin), `07_content/README.md` (launch content budget updated to 104 atlas pages), `03_systems/atlas.md` (page count updated if hardcoded there).
- Any future monster added to the field roster that warrants an atlas page must re-evaluate the `currency.special` faucet ceiling before the page is published.
- ADR-0028 §1 is amended to point at this ADR for the current authoritative page count.
