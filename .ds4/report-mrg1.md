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

## Round 2 — second main integration (2026-09-23 02:40)

After the round-1 merge (`f8bbf659`, main at `1be022eb`) landed, main advanced to
`edc24484` (the sibling ticket `cli-20260922T225141Z-1b1182e4` merged: MustBlock
enforcement, `ImprintOnHost$`, `RememberCounteredCMC`, first-strike phase gating).
Ran `git merge main` again; two conflicts:

### `internal/testutil/agentsdoc_test.go`

Both sides had already resolved `knownApproximationRows` to `42` (main's comment
lists its closure set: acc7878d, 63c07260, 7c4182ff, d56e404f, 78d3b764,
f76f59fd and earlier ones; our round-1 comment said the same count). Took main's
more detailed comment verbatim — the constants agree, only the explanatory text
differed. Verified by counting the merged `AGENTS.md` table: **42 data rows**,
matching the constant.

### `.ds4/report-mrg1.md`

Path collision with the sibling ticket `1b1182e4`, whose own `mrg1` report
(1799 lines) reached main through its merge chain. This file at the canonical
path is THIS ticket's report; kept ours (as round 1 did) and appended this
section. Main's copy is the sibling's report and stays in its own merge
commits (`af2f1648` etc.); nothing else referenced it.

Everything else auto-merged (`AGENTS.md`, `rules/combat.go`, `effects/misc.go`,
`decision/*`, `state/phase.go`, new tests and sources from main).

### Commands and outputs

- `git merge main`:
  `Auto-merging .ds4/report-mrg1.md / CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`
  `Auto-merging AGENTS.md / Auto-merging internal/testutil/agentsdoc_test.go / CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`
  `Automatic merge failed; fix conflicts and then commit the result.`
- Row count: `awk` over merged `AGENTS.md` → 42 data rows.
- `.cards` check: present (`CARDS_OK`).

### Issues

No new unfixed issue found in this round. The round-1 note stands: `legal.go`'s
`offerCastable` remains pool-only, so a replicate option can be withheld when
only convoke would fund it (documented in `0836163f`'s commit message).
