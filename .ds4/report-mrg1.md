# Merge conflict resolution — mrg1

## Result

Merged `main` into `wt/cli-20260922T225142Z-171edf8c`. The worktree started clean with no in-flight operation. The daemon's reported rebase conflict was reproduced using `git merge main`; the merge conflicted only in `internal/testutil/agentsdoc_test.go`.

## Conflict — `internal/testutil/agentsdoc_test.go`

Both sides set `knownApproximationRows = 38` and preserve the table ratchet. The branch comment said the count preserved main's deletions and this branch's bestow1 deletion. Main's comment listed the closure history, including its cascade and token-replacement closures and this branch's maxpower1 closure. These explanations are compatible, not competing behavior.

Resolution: retained main's closure-history comment, and added that this branch's bestow1 deletion is also included. Kept the shared count at 38. The resulting targeted ratchet tests pass.

The merge also brought in main's already-integrated files (including the AGENTS.md deletion and rules changes/tests); they were not hand-edited as part of resolving this conflict.

## Commands and output

- `git status --short --branch && git status` — `## wt/cli-20260922T225142Z-171edf8c`; clean before starting, no operation in progress.
- `git merge main` — `Auto-merging AGENTS.md`; `Auto-merging internal/testutil/agentsdoc_test.go`; `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`; automatic merge failed.
- Checked `.cards` — present.
- `go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/` — `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` — `ok github.com/adams-shaun/gorge/rules 0.814s`.

## Issues

No new unfixed issue was found while resolving the conflict. No uncertainty remains about the conflicting comment/count; both sides agree on the ratchet value, and the focused ratchets passed.
