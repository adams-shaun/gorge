# mrg1 conflict resolution report

## Conflict

Only `internal/testutil/agentsdoc_test.go` was conflicted. The branch side had `knownApproximationRows = 34` with a note that it was measured after rebase. Main's side said its auto-merged `AGENTS.md` had 32 rows, including several main-side closures, plus the branch's `(choosesource1)` deletion, and set the constant to 32. I retained the merged table count but measured the actual merged `AGENTS.md`: 31 approximation data rows. The conflict resolution therefore sets `knownApproximationRows = 31`; this retains main's table changes and the branch's row deletion while accurately matching the merged table.

The post-merge ratchet also exposed unclassified Goad static parameter-recognition reads in `rules/paramcensus_test.go`. I classified the helper's parsed SVar map as non-card Params and its `saMentionsGoaded` range as recognition rather than consumption, with comments. This was needed for the required post-merge ratchets to pass.

## Commands and output

- `git status --short --branch && git rev-parse --show-toplevel && git log -1 --oneline --decorate && git status`
  ```
  ## wt/cli-20260922T225142Z-885d3d75
  /home/sadams/projects/gorge/.worktrees/cli-20260922T225142Z-885d3d75
  e0f7658d3 (HEAD -> wt/cli-20260922T225142Z-885d3d75) fix(rules): preserve static goad in trigger LKI
  On branch wt/cli-20260922T225142Z-885d3d75
  nothing to commit, working tree clean
  ```
- `git merge --no-edit main`
  ```
  Auto-merging AGENTS.md
  Auto-merging effects/filter.go
  Auto-merging effects/misc.go
  Auto-merging effects/registry.go
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Auto-merging rules/cast.go
  Auto-merging rules/engine.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- `python3` count of `AGENTS.md` approximation rows: `approximation data rows: 31`.
- `ls .cards | head`: corpus present (`cards.lock`, `cardsfolder`, `ir.gob.gz`, `ir.v4.gob.gz`, `tokenscripts`).
- `go test ./internal/testutil/ -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`
  ```
  ok   github.com/adams-shaun/gorge/internal/testutil  0.001s
  ```
- First required ratchet run, `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`, failed in `TestEveryRepoDeckParamsAreRead`: seven findings for the new Goad helpers' `params` map and `saMentionsGoaded` range. I added the corresponding explicit census classifications.
- Rerun of that same required ratchet command after classification:
  ```
  ok   github.com/adams-shaun/gorge/rules  0.808s
  ```

## Issues

No engine behavior defect found during this integration. The only issue uncovered was the param-census classification noted above; it is fixed here. No additional CR-lane test issue identified.

## Completion

The merged approximation count is 31, not either stale conflict-side count. No uncertainty remains. The merge commit includes the conflict resolution and required ratchet classification.
