# Merge conflict resolution — mrg1

## Result

Integrated `main` into `wt/cli-20260922T225142Z-0ab0cb60`. The reviewed
`8d83f028` fix and main's changes are retained. The only content conflict was
`internal/testutil/agentsdoc_test.go`; the remaining main changes auto-merged.

## Conflict resolution

### `internal/testutil/agentsdoc_test.go`

- The branch side documented removal of the `(maxpower1)` approximation and
  set the row limit to 42 based on its pre-merge table.
- Main's side included its additional approximation closures and set the limit
  to 40, with `(maxpower1)` still present on main.
- `AGENTS.md` after combining both branches contains 39 data rows: main's
  closures plus the branch's `(maxpower1)` deletion. Kept the closure history
  from both sides and set `knownApproximationRows = 39` to the measured count.
  This preserves the branch's reviewed deletion and main's deletions.

No uncertainty remains about the row count: counting table data rows in the
merged `AGENTS.md` returned 39.

## Commands and output

- `git status --short --branch && git status` (initial):
  `## wt/cli-20260922T225142Z-0ab0cb60`; clean, no operation in progress.
- `git log --oneline --decorate -6`; `git rev-parse main HEAD`:
  confirmed HEAD `8d83f028` and main `4a7bb2fe` before integration.
- `git merge main`:
  `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`;
  other changes auto-merged.
- Count of `AGENTS.md` table rows after merge (Python table count):
  `total table lines 40 data rows 39`.
- `.cards` check: `.cards exists`.
- `git add internal/testutil/agentsdoc_test.go && git diff --check`:
  no diff-check output; resolved file staged.
- `go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/`:
  `ok github.com/adams-shaun/gorge/internal/testutil 0.006s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  `ok github.com/adams-shaun/gorge/rules 0.762s`.

## Issues

No new unfixed issue was found while resolving the merge. The incoming changes
were integrated without additional conflict; no engine code was changed as
part of conflict resolution.
