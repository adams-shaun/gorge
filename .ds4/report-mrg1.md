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

- Whether the branch's deletion of the four prior reports in `9ed10179` was
  deliberate. I judged it accidental (append-style is the file's convention,
  main preserves them) and restored them via main's side. If it WAS
  deliberate, the merge result still loses nothing: the rpteachopt1 report is
  intact and the other reports remain available on main's history.
- `effects/choose_control.go` and the other auto-merged code files were NOT
  hand-checked beyond the merge succeeding; the daemon's full gate run covers
  them.
