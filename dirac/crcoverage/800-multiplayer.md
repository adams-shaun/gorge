# CR 800s: Multiplayer rules

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by the dirac kernel (two players, FDN and BLB pools). Seeded from the CR audit of 2026-10-20 at `36c2469`; kept current by every change that touches a rule. `inherits gorge` means the kernel copies gorge's behaviour, so gorge's row in `engine/rules/crcoverage/` decides it. See [README](README.md).

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 800.1 | n/a | - | - | dirac two-player kernel; 2-8 seat tables |
| 800.2 | n/a | - | - | dirac two-player kernel; descriptive |
| 800.3 | n/a | - | - | dirac two-player kernel; tournament rules, outside engine |
| 800.4 | n/a | - | - | dirac two-player kernel; game continues past a departure |
| 800.4a | n/a | - | - | dirac two-player kernel; leftover controlled objects never exiled (NOT-IMPL SILENT); cleanup waits for SBA pass; |
| 800.4b | n/a | - | - | dirac two-player kernel; player-control ends; no central guard on control change/token/entry under departed |
| 800.4c | n/a | - | - | dirac two-player kernel; reverts to owner when base controller departed; CR exiles; |
| 800.4d | n/a | - | - | dirac two-player kernel; triggers dropped; departed-owned new objects swept next SBA, not uncreated |
| 800.4e | n/a | - | - | dirac two-player kernel; damage correct, but attackers removed from combat (EndCombatReset) |
| 800.4f | n/a | - | - | dirac two-player kernel; not located: no departed-payer guard in unlessPayerTargets |
| 800.4g | n/a | - | - | dirac two-player kernel; departed chooser's choice declined/released; CR has controller pick another player |
| 800.4h | n/a | - | - | dirac two-player kernel; deterministic default, not next player in turn order |
| 800.4i | n/a | - | - | dirac two-player kernel; REVISED: departed Player record retained; reads see last-known info |
| 800.4j | n/a | - | - | dirac two-player kernel; turn runs on without active player |
| 800.4k | n/a | - | - | dirac two-player kernel; departed seats' turns and extra turns skipped |
| 800.4m | n/a | - | - | dirac two-player kernel; post-departure effect falls back to source-leaves; extra turns don't move boundary |
| 800.4n | n/a | - | - | dirac two-player kernel; gorge models no ante zone |
| 800.4p | n/a | - | - | dirac two-player kernel; no planar controller; unreachable hosted (Planechase engine-only) |
| 800.5 | n/a | - | - | dirac two-player kernel; seat order fixed by table config |
| 800.6 | n/a | - | - | dirac two-player kernel; free mulligan still counts toward Config.Mulligans limit |
| 800.7 | n/a | - | - | dirac two-player kernel; no Two-Headed Giant |
| 801.1-801.18 | n/a | - | - | dirac two-player kernel; range of influence; LOUD (unreachable: host offers no such option) |
| 802.1 | n/a | - | - | dirac two-player kernel; attack multiple players is the only mode |
| 802.2 | n/a | - | - | dirac two-player kernel; every opponent is a defending player |
| 802.2a | n/a | - | - | dirac two-player kernel; defending player per attacker |
| 802.3 | n/a | - | - | dirac two-player kernel; per-creature player/planeswalker/battle choice |
| 802.3a | n/a | - | - | dirac two-player kernel; AGENTS.md Known approximation combatrestriction1 |
| 802.3b | n/a | - | - | dirac two-player kernel; LOUD: banding keyword unregistered, cards unsupported |
| 802.4 | n/a | - | - | dirac two-player kernel; APNAP block declarations |
| 802.4a | n/a | - | - | dirac two-player kernel; blocker controller must be attacked player |
| 802.4b | n/a | - | - | dirac two-player kernel; per-defender block legality |
| 802.5 | n/a | - | - | dirac two-player kernel; damage assigned in APNAP order |
| 803.1-803.1b | n/a | - | - | dirac two-player kernel; attack left/right; LOUD (unreachable: host offers no such option) |
| 804.1-804.2 | n/a | - | - | dirac two-player kernel; deploy creatures; LOUD (unreachable: no teams) |
| 805.1-805.10f | n/a | - | - | dirac two-player kernel; shared team turns; LOUD (unreachable: no teams) |
| 806.1 | n/a | - | - | dirac two-player kernel; Free-for-All is the multiplayer mode |
| 806.2-806.2c | n/a | - | - | dirac two-player kernel; attack multiple, no range of influence, no deploy |
| 806.3 | n/a | - | - | dirac two-player kernel; seats not randomised; starting toss random; no gameplay effect |
| 807.1-807.5b | n/a | - | - | dirac two-player kernel; Grand Melee; LOUD (unreachable: host offers no such variant) |
| 808.1-808.5 | n/a | - | - | dirac two-player kernel; Team vs. Team; LOUD (unreachable: no teams) |
| 809.1-809.7 | n/a | - | - | dirac two-player kernel; Emperor; LOUD (unreachable: no teams) |
| 810.1-810.11 | n/a | - | - | dirac two-player kernel; Two-Headed Giant; LOUD (unreachable: no teams) |
| 811.1-811.5 | n/a | - | - | dirac two-player kernel; Alternating Teams; LOUD (unreachable: no teams) |
