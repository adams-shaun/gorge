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
