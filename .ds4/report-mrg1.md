# Merge-conflict resolution report — mrg1, round 2 (agent-20260923T073156Z-d6f8c32b)

## Why a second round

Round 1 (report below) integrated main `935cefc4` as merge `de376741`. Main
then moved to `0eb36fbd` (21 commits: the `RevealAllValid$` fix
`3a8dc712`/`582a564c`, `CopySpellAbility.Optional` `d8a987be`, four-mode
trigger row closures `2786ed95`, and their report/doc commits), the daemon's
rebase of this branch onto `0eb36fbd` failed at `0f9dcb58`
(`.ds4/report-sol1.md`) and its merge fallback conflicted on
`.ds4/report-mrg1.md` and `.ds4/report-sol1.md` before aborting. This seat
entered with a clean tree, nothing in flight, and performed a fresh
`git merge main` against `0eb36fbd`.

## Conflicted files and resolutions

Both conflicts are `.ds4` report accumulators; no source file conflicted.

**`.ds4/report-sol1.md`** — the first 231 lines are byte-identical on both
sides (`diff` of `head -231` of each stage → identical: Attached predicates,
Deep Spawn UnlessCost Mill, RollDice, Gitaxian Probe). Each side appended a
DIFFERENT new report at the same tail position: the branch its Cascade report
(lines 232–292 of `:2`), main its `CopySpellAbility.Optional` rebase report
(lines 232–286 of `:3`). Resolution: main's full 286-line version kept
byte-for-byte, the branch's Cascade section appended after a `---` divider
under the file's own pointer-line convention. 350 lines; both reports intact.

**`.ds4/report-mrg1.md`** — `:1` (base) = the 50-line 210645Z-era report blob
`72b5b4f8`; `:2` (ours) = that base with this branch's round-1 report (87
lines) on top; `:3` (main) = the 1618-line accumulated mrg1 history, which
already contains the base blob's content byte-for-byte (verified
`diff <(base) <(sed -n 1438,1487p :3)` → identical) and does NOT contain this
branch's round-1 report (`grep -c 073156Z :3` = 0). Resolution: main's full
1618-line file kept byte-for-byte, the branch's round-1 report appended (it
ends with its own `---` separator). 1706 lines; nothing from either side
dropped or rewritten.

No conflict markers remain in either file. No engine code was touched by the
resolution; all code paths on both sides auto-merged.

## Commands and output

```
$ git merge main -m "Merge branch 'main' into wt/agent-20260923T073156Z-d6f8c32b"
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging .ds4/report-sol1.md
CONFLICT (content): Merge conflict in .ds4/report-sol1.md
Automatic merge failed; fix conflicts and then commit the result.
$ git add -f .ds4/report-mrg1.md .ds4/report-sol1.md && git commit --no-edit
[wt/agent-20260923T073156Z-d6f8c32b fc027b1a] Merge branch 'main' into wt/agent-20260923T073156Z-d6f8c32b
$ git rev-parse HEAD^2        # -> 0eb36fbd (main tip fully integrated)
$ git status --short          # clean
```

Post-merge gates (`.cards` present as a symlink to the real corpus):

```
$ go test ./internal/archtest/                                    -> ok (5.924s)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ -> ok (1.213s)
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
                                                                  -> ok (0.782s)
$ go test -run 'TestCascadeFreeCastAnnouncesNoX|TestCascadeXSpellUsesAnnouncedManaValue' ./rules/
                                                                  -> ok (0.629s)
$ go test -run 'TestRevealAllValid|TestCopySpellAbilityOptional' ./effects/
                                                                  -> ok (0.592s)
$ go test -run 'TestKnownApproximation' ./internal/testutil/      -> ok
```

Ratchets green after the merge: no new trigger `Mode$` to register, no
`knownUnsupported` / `knownUnsupportedParams` / `knownUnmodelledCountHeads`
entry to remove (this branch registers none and closes none), the
approximations register and the botbench golden did not move, and the branch's
cascade fix still passes on the merged tree.

## Issues

No new defect found; the only conflicts were independent report content at a
shared accumulator tail. Same standing note as round 1: repeated re-dispatch
of mrg1 on this ticket is driven by main advancing between rounds (both rounds
lost no content — the accumulator union preserved every report).

STATUS=DONE
COMMITS=fc027b1a
TESTS=archtest ok; botbench TestConstructedDefaultIsByteIdentical ok; rules ratchets (TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead) ok; cascade tests ok; RevealAllValid/CopySpellAbilityOptional ok; agentsdoc ok

---
# Merge-conflict resolution report — mrg1 (task agent-20260920T074357Z-b9ac41c2)

## Outcome

The dispatch's conflict was on `.ds4/report-t1.md` only. It is resolved: main's
accumulated report file is kept intact and the branch's
`# Report — NonRememberedController selectors` section is appended once at the
end. The branch now carries the reviewed code fix plus its report commit on top
of main, and the tree is clean.

Final branch (on top of `main` = `93e84c28`):

```
196211dc fix(effects): resolve nonremembered controller selectors
da2334ea docs: update nonremembered selector report
<this commit> docs: record mrg1 merge-conflict resolution   (this report's own commit;
                                                          its SHA changes on amend)
```

`git diff --stat main..HEAD`:

```
 .ds4/report-mrg1.md                      | 232 +++++++++++++++++++++----------
 .ds4/report-t1.md                        |  85 +++++++++++
 effects/context.go                       |  26 ++++
 effects/copypermanent.go                 |  31 ++++-
 effects/nonremembered_controller_test.go | 110 ++++++++++++++
 5 files changed, 404 insertions(+), 80 deletions(-)
```

The three code files are byte-identical to the reviewed fix (`diff` of
`45ff269d^..45ff269d` against `main..HEAD` per file → identical for all three).

## Entry state

`git status` on entry: **clean, no rebase or merge in flight**. The daemon's
earlier attempt had been aborted — the reflog showed `rebase (abort)` at
`HEAD@{1}`, returning to the branch tip `0156b143` with a clean tree. So this
was a fresh integration, not the completion of an in-flight operation.

Branch at entry:
```
8c6fdd56 merge(cli-20260923T060000Z-rv2b-countheads): approx: the exotic <Ref>$<Property> count heads evaluate to zero
45ff269d fix(effects): resolve nonremembered controller selectors
0156b143 docs: update nonremembered selector report
```
`git merge-base HEAD main` was `8c6fdd56`; the branch had exactly two commits
ahead, `45ff269d` (the reviewed code fix) and `0156b143` (docs), confirmed with
`git rev-list --count bf627f79..0156b143` = 2.

## The conflict and its root cause

Only **one** file conflicted: `.ds4/report-t1.md`.

That file is a tracked accumulator — every merged branch appends its task
report to it (main's copy at the time was 979 lines, starting with the
`TriggerController$ on ChangesZone` report). Both sides had changed it:

- **main / rebase-HEAD side**: the accumulated 979-line report file.
- **`0156b143` side**: an insertion of the branch's own
  `# Report — NonRememberedController selectors` section.

The complication: the branch's version of `.ds4/report-t1.md` was **itself a
botched earlier conflict resolution**. Its 324-line content was a literal
interleaving of main's rv2b report and the branch's report, separated by the
raw strings `--- main version ---` and `--- this commit version ---` — a prior
resolver had committed the conflict marker text into the file instead of
resolving it. `git diff 8c6fdd56 HEAD -- .ds4/report-t1.md` showed that commit
*adding* those markers as content, and the branch file had 8 such markers.

Because `0156b143` was derived from a clobbered 234-line base (not from main's
979-line file), the three-way merge produced two conflict regions: one where
the branch's base overwrote main's opening reports, one at the tail.

## Resolution of `.ds4/report-t1.md`

The two sides do not genuinely contradict — the branch's intent is "append my
report", main's intent is "keep the accumulated reports". The correct
integration is **main's file intact + the branch's NonRememberedController
report appended once at the end**.

Concretely:

1. Confirmed the rebase-HEAD stage-2 blob was byte-identical to
   `main:.ds4/report-t1.md` (`diff -q` → identical, 979 lines) — `45ff269d`
   never touched this file, so HEAD's side carries no branch content.
2. Reconstructed the branch's *clean* report section by concatenating the
   `--- this commit version ---` blocks from the branch file in order
   (`What changed`, `Fails without the fix`, `Gates`, `Issues`, `Commit`) into
   `.ds4/scratch/nonremembered-report.md`. The rv2b `--- main version ---`
   blocks were discarded — main already contains the full rv2b report (as
   `# Merged concurrent report: cli-20260923T060000Z-rv2b-countheads`).
3. Wrote the resolved file as main's file + the reconstructed section, purely
   additive (85 appended lines, zero deletions).
4. No `<<<<<<<` / `=======` / `>>>>>>>` and no `--- main version ---` /
   `--- this commit version ---` remain in the file.

`git add .ds4/report-t1.md` printed a harmless "paths are ignored" notice
(`.ds4` is in `.git/info/exclude`), but the path was already staged as the
resolved index entry, so `-f` was not needed for that file.

## Resolution of `.ds4/report-mrg1.md` (this task's report)

This file is git-excluded scratch that each mrg1 seat overwrites with its own
report — the version main carried at rebase time said so explicitly ("The
stale report-mrg1.md found in this worktree's `.ds4/` belonged to a different
branch's resolution; it was replaced by this report, as the brief names exactly
this path for this task's report."). So the resolution is **replace**: this
report is the file content. It has no conflict markers and does not resurrect
the stale accumulation.

## Main moved during the task (not a rebase defect)

When first inspected, `main` was `bf627f79`; a later `git diff --stat
main..HEAD` showed unrelated files (`rules/zone_table_batch_test.go`,
`effects/choose_control.go`, `rules/engine.go`, `rules/trigger_match.go`,
`rules/paramcensus_test.go`) differing. That was **not** a rebase defect: the
shared `main` ref advanced while this seat worked, because other agents merged
their branches:

```
315137c7 merge(agent-20260922T090929Z-07378594)   (main after bf627f79)
afc27969 merge(agent-20260920T070405Z-eea92966) stat:CountersRemain ...
93e84c28 merge(fb-20260923T005805Z-1301f55a): dargo - no option to sac ...
```

I confirmed the daemon-dispatched rebase was correct for `bf627f79`, then
rebased forward onto each newer main tip (`315137c7`, `afc27969`, then
`93e84c28`, the tip this branch is now based on). The first forward rebase was
clean; the one onto `afc27969` conflicted only on `.ds4/report-mrg1.md`, and the
one onto `93e84c28` was clean. (Main is a moving target in this fleet; the
daemon will rebase once more at merge time if it advances again.)

The branch content changed nothing in the code between rebases: the code fix
stays byte-identical.

## Commands and output

Initial integration (dispatched target `bf627f79`):

```
$ git status
On branch wt/agent-20260920T074357Z-b9ac41c2
nothing to commit, working tree clean

$ git rebase main
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
error: could not apply 0156b143... docs: update nonremembered selector report
   (commit 45ff269d applied cleanly; only the docs commit conflicted)

# resolved report-t1.md as described
$ GIT_EDITOR=true git rebase --continue
[detached HEAD 8dede373] docs: update nonremembered selector report
 1 file changed, 85 insertions(+)
Successfully rebased and updated refs/heads/wt/agent-20260920T074357Z-b9ac41c2.
```

Forward rebases onto the newer main tips (clean, then one report conflict):

```
$ git rebase 315137c7
Successfully rebased and updated refs/heads/wt/agent-20260920T074357Z-b9ac41c2.

$ git rebase afc27969
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
error: could not apply ... docs: record mrg1 merge-conflict resolution
# resolved report-mrg1.md as "replace with this seat's report"
Successfully rebased and updated refs/heads/wt/agent-20260920T074357Z-b9ac41c2.

$ git status
On branch wt/agent-20260920T074357Z-b9ac41c2
nothing to commit, working tree clean

$ git log --oneline main..HEAD
<this commit> docs: record mrg1 merge-conflict resolution
da2334ea docs: update nonremembered selector report
196211dc fix(effects): resolve nonremembered controller selectors
```

No conflict markers remain in either report file:

```
$ grep -nE '^(<<<<<<<|=======|>>>>>>>)' .ds4/report-t1.md .ds4/report-mrg1.md
(no output; exit 1)
```

## Post-merge ratchets (required by the brief)

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.741s
```

No new `Mode$` matcher is registered, and no `knownUnsupported` /
`knownUnsupportedParams` / `knownUnmodelledCountHeads` entry is closed by this
branch, so no ratchet table needed an `addedAfterTheSplit` entry or a removal.

## Targeted tests over the conflicted files' packages

```
$ go test -run 'TestFracturedIdentityGivesEveryOtherPlayerACopy|TestNonRememberedControllerDefinedPlayers' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.621s

$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	2.584s

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.491s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.344s
```

`.cards` is present as a symlink to `/home/sadams/projects/gorge/.cards`
(`ls -la .cards` reproduced it), and the corpus-backed effects run took 0.62s —
not the ~2ms of a skipped run — so the corpus was really exercised.

No `TestHeads` failure: the rebase preserves the code fix byte-identically and
no golden was edited. `TestConstructedDefaultIsByteIdentical` passed unchanged,
so the botbench 20-game split did not move.

## Notes / uncertainties

- `0156b143`'s original content (the marker-laden file) was discarded for the
  report body; only the branch's *intended* report text was kept. That is the
  correct read of the branch's intent — its subject is "docs: update
  nonremembered selector report", and the marker text was clearly an accident,
  not authored prose.
- Main's rv2b report was not duplicated; the branch base's copy of it is
  already present in main's accumulated file in canonical form.
- Both rewritten doc commits carry no `Ref:` trailer and no attribution
  (gorge convention); the commit-msg hook's rules were respected.
- The commit SHAs in the earlier draft of this report were provisional and are
  superseded by the final three above.

## Issues

None found. The conflict was confined to tracked, git-excluded report
accumulators; no engine or test behavior was ambiguous. Pre-existing issues
the branch's own report already records (`PlayerCountRemembered$LifeTotal`
unread; the remaining `PlayerCountPropertyYou$` shapes) remain documented in
the appended `report-t1.md` section and are neither introduced nor changed by
this resolution.

---

# Preserved prior merge report (from f3953a37)

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

---

# Merge-conflict resolution — task agent-20260918T222614Z-5b138b5e (kw:Vanishing)

## Entry state and operation

The worktree was clean at `8e145e0d` (`docs: record Vanishing implementation
verification`) — no rebase or merge in flight, no partial prior work. The
branch carries the reviewed Vanishing fix (`3f4072f3` + `8e145e0d`); main had
advanced past the merge base `8c6fdd56` (mentor, StoreVoteNum, Yuffie attach,
pw-numloyaltyact and related work). I ran `git merge main`.

## Conflicted files and resolution

Exactly **one** content conflict; everything else auto-merged cleanly
(`cards/kw_registry_test.go`, `rules/trigger_*`, `rules/stack.go`,
`rules/resolution.go`, `effects/*`, `view/*`, `web/*` merged without manual
touching).

1. **`.ds4/report-t1.md`** — the report file both lines write over. The merge
   base was `3b070589` (the rv2b-countheads report); after it, HEAD replaced
   the file with this task's Vanishing report, and main's later commits
   (`0c067498`, `4f96e2c6`, …) replaced it with unrelated tickets' reports
   (Yuffie attach, Vote.StoreVoteNum). Per the precedent the previous
   resolution (`fa9f2a30`) recorded for this same path — the branch's own
   report is kept at `report-t1.md` — I took the OURS side:
   `git checkout --ours .ds4/report-t1.md && git add -f .ds4/report-t1.md`.
   Main's Yuffie/StoreVoteNum reports remain in main's own history; no code
   was affected.

No other file was touched; nothing was "improved" beyond the conflict.

## Operation completed

- `git commit --no-edit` → merge commit **`089fe7c3`**
  (`Merge branch 'main' into wt/agent-20260918T222614Z-5b138b5e`, default
  message).
- `git status --porcelain` → clean.
- `git merge-base --is-ancestor main HEAD` → pass.
- `gofmt -l` over the merge's `.go` files → no output.

## Commands run and output

- `go test ./rules -run 'Vanishing'` →
  `ok github.com/adams-shaun/gorge/rules 0.696s` — the branch's fix survives
  the merge; main did not conflict with `cards/kw_vanishing.go`
  (`git diff main --stat -- cards/kw_vanishing.go`: +36 intact).
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.774s` — the post-merge ratchet
  sweep the brief requires. The branch registers no new `Mode$` matcher and
  closes no ratchet entry; none of the ratchet tables moved on either side.
- `go test ./cards` → `ok github.com/adams-shaun/gorge/cards 7.434s` — main
  added `cards/kw_mentor.go` and both sides touched `cards/kw_registry_test.go`
  (auto-merged); the Vanishing and Mentor registrations coexist.
- `.cards/` is present as a symlink to `/home/sadams/projects/gorge/.cards`
  (real corpus — this was a corpus-backed run, not a vacuous skip).

## Unsure about

Only the report-file ownership call, resolved per recorded precedent
(OURS). Main's report content at that path belongs to other tickets' histories
and nothing on this branch referenced it.

## Issues

None new. No engine defect was found in either side's content during the
merge; no CR-lane finding to report.

---

# Merge-conflict resolution — agent-20260922T194522Z-d7f24b09

## Starting state

`git status` on arrival was CLEAN — no rebase or merge in flight. The
daemon's `rebase onto main` had already been aborted before this seat
started, leaving branch `wt/agent-20260922T194522Z-d7f24b09` at `9ed10179`
(docs: rpteachopt1 report) with two commits ahead of the merge-base
`3f7fce71` (`a98d1819` feat(effects): honour RepeatOptionalForEachPlayer$ in
RepeatEach — the reviewed fix — and `9ed10179` its report).

Per the standing "never `git rebase`" rule, integration was done as a
`git merge main`, which reproduced the same conflicts the daemon saw
(`.ds4/report-t1.md` content conflict; `effects/choose_control.go`
auto-merged cleanly).

## Conflicted file: `.ds4/report-t1.md` (the only one)

This is the shared rolling report file. Three-way shape (base = merge-base
version, 590 lines):

- **base**: reports for api:Attach Optional$ / Yuffie, fb-20260922T145544Z,
  rv2b-countheads, Vote.StoreVoteNum.
- **main**: kept all base content and appended ~6 new reports (stat:
  CountersRemain, TriggerController$ on ChangesZone, trig-attackerblocked,
  PlayerCountPropertyYou, Vote.StoreVoteNum retained, NonRememberedController
  selectors) — 1156 lines.
- **branch (`9ed10179`)**: REPLACED the file with only the rpteachopt1 report
  (181 lines; the commit's stat is `160 insertions(+), 569 deletions(-)`), so
  the branch side deleted the other seats' reports that main preserves.

### What each side wanted

- main: keep every prior report and add the new ones.
- branch: carry the rpteachopt1 report (RepeatEach /
  RepeatOptionalForEachPlayer$ fix documentation).

### Resolution

The branch's deletion of the four other seats' reports in its docs commit is
not a deliberate change main contradicts — it reads as the seat overwriting
the shared file instead of appending, and main is the superset that preserves
them. Resolution: **main's full version + the branch's rpteachopt1 report
appended** with the file's `---` section convention:

```sh
{ cat main-version; printf '\n---\n\n'; cat head-version; } > .ds4/report-t1.md
```

Nothing was dropped from either side: every report in main's version is
present verbatim, and the branch's full report (What changed and why,
commands, fails-without-fix, new tests, Issues, Deviations) is appended.
No conflict markers remain (`grep` verified); file is 1340 lines.

`effects/choose_control.go` auto-merged: main's change (NonRememberedController
selector resolution in `definedSpec`/`Controller$`) and the branch's change
(`effRepeatEach` per-subject election, `poseRepeatEachElection`) touch
different parts of the file; both intents kept, no manual edit needed.

## Commands and output

```
$ git status                                     # clean; no in-flight op
$ git merge main
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
Auto-merging effects/choose_control.go
Automatic merge failed; fix conflicts and then commit the result.
$ # resolve report-t1.md as above
$ git commit --no-edit   # concludes the merge -> d58d13ad

## Ratchet check after the merge

Main carries ratchet tests the branch never met. The branch registers no new
`Mode$` matcher and closes no ratchet table entry, so the tables should hold
as main has them. `.cards` was PRESENT (symlink), so the corpus-backed tests
ran rather than skipped.

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  github.com/adams-shaun/gorge/rules  0.801s

$ go test -v ./rules -run 'TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead|TestEveryDispatchedTriggerModeHasAMatcher|TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched|TestEveryRepoDeckCountHeadResolves'
--- PASS: TestEveryRepoDeckIsFullySupported (0.71s)
--- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
--- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.13s)
--- PASS: TestEveryRepoDeckCountHeadResolves (0.63s)
ok  github.com/adams-shaun/gorge/rules
```

No SKIPs (so the corpus-backed deck tests genuinely ran); no table entry
grew or moved — the branch registers no new `Mode$` matcher and closes no
ratchet row.

Auto-merge sanity on the shared file:

```
$ go test ./effects -run 'TestRepeatEachOptional|TestNonRememberedController|TestPlayerCountPropertyYou'
ok  github.com/adams-shaun/gorge/effects  0.618s
$ go test ./rules -run 'TestRepeatEachOptional'
ok  github.com/adams-shaun/gorge/rules  0.630s
$ gofmt -l effects/choose_control.go   # no output
```

## Notes / unsure about

No `TestHeads` failure: the rebase preserves the code fix byte-identically and
no golden was edited. `TestConstructedDefaultIsByteIdentical` passed unchanged,
so the botbench 20-game split did not move.

## Notes / uncertainties

- `0156b143`'s original content (the marker-laden file) was discarded for the
  report body; only the branch's *intended* report text was kept. That is the
  correct read of the branch's intent — its subject is "docs: update
  nonremembered selector report", and the marker text was clearly an accident,
  not authored prose.
- Main's rv2b report was not duplicated; the branch base's copy of it is
  already present in main's accumulated file in canonical form.
- Both rewritten doc commits carry no `Ref:` trailer and no attribution
  (gorge convention); the commit-msg hook's rules were respected.
- The commit SHAs in the earlier draft of this report were provisional and are
  superseded by the final three above.

## Issues

None found. The conflict was confined to tracked, git-excluded report
accumulators; no engine or test behavior was ambiguous. Pre-existing issues
the branch's own report already records (`PlayerCountRemembered$LifeTotal`
unread; the remaining `PlayerCountPropertyYou$` shapes) remain documented in
the appended `report-t1.md` section and are neither introduced nor changed by
this resolution.

---

# Merge-conflict resolution — agent-20260922T191943Z-4ffa25b7 (current integration)

## State and resolution

At entry, the worktree was clean at `0f4f4410`, with no merge or rebase in
progress. `main` was at `cb8bf4d7` and was not an ancestor. I ran `git merge
main`; source and test changes auto-merged, with one content conflict in
`.ds4/report-mrg1.md`.

The conflict was between two closing `## Issues` paragraphs in the accumulated
merge report. The branch paragraph said there was no engine conflict; main's
paragraph added that existing `PlayerCountRemembered$LifeTotal` and
`PlayerCountPropertyYou$` issues are unchanged and documented in `report-t1.md`.
I combined those facts into one paragraph. No production-code intent conflicted.
The merge also auto-merged main's unrelated effects and rules changes; these
remain staged alongside the branch changes.

`effects/cardflow.go`, `AGENTS.md`, and `internal/testutil/agentsdoc_test.go`
were not changed by this merge (the current merge path only conflicted in the
report file). The earlier rebase transcript mentions additional conflicts, but
those are not present in the operation actually completed here.

## Commands and results

- `git status --short --branch; git rev-parse --show-toplevel; git log -1 --oneline`
  — branch `wt/agent-20260922T191943Z-4ffa25b7`, clean at `0f4f4410` before
  integration.
- `git merge main` — automatic merge of source/report changes; conflict only
  in `.ds4/report-mrg1.md`.
- `ls -ld .cards` — `.cards` is present as a symlink to
  `/home/sadams/projects/gorge/.cards`.
- `go test -run 'TestRevealAllValid|TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./effects/ ./internal/testutil/`
  — `effects` passed (0.595s); `internal/testutil` passed (0.001s).
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  — passed (`rules` 0.779s).

## Issues

No new issues found. The previously documented `PlayerCountRemembered$LifeTotal`
and remaining `PlayerCountPropertyYou$` shapes are unchanged.
---

# Merge-conflict resolution report — mrg1 (task agent-20260922T120916Z-0a3043f3)

## Outcome

The daemon's rebase of this branch onto main had conflicted on
`.ds4/report-t1.md` (commit 28295617, "docs: record rolldice cost
verification"); its merge fallback additionally conflicted on
`.ds4/report-sol1.md`, with `rules/cast.go` and `rules/mana.go` auto-merging.
No rebase/merge was in flight when this seat started (`git status` clean on
`wt/agent-20260922T120916Z-0a3043f3`), so the integration was performed here
as a merge of `main` into the branch — the same shape the branch's history
already used (merge commit 32029f5c).

Result: merge commit `5252e33d` ("Merge branch 'main' into
wt/agent-20260922T120916Z-0a3043f3"), tree clean, no unmerged paths.

## Conflicted files, both sides, resolution

### `.ds4/report-sol1.md` (add/add — the only content conflict)

- **Branch side:** the RollDice cost verification report for THIS ticket
  (agent-20260922T120916Z-0a3043f3), committed in 14c3bd33 after the earlier
  round moved it off the shared `report-t1.md` path.
- **Main side:** the Gitaxian Probe verification report for the unrelated
  ticket fb-20260923T015847Z-fad49275, which used the same designated
  round-report path on main.
- **Resolution:** keep both. Branch report first (verbatim), then a one-line
  separator heading, then main's report verbatim. Neither side's intent is
  altered.

### `.ds4/report-t1.md` (auto-merged, verified)

The merge took main's version wholesale (1160 lines, an accumulator of 10+
reports). The branch's version held the `playerspec-life-svar-threshold`
report, which the branch's 14c3bd33 had restored "byte-for-byte from main" —
but main itself has since deliberately overwritten that file (0c067498
"docs: record Yuffie attach verification", later than the branch's restore).
The original playerspec report remains on main in history (df247a3f, content
before 0c067498), so nothing durable is lost and main's later deliberate
change correctly wins. No manual edit needed; verified no conflict markers.

### `rules/cast.go`, `rules/mana.go` (auto-merged)

Git merged main's changes with the branch's RollDice cost-token work without
conflict. Verified post-merge that the RollDice parser/payment code survived
intact: `rules/mana.go` keeps `Cost.RollDice` + `rollDiceCost` (lines 219-222,
460, 729) and `rules/cast.go` keeps the cost-die payment block (lines
8072-8076).

## Commands and output

```
$ git merge main --no-edit
Auto-merging .ds4/report-sol1.md
CONFLICT (add/add): Merge conflict in .ds4/report-sol1.md
Auto-merging rules/cast.go
Auto-merging rules/mana.go
Automatic merge failed; fix conflicts and then commit the result.
```

Sanity checks (run on the merged tree, before the merge commit; same content
as the committed tree):

```
$ go test -run 'TestClayGolem|TestMonstrosity|TestRolledDie' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.730s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -v
5 tests RUN, 0 SKIP, 0 FAIL; ok github.com/adams-shaun/gorge/rules 0.733s
```

(5 = TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched,
TestEveryDispatchedTriggerMode*, TestEveryRepoDeckIsFullySupported,
TestEveryRepoDeckParamsAreRead, and the CountHead ratchet test; zero skips so
the corpus-backed ratchets were not vacuous — `.cards` was already present as
a symlink in this worktree.)

Post-commit: `git status --short --branch` → clean, on
`wt/agent-20260922T120916Z-0a3043f3`.

## Ratchet state after the merge

No ratchet table entries needed updating: the branch registers no new
trigger `Mode$` matcher and closes no `knownUnsupported` /
`knownUnsupportedParams` / `knownUnmodelledCountHeads` entry, so the
post-merge ratchet run passing green is the expected outcome (the RollDice
cost parser's OnlyXTicket entries were never in any of those tables).

## Unsure about / notes

- The merge was performed with the default message (no `Ref:` trailer, per
  gorge convention).
- `.ds4/report-mrg1.md` itself held a previous task's merge report (brought in
  via main); this round's report is appended below it rather than replacing
  it, matching the repo's report-preservation convention.
- Not run here (daemon gates): full suite, TestHeads, `make sim`, CR
  conformance, `go vet`.

## Issues

None new. The only conflicts were report-path collisions among unrelated
tickets' reports; no engine code conflicted.

---

# Merge-conflict resolution — agent-20260922T191943Z-4ffa25b7 (integration of main b2b49d51)

## State and operation

At entry the worktree was clean at `8f2d377c` (the daemon had aborted both its
rebase and its merge fallback, so no merge/rebase was in flight). Main was at
`b2b49d51`, not an ancestor (merge-base `cb8bf4d7`). I ran `git merge main`.

## Conflicted files, both sides, resolution

- **`internal/testutil/agentsdoc_test.go`** — comment-only conflict above
  `knownApproximationRows` (both sides kept the constant at 18; main deleted
  three rows — scrybottom `T:Mode$ Scry`, four-mode trigger, rv2b count-heads —
  and the branch had earlier deleted the rv1 RevealAllValid$ row, whose
  oversize lowering to `knownOversizeRows = 7` is on both sides). Merged the
  comment to name all four disjoint closures. Measured merged AGENTS.md: 18
  data rows, so the constants stand.
- **`.ds4/report-t1.md`** — accumulator. HEAD prepended the branch's rv1
  report; main prepended the fb-20260923T005857Z (Count$ResolvedThisTurn /
  Sephiroth) report; the remainder is shared. Deleted the three marker lines,
  keeping both sides in order (branch report first) — the same convention as
  the `8014a55d` resolution. 1535 lines, no markers, `git diff --check` clean.
- **`.ds4/report-mrg1.md`** — accumulator. HEAD's tail: its "current
  integration" report (of the previous merge `0ca4f00b`). Main's tail: the
  base `## Issues` paragraph plus its mrg1 report for
  agent-20260922T120916Z-0a3043f3. Kept main's version and inserted HEAD's
  report section before main's final report, then appended this section.
- Everything else auto-merged: `AGENTS.md` (both sides' row deletions compose
  to the measured 18), `effects/cardflow.go` (branch's RevealAllValid$ block
  and main's changes in disjoint regions), plus main's
  `Count$ResolvedThisTurn`/RollDice/scry/CountersRemain code and tests.

## Commands and results

```
git status                  # clean at 8f2d377c before merge
git merge main              # 3 conflicts: report-mrg1.md, report-t1.md, agentsdoc_test.go
python3 resolutions ...     # marker removal / section splice as described
git diff --check            # clean on all three
```

Ratchets and targeted checks (results pasted below in the final message block).

## Issues

None new. The only content conflicts were the report accumulators and a
comment block; no engine behaviour was ambiguous on either side.

---

# Final verification — agent-20260922T191943Z-4ffa25b7

The failed daemon rebase/fallback is no longer in progress in this worktree:
`git status` was clean at merge commit `4c991146`, which already contains the
resolution described above. The merge-conflict paths have no conflict markers.
`.cards` is present as a symlink to the shared corpus.

Post-merge targeted tests and ratchets run for this verification:

```text
$ go test -run 'TestRevealAllValid|TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./effects/ ./internal/testutil/
ok   github.com/adams-shaun/gorge/effects        0.664s
ok   github.com/adams-shaun/gorge/internal/testutil 0.002s

$ go test ./rules/ -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules 0.815s

$ grep -nE '^(<<<<<<<|=======|>>>>>>>)' .ds4/report-mrg1.md .ds4/report-t1.md internal/testutil/agentsdoc_test.go
(no output)
```

The existing merge commit completed the integration; no second merge or rebase
was started. No code conflict required further edits, and no new issue was
found.
### Additional notes from main's side of this report conflict

- Whether the branch's deletion of the four prior reports in `9ed10179` was
  deliberate. I judged it accidental (append-style is the file's convention,
  main preserves them) and restored them via main's side. If it WAS
  deliberate, the merge result still loses nothing: the rpteachopt1 report is
  intact and the other reports remain available on main's history.
- `effects/choose_control.go` and the other auto-merged code files were NOT
  hand-checked beyond the merge succeeding; the daemon's full gate run covers
  them.

---

# Merge-conflict resolution — agent-20260922T191943Z-4ffa25b7 (current main tip)

## Entry and operation

The worktree was clean at `ffb976c8`, with no operation in progress. That
branch already contained merge `4c991146` integrating `b2b49d51`; current
`main` had advanced to `7b4ab59e`, so I merged its newer commits rather than
leaving the branch behind. `.cards` is present as a symlink to
`/home/sadams/projects/gorge/.cards`.

`git merge --no-edit main` auto-merged the source and test changes, including
`effects/cardflow.go`, and conflicted only in `.ds4/report-mrg1.md`.

## Conflict and resolution

`.ds4/report-mrg1.md` is a rolling report accumulator. The branch side held
its previously recorded merge-resolution narrative; main's side added notes
about preserving the four earlier task reports and not manually auditing
`effects/choose_control.go`. Neither side contradicted the other's report
history. I kept the full branch-side content and appended main's distinct
notes under `Additional notes from main's side of this report conflict`,
removing only the conflict delimiters. The reported fixes remain intact in the
auto-merged code. The earlier conflict in `internal/testutil/agentsdoc_test.go`
was already resolved by the existing `4c991146` merge; its row-count ratchet
passes below. The failed rebase described in the dispatch was not in progress
when this integration began.

## Checks and results

```text
$ git status --short --branch
## wt/agent-20260922T191943Z-4ffa25b7

$ git merge --no-edit main
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging .ds4/report-t1.md
Auto-merging effects/cardflow.go
Automatic merge failed; fix conflicts and then commit the result.

$ go test -run 'TestRevealAllValid|TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./effects/ ./internal/testutil/
ok   github.com/adams-shaun/gorge/effects        0.653s
ok   github.com/adams-shaun/gorge/internal/testutil 0.001s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules 0.768s

$ git diff --check
(clean)
```

`git add .ds4/report-mrg1.md` warned that `.ds4` is ignored by the ignore
rule, but it is a tracked path already in the merge index; the conflict
resolution is staged. No remaining conflict markers were found in the
conflicted report or auto-merged `report-t1.md`, `cardflow.go`, and
`agentsdoc_test.go`.

## Issues

No new engine issue found. The two pre-existing PlayerCount filter issues
remain as documented above. Main's auto-merged changes outside the focused
checks are left to the daemon's full gate run.

---

# Merge-conflict resolution — task agent-20260918T222614Z-5b138b5e (kw:Vanishing, second integration)

## Entry state and operation

`git status` on arrival was CLEAN — no rebase or merge in flight; the daemon's
failed rebase and merge-fallback attempt had both been aborted before this seat
started. Branch `wt/agent-20260918T222614Z-5b138b5e` was 6 commits ahead of
merge-base `f3953a37`, and `main` (`c947f5c8`) was 87 commits ahead. The
dispatch named two conflicted files (`.ds4/report-t1.md`, `.ds4/report-mrg1.md`),
which the reported rebase/merge-fallback transcripts match. Per the standing
"never `git rebase`" rule the integration was done as `git merge main`, which
reproduced exactly those two conflicts; everything else (including
`cards/kw_registry_test.go` and `AGENTS.md`) auto-merged.

## What each side wanted

- `.ds4/report-t1.md` (append accumulator; base 832 lines, ours 1236, main 1565):
  - main: prepended the fb-20260923T005857Z Sephiroth `Count$ResolvedThisTurn`
    report at the top and appended the rpteachopt1
    (`RepeatEach` / `RepeatOptionalForEachPlayer$`) report at the tail.
  - branch (ours): appended the `# Vanishing implementation report` at the tail.
  - Both sides also carry the shared stat:CountersRemain /
    TriggerController$/NonRememberedController reports — common content.
- `.ds4/report-mrg1.md` (base 338, ours 656, main 115):
  - ours: the accumulator — the b9ac41c2 mrg1 report + all six prior merge
    reports (the whole base) + the Vanishing mrg1 report appended.
  - main: REPLACED the file with only the agent-20260922T194522Z-d7f24b09
    (rpteachopt1) resolution report.

## Resolution

No engine code was conflicted; both files are docs-only accumulators whose
sides compose rather than contradict.

- `report-t1.md`: main's side kept in full (Sephiroth report at top, rpteachopt1
  at tail) with the branch's Vanishing report inserted between the shared
  NonRememberedController report and rpteachopt1, joined by the file's `---`
  section convention. Result: 1645 lines, 14 report headings.
- `report-mrg1.md`: the branch's full accumulator kept, main's d7f24b09 report
  appended after a `---` divider. Result: 774 lines, 12 report headings.

A programmatic check confirmed every non-blank line of BOTH stage-2 and stage-3
blobs is present in each resolved file (0 missing per side, per file), and no
conflict markers remain.

## Commands and output

```
$ git status                                  # clean, no in-flight op
$ git merge main
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
$ # resolved both files as above; git add -f-free (paths are tracked)
$ git commit --no-edit   -> 0fc1657d  (Merge branch 'main' into wt/...)
$ git merge-base --is-ancestor main HEAD   -> pass
$ git status --porcelain -> clean
```

Post-merge ratchet sweep required by the brief (`.cards` PRESENT as the symlink
to the real corpus — 0.76s, a corpus-backed run, not a vacuous skip):

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.760s
```

The branch registers no new `Mode$` matcher and closes no ratchet row, and no
ratchet table moved on either side, so no `addedAfterTheSplit` entry or table
removal was needed. Branch-fix sanity after the merge:

```
$ go test ./rules -run 'Vanishing'   -> ok  github.com/adams-shaun/gorge/rules  0.608s
$ go test ./cards -run 'Vanishing'   -> ok  github.com/adams-shaun/gorge/cards  0.001s
```

## Unsure about

- `main:.ds4/report-mrg1.md` replaced the accumulator with a single report; I
  treated that as the seat overwriting rather than a deliberate deletion (the
  same judgement the d7f24b09 report itself recorded for report-t1.md), and
  preserved the accumulator. Nothing is lost either way: main's report is
  appended intact and ours' history stays.
- Only report files conflicted; `effects/choose_control.go` and the other
  auto-merged code files were not hand-audited beyond the merge succeeding —
  the daemon's full gate run covers them.

## Issues

None new. No engine defect surfaced in either side's content; no CR-lane
finding to report.

---

# Merge-conflict resolution — agent-20260922T191943Z-4ffa25b7 (main update)

## Entry state and operation

The worktree was clean at `afe37d17`, an existing merge commit integrating
main through `7b4ab59e`; no rebase or merge was in progress. Current `main` had
advanced to `f2e9c8d5`. I merged that tip (rather than re-running the failed
rebase). The reported earlier `internal/testutil/agentsdoc_test.go` conflict
was already resolved in the prior merge; this merge produced one content
conflict, `.ds4/report-mrg1.md`. `.ds4/report-t1.md` auto-merged, as did the
Vanishing implementation and its card/rules tests.

## Conflict and resolution

`.ds4/report-mrg1.md` is an accumulated report file. The branch side contained
the previous integration narrative for this branch; main's side contained the
Vanishing ticket's merge report. These are independent histories, so I kept
both verbatim with a `---` separator and removed only the conflict markers.
No production-code conflict occurred. The tracked `internal/testutil/agentsdoc_test.go`,
`AGENTS.md`, and `effects/cardflow.go` contents remain as already integrated;
I made no changes to them in this operation.

## Commands and results

```text
$ git status --short --branch
## wt/agent-20260922T191943Z-4ffa25b7

$ git merge --no-edit main
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging .ds4/report-t1.md
Automatic merge failed; fix conflicts and then commit the result.

$ ls -ld .cards
lrwxrwxrwx ... .cards -> /home/sadams/projects/gorge/.cards

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  github.com/adams-shaun/gorge/rules  0.807s
```

The corpus symlink was present. The post-merge ratchets passed, including the
corpus-dependent repo-deck checks. No ratchet table update was indicated by
the results. No uncertainty remains about the report conflict; the daemon's
full gates cover the auto-merged Vanishing changes.

## Issues

None found during this integration. No unresolved engine behavior was
introduced or discovered.

# Merge-conflict resolution — mrg1 (agent-20260922T191943Z-4ffa25b7)

## State found

`git status` was **clean**, on branch `wt/agent-20260922T191943Z-4ffa25b7`, with
**no rebase/merge in flight** (no `rebase-merge`/`rebase-apply`/`MERGE_HEAD`, no
conflict markers anywhere). The daemon's failure note described a *previous*
integration attempt; a prior mrg1 run had left the branch with two merge commits
(`afe37d17`, `0f37bf40`) already integrating main up to `f2e9c8d5`. Main had
since moved on by four commits:

```
5c84527e merge(agent-20260918T230554Z-74976c7c): kw:Backup ...
5b7f6231 docs(backup): record the r2 report without touching prior round reports
c8b97fb0 docs(backup): task report for the Backup keyword (CR 702.70)
7caad9fb feat(rules): implement the Backup keyword (CR 702.70)
```

`git rev-list --left-right --count HEAD...main` = `11 4`, and `main` was NOT an
ancestor of HEAD — so the operation to complete was a fresh merge of current
`main` into the branch, not a rebase --continue.

## Command run

```
git merge main --no-edit
```

Result:

```
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
Automatic merge failed; fix conflicts and then commit the result.
```

Auto-merged (no conflict): `cards/kw_backup.go` (new), `cards/kw_registry_test.go`,
`rules/backup_test.go` (new), `.ds4/report-r2.md`. Note this differs from the
daemon's rebase report, which named `AGENTS.md` and `internal/testutil/agentsdoc_test.go`
as conflicting: those two files had already been reconciled by the prior merge
commit `0f37bf40`, so this merge's base for them was already main's version and
they auto-merged cleanly (they are not in the merge's staged diff at all).

## The one conflict — `.ds4/report-t1.md`

`.ds4` is gitignored but this report file is tracked (force-added in an earlier
round), so it participates in merges.

Structure (`git show :1/:2/:3:.ds4/report-t1.md`):

- **base (`:1`)** — 1645 lines, beginning `# fb-20260923T005857Z-c1a24352 —
  Count$ResolvedThisTurn (Sephiroth transform) ...`
- **ours / HEAD (`:2`)** — 1799 lines = base **+ a 154-line prepend**:
  `# Task rv1 — \`RevealAllValid$\` unread: reveal every matching hand card ...`
  (this ticket's round report).
- **theirs / main (`:3`)** — 1797 lines = base **+ a 152-line prepend**:
  `# Report — task agent-20260918T230554Z-74976c7c (kw:Backup) ...`.

Both sides are pure additive prepends of an independent report to the top of the
same shared report file; nothing in either side contradicts or supersedes the
other. `diff` of each side against the base showed only that side's own prepend,
byte-for-byte, with the remainder identical.

**Resolution: keep both reports.** Removed the three marker lines only
(`<<<<<<< HEAD` at 1, `=======` at 156, `>>>>>>> main` at 309), concatenating our
report first, then main's, then the shared base tail. No text from either side
was dropped, reordered within a side, or rewritten.

Verification after edit:

- `grep -n '<<<<<<<\|=======\|>>>>>>>' .ds4/report-t1.md` → no markers.
- `grep -n '^# ' .ds4/report-t1.md` → both `# Task rv1 — RevealAllValid…` (line 1)
  and `# Report — task agent-20260918T230554Z-74976c7c (kw:Backup)` (line 155)
  present, followed by the original `# fb-20260923T005857Z-c1a24352 …` (line 307)
  and the rest of the file unchanged.

Staged with `git add .ds4/report-t1.md` (the file is already tracked; the
gitignore hint is about the untracked siblings in `.ds4/` and did not block it —
`git status` then showed `M  .ds4/report-t1.md`, no `UU`).

## Completed operation

```
git commit --no-edit
[wt/agent-20260922T191943Z-4ffa25b7 9e24b4a9] Merge branch 'main' into wt/agent-20260922T191943Z-4ffa25b7
```

Default merge message (carrying the `# Conflicts: .ds4/report-t1.md` block) —
unchanged, as instructed. No trailer (gorge rule).

Final state: `git status` = clean; `git merge-base --is-ancestor main HEAD` = YES
(main fully integrated); no conflict markers; branch 12 ahead of the old base.

## Merge side-effects checked

- `internal/testutil/agentsdoc_test.go` — the `knownApproximationRows` constant is
  **18** and the comment above it already documents the four disjoint closures this
  merge carries (rv1's `RevealAllValid$` row, main's scrybottom row, main's
  four-mode trigger row, main's rv2b row). AGENTS.md contains **no**
  `RevealAllValid` row (the ticket deleted it), and the constant matches the
  measured table — both agentsdoc ratchets pass. No edit needed.
- `effects/cardflow.go` — the ticket's fix is present (`RevealAllValid$` pool
  narrowing at line 2270; `pickable` gate at 2314-2315 untouched). Auto-merged
  cleanly.

## Commands and output

```
$ git status
On branch wt/agent-20260922T191943Z-4ffa25b7
nothing to commit, working tree clean

$ git merge main --no-edit
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
Automatic merge failed; fix conflicts and then commit the result.

$ git commit --no-edit
[wt/agent-20260922T191943Z-4ffa25b7 9e24b4a9] Merge branch 'main' into wt/agent-20260922T191943Z-4ffa25b7

$ git status && git merge-base --is-ancestor main HEAD && echo YES
On branch wt/agent-20260922T191943Z-4ffa25b7
nothing to commit, working tree clean
YES
```

Ratchets (post-merge, required):

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  github.com/adams-shaun/gorge/rules  0.743s

$ go test -v ./rules -run '<same pattern>'   # confirm none skipped
--- PASS: TestEveryRepoDeckIsFullySupported (0.59s)
--- PASS: TestEveryRepoDeckCountHeadResolves (0.00s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.13s)
--- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
--- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
```

Conflicted-file package (the fix under merge):

```
$ go test -run 'TestRevealAllValid' ./effects/
ok  github.com/adams-shaun/gorge/effects  0.598s
```

Supporting gates:

```
$ go build ./...
BUILD EXIT 0
$ go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  3.779s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  1.322s
$ go test -v -run 'TestKnownApproximation' ./internal/testutil/
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
```

`.cards` was present in this worktree as a symlink to
`/home/sadams/projects/gorge/.cards`; the ratchet run executed real corpus tests
(`TestEveryRepoDeckIsFullySupported` 0.59s), so the green is not a skip-vacuous
run.

## Deviations from the brief

- The brief assumed a rebase was in flight. It was not — the tree was clean and
  a prior mrg1 had left merge commits. I completed the integration as a merge of
  current `main`, which is the operation that actually remained. Per gorge
  context and the working-method clause ("finish the in-flight operation you
  find rather than starting a new one"), no rebase was started.
- The conflicting file set differed from the daemon's rebase report:
  `AGENTS.md` and `internal/testutil/agentsdoc_test.go` did not conflict here
  (already reconciled by merge `0f37bf40`); this merge's sole conflict was
  `.ds4/report-t1.md`, and `cards/kw_backup.go`,
  `cards/kw_registry_test.go`, `rules/backup_test.go`, `.ds4/report-r2.md`
  auto-merged.

## Open concerns / Issues

- No new defect found in the conflicted region or the merge as a whole.
- The merge is a merge commit, so the branch tip is not a single linear series;
  the daemon's squash/gate path should treat `9e24b4a9` as the integration tip
  (same shape as the prior `0f37bf40`). Flagging only because the daemon log
  shows repeated mrg1 re-dispatches on this ticket.
- `AlreadyRevealed$` remains unread in the Go tree (noted by the rv1 report;
  cosmetic no-op today). Already an open item in the branch report's `## Issues`,
  not caused by this merge.

STATUS=DONE
COMMITS=9e24b4a9
TESTS=go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -> ok (5 tests, none skipped); go test -run 'TestRevealAllValid' ./effects/ -> ok; archtest -> ok; botbench TestConstructedDefaultIsByteIdentical -> ok; go build ./... -> ok

---

# Merge-conflict resolution report — mrg1 (agent-20260922T210645Z-27e19c88)

## Entry state and operation

`git status --short --branch` showed a clean worktree on `wt/agent-20260922T210645Z-27e19c88`, at `1f9f933e`; no merge or rebase was in flight. The merge base with `main` was `cb8bf4d7`. Per the dispatch, ran `git merge main`.

The merge had one conflicted path: `.ds4/report-sol1.md`. Main's other changes auto-merged; no production source file conflicted. The `.cards` corpus was present as a symlink to `/home/sadams/projects/gorge/.cards`.

## Conflict and resolution

`.ds4/report-sol1.md` contains reports for separate tasks. The branch side held the approved Attached-predicate implementation and stale-binding regression report, including the earlier Gitaxian Probe text. Main's side held the Deep Spawn `Mill` review response, RollDice verification report, and the Gitaxian Probe report. These intents are independent and do not contradict. I kept both complete sides, separated them with a Markdown divider, and removed only the three merge-marker lines. Thus the Attached report and main's reports are preserved. No source behavior was changed to resolve the conflict. No other path was manually edited.

The branch does not register a new trigger mode or close a known-unsupported/parameter/count-head ratchet entry, so no ratchet table adjustment was needed.

## Commands and output

```text
$ git status --short --branch && git rev-parse --abbrev-ref HEAD && git log -1 --oneline
## wt/agent-20260922T210645Z-27e19c88
wt/agent-20260922T210645Z-27e19c88
1f9f933e docs: record Attached predicates merge-blocker resolution

$ git diff --name-only --diff-filter=U
(no output before starting the merge)

$ git merge main
Auto-merging .ds4/report-sol1.md
CONFLICT (content): Merge conflict in .ds4/report-sol1.md
Automatic merge failed; fix conflicts and then commit the result.

$ git diff --name-only --diff-filter=U
.ds4/report-sol1.md

$ [ -e .cards ] && readlink -f .cards
/home/sadams/projects/gorge/.cards

$ go test -run 'TestAttachedPredicate|TestAttachedToContextReferents|TestAttachedToReferentPluralBindingFailsClosed|TestAttachedToStaleReferentFailsClosed|TestAttachedToLiteralPredicate|TestAttachedToTargetedBoundFromContext|TestAttachedToPlayerWordStaysUnknown|TestAttachedToPredicateUnlocksCorpusTargeting|TestArnaCopy|TestArnaRealSourceFilterReachesCopyRider|TestStanggRealTriggerCopiesAttachedPermanents' ./effects ./rules/ 2>&1 | tail -30
ok   github.com/adams-shaun/gorge/effects  0.640s
ok   github.com/adams-shaun/gorge/rules  0.695s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' 2>&1 | tail -30
ok   github.com/adams-shaun/gorge/rules  0.781s

$ grep -nE '^(<<<<<<<|=======|>>>>>>>)' .ds4/report-sol1.md
(no output)
```

## Issues and uncertainty

No new engine issue was found during integration. No uncertainty about the code merge: the only conflict was independent report content. The concatenated report retains an earlier historical Gitaxian Probe section as well as main's later Probe report; this is redundant historical documentation, not lost or conflicting implementation content.

---

# Merge-conflict resolution addendum — mrg1 (agent-20260922T191943Z-4ffa25b7), second integration round

## Why a second round

Immediately after the first merge commit (`9e24b4a9`, integrating main
`5c84527e`) and the report commit, `main` moved again — a sibling ticket
(`agent-20260922T210645Z-27e19c88`, the `Attached` filter predicate) merged,
advancing `main` from `5c84527e` to `935cefc4`. The branch was therefore again
4-behind main (`935cefc4`, `b603182b`, `1f9f933e`, `28b0a7f8`, `6b7f8116`,
`82db540a`, `53403389`, `60976e28`), a merge was required, and one file
conflicted.

## Operation

```
$ git merge main --no-edit
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Automatic merge failed; fix conflicts and then commit the result.
```

Auto-merged with no conflict (main's production change plus its tests):
`effects/filter.go`, `effects/attached_predicate_test.go`,
`effects/attachedto_ambiguity_test.go`, `effects/attachedto_predicate_test.go`,
`effects/attachedto_stale_binding_test.go`, `rules/arna_source_filter_test.go`,
`rules/copypermanent_grants_test.go`, `.ds4/report-sol1.md`,
`.ds4/report-sol2.md`.

## The conflict — `.ds4/report-mrg1.md` (this seat's own accumulator)

`.ds4` is gitignored but `report-mrg1.md` is tracked, so it merges like source.

Root cause: main's report commit `bfdf9efb` ("docs: record mrg1
merge-conflict resolution (kw:Vanishing second integration)") rewrote
`report-mrg1.md` down to a single new 50-line report for *its own* ticket
(`agent-20260922T210645Z-27e19c88`), i.e. 28 insertions / **847 deletions**
against the common ancestor — it collapsed the whole accumulated file. Our side
(`:2`, 1434 lines) preserved that history and appended this ticket's report.

The two sides are not contradictions but independent additions: main adds a new
report (210645Z) that our copy does not contain (`grep -c '210645Z'` = 0), and
ours keeps the accumulated history main's commit dropped. Gorge's established
convention, visible on main itself (`83c134e4 docs: preserve accumulated reports
alongside Vanishing records`), is to preserve accumulated reports.

**Resolution: union.** Took our full accumulated file (`:2`) and appended main's
new report (`:3`) as a new section after a `---` divider:

```
cat /tmp/mrg1-ours.md  > .ds4/report-mrg1.md        # 1434 lines
printf '\n---\n\n'    >> .ds4/report-mrg1.md
cat /tmp/mrg1-theirs.md >> .ds4/report-mrg1.md       # +50 lines = 1487
```

Nothing from either side was dropped or rewritten. Verified:

```
$ grep -n '^<<<<<<<\|^=======$\|^>>>>>>>' .ds4/report-mrg1.md   # no output
$ grep -n '^# ' .ds4/report-mrg1.md | tail -3
1241:# Merge-conflict resolution — mrg1 (agent-20260922T191943Z-4ffa25b7)
1438:# Merge-conflict resolution report — mrg1 (agent-20260922T210645Z-27e19c88)
```

Then `git add -f .ds4/report-mrg1.md` (the file is tracked; `.ds4` being ignored
requires `-f` for a fresh add), and:

```
$ git commit --no-edit
[wt/agent-20260922T191943Z-4ffa25b7 49518987] Merge branch 'main' into wt/agent-20260922T191943Z-4ffa25b7
```

## Final state and gates

```
$ git status --short          # clean
$ git merge-base --is-ancestor main HEAD && echo YES
YES                            # main=935cefc4 fully integrated
$ go build ./...
BUILD EXIT 0

$ go test -v ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
--- PASS: TestEveryRepoDeckIsFullySupported (0.62s)
--- PASS: TestEveryRepoDeckCountHeadResolves (0.00s)
--- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
--- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.21s)
ok  github.com/adams-shaun/gorge/rules  0.881s

$ go test -run 'TestRevealAllValid|TestAttachedPredicate|TestAttachedToContextReferents|TestAttachedToReferentPluralBindingFailsClosed|TestAttachedToStaleReferentFailsClosed|TestAttachedToLiteralPredicate|TestAttachedToTargetedBoundFromContext|TestAttachedToPlayerWordStaysUnknown|TestAttachedToPredicateUnlocksCorpusTargeting' ./effects/
ok  github.com/adams-shaun/gorge/effects  0.621s

$ go test -run 'TestArnaCopy|TestArnaRealSourceFilterReachesCopyRider|TestStanggRealTriggerCopiesAttachedPermanents' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.631s

$ go test -run 'TestKnownApproximation' ./internal/testutil/     -> ok
$ go test ./internal/archtest/                                   -> ok (4.025s)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ -> ok (1.501s)
$ go build ./...                                                 -> ok
```

No ratchet table adjustment was needed: neither side of the merge registers a
new trigger `Mode$` or closes a `knownUnsupported` / `knownUnsupportedParams` /
`knownUnmodelledCountHeads` entry, and all five ratchet tests pass on the merged
tree.

## Branch tip

```
49518987 Merge branch 'main' into wt/agent-20260922T191943Z-4ffa25b7   (integration tip)
a058dd96 docs(mrg1): record merge-conflict resolution
9e24b4a9 Merge branch 'main' into wt/agent-20260922T191943Z-4ffa25b7
2d78bd6b (superseded by amend) docs(mrg1): ...
```

`git merge-base --is-ancestor main HEAD` = YES; tree clean; no conflict markers.

## Issues (second round)

- No new defect. The only non-report conflict this round was none — the
  production `Attached` changes auto-merged and their tests pass.
- Note for the controller: main's `bfdf9efb` deleted 847 lines of accumulated
  `report-mrg1.md` history. This merge restored it (union), but any other branch
  merging main from a base at or before `bfdf9efb` will hit the same conflict. A
  future main-side cleanup of the accumulator would be worth a ticket.

STATUS=DONE
COMMITS=a058dd96 49518987
TESTS=go test -v ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -> 5 PASS none skipped; go test -run 'TestRevealAllValid|TestAttached*...' ./effects/ -> ok; go test -run 'TestArna*|TestStangg*' ./rules/ -> ok; agentsdoc/archtest/botbench -> ok; go build ./... -> ok

# Merge-conflict resolution report — mrg1 (agent-20260923T073156Z-d6f8c32b)

## Outcome

Branch `wt/agent-20260923T073156Z-d6f8c32b` (the reviewed CR-legal cascade
free-cast fix) is now integrated with `main`. Because the daemon had already
aborted both its rebase attempt and its merge fallback before this seat
started, a fresh `git merge main` was performed and the single conflicted file
was resolved.

Merge commit: `de376741`. `git status` clean afterwards.

## Conflicted file and resolution

One file conflicted: `.ds4/report-sol1.md`. This is a shared multi-report
accumulation point.

- **HEAD (branch)**: commit `0f9dcb58` had replaced the whole file (which at
  the branch base `cb8bf4d7` held only the Gitaxian Probe report) with the
  cascade report `# Cascade free cast — CR 702.85a / CR 107.3b`.
- **main**: keeps the accumulated reports (Attached predicates, Deep Spawn
  UnlessCost Mill, RollDice) and RESTORED the Gitaxian Probe report at the end
  (labelled "# Second report stored at this shared path (from main)") — a
  later, deliberate anti-overwrite change on the same lines.
- **Resolution**: main's accumulated 228-line version kept byte-for-byte; the
  branch's cascade report appended verbatim at the end under a `---` separator
  with the same pointer-line convention main already uses ("# Second report
  stored at this shared path (from branch): Cascade free cast — CR 702.85a /
  CR 107.3b"). Both intents preserved; no report deleted or edited.
- No other file was touched by the resolution (the merge's other ~70 staged
  paths are main's and the branch's own committed changes, auto-merged).
- No code file was conflicted and none was edited, so no engine behaviour can
  have moved from the resolution itself.

## Entry state

`git status` on entry: clean, NO rebase or merge in flight (`rebase-merge`,
`rebase-apply`, `MERGE_HEAD` all absent). Reflog showed
`rebase (abort): returning to refs/heads/wt/agent-20260923T073156Z-d6f8c32b`
at `HEAD@{1}` — the daemon had aborted both its rebase (conflict at `0f9dcb58`
on `.ds4/report-sol1.md`) and its merge fallback. So this was a fresh
integration, not the completion of an in-flight operation.

## Commands run (real output)

```
$ git merge main -m "Merge branch 'main' into wt/agent-20260923T073156Z-d6f8c32b"
Auto-merging .ds4/report-sol1.md
CONFLICT (content): Merge conflict in .ds4/report-sol1.md
Automatic merge failed; fix conflicts and then commit the result.
$ git add -f .ds4/report-sol1.md && git commit -m "Merge branch 'main' into wt/agent-20260923T073156Z-d6f8c32b"
[wt/agent-20260923T073156Z-d6f8c32b de376741] Merge branch 'main' into wt/agent-20260923T073156Z-d6f8c32b
$ git status --short
(clean)
```

Post-merge gates (each run once, log captured; `.cards` present as a symlink
to the real corpus, so nothing skipped):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.287s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.492s
$ go test -run 'TestCascadeFreeCastAnnouncesNoX|TestCascadeXSpellUsesAnnouncedManaValue' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.644s
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.814s
```

Ratchets green: no new trigger mode to register, no `knownUnsupported` /
`knownUnsupportedParams` / `knownUnmodelledCountHeads` entry to remove, and
the botbench golden did not move.

## Report preservation

`.ds4/report-mrg1.md` itself held the prior round's resolution report
(agent-20260920T074357Z-b9ac41c2), preserved by main. This round's report is
prepended above it with a separator; the historical report is untouched below.

## Issues

No engine defect found in this integration round (docs-only resolution). No
head/ratchet movement; no Known-approximations row touched.

---

