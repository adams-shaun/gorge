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

<<<<<<< HEAD
None found in this scope. The merge introduced no new engine behaviour of
its own; both sides' reviewed changes were preserved.

---

# Merge-conflict resolution — agent-20260922T191943Z-4ffa25b7 (rv1 RevealAllValid$)

## State and operation

At entry the worktree was clean on `wt/agent-20260922T191943Z-4ffa25b7` at
`2786ed95` — an earlier integration had already merged main `e8d1d9f9` ("keep
both disjoint row closures — four-mode triggers (main) and RevealAllValid$
(rv1); merged register 19 -> 18 rows"), but main had since advanced to
`b4592552` (the ca8c201c `Defined$ Remembered` merge plus fleet merges). No
merge/rebase was in flight, so I ran `git merge main` myself.

Source files auto-merged cleanly on the merge route — including
`effects/cardflow.go`, which had conflicted in the daemon's per-commit rebase
but merged without conflict here (the branch's `RevealAllValid$` block in
`effReveal` and main's `CountersRemain`/`Defined$ Remembered` changes touch
disjoint regions; both retained, verified by grep and by the branch's
`TestRevealAllValid` regression). `AGENTS.md` and
`internal/testutil/agentsdoc_test.go` also auto-merged this time (main did not
move the row table since the earlier merge), and
`TestKnownApproximationsOnlyShrinks`/`TestKnownApproximationRowsAreShort` pass
at the merged `knownApproximationRows = 18`.

## Conflict and resolution

- **`.ds4/report-t1.md`** — the accumulating report log. HEAD side: the
  branch's approved rv1 `RevealAllValid$` report (ending with its STATUS block
  and `---` divider). Main side: the newer `stat:CountersRemain` report and
  companions, followed by content shared with HEAD. Kept both sides verbatim,
  branch report first with main's after the divider that already terminates the
  HEAD side: the resolution was exactly the deletion of the three conflict
  marker lines (1231 -> 1228 lines; zero markers remain; `git diff --check`
  clean). No report text was rewritten.

## Commands and results

```text
git status                # clean, no merge/rebase in flight
git merge main            # CONFLICT only in .ds4/report-t1.md; effects/cardflow.go auto-merged
sed -i markers-out .ds4/report-t1.md   # 0 markers, diff --check clean
git add .ds4/report-t1.md && git commit --no-edit
                          # 8014a55d, both parents, tree clean

ls -l .cards              # symlink -> /home/sadams/projects/gorge/.cards (present)

go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  github.com/adams-shaun/gorge/rules  0.779s

go test -run 'TestRevealAllValid' ./effects/
ok  github.com/adams-shaun/gorge/effects  0.640s

gofmt -l effects/cardflow.go internal/testutil/agentsdoc_test.go   # clean
```

## Result

- Branch `wt/agent-20260922T191943Z-4ffa25b7` at merge commit `8014a55d`;
  main `b4592552` is an ancestor; tree clean.
- No head/ratchet movement: the ratchet run passed unchanged; the branch
  registers no new `Mode$` matcher and closes no `knownUnsupported`/
  `knownUnsupportedParams`/`knownUnmodelledCountHeads` entry (its row closure
  was already merged in `2786ed95`).
- No `Ref:` trailers (gorge rule respected).

## Issues

None found in the merge itself. The conflict was confined to tracked, git-excluded report accumulators; no engine or test behavior was ambiguous. Pre-existing issues the branch's report records (`PlayerCountRemembered$LifeTotal` unread and remaining `PlayerCountPropertyYou$` shapes) remain documented in `report-t1.md` and are neither introduced nor changed by this resolution.

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
