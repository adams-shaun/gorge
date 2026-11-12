# CR coverage

Rule by rule, how the dirac kernel stands against the Magic: The
Gathering Comprehensive Rules (effective 2026-09-25), and where the code that
implements each rule lives (hand-written rules in `kernel/*.go`, card ops in
`dirac/kop`, lowering in `dirac/kgen`). The kernel is two-player and runs fixed
pools (FDN, and BLB's supported cards), so many rules are `n/a` here with the
reason in the note. One file per CR chapter; one row per numbered rule, split into subrules
only where the status or the code differs.

The tables were seeded from a full CR audit on 2026-10-20 (eleven slices, each
rule checked against the code and, where gorge was suspected wrong, against
XMage and Forge). The kernel mirrors gorge and is verified by lockstep
against it: `inherits gorge` in a note means the kernel copies gorge's
behaviour, so gorge's row (`engine/rules/crcoverage/` in gorge) decides it, and
a gorge fix there needs a kernel follow-up here. They are not a snapshot: **a change that alters how the kernel
handles a rule updates that rule's row in the same commit**, and
`TestCRCoverageIsCurrent` (`dirac/crcoverage/crcoverage_test.go`, its own
package so the kernel bench binary is unchanged) fails the build
when a row stops matching the code.

## Reading a row

| column | meaning |
|---|---|
| rule | CR number: `601.2c`, a range `100.2-100.3`, or a list `702.1a,d`. Each rule appears in exactly one row. |
| status | one of the values below |
| code | where the rule is decided: `` `path#Symbol` `` (a function, `Type.Method`, type, var, const or `Type.Field`) or a bare `` `path` ``. For `deviates`, the code doing the wrong thing. No line numbers: they rot. |
| tests | tests that exercise the rule |
| notes | what is partial or wrong |

| status | meaning |
|---|---|
| follows | the kernel does what the rule says |
| partial | some cases follow, some don't (the note says which) |
| deviates | the kernel does something other than what the rule says |
| not-impl | the kernel does not implement it; LOUD (kgen refuses the card) or SILENT (it compiles but misbehaves) |
| definition | the rule only defines a term; nothing to point at |
| n/a | the rule has no subject in the kernel (multiplayer, a mechanic no pool card has) |
| unknown | not established yet |

## Keeping it current

- Changing a rule's behaviour: update its row (status, code, tests, note), then
  run `go test -run TestCRCoverageIsCurrent ./dirac/crcoverage`.
- Renaming or moving a referenced declaration: the test names the row; fix the
  reference.
- The summary below is generated: `KERNEL_UPDATE_CRCOVERAGE=1 go test -run
  TestCRCoverageIsCurrent ./dirac/crcoverage` rewrites it. The test never rewrites
  references.
- `unknown` rows are open questions. Settling one is a welcome small change.

## Summary

<!-- crcoverage:summary -->
| file | rows | follows | partial | deviates | not-impl | definition | n/a | unknown |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| [100-game-concepts](100-game-concepts.md) | 177 | 84 | 17 | 3 | 1 | 0 | 48 | 24 |
| [200-parts-of-a-card](200-parts-of-a-card.md) | 81 | 40 | 2 | 0 | 0 | 3 | 36 | 0 |
| [300-card-types](300-card-types.md) | 98 | 55 | 8 | 0 | 0 | 3 | 29 | 3 |
| [400-zones](400-zones.md) | 70 | 44 | 0 | 0 | 1 | 4 | 20 | 1 |
| [500-turn-structure](500-turn-structure.md) | 125 | 77 | 6 | 1 | 0 | 0 | 41 | 0 |
| [600-spells-abilities-effects](600-spells-abilities-effects.md) | 288 | 136 | 23 | 5 | 1 | 9 | 98 | 16 |
| [700-additional-rules](700-additional-rules.md) | 439 | 196 | 32 | 1 | 1 | 1 | 206 | 2 |
| [702-keyword-abilities](702-keyword-abilities.md) | 252 | 81 | 22 | 2 | 0 | 0 | 146 | 1 |
| [800-multiplayer](800-multiplayer.md) | 43 | 0 | 0 | 0 | 0 | 0 | 43 | 0 |
| [900-casual-variants](900-casual-variants.md) | 49 | 0 | 0 | 0 | 0 | 0 | 49 | 0 |
| **all** | 1622 | 713 | 110 | 12 | 4 | 20 | 716 | 47 |
<!-- /crcoverage:summary -->
