# Scry replacement — sol3 integration blocker

## Finding resolution

The only sol3 finding is the failed rebase/merge due to dirty `.ds4/report-sol1.md` and `.ds4/report-sol2.md`. Both paths are tracked reports for unrelated tickets; the working-copy overwrites were byte-for-byte copies of the already committed, ticket-unique `.ds4/report-sol1-scry-repl.md` and `.ds4/report-sol2-scry-repl.md`. Restored each unrelated report from this branch's `HEAD` using `git show HEAD:<path> > <path>`. `cmp` confirmed both restored byte-identically; `git status --short` and `git diff --check` both printed nothing. No code or tests changed in this round; no merge/rebase attempted (controller owns integration). The required `.ds4/report-sol3.md` is an ignored, worktree-only reviewer handoff; this unique copy is the committed durable report, avoiding another shared report overwrite.

The Scry implementation is committed in `ae4fed1e`, `8d1151fb`, `1dab341a`: proposal-time Kenessos count and Eligeth draw replacements, completed marker bypass, affected-player order choice for competing replacements, and nested Dredge continuation. Focused Kenessos, Eligeth, order, nested-Dredge and registry tests are committed and passed. `.cards` exists as a symlink (not skipped); two real `R:Event$ Scry` carriers were measured. No Known-approximations row or `rules/heads_test.go` was changed; no head movement measured; the botbench split did not move.

## Fails without the fix

No new tests or production changes in sol3. Prior round's exact fix-reverted outputs (see `.ds4/report-sol2-scry-repl.md`):

```
--- FAIL: TestScryReplacementOrderDrawDredgeResumesSpell (0.67s)
    scry_replacement_dredge_test.go:112: accepted Dredge did not return Thug to hand: zone = graveyard
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.680s
```

```
--- FAIL: TestScryReplacementOrderChoiceAndResume (0.56s)
    --- FAIL: TestScryReplacementOrderChoiceAndResume/kenessos_first (0.56s)
        scry_replacement_order_test.go:68: missing affected-player Scry replacement-order choice: pending=priority options=13 spell zone=graveyard
    --- FAIL: TestScryReplacementOrderChoiceAndResume/eligeth_first (0.00s)
        scry_replacement_order_test.go:68: missing affected-player Scry replacement-order choice: pending=priority options=13 spell zone=graveyard
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.578s
```

The initial Kenessos/Eligeth no-fix failures are in `.ds4/report-t1-scry-repl.md`. Production files were restored byte-identically after each temporary revert.

## Gates (actual outputs, run in sol2; no code changed since)

```
$ go test -run 'TestKenessosReplacesScryCountBeforeLooking|TestEligethDrawsInsteadOfScrying|TestScryReplacementPrimitiveIsRegistered|TestScryReplacementOrderChoiceAndResume|TestScryReplacementOrderDrawDredgeResumesSpell' ./rules/
ok  github.com/adams-shaun/gorge/rules 0.999s
$ go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest 2.424s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench 1.156s
$ gofmt -l rules/replacement.go rules/scry_replacement_order_test.go rules/scry_replacement_dredge_test.go
(no output)
$ go run ./cmd/gentypes -check
(exit 0, no output)
$ git diff --check
(no output)
```

Sol3 restoration check:

```
$ cmp .ds4/report-sol1.md <(git show HEAD:.ds4/report-sol1.md) && echo 'sol1 RESTORED_IDENTICAL'
sol1 RESTORED_IDENTICAL
$ cmp .ds4/report-sol2.md <(git show HEAD:.ds4/report-sol2.md) && echo 'sol2 RESTORED_IDENTICAL'
sol2 RESTORED_IDENTICAL
$ git status --short; git diff --check
(no output)
```

## Issues

Pre-existing Temporal Anchor `ToBottom$` Scry trigger count issue remains in `rules/arrange.go` / Scry marker consumption; CR 701.18 trigger-count lane test would surface it. Exactly two corpus `R:Event$ Scry` cards. No new defects in this report-only round.
