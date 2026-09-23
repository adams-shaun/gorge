# Merge conflict resolution report — cli-20260922T225141Z-1b1182e4

## Operation and conflict

- Initial `git status --short --branch`: `## wt/cli-20260922T225141Z-1b1182e4`; working tree clean, no operation in progress.
- Ran `git merge main`. It completed automatically except for one content conflict: `internal/testutil/agentsdoc_test.go`. `AGENTS.md` and the other main changes merged automatically; `rules/trigmatch_misc.go` also auto-merged.
- The conflict was `knownApproximationRows`: this branch said `53`, while main said `50`. The branch closes the First Strike Damage approximation row; main contains other approximation closures. After the merge, I measured 46 data rows in the merged `AGENTS.md` Known approximations table (one header row excluded), so resolved the constant to `46` to match the merged content. No other files were conflicted.
- Completed the merge with its default message. Merge commit: `af2f1648` (`Merge branch 'main' into wt/cli-20260922T225141Z-1b1182e4`).

## Commands and outputs

`git status --short --branch && git rev-parse --show-toplevel && git branch --show-current && git status`

```text
## wt/cli-20260922T225141Z-1b1182e4
/home/sadams/projects/gorge/.worktrees/cli-20260922T225141Z-1b1182e4
wt/cli-20260922T225141Z-1b1182e4
On branch wt/cli-20260922T225141Z-1b1182e4
nothing to commit, working tree clean
```

`git merge main`

```text
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

Measured table counts from `git show <ref>:AGENTS.md`: branch `HEAD` had 53 data rows; main had 47. The automatically merged table had 46 data rows after preserving both sides' deletions.

`git add internal/testutil/agentsdoc_test.go && GIT_EDITOR=true git merge --continue && git status --short --branch`

```text
[wt/cli-20260922T225141Z-1b1182e4 af2f1648] Merge branch 'main' into wt/cli-20260922T225141Z-1b1182e4
## wt/cli-20260922T225141Z-1b1182e4
```

Corpus check: `ls .cards | head`

```text
cards.lock
cardsfolder
ir.gob.gz
ir.v4.gob.gz
tokenscripts
```

Conflicted-package check, `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`

```text
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
```

Post-merge ratchets, `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`

```text
ok  github.com/adams-shaun/gorge/rules  0.792s
```

Final `git status --short --branch`:

```text
## wt/cli-20260922T225141Z-1b1182e4
```

No unresolved concerns.
