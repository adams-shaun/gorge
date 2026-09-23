# Merge-conflict resolution — agent-20260918T225913Z-5db23024 (mrg1)

Ticket: `kw:Emerge` — sacrifice-for-reduction alternative cast (CR 702.118a).
Base: `main` @ `767f3dd4` (`merge(agent-20260922T234314Z-bbfff2fb)`).
Branch before: `wt/agent-20260918T225913Z-5db23024` @ `d58cc764`, 41 behind / 8 ahead.
Outcome: rebased onto `main`; `main` is now an ancestor; branch is 7 commits ahead; tree clean.

## In-flight operation found

`git status` on entry reported a clean tree on
`wt/agent-20260918T225913Z-5db23024`, but the branch was still based on
`c4560130` (main's old tip at the branch's own earlier merge) with 41 commits
behind and 8 ahead — i.e. the failed rebase had been aborted, not completed.
No `rebase-merge`/`MERGE_HEAD` state existed. I restarted the operation with
`git rebase main`, which reproduced the reported conflict at pick 4/7
(`08cef97c docs(rules): record Emerge integration verification`) on
`.ds4/report-sol1.md`.

## Conflicted files

### `.ds4/report-sol1.md` (only conflict)

This path is a shared, tracked "designated seat report" file that many
tickets append to. Both sides had appended a different ticket's report at the
same anchor (the file's end):

- **HEAD (branch) side** — `# Mill-trigger replacement redirection —
  agent-20260919T183731Z-085022e9`: a prior ticket's mill-trigger
  replacement-redirection report. This content had arrived on the branch
  through the branch's earlier `Merge branch 'main' ...` (`cf798080`) and was
  already present in the pre-conflict portion of the file (lines 1–682
  matched `main:.ds4/report-sol1.md` exactly, ending with that
  IgnoreLegendRule report and a `---`).
- **Incoming (`08cef97c`) side** — `# Emerge integration —
  agent-20260918T225913Z-5db23024 (sol1)`: this ticket's own integration
  report, which the branch had stored at this same path.

**Resolution: keep BOTH sections.** The branch's own recorded policy (see the
Emerge report text and commit `209ced69 docs(rules): preserve Emerge
implementation and review reports`) is precisely that one ticket's report must
never substitute for another ticket's report at a shared path. Main's
mill-trigger section and the branch's Emerge section are non-contradictory
appends, so the resolved file is main's full `.ds4/report-sol1.md` sequence
(Convoked$Amount → … → IgnoreLegendRule → Mill-trigger) followed by a `---`
separator and the Emerge integration report. No mill-trigger content was
dropped and no Emerge content was dropped.

Mechanically: removed the three conflict markers
(`<<<<<<< HEAD`, `=======`, `>>>>>>> 08cef97c …`) and inserted a blank line +
`---` before the Emerge heading so the appended section is a clean markdown
separation. No other textual content was altered.

`git add` on this path needed `-f`: `.ds4/` is in `.gitignore` (line 25) and
the worktree exclude list, while the file is also tracked — git refuses a bare
`git add` of an ignored path even when tracked. `git add -f .ds4/report-sol1.md`
staged the resolution; `git ls-files -u` confirmed no unmerged entries
remained.

No Go source was touched by the conflict; no other file conflicted.

## Commands run (exact, with output)

```
$ git rebase main
…
Auto-merging .ds4/report-sol1.md
CONFLICT (content): Merge conflict in .ds4/report-sol1.md
error: could not apply 08cef97c... docs(rules): record Emerge integration verification

$ grep -n '^<<<<<<<\|^=======\|^>>>>>>>' .ds4/report-sol1.md
683:<<<<<<< HEAD
752:=======
795:>>>>>>> 08cef97c (docs(rules): record Emerge integration verification)

# (edited: kept both sections, removed markers)

$ git add -f .ds4/report-sol1.md && GIT_EDITOR=true git rebase --continue
[detached HEAD 22071b59] docs(rules): record Emerge integration verification
 1 file changed, 45 insertions(+)
Rebasing (5/7)… (6/7)… (7/7)…
Successfully rebased and updated refs/heads/wt/agent-20260918T225913Z-5db23024.

$ git status
On branch wt/agent-20260918T225913Z-5db23024
nothing to commit, working tree clean

$ git merge-base --is-ancestor main HEAD && echo YES
YES
$ git rev-list --left-right --count main...HEAD
0	7
```

### Ratchets required after merging main

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.862s
ratchet_exit=0
```

Verbose run confirmed the five named tests actually executed (no skips, corpus
present):

```
--- PASS: TestEveryRepoDeckIsFullySupported (0.74s)
--- PASS: TestEveryRepoDeckCountHeadResolves (0.01s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.19s)
--- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
--- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
5 PASS, 0 SKIP
```

No `addedAfterTheSplit` entry or `knownUnsupported` /
`knownUnsupportedParams` / `knownUnmodelledCountHeads` removal was required:
the branch adds no new `Mode$` matcher, and `kw:Emerge` was registered via
`effects.RegisterNonAPI` (a casting option, not a trigger mode). The ratchets
are green unchanged.

### Targeted Emerge tests

```
$ go test -run 'TestEmerge' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.660s
emerge_exit=0
```

### Mandatory behaviour goldens

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.841s
arch_exit=0
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.319s
bot_exit=0
```

`.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`
(inherited at worktree creation), so the corpus-backed tests ran rather than
skipping — 0 skips in the verbose ratchet run confirms it.

## Head / ratchet movement

None. `rules/heads_test.go` was not edited and `TestHeads` was not run
(daemon gate). The botbench `TestConstructedDefaultIsByteIdentical` split pins
green. No acceptance-table / Known-approximations row change.

## Deviations / open concerns

- The conflict forced no Go change; the only edit was the shared report file,
  resolved by keeping both tickets' appended sections, consistent with the
  branch's own documented report-preservation policy.
- `.ds4/*.md` reports are tracked while `.ds4/` is gitignored; staging the
  resolution required `git add -f`. This is pre-existing repo/tooling
  behaviour, not introduced here.
- Files new-vs-main: `.ds4/report-r2-emerge.md`, `.ds4/report-t1-emerge.md`
  (branch-added unique report paths), plus `rules/emerge*.go` and small edits
  in `rules/cast.go` / `rules/legal.go` — all pre-existing branch commits, not
  touched by this resolution.

## Issues

No unresolved defect found in this round. The integration was a report-file
merge only; the Emerge implementation and its tests are unchanged from the
reviewed fix (`29de0b65`, `14563e8d`, `6afea940`).
