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
- The per-file counts are not committed (every CR change used to rewrite them,
  so open tickets conflicted). `go test -v -run TestCRCoverageIsCurrent
  ./engine/rules` logs them. The test never rewrites references.
- `unknown` rows are open questions. Settling one is a welcome small change.

## Summary

Run `go test -v -run TestCRCoverageIsCurrent ./engine/rules` for the counts.
