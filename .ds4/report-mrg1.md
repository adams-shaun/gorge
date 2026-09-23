# Merge-conflict resolution — mrg1

## Conflict and resolution

Only `.ds4/report-sol2.md` was conflicted (add/add). The branch side contains the Dismantle targeted-counter-LKI sol2 report; main's side contains an Attached-predicates sol2 merge-blocker report. These are independent historical reports that happened to claim the same report path. I preserved both complete reports in that file under separate headings, retaining their findings, gate outputs, and issue notes. No report content was discarded. All other main changes auto-merged; there were no conflicted Go files.

## Commands and results

```
$ git status --short --branch
## wt/agent-20260922T215327Z-0900a39d
nothing to commit, working tree clean
$ git merge main
Auto-merging .ds4/report-sol2.md
CONFLICT (add/add): Merge conflict in .ds4/report-sol2.md
Auto-merging effects/count.go
Auto-merging effects/registry.go
Auto-merging rules/engine.go
Auto-merging rules/resolution.go
Automatic merge failed; fix conflicts and then commit the result.
$ git diff --check
(no output; exit 0)
$ git add .ds4/report-sol2.md
The following paths are ignored by one of your .gitignore files:
.ds4
hint: Use -f if you really want to add them.
$ git add -f .ds4/report-sol2.md && GIT_EDITOR=: git merge --continue
[wt/agent-20260922T215327Z-0900a39d 08c5dd1a] Merge branch 'main' into wt/agent-20260922T215327Z-0900a39d
$ test -e .cards && readlink .cards
/home/sadams/projects/gorge/.cards
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.970s
$ git status --short --branch
## wt/agent-20260922T215327Z-0900a39d
```

The required post-merge ratchets passed with the real `.cards` corpus available. No behavior conflict or uncertainty remained. Merge commit: `08c5dd1a`; merged main tip: `935cefc4`.

## Issues

No new issues found during conflict resolution. No code conflict required behavior changes.
