# Dismantle target-counter LKI — final report

## Changes

- `30b4e31c` (`effects/registry.go`, `effects/conditions.go`, `effects/count.go`, `effects/counters.go`, `rules/resolution.go`, `rules/clone.go`): capture a resolving object's targeted battlefield counters, carry them across suspension, and use one look-back helper for the chained condition and amount read. Added `effects/targeted_counters_lki_test.go` and the ordinary-destruction real-card test in `rules/dismantle_lki_test.go`. The indestructible test in `rules/countertypechoice_test.go` remains separate.
- `a303708a` (`rules/engine.go`, `rules/resolution.go`): refresh the snapshot immediately before a final, replacement-adjusted battlefield departure, after any earlier counter changes in the same resolution. Added `TestChainReadsTargetCountersChangedEarlierInResolution` in this ticket's `rules/dismantle_lki_test.go` (pre-move 3 versus resolution-start 2).
- `580f18d4` (`effects/conditions.go`, `effects/count.go`, `effects/targeted_counter_scope_test.go`): restrict the LKI substitution to `ConditionDefined$ Targeted` and `Targeted$CardCounters.*`; a `Remembered` read of the same object must still use the live object. Both new test paths establish a departed object with zero live counters and a distinct two-counter LKI value. No unrelated condition family is changed.

The real-card test asserts opposing destructible artifact on the battlefield with two counters, clean separate caster artifact on the battlefield, target in graveyard with cleared live counters before the kind answer, and two CHARGE (not P1P1) on the caster's recipient afterward. The no-ask indestructible counter-type test remains intact. Targeted live-object reads continue to win over an old snapshot. No state.Game mutation outside `events.Apply`, no new event kind or encoding changes. Structural fix: all target departures cross the same pre-Apply `Engine.emit` boundary and both counter reads use the one helper, rather than branching on Dismantle's name.

The earlier review found a MAJOR: resolution-entry counter snapshots were stale if an effect changed counters before destruction. `a303708a` fixed it at the final departure boundary and pinned 3 vs 2. The attached `findings-sol1.md` reports a rebase/merge blocked by unstaged report overwrites. Those two Dismantle reports were moved to unique names (`.ds4/report-t1-dismantle.md`, `.ds4/report-r2-dismantle.md`) and the original unrelated tracked `.ds4/report-t1.md` and `.ds4/report-r2.md` were restored exactly from HEAD. The pre-existing Gitaxian Probe `.ds4/report-sol1.md` was preserved byte-for-byte at `.ds4/report-sol1-gitaxian-probe.md` before this designated report was written. No unrelated historical report content was discarded. This seat did NOT rebase/merge; the controller owns integration with main (whose report-sol1 path is also occupied).

`.cards` was present as a symlink to the corpus before testing. Re-measured exact script census: 1 file; repo-deck mentions of Dismantle: 0. No Known-approximations row closed or expanded; no head/ratchet edited. Botbench split unchanged.

## Fails without the fix

Original ordinary Dismantle regression with `30b4e31c`'s non-test fix reverted (round-1 scratch log):
```
--- FAIL: TestDismantleDestroysCounteredTargetAndPlacesCounters (0.63s)
    dismantle_lki_test.go:63: never reached a non-priority decision within 30 passes
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.646s
```
Departure-boundary regression with `a303708a`'s non-test hunks reverted, restored byte-identically (round-2 report):
```
--- FAIL: TestChainReadsTargetCountersChangedEarlierInResolution (0.62s)
    dismantle_lki_test.go:147: recipient CHARGE = 2, want 3 (the target's counters immediately before destruction: 2 + the chain's own 1)
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.632s
```
New scope test with both non-test hunks of `580f18d4` reverted, then restored byte-identically (`cmp`):
```
--- FAIL: TestTargetCounterLKIOnlyAppliesToTargetedReads (0.00s)
    targeted_counter_scope_test.go:33: Remembered gate: met=true resolved=true, want false true
    targeted_counter_scope_test.go:40: remembered live counter count = 2, want 0
FAIL
FAIL github.com/adams-shaun/gorge/effects 0.002s
FAIL
RESTORED_IDENTICAL
```

## Gates run (real output, after scope change)

```
$ go test -run 'TestTargetCounterLKIOnlyAppliesToTargetedReads|TestConditionGateTargetedCountersUseLKI' ./effects/
ok   github.com/adams-shaun/gorge/effects 0.007s
$ go test -run 'TestConditionGateTargetedGroup|TestDismantleCounterTypeChoiceContinuesAfterDeterministicRecipient|TestDismantle.*Counter|TestChainReadsTargetCountersChangedEarlierInResolution' ./rules/ 2>&1 | tail -30
ok   github.com/adams-shaun/gorge/rules 0.722s
$ go test ./internal/archtest/ 2>&1 | tail -15
ok   github.com/adams-shaun/gorge/internal/archtest (cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok   github.com/adams-shaun/gorge/cmd/botbench 1.180s
$ gofmt -l effects/conditions.go effects/count.go effects/targeted_counter_scope_test.go
(no output, after formatting)
$ go run ./cmd/gentypes -check
(no output; exit 0)
```
The full changed-file formatting check (including the files in the two earlier commits) was also empty before the scope change. No full rules-suite run was made in the implementer seat.

## Issues

- Only target counters are looked back up; other departed-target characteristics such as dynamic P/T still read their live/printed state in `effects/count.go:evalRefProperty`. A full-object look-back is outside this targeted-counter brief; a future CR 608.2b/h conformance test could expose it. Corpus prevalence for a specific post-departure P/T chain has not been established.
- The `ConditionNotPresent$ Targeted` branch in `effects/conditions.go:conditionNotPresentMet` does not use targeted counter LKI; no such `NotPresent$ Card.HasCounters` corpus shape was identified in the earlier census. Separate follow-up if observed; not widened here.
- Integration: this branch predates current main and collides with shared report paths on rebase. The two overwritten historical reports were restored and all Dismantle/Gitaxian report contents retained under unique paths; the controller must choose the appropriate report path when integrating. No rebase was attempted here.
