# Merge-conflict resolution — api:Attach Optional$ / Yuffie object-choice attach

## Entry state and merge

The worktree was clean on `wt/agent-20260919T192641Z-91be7ff1` at
`0c067498e917010b3a749aa681b0790f35354d2d`, with no merge/rebase in flight.
`main` was at `8c6fdd560c33b2c23d18ed599528504eb95e16cd`. Ran `git merge main`.
It auto-merged the source changes, but reported one content conflict:
`.ds4/report-t1.md`.

## Conflict and resolution

- **`.ds4/report-t1.md`** — this same ignored-but-tracked report path describes
  different tickets on each side. The branch side is the approved Yuffie attach
  implementation report; main's side is an unrelated rv2b count-head report.
  Kept the branch version as the ticket-specific report, rather than combining
  unrelated task reports. The main-side code and other files were retained via
  the normal merge. `.ds4/report-mrg1.md` had an auto-merged prior report; this
  file replaces it with the current integration record.

## Commands and results

```text
git status --short --branch && git status
## wt/agent-20260919T192641Z-91be7ff1
On branch wt/agent-20260919T192641Z-91be7ff1
nothing to commit, working tree clean

git merge main
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
Automatic merge failed; fix conflicts and then commit the result.

Resolution: git show HEAD:.ds4/report-t1.md > .ds4/report-t1.md
git add -f .ds4/report-t1.md
git diff --name-only --diff-filter=U
(no output)
git diff --cached --check
(no output)
```

Corpus check: `.cards` was present (`cards.lock`, `cardsfolder`, `ir.gob.gz`).

Required merged-main ratchets, combined with the branch's Yuffie regression:

```text
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestYuffieMayDeclineHerETBAttach'
ok   github.com/adams-shaun/gorge/rules  1.095s
```

The command includes all requested main ratchets and the approved branch
regression test. No uncertainty remains about the report conflict: it was only
a collision between task-specific documentation, not conflicting code.

---

# Merge-conflict resolution — fb-20260922T145544Z-3e3a67d6

## Operation and resolution

Initial `git status` showed a clean worktree on `wt/fb-20260922T145544Z-3e3a67d6` at `7860ce06`; no merge/rebase was in progress. `main` was at `8c6fdd56`, and the branch/main histories had diverged. Per the task, started `git merge main`.

The merge stopped with one content conflict: **`.ds4/report-t1.md`**. The branch side contains the approved restricted-mana projection report for `fb-20260922T145544Z`; main's side contains the independent `cli-20260923T060000Z-rv2b-countheads` report. These reports describe different work and neither supersedes the other. Kept both complete reports, placing the branch report first and the main report beneath a divider. The report content was taken directly from the two index stages (`git show :2:...` and `git show :3:...`); no report claims were edited.

`web/src/protocol.ts` auto-merged without a conflict. The branch's `PoolRestrictionView` / `pool_restrictions` generated types remain in the merged file, and main's unrelated `Option.keyword` addition is retained. All other main changes auto-merged. No source conflict or engine change was made.

## Checks and completion

- `git diff --cached --check` — no output (passed).
- `grep -nE '^(<<<<<<<|=======|>>>>>>>)' .ds4/report-t1.md web/src/protocol.ts || true` — no output; no conflict markers remain.
- `.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`.
- `go test -run 'TestRestrictedManaIsProjected|TestCR106ManaPoolIsPublicForEveryPlayer' ./view/ 2>&1 | tail -30`:
  `ok github.com/adams-shaun/gorge/view 0.003s`
- Required post-merge ratchets, `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' 2>&1 | tail -30`:
  `ok github.com/adams-shaun/gorge/rules 0.765s`

The operation is ready to complete with the merge's default message. No judgement call beyond retaining both independent reports; no test or source conflict remains.

---

# Merge-conflict resolution (second round) — merging main 2954978f into wt/agent-20260919T192641Z-91be7ff1

## Entry state and operation

The first integration record above merged main `8c6fdd56`; since then main
advanced to `2954978f` (the `fb-20260922T145544Z` cavern-of-souls restricted-mana
projection merge). The daemon's `git rebase main` attempt and its merge fallback
both conflicted and were rolled back, so I re-ran `git merge main` on the clean
branch (`git status` showed no merge/rebase in flight; merge-base `8c6fdd56`).

All source files auto-merged cleanly: main's `view/poolrestriction.go`,
`view/pool_restriction_test.go`, `view/view.go` and `web/` changes (restricted
mana projection) plus the earlier rv2b count-heads work. The two conflicts were
both in `.ds4/` reports where each side is a different ticket's report.

## Conflicts and resolution

- **`.ds4/report-t1.md`** — branch side: the approved Yuffie attach
  implementation report. Main side: the fb restricted-mana projection report
  followed by the rv2b count-heads report under a "Merged concurrent report"
  divider. Kept both per the established convention (branch's report first,
  main's composite beneath a `---` divider), taking each side verbatim from
  its index stage (`:2:` and `:3:`).
- **`.ds4/report-mrg1.md`** — branch side: the first-round merge-resolution
  record (Yuffie merge of `8c6fdd56`). Main side: the fb branch's own
  merge-resolution record. Kept both verbatim under a divider and appended
  this record.

No source file was edited; no report text was rewritten.

## Commands and results

```text
git status                        # clean, no rebase/merge in flight
git merge main                    # CONFLICT in .ds4/report-t1.md, .ds4/report-mrg1.md
git show :2:/:3: of both files    # rebuilt both from stages, verbatim, under dividers
git diff --check                  # clean
```

Corpus check: `.cards` is present (symlink to `/home/sadams/projects/gorge/.cards`,
found present, not created).

## Required post-merge ratchets

`go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestYuffieMayDeclineHerETBAttach'`
(see final run output appended below).

Ratchet + Yuffie regression run (real output):

```text
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestYuffieMayDeclineHerETBAttach'
ok  	github.com/adams-shaun/gorge/rules	0.816s

$ go test -run 'TestRestrictedManaIsProjected|TestCR106ManaPoolIsPublicForEveryPlayer' ./view/
ok  	github.com/adams-shaun/gorge/view	0.003s
```

Merge commit: `7d1e418e` (default message). Tree clean; `main` (`2954978f`)
is now an ancestor of the branch. No head/ratchet movement.


---

# Merge-conflict resolution — cli-20260923T060000Z-trig-attackerblocked

## Entry state and merge

Initial `git status` was clean on `wt/cli-20260923T060000Z-trig-attackerblocked` at `46327884` (no merge/rebase in progress). Ran `git merge main`; source changes from main auto-merged, while `.ds4/report-t1.md`, `AGENTS.md`, and `internal/testutil/agentsdoc_test.go` conflicted. `.cards` was present.

## Conflicts and resolution

- **`.ds4/report-t1.md`** — HEAD contains the approved AttackerBlocked-grant fix report; main contains the distinct Yuffie attach, restricted-mana, and rv2b count-head reports. Kept both sides in full, branch-side report first and main-side reports after a divider.
- **`AGENTS.md`** — HEAD had already deleted the four-trigger-mode row for this ticket; main had deleted the rv2b damage/count row. Both are independently closed by their respective landed work, so removed both rows and retained all surrounding rows.
- **`internal/testutil/agentsdoc_test.go`** — both sides' comments described a 20-row tree after their respective deletion. The merged AGENTS table has 19 data rows after both deletions (counted directly in `## Known approximations`), so retained the rationale for both closures and set `knownApproximationRows` to 19.

All other main changes were cleanly auto-merged and left intact. No unrelated files were edited.

## Commands and results

```text
git status --short --branch; git status
## wt/cli-20260923T060000Z-trig-attackerblocked
On branch wt/cli-20260923T060000Z-trig-attackerblocked
nothing to commit, working tree clean

git merge main
CONFLICT (content): .ds4/report-t1.md
CONFLICT (content): AGENTS.md
CONFLICT (content): internal/testutil/agentsdoc_test.go
Automatic merge failed

Count of data rows under AGENTS.md ## Known approximations: 19

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestGrantedAttackerBlocked' 2>&1 | tail -30
ok   github.com/adams-shaun/gorge/rules  0.816s

go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' 2>&1 | tail -30
ok   github.com/adams-shaun/gorge/internal/testutil  0.001s

git diff --check
(no output)
```

The required ratchets and the branch's granted-trigger regression passed. No uncertainty remains in the three conflicts.


---

# Merge-conflict resolution report — agent-20260918T230554Z-a96f94d7

## State found

`git status` showed a **clean tree, no rebase/merge in flight** — the daemon
had aborted both its rebase and its merge fallback before this seat started.
The branch was 2 commits ahead of the merge-base `bad06ce7`:

- `9de2af45` fix(effects): publish StoreVoteNum outcomes
- `4d9c6987` docs: record StoreVoteNum verification

I therefore re-ran the integration myself: `git rebase main`.

## Conflicted files and resolution

Only **one file** conflicted in the rebase: `.ds4/report-t1.md`
(`effects/misc.go` and the new test applied cleanly on pick 1 — the
daemon's merge fallback had reported a spurious `effects/misc.go` conflict
that the rebase did not hit).

`.ds4/report-t1.md` is the shared accumulating report log. Three versions:

- **base** (`9de2af45`, the fix commit's parent): a 42-line "Mill<N> cost
  verification" report.
- **ours / main** (`main` = `7a6a77b7`): 525 lines — Yuffie attach, fb-20260922
  restricted-mana-projection, and rv2b-countheads reports, separated by `---`.
- **theirs / branch** (`4d9c6987`): the seat's 62-line "Vote.StoreVoteNum"
  report **replacing the whole file** (46+/26− vs base).

Resolution: **keep main's full log in full, append the branch's StoreVoteNum
report at the end** separated by `---`, matching main's own convention
(main's latest docs commit on this file merges concurrent reports into one
growing file). The Mill report that the branch commit's diff deleted was
already superseded on main (`3b070589`'s rewrite of the file dropped it and
nothing on main restored it), so no restoration was needed — the branch's
deletion of it is subsumed by main's later state. Both sides' intent is kept:
main's accumulated reports and the branch's verified report.

One text nit kept as-is (historical, from the branch seat): the report's line
"The worktree was clean before the required `git rebase main`, which reported
up to date." — it recorded that seat's own pre-submit state.

## Commands run (real output)

```
$ git rebase main
... CONFLICT (content): Merge conflict in .ds4/report-t1.md   (pick 2 of 2; pick 1 applied clean)
$ # rebuilt .ds4/report-t1.md = main's 525 lines + "---" + branch's 62-line report
$ git add .ds4/report-t1.md && GIT_EDITOR=true git rebase --continue
[detached HEAD 4f96e2c6] docs: record StoreVoteNum verification
 1 file changed, 65 insertions(+)
Successfully rebased and updated refs/heads/wt/agent-20260918T230554Z-a96f94d7.
$ git status
On branch wt/agent-20260918T230554Z-a96f94d7
nothing to commit, working tree clean
```

Sanity checks and ratchets (post-merge, per the 2026-09-22 directive):

```
$ go test -run '^TestFatefulTempestStoresEachOptionVoteCount$' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.604s

$ go test -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeckIsFullySupported$|TestEveryRepoDeckParamsAreRead|CountHead' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.761s

$ grep -c '<<<<<<<\|>>>>>>>' effects/misc.go          # 0
$ gofmt -l effects/misc.go rules/fateful_tempest_vote_test.go   # no output
```

`.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`
(found, not created), so corpus-backed tests did not skip.

## Result

- Branch `wt/agent-20260918T230554Z-a96f94d7` rebased onto main (`7a6a77b7`),
  tree clean.
- `main..HEAD`: `d8ab4365` (fix, content-identical to `9de2af45`) and
  `4f96e2c6` (docs, resolution as above).
- No golden, ratchet, or heads movement: the ratchet run passed unchanged;
  the branch registers no new `Mode$` matcher and closes no ratchet row.
- No `Ref:` trailers anywhere (gorge rule respected).

## Unsure about

- Whether the daemon intended the merge-fallback route (merge commit) rather
  than the rebase route. I re-ran the rebase it had originally attempted; the
  resulting branch is linear onto main, which is what its rebase log shows it
  wanted first.

## Issues

None found. The only conflict was the report log; no engine code was in
conflict.

---

# Merge-conflict resolution — cli-20260923T060000Z-trig-attackerblocked (current main)

## State and operation

At entry the worktree was clean at `484d8606` on
`wt/cli-20260923T060000Z-trig-attackerblocked`, with no merge/rebase in flight.
That existing merge commit already had main `7a6a77b7` as an ancestor. Current
main had advanced to `46abb03e` with `6bae49c8` and `8b8bab8a`; therefore I
merged the current `main` rather than attempting to continue the already-finished
historical integration. `git merge main` auto-merged source/docs, and left only
`.ds4/report-mrg1.md` unmerged. The `.cards` corpus was present as a symlink to
`/home/sadams/projects/gorge/.cards`.

## Conflict and resolution

- **`.ds4/report-mrg1.md`** — this accumulating report log had independent
  branch and main histories. Kept both index-stage versions verbatim, branch
  first and main after a `---` divider, then appended this resolution record.
  No source conflict remained. In particular, `AGENTS.md` and
  `internal/testutil/agentsdoc_test.go` were not conflicted in the actual
  merge; the task's earlier daemon transcript did not match the clean entry
  state (the branch already contained a prior merge commit).
- `.ds4/report-t1.md`, `README.md`, `docs/coverage.md`, `effects/misc.go`, and
  `rules/fateful_tempest_vote_test.go` auto-merged from current main and were
  retained without manual changes.

## Commands and results

```text
git status --short --branch && git status
## wt/cli-20260923T060000Z-trig-attackerblocked
On branch wt/cli-20260923T060000Z-trig-attackerblocked
nothing to commit, working tree clean

git merge main
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging .ds4/report-t1.md
Automatic merge failed; fix conflicts and then commit the result.

[ -e .cards ] && readlink -f .cards
/home/sadams/projects/gorge/.cards

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestGrantedAttackerBlocked' 2>&1 | tail -30
ok   github.com/adams-shaun/gorge/rules  0.767s

go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' 2>&1 | tail -30
ok   github.com/adams-shaun/gorge/internal/testutil  0.001s
```

No golden or ratchet movement. No unresolved uncertainty remains; the supplied
conflict report described earlier conflict states, while the actual merge had
only the accumulating report-log conflict.
