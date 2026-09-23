# Attached predicates — sol2 merge-blocker resolution

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
