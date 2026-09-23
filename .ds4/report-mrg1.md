# Merge resolution report — cli-20260922T225140Z-677ee477

## Operation and conflicts

- Initial `git status --short --branch`: `## wt/cli-20260922T225140Z-677ee477`; working tree clean, no operation in progress.
- Ran `git merge main`. It stopped with one content conflict: `internal/testutil/agentsdoc_test.go`. `AGENTS.md` and the other main-side changes merged automatically.
- In the conflict, this branch had `knownApproximationRows = 55`; main had `57`. The branch's `AGENTS.md` had already removed the `(ft1)` bot-target approximation row. Main's changes remove the `(setname1)` row. The merge result preserves both closures (and main's code/tests), so I kept the `(ft1)` deletion, retained the `(setname1)` deletion, measured the merged table at 54 data rows, and resolved the constant to 54. Main's `57` did not describe the actual merged table.
- No other file had conflict markers. There were no unresolved behavioral choices.

## Commands and outputs

`git status --short --branch && git status`

```text
## wt/cli-20260922T225140Z-677ee477
On branch wt/cli-20260922T225140Z-677ee477
nothing to commit, working tree clean
```

`git merge main`

```text
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

Resolved and staged `AGENTS.md` plus `internal/testutil/agentsdoc_test.go`, then ran `GIT_EDITOR=true git merge --continue`:

```text
[wt/cli-20260922T225140Z-677ee477 a70c4483] Merge branch 'main' into wt/cli-20260922T225140Z-677ee477
```

`.cards` check: `.cards present`.

`go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`

```text
ok  github.com/adams-shaun/gorge/rules  0.770s
```

`go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`

```text
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
```

Final check: `git status --short --branch` returned only `## wt/cli-20260922T225140Z-677ee477` (clean). `git rev-parse --short HEAD` returned `a70c4483`. The merged approximation table measures 54 data rows.
