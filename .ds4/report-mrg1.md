# Merge-conflict resolution — task cli-20260922T225143Z-4b0bde0d

## Operation

The worktree was clean at `a2ff6603`, with no merge or rebase in progress. It already contained the prior integration of `main` through `fead1e59`; current `main` had since advanced to `4a7bb2fe`. Ran `git merge main --no-edit`, which conflicted in `internal/testutil/agentsdoc_test.go` and this report path. Resolved both conflicts, preserving the reviewed ExchangeLifeVariant fix and main's intervening changes.

`.cards` is present and resolves to `/home/sadams/projects/gorge/.cards`.

## Conflicts and resolutions

### `internal/testutil/agentsdoc_test.go`

The branch side described the transactional ExchangeLifeVariant fix and prior table closures. Main's side reflected its token-replacement closure and its own closure history. The merged `AGENTS.md` retains both sides' table deletions; the `TestKnownApproximationsOnlyShrinks` ratchet measures **39** data rows. Kept the branch's ExchangeLifeVariant closure and main's closures in the explanatory comment, and set `knownApproximationRows` to 39. No table row was added or grown.

### `.ds4/report-mrg1.md`

The branch's version was this task's cumulative integration report. Main's version was the unrelated report for the sibling token-replacement task. Kept this task's report path and replaced the conflict with this task's current resolution record.

### Other merged paths

`AGENTS.md`, `effects/context_test.go`, `effects/filter.go`, `effects/misc.go`, `effects/registry.go`, `effects/token.go`, `effects/zone.go`, `rules/engine.go`, `rules/replacement.go`, `rules/stack.go`, `rules/combat_history_filter_test.go`, `rules/token_replacement_standins_test.go`, and `rules/token_replacement_test.go` merged automatically and were retained.

## Commands and output

- `git status --short --branch; git rev-parse --show-toplevel; git log -5 --oneline --decorate`: clean worktree at `a2ff6603` on `wt/cli-20260922T225143Z-4b0bde0d`.
- `git merge main --no-edit`: conflicts in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`; remaining paths auto-merged.
- The initial manual row count was off by one; `TestKnownApproximationsOnlyShrinks` reported the exact count, **39**.
- `[ -e .cards ] && readlink -f .cards`: `/home/sadams/projects/gorge/.cards`.
- `git diff --check`: passed (no output).
- `go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/`: `ok github.com/adams-shaun/gorge/internal/testutil 0.002s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`: `ok github.com/adams-shaun/gorge/rules 0.827s`.

## Issues

No new engine issue was found during conflict resolution. The transactional life-exchange fix remains intact; main's changes are retained. The approximation ratchet is set to the measured merged count of 39.
