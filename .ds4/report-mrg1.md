# Merge conflict resolution: mrg1 round 2 (branch wt/cli-20260922T225138Z-c4106938, main at 84cb68b9)

## Starting state

The worktree was CLEAN, HEAD = `f14b56a3` (round 1's merge of main@e53c80a2,
status DONE), with no rebase/merge in flight — the daemon's failed rebase had
been rolled back, so the conflict dispatch was against a state that no longer
existed. `main` had since advanced by one ticket: `0679b1cd` (the
withForetell/withoutForetell + Cosmos Charger + effect-delivered MayPlay
closure, merged to main via `98dcf594`/`84cb68b9` of the sibling ticket
cli-20260922T225141Z-8d166e9c).

Rebase is forbidden in this worktree (shared-`git` seat rule), and merge is
the established integration shape here, so the integration was done as
`git merge main`.

## Conflicted files and resolution

1. **`internal/testutil/agentsdoc_test.go`** — one conflict hunk: this branch's
   explanatory comment above `knownApproximationRows` (main never had it; both
   sides set the constant to 74). Resolved to **73**, the merged table's
   measured data-row count (74 on each side; the merge deletes main's
   foretell row — closed in `0679b1cd` — on top of this branch's
   stack-option-kind closure). Comment updated to state the new measurement.
   Never raised. `knownOversizeRows` stayed 8 on both sides (merged table
   measures 6 oversize rows — shrinkage only). gofmt clean.
2. **`.ds4/report-mrg1.md`** — this tracked report file carried each side's
   prior-round report (ours from round 1, main's from the sibling ticket).
   Replaced with THIS round's report per the report-path contract.
3. **`AGENTS.md`** — auto-merged with NO textual conflict. Verified the merged
   table with the test's own parsing algorithm: **73 data rows**, both closed
   rows absent (this branch's "A spell on the stack is offered with
   `Option.Kind` \"permanent\"…" row and main's foretell row), and
   `git diff main -- AGENTS.md` shows exactly this branch's one row deletion.

All other main-side changes (`effects/filter.go`, `effects/misc.go`,
`rules/legal.go`, `rules/mayplay.go`, `rules/playerkeywords.go`,
`state/continuous.go`, `rules/foretell_grant_test.go`,
`rules/paramcensus_test.go`) auto-merged and were retained unmodified — this
branch never touched those files.

## Ratchet cross-check after the merge

The brief's ratchet command list plus the agentsdoc ratchet itself. No ratchet
table needed fixing: this branch registers no new trigger `Mode$` matcher, and
main's foretell closure already removed its own `knownUnsupportedParams`
entries together with its AGENTS.md row; the merged tree's `knownUnsupported`
/ `knownUnsupportedParams` / `knownUnmodelledCountHeads` tables are otherwise
untouched by either side.

## Commands run and output

```
git merge main
  -> AGENTS.md auto-merged; .ds4/report-mrg1.md CONFLICT;
     internal/testutil/agentsdoc_test.go CONFLICT
python3 (the test's own parsing algorithm) on merged AGENTS.md
  -> data rows: 73; oversize: 6

go test ./internal/testutil -run 'TestKnownApproximation' 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/internal/testutil
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/rules
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/cmd/botbench
go test ./internal/archtest/ 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/internal/archtest
gofmt -l internal/testutil/agentsdoc_test.go
  -> (no output, clean)
```

## Result

Merge commit with the default merge message; working tree clean after.

## Issues

None found beyond the resolved conflict itself.

## Earlier report preserved before rebase (historical)

## Status and conflicted file

At task start, `git status` showed a clean worktree on `wt/cli-20260922T225138Z-ec432b8c`; there was no in-progress rebase or merge and no unmerged path. The reported conflict in `internal/testutil/agentsdoc_test.go` had already been resolved by an earlier integration run. The existing resolution is reflected in the committed branch history and the file has no conflict markers. The test constant is `knownApproximationRows = 75` in this current checkout.

The previous report in this file documented a conflict on another task branch and its resolution (constant 76); that does not describe this checkout. There was no additional resolution to make and no new commit to create without changing project files. Existing branch HEAD: `a44cb5f5 fix(effects): record random discard RNG choice on applied move`.

## Checks run

- `git status --short --branch`
  ```
  ## wt/cli-20260922T225138Z-ec432b8c
  ```
  Clean worktree (`git status` also reported “nothing to commit, working tree clean”).
- `go test ./internal/testutil/ -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`
  ```
  ok   github.com/adams-shaun/gorge/internal/testutil 0.002s
  ```
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  ```
  ok   github.com/adams-shaun/gorge/rules 0.736s
  ```
- `.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`.

No unresolved merge conflict remains. Note: the issue history records a prior full module gate failure in `TestCR704NoLifeSBAInsideSmallpoxDiscard`; this merge-resolution pass did not rerun or address that unrelated engine failure.
