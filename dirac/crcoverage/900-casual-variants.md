# CR 900s: Casual variants

Coverage of this chapter of the Comprehensive Rules (2026-09-25) by the dirac kernel (two players, FDN and BLB pools). Seeded from the CR audit of 2026-10-20 at `36c2469`; kept current by every change that touches a rule. `inherits gorge` means the kernel copies gorge's behaviour, so gorge's row in `engine/rules/crcoverage/` decides it. See [README](README.md).

| rule | status | code | tests | notes |
|---|---|---|---|---|
| 900.1-900.2 | n/a | - | - | dirac two-player kernel; descriptive |
| 901.1-901.2 | n/a | - | - | dirac two-player kernel; engine-only scaffold; no hosted table deals a planar deck |
| 901.3 | n/a | - | - | dirac two-player kernel; no planar deck size/duplicate validation; engine-only |
| 901.3a | n/a | - | - | dirac two-player kernel; planar die faces |
| 901.4 | n/a | - | - | dirac two-player kernel; planes live in private ZPlanarDeck, not command zone |
| 901.5 | n/a | - | - | dirac two-player kernel; every seat reveals a plane at genesis |
| 901.6 | n/a | - | - | dirac two-player kernel; no planar controller; SILENT engine-side, unreachable hosted |
| 901.7 | n/a | - | - | dirac two-player kernel; only chaos and planeswalk triggers fire |
| 901.7a | n/a | - | - | dirac two-player kernel; not located: PlanarWalk keeps the plane's ObjID |
| 901.8 | n/a | - | - | dirac two-player kernel; engine-only; no planar-die trigger |
| 901.9-901.9d | n/a | - | - | dirac two-player kernel; no planar-die special action; SILENT engine-side, unreachable hosted |
| 901.10-901.15c | n/a | - | - | dirac two-player kernel; departure, 2HG, Grand Melee, single deck; unreachable hosted |
| 902.1-902.7 | n/a | - | - | dirac two-player kernel; Vanguard; LOUD (unreachable: host offers no such variant) |
| 903.1 | n/a | - | - | dirac two-player kernel; FormatCommander |
| 903.2 | n/a | - | - | dirac two-player kernel; 2-8 seat FFA attack-multiple |
| 903.3 | n/a | - | - | dirac two-player kernel; designation keyed to card object id |
| 903.3a | n/a | - | - | dirac two-player kernel; oracle 'can be your commander' phrase |
| 903.3b | n/a | - | - | dirac two-player kernel; REVISED: melded permanent keeps result card's id; commander partner half loses status |
| 903.3c | n/a | - | - | dirac two-player kernel; REVISED: pile keeps target's id; commander mutating onto another creature loses status (inferred) |
| 903.3d | n/a | - | - | dirac two-player kernel; IsCommander filter reads Player.Commanders per zone |
| 903.3e | n/a | - | - | dirac two-player kernel; REVISED: commander objects read live in any zone |
| 903.4 | n/a | - | - | dirac two-player kernel; cost, indicator, rules text, CDA |
| 903.4a | n/a | - | - | dirac two-player kernel; identity fixed from card data |
| 903.4b | n/a | - | - | dirac two-player kernel; game honours choice; deck validator ignores ChosenColor |
| 903.4c | n/a | - | - | dirac two-player kernel; parenthesised reminder text skipped |
| 903.4d | n/a | - | - | dirac two-player kernel; union over faces |
| 903.4e | n/a | - | - | dirac two-player kernel; adventure faces included in union |
| 903.4f | n/a | - | - | dirac two-player kernel; no commander: fails closed |
| 903.5 | n/a | - | - | dirac two-player kernel; - |
| 903.5a | n/a | - | - | dirac two-player kernel; exactly 100 |
| 903.5b | n/a | - | - | dirac two-player kernel; per-entry count; duplicate entries not merged |
| 903.5c | n/a | - | - | dirac two-player kernel; - |
| 903.5d | n/a | - | - | dirac two-player kernel; - |
| 903.5e | n/a | - | - | dirac two-player kernel; sideboard not rejected; host passes it in |
| 903.6 | n/a | - | - | dirac two-player kernel; - |
| 903.7 | n/a | - | - | dirac two-player kernel; 40 life |
| 903.8 | n/a | - | - | dirac two-player kernel; - |
| 903.9 | n/a | - | - | dirac two-player kernel; see 903.9a/b |
| 903.9a | n/a | - | - | dirac two-player kernel; HIGH: replacement not SBA; dies triggers lost; |
| 903.9b | n/a | - | - | dirac two-player kernel; always first; decline skips other replacements; |
| 903.9c | n/a | - | - | dirac two-player kernel; not located |
| 903.10 | n/a | - | - | dirac two-player kernel; - |
| 903.10a | n/a | - | - | dirac two-player kernel; - |
| 903.11 | n/a | - | - | dirac two-player kernel; wishes reach sideboard; host passes Commander sideboard in |
| 903.11a | n/a | - | - | dirac two-player kernel; SILENT: no name or colour-identity check on wished card |
| 903.12-903.12h | n/a | - | - | dirac two-player kernel; Brawl; LOUD (unreachable: host offers no such format) |
| 903.13-903.13g | n/a | - | - | dirac two-player kernel; Commander Draft; LOUD (unreachable: no draft in gorge) |
| 904.1-904.13d | n/a | - | - | dirac two-player kernel; Archenemy; LOUD (unreachable: host offers no such variant) |
| 905.1-905.6 | n/a | - | - | dirac two-player kernel; Conspiracy draft; LOUD (unreachable: no draft in gorge) |
