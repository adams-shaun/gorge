# Merge conflict resolution report — mrg1

## Conflicted files

- `internal/testutil/agentsdoc_test.go`: the branch had lowered `knownApproximationRows` from 84 to 83 for its deleted approximation row. Main had independently lowered it to 77 while deleting additional rows. The merge result in `AGENTS.md` contains 76 data rows, so resolved the conflict to 76 to retain both sides' row deletions and keep the ratchet synchronized with the merged table.
- `AGENTS.md`: auto-merged without a textual conflict. Kept all main's removals and the branch's removal; no manual edits were needed.

## Commands and output

`git status --short --branch; git status; git diff -- internal/testutil/agentsdoc_test.go; git diff --cc -- internal/testutil/agentsdoc_test.go; git log --oneline -5 --decorate`

```text
## wt/cli-20260922T225139Z-30adfed9
On branch wt/cli-20260922T225139Z-30adfed9
nothing to commit, working tree clean
```

The initial branch was clean; the reported conflict had not yet been initiated in this worktree. Main had advanced, so merged current `main` to integrate it and resolve the same reported conflict.

`git merge --no-commit main`

```text
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

`go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`

```text
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
```

`go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`

```text
ok  github.com/adams-shaun/gorge/rules  0.947s
```

No additional uncertainties or unresolved deviations.
