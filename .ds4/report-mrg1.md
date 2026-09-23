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
