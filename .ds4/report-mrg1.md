# Round record — merge resolver, worktree cli-20260923T060000Z-pw-numloyaltyact, 2026-09-23

## Entry state

`git status` found the tree CLEAN at the branch's completed earlier merge
`33f1af28` (fix `c9938967` reads Effect-delivered `NumLoyaltyAct` in
`loyaltyAbilityLimit` + that merge); no rebase or merge in flight. `main` had
since advanced 12 commits to `c5669fdf` (choose-number, ct1 closure lineage),
so this round ran the integration itself: `git merge main --no-edit`.

## Conflicts and resolution

Auto-merged: `AGENTS.md` (this branch's `(pw1)` deletion and main's landed
closures compose; 0 `(pw1)` occurrences remain), `rules/cast.go`,
`effects/choose.go`, `rules/resolution.go` and the rest. ONE content conflict:

- **`internal/testutil/agentsdoc_test.go`** — the `knownApproximationRows`
  constant and comment. Merge base `2acd1d4d` measured **23** data rows; the
  branch deleted `(pw1)` (→22), main deleted `(ct1)` (→22) — both constants
  stale for the merge. The merged `AGENTS.md` measures **21** data rows
  (verified with `approximationRows()`'s own counting rule). Resolution:
  `knownApproximationRows = 21` with a comment naming both disjoint closures.

The branch's fix survived the auto-merge: `NumLoyaltyAct` reads remain in
`rules/legal.go` (`loyaltyAbilityLimit` static accumulator) and
`effects/misc.go` (`effEffect` + `NumLoyaltyActParamsReadable`).

## Commands and output

```text
git status                              # clean at 33f1af28, nothing in flight
git rev-list --count HEAD..main         # 12
git merge main --no-edit
  Auto-merging AGENTS.md / internal/testutil/agentsdoc_test.go ...
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
row counts: base 2acd1d4d = 23, branch 33f1af28 = 22 (pw1 gone),
            main c5669fdf = 22 (ct1 gone), merged AGENTS.md = 21
.gcards check: .cards is the real symlink to /home/sadams/projects/gorge/.cards
gofmt -l internal/testutil/agentsdoc_test.go   # clean
go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' -v
  --- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
  --- PASS: TestKnownApproximationRowsAreShort (0.00s)
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|NumLoyaltyAct|Loyalty' -v
  46 PASS, 0 FAIL, 0 SKIP (incl. the trigger-mode registry ratchets,
  TestEveryRepoDeckIsFullySupported / TestEveryRepoDeckParamsAreRead, the
  CountHead ratchet and the loyalty/NumLoyaltyAct suite) — ok rules 0.804s
git commit --no-edit -> 02b39858 Merge branch 'main' into wt/cli-20260923T060000Z-pw-numloyaltyact
git status -> clean; main (c5669fdf) is an ancestor of the branch
```

## Issues

None new. Integration only; the merged state closes `(pw1)` (this branch) plus
main-side `(ct1)` and its lineage's closures, measured at 21 data rows.

---

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
