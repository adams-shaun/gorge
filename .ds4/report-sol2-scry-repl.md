# Scry replacement — nested Dredge review fix

## Changes

- `rules/replacement.go`: preserve the fresh Dredge resume point when the **remaining** Scry candidate (Eligeth after the player chooses Kenessos first) draws instead and parks on Dredge. Link the original Scry continuation behind the nested draw; never overwrite the Dredge frame with the Scry frame. The same branch handles every subsequent Draw-instead candidate, not a named-card special case. The earlier direct-selected Eligeth path already carried the same continuation.
- `rules/scry_replacement_dredge_test.go` (new): real Kenessos, Eligeth and Golgari Thug, inline Scry-then-gain-life spell. Asserts active battlefield sources, distinct initial Scry 3 vs Kenessos Scry 4, legal Dredge 4 in the graveyard and adequate library, affected player's order decision, accepted Dredge, three remaining draws, four milled cards, post-Scry life gain, spell completion, no Scry marker and replay identity. The fixed test passes; removing only the new production hunk makes the accepted Dredge fail.
- `rules/scry_replacement_order_test.go`: stop the casting driver when the stack resolves instead of passing through cleanup. A reverted order branch now fails on the *missing CR 616.1 choice* rather than an unrelated hand-size discard ask; both 3- and 4-draw outcomes remain pinned.
- The original Kenessos/Eligeth and `repl:Scry` registry tests still pass. `.cards` was present (not skipped); `/usr/bin/grep -rlE 'R:Event\$ Scry' .cards/cardsfolder | wc -l` yielded `2`. No Known-approximations row closed, no chain head edited. Botbench split unchanged.

## Fails without the fix

Copied `rules/replacement.go` to `.ds4/scratch/replacement-sol2.fixed.go`; removed ONLY the new nested-resume branch, ran `go test -run '^TestScryReplacementOrderDrawDredgeResumesSpell$' ./rules/` (exit 1), then restored with `cp` and verified `cmp` returned `RESTORED_IDENTICAL`:

```
--- FAIL: TestScryReplacementOrderDrawDredgeResumesSpell (0.67s)
    scry_replacement_dredge_test.go:112: accepted Dredge did not return Thug to hand: zone = graveyard
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.680s
FAIL
```

Temporarily disabled only `continueScryReplacements`' competition branch (the Scry-specific `if len(applicable)>1`, not the other three replacement classes) and ran `go test -run '^TestScryReplacementOrderChoiceAndResume$' ./rules/` (exit 1); `cp` plus `cmp` again printed `RESTORED_IDENTICAL`. This is a direct no-order-choice failure before cleanup:

```
--- FAIL: TestScryReplacementOrderChoiceAndResume (0.56s)
    --- FAIL: TestScryReplacementOrderChoiceAndResume/kenessos_first (0.56s)
        scry_replacement_order_test.go:68: missing affected-player Scry replacement-order choice: pending=priority options=13 spell zone=graveyard
    --- FAIL: TestScryReplacementOrderChoiceAndResume/eligeth_first (0.00s)
        scry_replacement_order_test.go:68: missing affected-player Scry replacement-order choice: pending=priority options=13 spell zone=graveyard
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.578s
FAIL
```

## Gates (commands and actual output)

```
$ go test -run 'TestKenessosReplacesScryCountBeforeLooking|TestEligethDrawsInsteadOfScrying|TestScryReplacementPrimitiveIsRegistered|TestScryReplacementOrderChoiceAndResume|TestScryReplacementOrderDrawDredgeResumesSpell' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.999s
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	2.424s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.156s
$ gofmt -l rules/replacement.go rules/scry_replacement_order_test.go rules/scry_replacement_dredge_test.go
(no output)
$ go run ./cmd/gentypes -check
(exit 0, no output)
$ git diff --check
(no output)
```

## Issues

- Pre-existing The Temporal Anchor `ToBottom$` Scry-trigger count deviation remains in `rules/arrange.go` (completed Scry marker consumed by trigger matcher); not part of this fix. A CR 701.18 trigger-count lane test would expose it. Exactly two corpus `R:Event$ Scry` carriers at the pin. The competing Kenessos/Eligeth order is now covered, including a nested Dredge draw.
- `.ds4/report-sol2.md` is tracked by an unrelated Attached-predicates task; the dispatch requires writing it for the reviewer, but it must NOT be committed or replace that ticket's report on main. This task's unique committed report is `.ds4/report-sol2-scry-repl.md`. Likewise, the already modified `.ds4/report-sol1.md` belongs to another task and was not staged.
