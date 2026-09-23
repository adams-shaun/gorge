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
