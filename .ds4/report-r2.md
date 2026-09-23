# Report — r2 (agent-20260923T002719Z-c0b56143) — merge resolution

Ticket: `UnlessCost$ Mill<2>` is a hard decline (Deep Spawn always
sacrificed). The **implementation itself was completed in round t1** — commit
`01c62856 fix(rules): settle fixed Mill<N> in the mid-resolution UnlessCost
path` — and its full verification report is preserved in this worktree at
`.ds4/report-t1-mill-unless.md` (previously uncommitted at
`.ds4/report-t1.md`).

## What this round did

`findings-r2.md` reported that the rebase/merge onto main failed:

```
error: cannot rebase: You have unstaged changes.
--- merge fallback ---
error: Your local changes to the following files would be overwritten by merge:
	.ds4/report-t1.md
```

Root cause: round t1 wrote its report at the SHARED path `.ds4/report-t1.md`
and left it uncommitted. Main meanwhile tracks `.ds4/report-t1.md` with a
different ticket's report (`fb-20260923T005857Z-c1a24352`,
Count$ResolvedThisTurn), so the merge refused to touch the file. Resolution,
following the precedent the split-cast round set (see the old r2 report,
preserved here as `.ds4/report-r2-split-cast.md`):

1. Moved the t1 report to the unique path `.ds4/report-t1-mill-unless.md`
   and restored `.ds4/report-t1.md` to its tracked content
   (`git restore` — no branch switch, no shared-state change), then
   committed the new file (`83d640d5`). Main's tracked `report-t1.md`
   is untouched by this branch, so the merge takes main's version cleanly.
2. Merged main (`c947f5c8`) into the branch → merge commit `4807a97c`,
   **clean, no conflicts**. Main's `rules/mana.go` change (the RollDice
   `ParseCost` token) is in a different function than this ticket's
   `ParseUnlessCost` hunk; main's `rules/stack.go` change (`offeredTargetSA`,
   line ~2970) is far from `payUnlessCost`; `rules/unless_unpriceable_test.go`
   was not modified on main. Verified overlap before merging by diffing
   `base..main` against my commit's hunks.
3. Re-ran the brief's gates on the MERGED tree (output below) — all green.

## Gates run on the merged tree (real output)

```
$ go test -run 'TestDeepSpawnUnlessMillCost|TestUnlessCostStrictParsePopulation' ./rules/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/rules	0.686s

$ go test ./internal/archtest/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/internal/archtest	3.965s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.259s
```

`.cards` was present (symlink to `/home/sadams/projects/gorge/.cards`); the
0.69s Deep Spawn test ran the corpus, not a skip.

## Status of the brief

The brief's "Done means" items were all satisfied and verified in round t1
(see `.ds4/report-t1-mill-unless.md` for the per-item evidence, including the
"## Fails without the fix" failing-output paste: parser boundary tests, the
real-corpus end-to-end Pay path, the short/empty-library CR 701.13a halves,
the population-table update, the revert-and-restore byte-identity proof).
This round re-verified the whole set still holds after the merge with main.
The ratchet tables (`knownUnsupported`, `knownUnsupportedParams`,
`knownUnmodelledCountHeads`) and `TestHeads` are daemon gates, not seat
gates; nothing in this ticket's change or the merge is expected to move them,
and the two ~2 s behaviour goldens both pass on the merged tree.

## Issues

- **Process, not code:** shared report filenames (`.ds4/report-t1.md`,
  `.ds4/report-r2.md`) collide across tickets because main tracks them while
  concurrent worktrees write their own. The durable convention this and the
  split-cast round both landed on: every report goes under a ticket-unique
  path (`report-t1-mill-unless.md`, `report-r2-split-cast.md`); the
  dispatched shared path holds the CURRENT round's report only. Worth a
  controller-level rule so future rounds are dispatched with unique report
  paths up front.
- No engine defect was found this round; the merge introduced no conflict and
  no behaviour movement (botbench split unchanged).
