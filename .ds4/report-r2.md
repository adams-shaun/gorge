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
Count$ResolvedThisTurn), so the merge refused to touch the file. Resolution:

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
  concurrent worktrees write their own. Keep historical reports under
  ticket-unique paths (such as `report-t1-mill-unless.md`) while leaving
  main's unrelated reports untouched. Worth a controller-level rule so
  future rounds are dispatched with unique report paths up front.
- No engine defect was found this round; the merge introduced no conflict and
  no behaviour movement (botbench split unchanged).

---

# Report — r2 (agent-20260918T230554Z-74976c7c) — kw:Backup fix round

Ticket: kw:Backup (CR 702.70). The implementation landed in round t1
(`7caad9fb feat(rules): implement the Backup keyword (CR 702.70)` on the
rebased branch). `findings-r2.md` carried exactly one MAJOR, about the report
commit, not the code; this round resolves it and re-verifies everything on
the rebased tree.

## The MAJOR, and what changed

**[MAJOR] `.ds4/report-t1.md` replaced a 1,156-line accumulated report with
this ticket's 152-line report.** Resolved by rebase, not by hand-editing
history: the controller directive required `git rebase main` before
continuing, and main had itself grown the accumulated file (1,645 lines, 15
report sections, including the Count$ResolvedThisTurn report). The rebase
conflicted on exactly this file; the resolution took **main's full 1,645
lines untouched** and **prepended this ticket's 152-line report** (newest-first,
matching the file's existing convention). The new docs commit is a pure
addition:

```
$ git show --stat HEAD
 .ds4/report-t1.md | 152 ++++++++++++++++++++++++++++++++++++++++++++++++++++++
 1 file changed, 152 insertions(+)
```

No prior report content was removed; the accumulated file is now 1,797 lines
with all 15 prior sections plus this ticket's on top. The deletion class is
structurally prevented for this branch: the docs commit no longer rewrites
main's tracked file at all.

## Gates run on the rebased tree (real output)

Rebase: `git rebase main` — one conflict (`.ds4/report-t1.md`, resolved as
above), implementation commit `e6c716e1`→`7caad9fb` replayed clean (main's
Vanishing merge touched no overlapping code).

`.cards` present (symlink to `/home/sadams/projects/gorge/.cards`, verified
before running).

```
$ go build ./... && go test -run 'TestGuardianScalelordBackup' ./rules/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/rules	0.620s

$ go test ./internal/archtest/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/internal/archtest	3.287s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.204s
```

The 0.62s rules run exercised the real corpus (`.cards` present; the test
builds a real-corpus Guardian Scalelord table). The reviewer's own break
attempts in `findings-r2.md` (Memnite other-target, Scalelord self-target,
granted AttackTrig rider, targeted command) all held — no code change was
needed this round, so the round-1 "Fails without the fix" proof carries over
unchanged; the tests were not touched.

## Fails without the fix

No fix-round code change; the only fix is the report restructure above. Its
"failing before" state is exactly the findings MAJOR: the pre-rebase commit
`56479b6a` deleted 1,134 lines of prior reports (visible in its stat:
`130 insertions(+), 1134 deletions(-)`); the rebased commit `c8b97fb0`
inserts 152 and deletes 0.

## Issues

- **Process, recurring class:** the controller dispatches multiple tickets'
  round reports at the same tracked paths (`.ds4/report-t1.md`,
  `.ds4/report-r2.md`). This is the second ticket to collide on
  `report-t1.md` in two days (the Deep Spawn Mill r2 round hit the same
  file). Suggest controller-level unique report paths per ticket
  (`report-<ticket-slug>-r<N>.md`) so no round is ever asked to touch another
  ticket's tracked report. (Same note as the Deep Spawn r2 report below; not
  fixed here — it is controller policy, not engine code.)
- No engine defect found this round. The frozen "Known approximations" table
  was not touched (kw:Backup was never a row there).
