# Merge-conflict resolution — task cli-20260922T225143Z-4b0bde0d

## Operation

Found the worktree clean with no operation in flight: the branch's own merge
had already been completed at `bbd973b4` ("Merge branch 'main' into
wt/cli-20260922T225143Z-4b0bde0d", integrating main as of `4a7bb2fe` and the
branch's transactional life-exchange fix). But `main` had since advanced to
`08a1d59a` (sibling merges: the cascade closures `e46f051d` and the
`MaxTotalTargetPower$`/`TargetMax$` fix `8d83f028`). Ran `git merge main
--no-edit` to complete the integration. Conflicted in
`internal/testutil/agentsdoc_test.go` and this report path; everything else
auto-merged.

`.cards` is present and resolves to `/home/sadams/projects/gorge/.cards`.

## Conflicts and resolutions

### `internal/testutil/agentsdoc_test.go`

Only the explanatory comment and the `knownApproximationRows` constant
conflicted. HEAD said 39 (measured at the previous merge), main said 38
(measured at its own merge). The merged `AGENTS.md` — which keeps both sides'
table deletions, this branch's transactional life-exchange closure plus main's
cascade1 and maxpower1 closures — measures **37** data rows. Kept the branch's
fuller closure-history comment, extended it to name main's cascade1
(`e46f051d`) and maxpower1 (`8d83f028`) closures, and set
`knownApproximationRows = 37`. No row was added or grown.

### `.ds4/report-mrg1.md`

A tracked per-merge report artifact that collides on every branch (main's
version was the sibling maxpower1 task's report). Replaced the conflict with
this merge's own report (this file).

### Auto-merged paths retained

`AGENTS.md`, `effects/cascade.go`, `effects/misc.go`, `rules/engine.go`,
`rules/stack.go`, `rules/cast.go`, `rules/layers.go`, `rules/clone.go`,
`rules/mana_activation.go`, `rules/paramcensus_test.go`, plus main's new
`rules/all_land_types_test.go`, `rules/cascade_approx_test.go` and
`rules/target_max_power_cap_test.go`.

## Commands and output

- `git status` — clean, no rebase/merge in flight; HEAD `bbd973b4`, main
  `08a1d59a`; `git merge-base main HEAD` = `4a7bb2fe`, so main was NOT an
  ancestor and the new commits still needed integrating.
- `git merge main --no-edit` — conflicts in `.ds4/report-mrg1.md` and
  `internal/testutil/agentsdoc_test.go`; other paths auto-merged.
- Row count of the merged `AGENTS.md` (same section walk the test does):
  `data rows: 37` (HEAD's AGENTS.md: 39, main's: 38 — each side had already
  deleted the other side's row in its own table, hence the drift).
- `go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/`
  → `ok github.com/adams-shaun/gorge/internal/testutil`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules`.
- `gofmt -l internal/testutil/agentsdoc_test.go` — no output.

## Issues

No new engine issue was found during conflict resolution; the merge introduces
no engine-code change of its own. The branch's transactional life-exchange fix
and main's cascade/maxpower/land-type work are both retained; the approximation
ratchet is set to the measured merged count of 37.
