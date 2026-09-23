# Merge-conflict resolution — cli-20260922T225140Z-62421b89

## Situation found

The worktree was clean on entry at `7a45ef9a` with no in-flight merge or rebase. `main` was at `0104252d2aac00c38b8ee63675239b78dddef2ff`, not an ancestor of the branch. `.cards` was present.

Ran `git merge main`. It auto-merged the main changes and reported one conflicted file: `AGENTS.md`.

## Conflicted file and resolution

### `AGENTS.md`

The branch side had deleted its now-closed approximation row for CR 616.1 replacement ordering; it also carried the CR 704.5j legend-rule approximation. Main's conflicting side contained the old CR 616.1 row and a botpolicy fallback row, as well as the legend-rule row. The branch has implemented both the CR 616.1 order choice and `botpolicy`'s `KReplacement` policy (`botpolicy/policy.go`, `chooseReplacementOrder`), so the two main-side CR 616.1 rows describe behavior no longer present and were not retained. Kept the branch's removal and retained main's still-applicable CR 704.5j legend-rule row. Other main edits to this file, including removal of the closed fx20/pc1 rows, were preserved by the auto-merge.

No other file had merge markers. The remaining main changes auto-merged. The required main ratchet run found two stale `apiSpecificRulesSA` entries in `rules/paramcensus_test.go`: `Engine.applyAddCounterBody` and `Engine.applyAddCounterReplacements` consume the priced result and no longer read SA params. Removed those stale classifications, retaining `Engine.counterReplaceOp`, which does read the parameters. This is the ratchet-table correction required for the merged branch; no engine behavior was changed by it.

## Commands and output

```text
$ git status --short --branch && git rev-parse --abbrev-ref HEAD && git diff --name-only --diff-filter=U
## wt/cli-20260922T225140Z-62421b89
wt/cli-20260922T225140Z-62421b89

$ git rev-parse main
0104252d2aac00c38b8ee63675239b78dddef2ff

$ git merge main
Auto-merging AGENTS.md
CONFLICT (content): Merge conflict in AGENTS.md
Auto-merging rules/engine.go
Auto-merging rules/turn.go
Automatic merge failed; fix conflicts and then commit the result.

$ git add AGENTS.md && go test -run 'TestKnownApproximation' ./internal/testutil/
ok   github.com/adams-shaun/gorge/internal/testutil  0.001s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
--- FAIL: TestEveryRepoDeckParamsAreRead (0.15s)
    paramcensus_test.go:2738: paramcensus rot guard: 2 findings:
        paramcensus: apiSpecificRulesSA entry "Engine.applyAddCounterBody" no longer reads SA params -- delete the stale entry
        paramcensus: apiSpecificRulesSA entry "Engine.applyAddCounterReplacements" no longer reads SA params -- delete the stale entry
FAIL
github.com/adams-shaun/gorge/rules  0.892s
FAIL

# Removed the two stale entries from rules/paramcensus_test.go.
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.750s
```

`git diff --check` reported no whitespace errors after conflict resolution. The first focused approximation test passed. The first ratchet run exposed the two stale entries above; the rerun passed after removing them. `.cards` existed, so the corpus-dependent ratchets were not vacuous skips.

## Uncertainties / issues

No unresolved conflict or uncertain resolution remains. The replacement-order approximation rows from main were intentionally omitted because the reviewed branch fix and the already-present botpolicy arm close those claims. The unrelated CR 704.5j row remains intact. No engine behavior change or golden edit was made during conflict resolution.
