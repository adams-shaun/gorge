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
