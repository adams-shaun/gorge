# Cascade free cast — CR 702.85a / CR 107.3b

## Outcome

The brief's premise is not a legal Magic play: when casting Villainous Wealth *without paying its mana cost*, CR 107.3b fixes its mana-cost X at **0**, not an announced 1. The library scan compares its printed MV 3 with Bloodbraid Elf's on-stack MV 4; the free cast remains MV 3. There is no legal at-or-above-4 free-cast X choice to reject. An earlier attempt on this branch added such an X choice (`a1b718ea`), but review identified the CR violation; `f0ae814e` reverted that code and `cfc32dfd` pinned the legal behavior in `rules/cascade_resulting_mv_test.go`. No new cast-flow restriction is warranted. The source's own announced X remains covered by `TestCascadeXSpellUsesAnnouncedManaValue`.

This round strengthened `rules/cascade_resulting_mv_test.go`: asserts the library card *actually has an X mana-cost symbol*, its printed 3 is exactly one below the source's 4, and both cards are on the stack after the free cast with candidate X=0 and resulting MV strictly below source MV. The pre-existing test in that file checks that no X announcement occurs at any stage of the free cast, that it resolves and is never bottomed, and that replay agrees. Its companion guards a non-X candidate. The card scripts were read from the existing `.cards` symlink, not committed. No Known-approximations row was changed; the existing-order bottom stand-in is unchanged.

The required brief test name `TestCascadeFreeCastXMustRemainBelowCascadeManaValue` is intentionally replaced by `TestCascadeFreeCastAnnouncesNoX`: an X=1 reject test would enforce a choice that the rules do not permit. This is also why there is no separate legal X>=1 boundary test.

## Fails without the fix

Proof: copied `effects/cascade.go`, `rules/cast.go`, `rules/resolution.go`, and `rules/play_cost_test.go` to `.ds4/scratch/cascade-sol1/`; temporarily reinstated the prior illegal-X implementation from `a1b718ea`, ran the legal-case test, restored the four files from their copies and verified `cmp` on every file. Real command output (the decision has five X options; elided unrelated option fields here):

```text
$ go test -run 'TestCascadeFreeCastAnnouncesNoX$' ./rules/
--- FAIL: TestCascadeFreeCastAnnouncesNoX (0.58s)
    cascade_resulting_mv_test.go:85: after the election want CR 601.2c's target ask, got &{Seq:78 Player:0 Kind:choose Prompt:Choose a value for X Min:1 Max:1 Options:[{Index:0 Kind:x Label:X = 0 ... Amount:0 ...} {Index:1 Kind:x Label:X = 1 ... Amount:1 ...} {Index:2 Kind:x Label:X = 2 ... Amount:2 ...} {Index:3 Kind:x Label:X = 3 ... Amount:3 ...} {Index:4 Kind:x Label:X = 4 ... Amount:4 ...}] ...}
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.599s
FAIL
reverted-fix exit=1; restored files byte-identical
```

The full unabridged failure is `.ds4/scratch/cascade-sol1/fails.log` (git-excluded). X=1 would make the candidate MV 4, equal to the source; the faulty version wrongly offered that choice at all. This is the only failure against the reverted code claimed for this round; the non-X companion checks scope rather than depending on this fix.

## Gates (real output)

```text
$ go test -run 'TestCascadeFreeCastAnnouncesNoX|TestCascadeNonXFreeCastDoesNotAskX|TestCascadeXSpellUsesAnnouncedManaValue' ./rules/
ok   github.com/adams-shaun/gorge/rules 0.826s

$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest 1.570s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench (cached)

$ gofmt -l rules/cascade_resulting_mv_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output; exit 0)
$ git diff --check
(no output)
```

Botbench golden unchanged (cached result valid for this test-only amendment); no split re-pin or attribution needed. The `.cards` symlink was present and the real-corpus test executed, not skipped. The prior round's proof and gate output are in `.ds4/scratch/cascade-sol1/` and the earlier committed test; the branch history records the reviewed revert explicitly.

## Merge hygiene

Before this round, tracked `.ds4/report-r2.md` and `.ds4/report-t1.md` had uncommitted *unrelated report overwrites*, blocking the controller's rebase/merge (`findings-sol1.md`). Preserved both to `.ds4/scratch/cascade-sol1/report-*-uncommitted.md`, then restored their tracked HEAD content byte-for-byte; neither was staged. Did not rebase, switch branches, or touch shared git settings. Only this task's test and report are being committed.

## Issues

No new out-of-scope defect verified in this round. The brief's X=1 counterexample is ruled out by CR 107.3b, not an outstanding implementation bug. The existing-order cascade bottoming approximation is unchanged. A prior report notes a possible `rules/cast.go:recheckIllegal` mismatch for an X in an *additional cost* on a free cast, but reachability and card prevalence have not been established; no general-X changes are attempted here.

## Commits

`f0ae814e` (review-requested revert of illegal free-cast X choice), `cfc32dfd` (new legal-path regression), `6bcfe1fc` (strict-MV/asserted-X preconditions).
