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
