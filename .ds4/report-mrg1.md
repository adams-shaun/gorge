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
