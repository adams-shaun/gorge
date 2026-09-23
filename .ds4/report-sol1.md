# Convoked$Amount — fix-round report

STATUS: DONE. Commit `88d159e4` (following initial implementation `9e650da1`). `.cards` was present as a symlink to the shared corpus; `/usr/bin/grep -rlE 'Convoked\$Amount' .cards/cardsfolder | wc -l` returned **2**.

## Changes

- `effects/count.go`: only the new Convoked head rejects unknown/invalid `/Op` suffixes for both bare and `Count$` forms; known operators still compose via the existing arithmetic. No changes to the behavior of other count heads.
- `effects/convoked_amount_test.go`: assert `(0,true)` for empty and absent provenance and `(0,false)` for unsupported operators (both spellings), rather than losing evaluability through `Num`.
- `rules/convoked_amount_test.go`: the real cast of Imperiosaur captures exactly two selected bears and excludes a third untapped battlefield bear, then ETB adds four counters. The real Knight ETB Dig is driven to its actual ask with a replay-visible library order and MV 0/2/4/5 fixture preconditions; only 0 and 2 are offered, and the MV-2 bear is selected and reaches hand. The prior post-resolution count assertion remains. This proves the engine consumer, rather than only a manually seeded SVar read.

The structural fix uses `Object.Convoked` already captured by the cast and the existing `/Op` evaluator; no cast-time gating or event changes. No count-head ratchet entry or Known-approximations row corresponded to this head; neither was changed. Botbench split did not move.

## Fails without the fix

Copied `effects/count.go` to `.ds4/scratch/count-fixed.go`, replaced it temporarily with `git show 9e650da1^:effects/count.go`, ran `go test -run 'TestConvokedAmount|TestAncientImperiosaurEntersWithTwoCountersPerConvoker|TestKnightErrantOfEosXCountsConvokers' ./effects ./rules`, then restored the exact copy (`cmp` exit 0). Exit 1:

```
--- FAIL: TestConvokedAmountReadsTheCorpusHeads (0.59s)
    convoked_amount_test.go:75: Num Amount$ X (Knight-Errant SVar) = 0, want 2
--- FAIL: TestConvokedAmountEmptyAndAbsentAreEvaluatedZero (0.00s)
    convoked_amount_test.go:111: empty provenance = (0,false), want (0,true)
FAIL github.com/adams-shaun/gorge/effects
--- FAIL: TestAncientImperiosaurEntersWithTwoCountersPerConvoker (0.57s)
    convoked_amount_test.go:135: Ancient Imperiosaur P1P1 counters = 0, want 4 (2 convokers x /Twice)
--- FAIL: TestKnightErrantOfEosXCountsConvokers (0.00s)
    convoked_amount_test.go:216: Knight Dig never offered a choice
FAIL github.com/adams-shaun/gorge/rules
```

Separately removed just the two operator-validation checks temporarily, ran `go test -run '^TestConvokedAmountEmptyAndAbsentAreEvaluatedZero$' ./effects` and restored the byte-identical copy (`cmp` exit 0). Exit 1:

```
--- FAIL: TestConvokedAmountEmptyAndAbsentAreEvaluatedZero (0.58s)
    convoked_amount_test.go:124: unknown operator "Convoked$Amount/Unmodelled" = (0,true), want unresolved
FAIL github.com/adams-shaun/gorge/effects
```

## Gates

`go test -run 'TestConvokedAmount|TestAncientImperiosaurEntersWithTwoCountersPerConvoker|TestKnightErrantOfEosXCountsConvokers|TestEveryRepoDeckCountHeadResolves' ./effects ./rules` (exit 0):

```
ok   github.com/adams-shaun/gorge/effects (cached)
ok   github.com/adams-shaun/gorge/rules 0.701s
```

`go test ./internal/archtest/` (exit 0):

```
ok   github.com/adams-shaun/gorge/internal/archtest 3.086s
```

`go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` (exit 0):

```
ok   github.com/adams-shaun/gorge/cmd/botbench 1.191s
```

`gofmt -l effects/count.go effects/convoked_amount_test.go rules/convoked_amount_test.go` (exit 0): no output.
`go run ./cmd/gentypes -check` (exit 0): no output.
`git diff --check` (exit 0): no output.

## Issues

None found outside scope. No ledger entry closed; no CR lane test warranted for this card-specific count head.
