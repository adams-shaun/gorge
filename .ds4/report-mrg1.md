# Merge conflict resolution — mrg1

## Result

Integrated `main` into `wt/cli-20260922T225141Z-22391c4d` with merge commit
`00d48a32`. The branch's reviewed token-replacement work and main's changes are
both retained. The only conflicted source file was
`internal/testutil/agentsdoc_test.go`.

## Conflict resolution

### `internal/testutil/agentsdoc_test.go`

- The branch side recorded the table count and closures for its token-replacement
  approximation deletion (and its prior mulligan-redraw closure).
- Main's side recorded the same constant but a different set of main-side
  closures, including First-Strike Damage and the layer-4 filter grammar.
- Kept both sides' intent in the explanatory history and set
  `knownApproximationRows` to 40, the current data-row count in the merged
  `AGENTS.md` after retaining the branch's `(tokrepl1)` deletion and main's
  deletions. The table ratchet test passed at that value.

The conflicted file was staged and the merge completed. Other main changes were
automatically integrated. `.ds4/report-mrg1.md` arrived in the merge from a
sibling integration report; this file is the report path required by this
work item, so it was replaced with this integration's report.

## Commands and output

- `git status --short --branch; git rev-parse --show-toplevel; git log -5 --oneline --decorate`:
  clean at start on `wt/cli-20260922T225141Z-22391c4d`, at `f89770be`.
- `git merge --no-edit main`:
  `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`;
  merge otherwise auto-merged.
- `git diff --check`: no output (passed).
- `.cards` check: present as a symlink to `/home/sadams/projects/gorge/.cards`.
- `git -c core.editor=true merge --continue`:
  `[wt/cli-20260922T225141Z-22391c4d 00d48a32] Merge branch 'main' into wt/cli-20260922T225141Z-22391c4d`.
- `go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/`:
  `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  `ok github.com/adams-shaun/gorge/rules 0.792s`.
- Final tree status before writing this report: clean after merge commit.

## Issues

No new unfixed issue was found while resolving the merge. The token-replacement
implementation's out-of-scope remainder is recorded in the branch's ticket
report/commit history; this conflict resolution did not alter engine behavior
outside retaining main's changes.
