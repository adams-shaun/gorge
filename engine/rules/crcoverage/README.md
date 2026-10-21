# CR coverage

Rule by rule, how gorge stands against the Magic: The Gathering Comprehensive
Rules (effective 2026-09-25), and where the code that implements each rule
lives. One file per CR chapter; one row per numbered rule, split into subrules
only where the status or the code differs.

The tables were seeded from a full CR audit on 2026-10-20 (eleven slices, each
rule checked against the code and, where gorge was suspected wrong, against
another rules engine). They are not a snapshot: **a change that alters how gorge
handles a rule updates that rule's row in the same commit**, and
`TestCRCoverageIsCurrent` (`engine/rules/crcoverage_test.go`) fails the build
when a row stops matching the code.

## Reading a row

| column | meaning |
|---|---|
| rule | CR number: `601.2c`, a range `100.2-100.3`, or a list `702.1a,d`. Each rule appears in exactly one row. |
| status | one of the values below |
| code | where the rule is decided: `` `path#Symbol` `` (a function, `Type.Method`, type, var, const or `Type.Field`) or a bare `` `path` ``. For `deviates`, the code doing the wrong thing. No line numbers: they rot. |
| tests | tests that exercise the rule |
| notes | what is partial or wrong, and the fix ticket (`cr-gNN`) when one is open |

| status | meaning |
|---|---|
| follows | gorge does what the rule says |
| partial | some cases follow, some don't (the note says which) |
| deviates | gorge does something other than what the rule says |
| not-impl | gorge does not implement it; the note says LOUD (the card reports unsupported) or SILENT (it looks supported but isn't) |
| definition | the rule only defines a term; nothing to point at |
| n/a | the rule has no subject in gorge (for example a tournament-only rule) |
| unknown | not established yet |

## Keeping it current

- Changing a rule's behaviour: update its row (status, code, tests, note), then
  run `go test -run TestCRCoverageIsCurrent ./engine/rules`.
- Renaming or moving a referenced declaration: the test names the row; fix the
  reference.
- The summary below is generated: `GORGE_UPDATE_CRCOVERAGE=1 go test -run
  TestCRCoverageIsCurrent ./engine/rules` rewrites it. The test never rewrites
  references.
- `unknown` rows are open questions. Settling one is a welcome small change.

## Summary

<!-- crcoverage:summary -->
| file | rows | follows | partial | deviates | not-impl | definition | n/a | unknown |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| [100-game-concepts](100-game-concepts.md) | 167 | 102 | 8 | 7 | 7 | 0 | 17 | 26 |
| [200-parts-of-a-card](200-parts-of-a-card.md) | 81 | 46 | 10 | 0 | 5 | 3 | 14 | 3 |
| [300-card-types](300-card-types.md) | 98 | 62 | 11 | 5 | 5 | 4 | 7 | 4 |
| [400-zones](400-zones.md) | 70 | 43 | 9 | 0 | 4 | 4 | 3 | 7 |
| [500-turn-structure](500-turn-structure.md) | 125 | 85 | 22 | 3 | 8 | 0 | 5 | 2 |
| [600-spells-abilities-effects](600-spells-abilities-effects.md) | 286 | 161 | 37 | 13 | 8 | 8 | 3 | 56 |
| [700-additional-rules](700-additional-rules.md) | 437 | 247 | 55 | 46 | 34 | 0 | 33 | 22 |
| [702-keyword-abilities](702-keyword-abilities.md) | 249 | 178 | 13 | 7 | 38 | 0 | 3 | 10 |
| [800-multiplayer](800-multiplayer.md) | 43 | 18 | 6 | 4 | 11 | 0 | 3 | 1 |
| [900-casual-variants](900-casual-variants.md) | 49 | 23 | 12 | 2 | 9 | 0 | 1 | 2 |
| **all** | 1605 | 965 | 183 | 87 | 129 | 19 | 89 | 133 |
<!-- /crcoverage:summary -->
