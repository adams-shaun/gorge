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
