# Merge-conflict resolution report — mrg1 (agent-20260919T062939Z-4b5f8950)

## Entry state and operation

`git status` showed a clean worktree on `wt/agent-20260919T062939Z-4b5f8950` at `6da89ffd`; no merge or rebase was in flight. The pre-existing `.ds4/report-mrg1.md` in this worktree was a stale copy from a *different* ticket's resolver run (`agent-20260922T210645Z-27e19c88`); it was overwritten by this report. Per the dispatch, ran `git merge main`.

`.cards` was present (symlink to `/home/sadams/projects/gorge/.cards`) — corpus-backed tests ran, not skipped.

## Conflicted files

Two paths, both `.ds4` documentation (no production source conflicged):

### `.ds4/report-t1.md`

- **Branch side (`84d276a8`)**: REPLACED the 1,797-line accumulated report file with this ticket's 167-line RepeatOptional premise-false report — a destructive rewrite (124 insertions, 1,754 deletions).
- **Main side (`379338d2`)**: kept the entire accumulated history and prepended 197 lines (the DestroyAll.Zone report and the RevealAllValid$ report).
- **Resolution**: concatenated — the branch's RepeatOptional t1 report at the top, then main's full 1,994-line content verbatim below. This preserves main's intent (the complete accumulated history, verified: all 13 `# Report` headings incl. the base's kw:Backup, stat:CountersRemain, TriggerController$, Attach Optional$, PlayerCountPropertyYou, fb-20260922T145544Z, rv2b-countheads, Vote.StoreVoteNum, NonRememberedController selectors reports) AND the branch's intent (this ticket's report). The base's kw:Backup report survives via main's copy, which is a superset of the base (measured `diff` base→main: prepend-only, `0a1,197`).

### `.ds4/report-t2.md`

- **Branch side (`6da89ffd`)**: REPLACED the base's 67-line player-count sacrifice-attribution report with this ticket's 155-line round-t2 RepeatOptional verification report (also destructive; 136 insertions, 48 deletions).
- **Main side (`aa0a7be2`)**: its DestroyAll.Zone fix-round-2 report (which itself restored main's report-t1.md) prepended, with the MustBlock verification report preserved below — 321 lines. Neither side contains the other's content; both independently diverged from the base.
- **Resolution**: concatenated — the branch's round-t2 report at the top, then main's full content verbatim below (all 3 `# Report` headings present: round-t2, DestroyAll.Zone fix round 2, MustBlock). No report on either side was dropped.

No code, test, table, or golden file was touched by this resolution. `grep -cE '^(<<<<<<<|=======|>>>>>>>)'` on both resolved files → 0.

## Completing the operation

`git add -f .ds4/report-t1.md .ds4/report-t2.md` (the `-f` is needed because `.ds4` is in `.git/info/exclude` even though these files are tracked), then `git commit --no-edit` (default merge message):

```
[wt/agent-20260919T062939Z-4b5f8950 8c986e8e] Merge branch 'main' into wt/agent-20260919T062939Z-4b5f8950
```

`git status` after: clean (`nothing to commit, working tree clean` on the full form).

## Ratchets after merging main

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.790s
```

The branch registers no new `Mode$` matcher and closes no `knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads` entry (the branch's only code commit `489716a6` predates the merge base), so no ratchet table adjustment was needed.

## Targeted sanity checks

```
$ go test -run 'TestAdNauseamOptionalRepeat|TestRepeatEachOptional' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.623s

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.220s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.591s
```

No head/ratchet movement: the merge is docs-only in its resolution and the merged engine behaviour is main's, which the daemon's gates have already validated.

## Issues

- **Bookkeeping, not code**: this ticket's own seat reports (the RepeatOptional premise-false finding) document that the ticket was already implemented on main before dispatch — the ticket should be closed as superseded/duplicate by the controller; the duplicate ledger entry needs the controller (the ledger is derived).
- **Recurring class of defect worth a ticket**: seat reports that REPLACE `.ds4/report-*.md` wholesale (as `84d276a8` and `6da89ffd` did) destroy accumulated report history and cause exactly these merge conflicts; main's fix-round convention ("restore accumulated history, prepend narrowly", commit `aa0a7be2`) is the remedy. A CR-lane test does not apply (not engine-visible); a dispatch-time instruction or a pre-commit doc check would prevent it.

STATUS=DONE
COMMITS=8c986e8e
TESTS=go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' → ok; TestAdNauseamOptionalRepeat|TestRepeatEachOptional ./rules/ → ok; archtest → ok; botbench TestConstructedDefaultIsByteIdentical → ok
