# Merge conflict resolution — mrg1

## Result

Integrated `main` (`1be022ebce54f7adc60a491e961dd7808e84e971`) into `wt/cli-20260922T225141Z-5641f97b` with a merge commit. The requested branch fix `0836163f` is retained. No code outside the reported merge was authored.

## Conflict

### `internal/testutil/agentsdoc_test.go`

- Branch side changed `knownApproximationRows` from 50 to 49 when its repeatable-cost approximation row was deleted.
- Main side had its own approximation closures and declared 43 rows in its explanatory comment and constant.
- The merged `AGENTS.md` contains main's deletions plus the branch's repeatable-cost row deletion. Counting the actual merged table gives 42 data rows, so resolved the constant to 42 and described the combined result. This preserves both sides' intended ratchet updates.

`AGENTS.md` and `rules/cast.go` auto-merged without conflict. Main's changes to the remaining files were incorporated by the merge; no further conflict resolution was needed.

## Commands and output

- `git status --short --branch; git status`:
  `## wt/cli-20260922T225141Z-5641f97b`; clean before integration.
- `git merge main`:
  `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`; `AGENTS.md` and `rules/cast.go` auto-merged.
- Counted the data rows in the merged `AGENTS.md`: **42**; main's table before the branch-only deletion had 43.
- `git diff --check`: no output (passed).
- `.cards` check: `.cards exists`.
- `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`:
  `ok github.com/adams-shaun/gorge/internal/testutil 0.004s`
- `go test ./rules -run 'TestReplicateCountBound|TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  `ok github.com/adams-shaun/gorge/rules 0.777s`

The requested post-merge trigger/deck/parameter/count-head ratchets and the branch's repeatable-cost tests passed. No uncertainty remains.

## Issues

No new unfixed issue was found while resolving this integration conflict. The branch commit's reported replicated-mode offer-gate limitation remains as documented in its commit message: `legal.go`'s `offerCastable` remains pool-only, so a replicate option can be withheld when only convoke would fund it. It was outside the conflict and was not changed here.
