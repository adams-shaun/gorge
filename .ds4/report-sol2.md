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

---

# Attached predicates — sol2 merge-blocker resolution (preserved from main)

## Finding resolution

`findings-sol2.md` reports that rebase/merge was blocked by unstaged overwrites of `.ds4/report-r2.md` and `.ds4/report-t1.md`. These files belong to unrelated tasks; the current worktree's overwrites contained this ticket's round-2 report and the counters-remain report respectively. I preserved the overwritten bytes in ignored `.ds4/scratch/report-{r2,t1}-pre-sol2.md`, then restored each report from `HEAD` using `git show HEAD:<path> > <path>`. `git status --short` and `git diff --check` printed nothing afterward. No unrelated report was committed or replaced with this ticket's content. I did **not** rebase or merge: the worktree instructions forbid `git rebase` and moving shared branches; the controller owns integration.

The Attached implementation and regression tests remain committed in `60976e28`, `53403389`, `82db540a`, and `6b7f8116`. The earlier round's `.ds4/report-sol1.md` records per-file changes and the original fix-reverted failures. No new code or tests were introduced in sol2; there is no new `## Fails without the fix` run. `.cards` was already present and linked to the shared corpus, not a vacuous corpus test. No Known-approximations row was closed; no head golden or ratchet edited.

## Gates (exact commands and output)

```
$ go test -run 'TestAttachedPredicate|TestAttachedToContextReferents|TestAttachedToReferentPluralBindingFailsClosed|TestAttachedToStaleReferentFailsClosed|TestAttachedToLiteralPredicate|TestAttachedToTargetedBoundFromContext|TestAttachedToPlayerWordStaysUnknown|TestAttachedToPredicateUnlocksCorpusTargeting|TestArnaCopy|TestArnaRealSourceFilterReachesCopyRider|TestStanggRealTriggerCopiesAttachedPermanents' ./effects ./rules/
ok   github.com/adams-shaun/gorge/effects  0.714s
ok   github.com/adams-shaun/gorge/rules    0.802s
$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest (cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench (cached)
$ git status --short
(no output after report restoration)
$ git diff --check
(no output)
```

## Issues

No new engine defects found in sol2. Pre-existing plural-target `AttachedTo Targeted` limitations and the separate Defined-selector issue are documented in `.ds4/report-r2.md`'s ticket report copy (`.ds4/scratch/report-r2-pre-sol2.md`) and earlier commit messages; no new or grown AGENTS.md row. Integration/rebase remains for the controller, not this worktree.

---

# Bare Vanishing — sol2 merge-blocker resolution (agent-20260923T145419Z-c0d85ef9)

## Finding resolved

`findings-sol2.md` reports rebase/merge blocked by three unstaged report overwrites. I copied the overwritten bytes to ignored `.ds4/scratch/vanishing-report-{r2,sol1,t1}-pre-sol2.md`, then restored `.ds4/report-r2.md`, `.ds4/report-sol1.md`, and `.ds4/report-t1.md` exactly from `HEAD` with `git show HEAD:<path>`. `git status --short` and `git diff --check` printed nothing afterward. The existing unrelated sol2 reports in this file are preserved above; this addendum is committed so the worktree remains clean. Integration/rebase belongs to the controller, not this worktree.

## Work already committed

`09c5dcea` changes `cards/kw_vanishing.go` so only the fixed-N replacement is conditional; `cards/kw_vanishing_bare_test.go` asserts bare triggers without a replacement and numeric preservation. `7c11a324` leaves `DB$ Phases` loudly unsupported rather than falsely registering incomplete global phasing. `rules/vanishing_oot_test.go` verifies unassisted dynamic entry counters from Tidewalker's Island count, and Out of Time's printed dynamic count, upkeep tick, and last-counter sacrifice after the fixture supplies remembered creatures through events. No GPL card scripts or Known-approximations rows changed. `.cards` was present as a symlink to the real corpus, not a skipped corpus run. No head/ratchet movement measured in this seat.

## Fails without the fix

The previous round copied `cards/kw_vanishing.go` into `.ds4/scratch`, reinstated the original early return while keeping the tests, ran the focused tests, then restored the source byte-identically (`cmp` passed). Its recorded output, preserved in `.ds4/scratch/vanishing-report-sol1-pre-sol2.md`:

```
--- FAIL: TestVanishingBareExpansion (0.00s)
    kw_vanishing_bare_test.go:15: bare Vanishing triggers = 0, want upkeep removal and last-counter sacrifice
FAIL
FAIL github.com/adams-shaun/gorge/cards 0.002s
--- FAIL: TestVanishingTidewalkerDynamicCountUpkeepAndLastCounter (0.61s)
    vanishing_oot_test.go:71: controller upkeep put 0 stack objects, want one Vanishing removal trigger
--- FAIL: TestVanishingOutOfTimeDynamicCountUpkeepAndLastCounter (0.00s)
    vanishing_oot_test.go:202: controller upkeep put 0 stack objects, want one Vanishing removal trigger
--- FAIL: TestVanishingOutOfTimeSeededCounterClock (0.00s)
    vanishing_oot_test.go:257: controller upkeep put 0 stack objects, want one Vanishing removal trigger
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.642s
FAIL
exit=1
restored-byte-identical
```

No new test or production change in sol2; no repeated revert test needed.

## Gates (this round; exact commands and output)

```
$ go test -run 'TestVanishingExpansion|TestVanishingBare|TestVanishingOutOfTime|TestVanishingTidewalker|TestVanishingDeepForestHermit|TestVanishingOnlyTriggers' ./cards/ ./rules/
ok  	github.com/adams-shaun/gorge/cards	0.003s
ok  	github.com/adams-shaun/gorge/rules	0.668s
exit=0
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)
exit=0
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)
exit=0
$ gofmt -l cards/kw_vanishing.go cards/kw_vanishing_bare_test.go rules/vanishing_oot_test.go
exit=0
$ go run ./cmd/gentypes -check
exit=0
```

No full package suite run in the seat.

## Issues

Out of Time's `DB$ Phases` remains unimplemented (`effects/registry.go` fallback) and cannot populate remembered creatures or phase them out; this fixture provides only event-backed remembered count inputs, not phasing semantics. The separate CR 702.25 phasing ticket is filed at `.ds4/new-tickets/faithful-phasing-layer.md.filed` (earlier census: 39 `DB$ Phases` carriers). No new defects found in this round.
