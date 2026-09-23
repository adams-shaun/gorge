# Merge-conflict resolution report — mrg1 (ticket cli-20260922T225139Z-18baec47)

## Starting state

`git status` showed a CLEAN tree on `wt/cli-20260922T225139Z-18baec47` at `2fb804c3` —
the daemon's rebase (`could not apply 2001c516…`) had been aborted, nothing was in
flight. The branch was 3 commits ahead of `main` (fix `2001c516` + two follow-ups) and
7 behind (`main` at `84cb68b9`). Since no operation was in flight, I performed the
integration as a merge (`git merge main --no-edit`), which reproduced exactly the
conflict the daemon hit.

## Conflicted files

Only ONE file conflicted: `internal/testutil/agentsdoc_test.go`. `AGENTS.md` and every
code file auto-merged cleanly.

### internal/testutil/agentsdoc_test.go — the `knownApproximationRows` constant

- **Both sides descended from the same base value 77.**
- **Branch side (HEAD, `2fb804c3`)**: `knownApproximationRows = 76` — commit `2001c516`
  (fix(search): ask before fail-to-find shuffle) closed one Known-approximations row and
  lowered the constant by 1.
- **Main side (`84cb68b9`)**: `knownApproximationRows = 74` — the foretell/affinity
  commits (`0679b1cd`, `852ffe4f` and the merges carrying them) closed 3 rows and lowered
  the constant by 3.

### Resolution

The register is delete-only and both sides deleted DISJOINT rows, so the merged value is
the base minus the union: **77 − 4 = 73**. I set `knownApproximationRows = 73`, keeping
both sides' row deletions (verified: `git add` + `TestKnownApproximationsOnlyShrinks` and
`TestKnownApproximationRowsAreShort` both PASS against the merged `AGENTS.md`, which
auto-merged both sides' deleted rows). This is the standard union resolution for this
ratchet — no behavioural choice was involved.

## Commands and output

```
git merge main --no-edit
  → Auto-merging AGENTS.md / agentsdoc_test.go; CONFLICT in internal/testutil/agentsdoc_test.go
# resolved constant to 73, git add
go test ./internal/testutil/ -run 'TestKnownApproximation|TestKnownOversize' -v
  → TestKnownApproximationsOnlyShrinks PASS, TestKnownApproximationRowsAreShort PASS
git commit --no-edit  → 9efdb11f "Merge branch 'main' into wt/cli-20260922T225139Z-18baec47"
git status --short → clean

# Post-merge ratchets (per the 2026-09-22 merge instruction):
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
  → ok github.com/adams-shaun/gorge/rules 0.738s
go test -list '<same pattern>' ./rules  → all 5 tests matched (not vacuous):
  TestEveryRepoDeckIsFullySupported, TestEveryRepoDeckCountHeadResolves,
  TestEveryRepoDeckParamsAreRead, TestEveryDispatchedTriggerModeHasAMatcher,
  TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched

# Behaviour goldens (repo-notes requirement before DONE):
go test ./internal/archtest/  → ok 2.903s
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/  → ok 1.047s
```

`.cards` was present (symlink to `/home/sadams/projects/gorge/.cards`, verified before any
test run), so no corpus-backed test was vacuous.

## Ratchet table entries

No branch commit registered a new trigger `Mode$` or closed a
`knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads` entry, and the
ratchet run above is green as-is — nothing to fix in those tables.

## Issues

None found. No defects beyond the conflict itself.

## Final state

Branch `wt/cli-20260922T225139Z-18baec47` at merge commit `9efdb11f`, tree clean, all
targeted checks green.
