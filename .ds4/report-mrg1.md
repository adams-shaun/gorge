# Merge-conflict resolution — task cli-20260923T060000Z-rv2b-countheads

## Entry state and operation

The worktree was at `a62d152c` (the countheads fix) with one uncommitted
change: `.ds4/report-t1.md` (this task's own report, updated after the fix
commit). No rebase or merge was in flight. `main` had advanced past the merge
base `c5669fdf` (the choose-number merge), so I ran `git merge main`.

First merge attempt aborted: main touches `.ds4/report-t1.md` too, so the
uncommitted local report blocked the ort strategy. I committed the report
first (`3b070589` `docs: record rv2b-countheads task report`, `git add -f`
because `.ds4` is gitignored but these report files are tracked on this
branch), then re-ran the merge.

## Conflicted files and resolution

Three content conflicts; everything else auto-merged cleanly (main's
pw-numloyaltyact, cycled-trigger, hybrid-cost and vote-suspension work in
`rules/`, `effects/`, `events/`, `decision/`, `cards/`, `web/`).

1. **`AGENTS.md`** (Known approximations table, line ~221): the base table
   held BOTH the `(pw1)` and `(rv2b)` rows. The branch deleted `(rv2b)` (the
   countheads ticket drained its count-head half; the sibling damage-source
   and valid-players tickets had landed); main deleted `(pw1)` (the
   pw-numloyaltyact ticket). Both deletions are disjoint row closures of the
   same base row set, so I kept BOTH intents: the conflict block (which
   showed pw1 on HEAD, rv2b on main) resolves to NEITHER row. The merged
   table now carries no pw1 and no rv2b row.
2. **`internal/testutil/agentsdoc_test.go`**: both sides carried
   `knownApproximationRows = 21` (each against its own table with its own row
   still present). The merged table — 22 base data rows minus rv2b minus pw1
   — measures **21 counted lines** with `approximationRows()`'s rule (it
   counts the `| Stand-in | Where | …` header line too), i.e. **20 data
   rows**. Set `knownApproximationRows = 20` and merged the comment to name
   both closures (main's pw1, cli-20260923T060000Z-pw-numloyaltyact; this
   branch's rv2b, cli-20260923T060000Z-rv2b-countheads).
3. **`.ds4/report-t1.md`**: HEAD carried this worktree's countheads report;
   main carried an unrelated ticket's Mill-cost verification report committed
   over the same path. Kept the branch's (this task's) report
   (`git show :2:.ds4/report-t1.md > .ds4/report-t1.md`), per the same
   resolution the choose-number merge used for `report-mrg1.md`.

I also verified the two-sided intent directly: `git diff main -- AGENTS.md`
shows exactly one deletion (the rv2b row) relative to main — pw1 is already
gone on main, rv2b's deletion is the branch's reviewed closure. No
uninvolved file was touched.

## Operation completed

- `git add AGENTS.md internal/testutil/agentsdoc_test.go && git add -f
  .ds4/report-t1.md .ds4/report-mrg1.md`
- `git commit --no-edit` → merge commit **`fa9f2a30`**
  (`Merge branch 'main' into wt/cli-20260923T060000Z-rv2b-countheads`,
  default message).
- `git status --porcelain` → clean;
  `git merge-base --is-ancestor main HEAD` → pass.

## Commands run and output

- `go test ./internal/testutil -run 'TestKnownApproximation'`
  → `ok github.com/adams-shaun/gorge/internal/testutil 0.001s` (constant 20
  matches the merged table).
- `go test ./rules -run
  'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.786s` — the post-merge ratchet
  sweep the brief requires; the branch registers no new `Mode$` matcher and
  closes no further ratchet entry, and none of the ratchet tables moved on
  either side of the merge.
- `go test ./rules -run 'TestEveryRepoDeckIsFullySupported$' -v`
  → `--- PASS (0.66s)`, no SKIP — `.cards/` is the expected symlink to
  `/home/sadams/projects/gorge/.cards` (corpus present, so this was a real
  corpus-backed run, not a vacuous one).
- `go test ./effects -run 'TestRefProperty'`
  → `ok github.com/adams-shaun/gorge/effects 0.937s` — the branch's count-head
  fix survives the merge; main did not conflict with `effects/count.go`
  (`git diff main --stat -- effects/count.go`: the branch's +52/-4 intact).

## Issues

None new. No defect was found in either side's content; the only judgement
call was the report-t1.md ownership (branch's report kept, main's Mill
verification report remains in main's own history) and the row-count
arithmetic above, both measured rather than assumed.
