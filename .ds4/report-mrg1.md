# Merge conflict resolution: mrg1 (round 2 — main advanced to 6963c6b6)

## This resolution

`git status` found no operation in flight (a prior round's merge, `25bfc8e5`, was already
complete), but main had advanced past it. Re-ran `git merge --no-commit --no-ff main`
to integrate main's new work (TargetUnique accumulator, Count$Compare thresholds,
Exile/Discard/Return cost verbs) and hit the reported conflicts:

- `AGENTS.md` — three rows deleted in total by the two sides on the same block:
  main deleted the `TargetUnique$` row (506e7167) and the sacrifice-cost
  Exile/Discard/Return row (b3fd6a77); this branch deleted the `effCharm` row (1eb845f5).
  Resolution: all three rows removed — the merged table carries both sides' deletions.
  Measured merged table: 80 data rows; `knownApproximationRows` lowered 82 -> 80
  (auto-merge had taken main's 82; the merged table must reflect this merge's own
  combined deletions, and the constant is never raised).
- `.ds4/report-mrg1.md` — add/add from both sides' resolver reports; combined below.

## Prior round (HEAD side, commit 25bfc8e5 + 3f3ab313)

# Merge conflict resolution: mrg1

## Operation found

`git status` showed a clean worktree on `wt/cli-20260922T225138Z-a850f8be`; there was no rebase/merge currently in progress. `git log` showed HEAD `25bfc8e5`, a completed merge commit with parents `1eb845f5` (the reviewed branch fix) and `7dd56763` (main). Thus the reported conflict had already been resolved and the merge operation completed before this run. No conflict markers remained in either reported file.

## Conflicted files and resolution

- `AGENTS.md`: the reviewed branch deleted the stale effCharm approximation row while main's updates were preserved. Search found no remaining matching stale row. The merge commit includes the combined main changes and the branch commit.
- `internal/testutil/agentsdoc_test.go`: the merged row count is `84`; main's intervening removals and this branch's deletion are reflected. The merge commit includes the combined changes.

I did not alter either file because the completed merge already contained the resolution. `.cards` exists in this worktree (`ls .cards` succeeded).

## Commands and output

- `pwd; git status --short --branch; git status`
  - `## wt/cli-20260922T225138Z-a850f8be`; clean, no operation in progress.
- `git log --oneline --decorate -6`
  - HEAD was `25bfc8e5 Merge branch 'main' into wt/cli-20260922T225138Z-a850f8be`.
- Conflict-marker searches in `AGENTS.md` and `internal/testutil/agentsdoc_test.go`
  - No matches found.
- `ls .cards >/dev/null 2>&1; echo cards_exit=$?`
  - `cards_exit=0`.
- `go test -run 'TestCharmModeLoopStopsAtAMidModeSuspension' ./effects/`
  - `ok github.com/adams-shaun/gorge/effects (cached)`; exit 0.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  - `ok github.com/adams-shaun/gorge/rules 0.820s`; exit 0.
- Final status before report commit: clean.

No uncertainty remains about the merge state; the in-flight operation described in the dispatch had already been completed as commit `25bfc8e5`.

## Main side's own mrg1 report

# Merge conflict resolution mrg1

- `internal/testutil/agentsdoc_test.go`: the branch lowered `knownApproximationRows` from 88 to 87 after deleting its stale cost-verb approximation row. Main independently removed three other rows and set its count to 84. The merged `AGENTS.md` includes both sides' deletions; its measured data-row count is 83, so the resolved constant is 83.
- `AGENTS.md`: main's independent table updates were retained alongside the branch's deletion of the now-false Exile/Discard/Return cost row. No unrelated files were manually edited; merge changes from main are retained as part of integration.

Commands and results:
- `git status` — initially clean; no operation was in flight. The supplied conflict note described the failed rebase, but current branch was clean.
- `git merge main` — content conflict in `internal/testutil/agentsdoc_test.go` only; `AGENTS.md` auto-merged.
- Counted Known approximations rows from both refs: branch `HEAD` had 86, main had 84 (main still contained the cost-verb row). Combined intended state is 83.
- `[ -e .cards ]` — `CORPUS_PRESENT`.
- `go test ./internal/testutil/ -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'` — `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` — `ok github.com/adams-shaun/gorge/rules 0.855s`.

Uncertainty: none. Main's three independent row deletions plus this branch's one deletion account for the combined count of 83.
