# Non-Goals
status: LOCKED

Launch explicitly does **not** require or include:

- full 3D gameplay
- one seamless open-world map
- turn-based combat
- offline gameplay mode
- separate core gameplay rules for PC and mobile
- client-authoritative combat, movement, reward, inventory or economy state
- open-world PK
- mandatory tank/healer/DPS party composition
- class advancement/evolution tiers
- unrestricted class change
- character deletion (created characters are permanent)
- character appearance customization during creation (base appearance is fixed per class)
- same-account gameplay-resource sharing (vault mule, same-account trade/AH, shared common/bound/special, shared Linh Đan/materials). Only payment/IAP is account-shared (ADR-0029)
- concurrent play of two characters on one account, or two live gameplay sessions on two devices (ADR-0030)
- active dodge button, dodge roll, or dodge iframes in combat (dodge is a percentage stat; the 150ms Just Guard movement-timed 40% mitigation in `../01_gameplay/combat.md` is the explicit exception and is not an iframe)
- generic dungeon difficulty ladders built only from stat inflation
- equipment durability, breakage or repair
- normal-play stamina/energy gating
- infinite paragon/stat progression after Level 60 (seasonal horizontal cosmetic prestige is explicitly allowed; bonus skill/potential books 25-60 are part of 1-60, not paragon)
- mandatory daily login streaks or daily-only permanent-power progression
- a profession/crafting skill tree
- random recipe discovery for baseline equipment
- Soul recycle/fusion/pity currency at launch
- territory empire, guild treasury/research tree or custom guild ACL complexity
- a fourth gameplay currency to solve tuning problems
- mandatory auction purchases for story progression
- historical-simulation accuracy
- exact reproduction of one folklore source/version as definitive belief
- generic Chinese-xianxia, Japanese-yokai or Western-medieval fantasy as the primary setting
- copying identifiable characters, maps, assets, UI, audio or animations from reference games
- any feature assumed necessary only because another MMORPG commonly has it

Future work may add a non-goal only after the owning specs are updated; architecture/data-contract changes also require an ADR.