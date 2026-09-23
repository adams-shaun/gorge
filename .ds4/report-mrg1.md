# Merge-conflict resolution report — mrg1

## Conflicted files

- `AGENTS.md` — the only unmerged path.

## Resolution

The initial `git status` showed a clean worktree and no active rebase/merge, so I completed integration with the main branch via `git merge main`. That merge conflicted in `AGENTS.md` at the adjacent Known approximations rows.

- The branch side contained the new main-side approximation for the `KReplacement` bot fallback. Main contained the obsolete CR 704.5j legend-rule row, which this reviewed branch closes by adding the controller choice.
- Kept main's `KReplacement` row and removed the superseded legend-rule row, preserving both current intents. No other conflict markers or unmerged paths remained.
- Completed the merge as `760578de` (`Merge branch 'main' into wt/cli-20260922T225140Z-68ca4d95`).

## Commands and output

- `git status --short --branch && git status`
  ```
  ## wt/cli-20260922T225140Z-68ca4d95
  On branch wt/cli-20260922T225140Z-68ca4d95
  nothing to commit, working tree clean
  ```
- `git merge main`
  ```
  Auto-merging AGENTS.md
  CONFLICT (content): Merge conflict in AGENTS.md
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- `git add AGENTS.md && GIT_EDITOR=true git merge --continue`
  ```
  [wt/cli-20260922T225140Z-68ca4d95 760578de] Merge branch 'main' into wt/cli-20260922T225140Z-68ca4d95
  ```
- Corpus check: `.cards` was present.
- `go test -run 'TestLegendRule' ./rules/`
  ```
  ok  github.com/adams-shaun/gorge/rules  0.008s
  ```
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  ```
  ok  github.com/adams-shaun/gorge/rules  0.759s
  ```
- Final `git status --short --branch`
  ```
  ## wt/cli-20260922T225140Z-68ca4d95
  ```

No uncertainty remained in the conflict resolution. No non-conflict files were manually changed.
