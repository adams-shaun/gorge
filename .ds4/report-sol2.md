# Dismantle target-counter LKI — sol2 report

## Findings resolved

- **MAJOR (`findings-sol2.md`, destructive report overwrite):** Restored the pre-existing Gitaxian Probe `.ds4/report-sol1.md` byte-for-byte from the parent of `3e243fd1` (`git show HEAD^:.ds4/report-sol1.md | cmp - .ds4/report-sol1.md` passed). Removed the redundant `.ds4/report-sol1-gitaxian-probe.md` copy. This ticket's report is now this unoccupied `.ds4/report-sol2.md`. No unrelated historical report was rewritten. The Dismantle-specific reports at `.ds4/report-t1-dismantle.md` and `.ds4/report-r2-dismantle.md` remain.
- No new Go edits in this round; prior reviewers' break attempts held. No rebase or merge attempted in this worktree.

## Changes from this ticket

- `30b4e31c`: `effects/registry.go`, `conditions.go`, `count.go`, `counters.go`, `rules/resolution.go`, `clone.go` capture resolving targeted counters and share the look-back between the chained gate and amount read; `effects/targeted_counters_lki_test.go`, `rules/dismantle_lki_test.go` assert the condition and real destructible Dismantle path.
- `a303708a`: `rules/engine.go`, `resolution.go` refresh the targeted snapshot at the final battlefield-departure boundary (after earlier counter changes in the resolution); `TestChainReadsTargetCountersChangedEarlierInResolution` pins 3 rather than stale 2.
- `580f18d4`: `effects/conditions.go`, `count.go`, `targeted_counter_scope_test.go` scope substitution to `ConditionDefined$ Targeted` and `Targeted$CardCounters.*`; Remembered continues reading live values. Existing indestructible Dismantle test is separate and unchanged.

The real-card test establishes an opposing destructible artifact on the battlefield with two counters and a separate clean caster-controlled artifact on the battlefield. It checks that destruction sends the target to the graveyard and the recipient receives two CHARGE (not P1P1). The snapshot is refreshed at the common pre-Apply departure boundary, not at a Dismantle-specific call site; both readers use the shared scoped helper. No state mutation outside `events.Apply`. No known-approximations row closed or grown, no goldens or ratchets edited. `.cards` was present as a corpus symlink; the script-shape census returned 1 file, and repo deck mentions of Dismantle returned 0.

## Fails without the fix

Prior rounds reverted the production hunks while keeping new tests, ran them and restored the hunks byte-identically. Their actual outputs:

```
--- FAIL: TestDismantleDestroysCounteredTargetAndPlacesCounters (0.63s)
    dismantle_lki_test.go:63: never reached a non-priority decision within 30 passes
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.646s
```
```
--- FAIL: TestChainReadsTargetCountersChangedEarlierInResolution (0.62s)
    dismantle_lki_test.go:147: recipient CHARGE = 2, want 3 (the target's counters immediately before destruction: 2 + the chain's own 1)
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.632s
```
```
--- FAIL: TestTargetCounterLKIOnlyAppliesToTargetedReads (0.00s)
    targeted_counter_scope_test.go:33: Remembered gate: met=true resolved=true, want false true
    targeted_counter_scope_test.go:40: remembered live counter count = 2, want 0
FAIL
FAIL github.com/adams-shaun/gorge/effects 0.002s
FAIL
```

## Gates (this round; exact commands and output)

```
$ go test -run 'TestConditionGateTargetedGroup|TestDismantleCounterTypeChoiceContinuesAfterDeterministicRecipient|TestDismantle.*Counter|TestChainReadsTargetCountersChangedEarlierInResolution' ./rules/
ok   github.com/adams-shaun/gorge/rules (cached)
$ go test -run 'TestTargetCounterLKIOnlyAppliesToTargetedReads|TestConditionGateTargetedCountersUseLKI' ./effects/
ok   github.com/adams-shaun/gorge/effects (cached)
$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest (cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench (cached)
$ gofmt -l effects/conditions.go effects/count.go effects/counters.go effects/registry.go effects/targeted_counter_scope_test.go effects/targeted_counters_lki_test.go rules/clone.go rules/dismantle_lki_test.go rules/engine.go rules/resolution.go
(no output; exit 0)
$ go run ./cmd/gentypes -check
(no output; exit 0)
```

The earlier uncached botbench gate returned `ok github.com/adams-shaun/gorge/cmd/botbench 1.180s` with no split movement. No full rules-suite run in the implementer seat.

## Issues

- Other departed-target characteristics (e.g. dynamic P/T in `effects/count.go:evalRefProperty`) do not have this targeted-counter look-back. No specific post-departure P/T corpus-chain prevalence measured; outside this brief. CR 608.2b/h conformance could expose it.
- `effects/conditions.go:conditionNotPresentMet` does not use counter LKI for `ConditionNotPresent$ Targeted`; no `NotPresent$ Card.HasCounters` corpus shape was identified in the earlier census. A future corpus instance would need a separate ticket.
