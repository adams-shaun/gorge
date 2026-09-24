# K:Retrace implementation report

## Changes

- `rules/retrace_formless_test.go`: added an end-to-end regression test for the deck card Formless Genesis. It asserts the card is in its graveyard and still has Retrace, makes a land available in hand to discard, verifies a Retrace cast is offered, submits the discard choice, then verifies the land and spell move to their expected zones.
- No production or census code change was needed in this round: the rebased base already contains Retrace implementation commit `133495da` (`feat(rules): implement kw:Retrace graveyard cast with land discard`). Its rule-side graveyard offer and additional-cost composition are in `rules/legal.go`/`rules/cast.go`; the registered `kw:Retrace` coverage-ratchet entry is in `rules/cast.go`. Retrace is a casting option, not an ability expansion, so the existing rule-side implementation is the appropriate path rather than adding an expansion to `cards/keywords.go`. The fixed additional discard cost has no card-authored parameter for the parameter census to measure.
- Workspace: `.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`. The corpus measurement found 17 `K:Retrace` files, as stated in the brief.
- Rebase directive: `git rebase main` reported `Current branch ... is up to date.`

## Verification

Targeted Retrace tests:

```text
$ go test -run 'Retrace' ./rules/ > .ds4/scratch/retrace-tests.log 2>&1; rc=$?; tail -30 .ds4/scratch/retrace-tests.log; exit $rc
ok   github.com/adams-shaun/gorge/rules 1.362s
```

The new test was proven to fail when the Retrace graveyard offer loop was temporarily removed from `rules/legal.go`; the source file was restored byte-identically (`cmp` exit 0):

```text
--- FAIL: TestFormlessGenesisRetraceFromGraveyard (0.61s)
    retrace_formless_test.go:40: Formless Genesis Retrace cast not offered: [{Index:0 Kind:play_land Label:Play Test Forest Obj:82 Counter: Player:0 Attacker:0 Battle:0 Required:false BlockMust:false MinBlockers:0 MaxBlockers:0 Controller:0 Group: AltCostIndex:0 CostLife:0 CostTaps:0 Mode: Amount:0 Ability:0 SVar: Keyword: Cost: Grant:<nil> GrantSource:0 GainedSource:0 GainedIdx:0 Value:0} {Index:1 Kind:pass Label:Pass priority Obj:0 Counter: Player:0 Attacker:0 Battle:0 Required:false BlockMust:false MinBlockers:0 MaxBlockers:0 Controller:0 Group: AltCostIndex:0 CostLife:0 CostTaps:0 Mode: Amount:0 Ability:0 SVar: Keyword: Cost: Grant:<nil> GrantSource:0 GainedSource:0 GainedIdx:0 Value:0} {Index:2 Kind:concede Label:Concede Obj:0 Counter: Player:0 Attacker:0 Battle:0 Required:false BlockMust:false MinBlockers:0 MaxBlockers:0 Controller:0 Group: AltCostIndex:0 CostLife:0 CostTaps:0 Mode: Amount:0 Ability:0 SVar: Keyword: Cost: Grant:<nil> GrantSource:0 GainedSource:0 GainedIdx:0 Value:0}]
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.628s
FAIL
restore cmp exit: 0
```

Required behavior goldens:

```text
$ go test ./internal/archtest/ 2>&1 | tail -15
ok   github.com/adams-shaun/gorge/internal/archtest 7.969s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok   github.com/adams-shaun/gorge/cmd/botbench 2.866s
```

Formatting / generated types:

```text
$ gofmt -l rules/retrace_formless_test.go; go run ./cmd/gentypes -check
[no output; both passed]

$ git diff --check
[no output; passed]
```

During test setup, the first compile attempt caught an unused test import (`events`); it was removed before the successful focused run.

## Issues

None found and left unfixed. No acceptance ratchet or chain-head suite was run; those are daemon gates per the worktree instructions.
