# Merge-conflict resolution report — mrg1

## Conflicted file

- `internal/testutil/agentsdoc_test.go`: the branch removed one Known approximations row and lowered `knownApproximationRows` from 88 to 87. Main had independently removed ten rows and set the constant to 77. Resolved by retaining both sides' changes: the merged `AGENTS.md` contains both sets of row deletions, and the constant is 76. This preserves the delete-only ratchet at the combined table size; no behaviour or test logic changed.

The conflict-free merge changes in `AGENTS.md`, `effects/count.go`, and `effects/filter.go` were retained as merged. Branch fix continues to count affinity keyword permanents.

## Commands and output

- `git status --short --branch`
  ```
  ## wt/cli-20260922T225141Z-d9f9a1a7
  ```
- `git merge main`
  ```
  Auto-merging AGENTS.md
  Auto-merging effects/count.go
  Auto-merging effects/filter.go
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- `go test ./internal/testutil/ -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`
  ```
  ok   github.com/adams-shaun/gorge/internal/testutil 0.001s
  ```
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  ```
  ok   github.com/adams-shaun/gorge/rules 0.751s
  ```

No unresolved uncertainty.
