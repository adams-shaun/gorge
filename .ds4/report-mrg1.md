# Merge-conflict resolution report — mrg1

## Conflicts and resolution

- `internal/testutil/agentsdoc_test.go`: the branch's approximation row count and main's independently lowered count conflicted. Kept both sides' `AGENTS.md` row deletions and set `knownApproximationRows` to 74, matching the merged table's 74 data rows. This preserves the reviewed branch change while accounting for main's additional deletions.
- `AGENTS.md` and `rules/clone.go`: auto-merged; retained both sides' changes.
- Merge-fallback conflict in `.ds4/report-mrg1.md`: resolved by recording this integration's final outcome here. No engine code was changed during conflict resolution.

The operation is complete in merge commit `757ffdee` (`Merge main: resolve approximation row count conflict`). The working tree is clean. `.cards` is present.

## Commands and output

- `git status --short --branch; git rev-parse --abbrev-ref HEAD`
  ```
  ## wt/cli-20260922T225139Z-230e9834
  wt/cli-20260922T225139Z-230e9834
  ```
- `git status`
  ```
  On branch wt/cli-20260922T225139Z-230e9834
  nothing to commit, working tree clean
  ```
- `git show --stat --oneline HEAD; git status --short --branch; git rev-parse HEAD^1 HEAD^2` confirmed merge commit `757ffdee`, clean branch, parents `f4c04e2f092e61ecb37b28335a95ade095874050` and `e53c80a2132990a82767efa346dd3ee6dafc79c4`.
- Corpus check: `.cards present`.
- `go test ./internal/testutil ./rules -run 'TestKnownApproximation|TestClone'`
  ```
  ok   github.com/adams-shaun/gorge/internal/testutil  0.001s
  ok   github.com/adams-shaun/gorge/rules  0.823s
  ```

No unresolved conflict or uncertainty remains.
