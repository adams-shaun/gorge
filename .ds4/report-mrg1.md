# Merge-conflict resolution — task cli-20260923T060000Z-choose-number

## Entry state and operation

The worktree was clean at `27c913888` before integration; no rebase or merge was in flight. `main` had advanced beyond the branch's earlier merge, so I ran `git merge main`. It stopped on content conflicts in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`.

## Conflicts and resolution

- `internal/testutil/agentsdoc_test.go`: the branch count/comment reflected 25 rows at an earlier main tip; current main's side reflected 23. Kept both sides' row deletions, measured the auto-merged `AGENTS.md` table using the test's counting rule (22 data rows), and set `knownApproximationRows = 22`. The comment records the ct1 closure and main's landed closures.
- `.ds4/report-mrg1.md`: the branch contained an earlier report for this choose-number task; main's version was a report for an unrelated ticket/worktree. Replaced both conflict sides with this report of the current integration.
- `AGENTS.md`, `rules/cast.go`, and other files from main auto-merged; retained those changes. No unrelated conflict resolution edits were made.

## Commands and results

- `git status --short --branch && git status` — initially clean on `wt/cli-20260923T060000Z-choose-number`.
- `git merge main` — conflicts in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`; other changes auto-merged.
- Python count of the merged `AGENTS.md` Known approximations table — 22 data rows.
- `ls .cards | head` — corpus present (`cards.lock`, `cardsfolder`, `ir.gob.gz`, `ir.v4.gob.gz`, `tokenscripts`).
- `go test -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestChosenNumber|TestKnownApproximation' ./rules ./internal/testutil`:
  ```
  ok   github.com/adams-shaun/gorge/rules 0.820s
  ok   github.com/adams-shaun/gorge/internal/testutil 0.002s
  ```
- `git add internal/testutil/agentsdoc_test.go && git add -f .ds4/report-mrg1.md && git diff --check && git diff --cached --check` — passed. (`.ds4` is ignored, so the report required `git add -f`.)
- `GIT_EDITOR=true git merge --continue` — completed as `6bd4bf07` (`Merge branch 'main' into wt/cli-20260923T060000Z-choose-number`).
- `git status --short --branch` — clean; `git merge-base --is-ancestor main HEAD` — passed (`main_ancestor=0`).

## Issues / uncertainty

No uncertainty remains. The current merge includes main's ratchets and the ChooseNumber row deletion; the merged row count matches the ratchet constant.
