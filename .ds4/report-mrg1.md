# Merge-conflict resolution — task cli-20260923T060000Z-choose-number

## State and operation

The initial `git status --short --branch` was clean on `wt/cli-20260923T060000Z-choose-number`, with HEAD `ecd0c313d` (`test(rules): answer Void number ask in unresolved-count regression`). No rebase or merge was in flight. The branch already contained earlier main merges, but current `main` had advanced. I ran `git merge main` to integrate the current main tip; it stopped on conflicts in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`.

## Conflicted files and resolution

### `internal/testutil/agentsdoc_test.go`

- The branch side recorded the `ct1` row deletion for ChooseType/ChooseColor/ChooseNumber and had the row-count constant at 27 for its then-current merged table.
- Current main included additional landed approximation-row deletions, including `(kw:Flanking)` and `(castfilter1/2)`, and its side recorded a table count of 26. The two constants/comments described different merged bases, so neither value could simply be kept.
- Kept both sides' deletions. Measured the auto-merged `AGENTS.md` table with the test's row-count algorithm: 25 data rows. Updated the explanatory history to include the `mtsp1`, `kw:Flanking`, `battle1`, `ct1`, and `castfilter1/2` deletions and set `knownApproximationRows = 25`.

### `.ds4/report-mrg1.md`

- The branch side contained the prior choose-number resolver's report; current main's copy was a report for an unrelated CTMS-refhead integration and described that other task's branch and tests.
- Retained the choose-number task context and replaced the stale report body with this resolution report, documenting the current merge and its actual verification. No engine implementation was changed to resolve this documentation conflict.

### Automatically merged files

`AGENTS.md`, `rules/cast.go`, and the other auto-merged paths were left as produced by Git. The `ct1` row remains deleted along with main's landed row deletions. No conflict markers remain.

## Commands and results

- `git status --short --branch; git rev-parse --show-toplevel; git log -1 --oneline --decorate` — initially clean; worktree root `/home/sadams/projects/gorge/.worktrees/cli-20260923T060000Z-choose-number`; HEAD `ecd0c313d`.
- `git merge main` — stopped with content conflicts in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`; `AGENTS.md` and `rules/cast.go` auto-merged.
- Measured merged `AGENTS.md` Known approximations table with Python — `data_rows 25`; `.cards` corpus was present (`cards.lock`, `cardsfolder`, `ir.gob.gz`, `ir.v4.gob.gz`, `tokenscripts`).
- `go test -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|CountHead|TestChosenNumber|TestKnownApproximation' ./rules ./internal/testutil`:
  ```
  ok   github.com/adams-shaun/gorge/rules 1.144s
  ok   github.com/adams-shaun/gorge/internal/testutil 0.005s
  ```

## Issues / uncertainty

No additional issue found. No uncertainty in the resolution: both sets of table deletions are present and the measured row count matches the updated ratchet constant.
