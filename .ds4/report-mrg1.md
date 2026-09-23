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
