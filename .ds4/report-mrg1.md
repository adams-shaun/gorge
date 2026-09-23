# Merge conflict resolution: mrg1 round 3 (branch wt/cli-20260922T225138Z-e21c29e8, main at c12e10ef)

## Starting state

The worktree was CLEAN, HEAD = `7a83891e` (`docs: record mrg1 verification`),
whose merge commit `2b884e5b` had already integrated main@`e53c80a2`. No
rebase/merge was in flight — the daemon's failed rebase (conflict applying
`f68b9396`, "Rebasing (1/7)") and its merge fallback had both been rolled
back before this dispatch, so the operation was redone from scratch as
`git merge main` (rebase is forbidden in this worktree; merge is the
established integration shape here).

Main had advanced past the previous integration point by the sibling
tickets' work: `0679b1cd` (foretell predicates / Cosmos Charger /
effect-delivered MayPlay), the stack target option kind commits
(`0a23564e`/`578affca`/`8782f1e8`), and their merges
(`98dcf594`, `f14b56a3`, `51e249b0`, `c12e10ef`).

## Conflicted files and resolution

1. **`.ds4/report-mrg1.md`** — the ONLY conflict. Both sides carried their
   own prior-round report text (this branch's round-2 report vs the sibling
   ticket cli-20260922T225138Z-c4106938's round-2 report, which reached this
   file via main's `b6e82601`/`51e249b0`). Both prior narratives describe
   rounds that are now history; per the report-path contract this file is
   replaced with THIS round's report (the text you are reading).
2. **`AGENTS.md`** — auto-merged, no textual conflict. Verified via
   `internal/testutil` ratchet (below): merged table measures 73 data rows
   and `knownApproximationRows = 73`, exactly main's round-2 resolution —
   both sides' row closures retained, neither raised.
3. **`rules/legal.go`** — auto-merged, no textual conflict. Main's heavy
   rework (foretell/MayPlay grant work) plus this branch's target-minima
   census coexist; the branch's tests (`cast_target_census_test.go`,
   `cast_offer_pairwise_test.go`, `cast_liveness_test.go`) pass against the
   merged `legal.go`.
4. **`internal/testutil/agentsdoc_test.go`** — auto-merged at 73 (matching
   the merged AGENTS.md table); ratchet green.

All other main-side changes (`effects/filter.go`, `effects/misc.go`,
`rules/mayplay.go`, `rules/playerkeywords.go`, `rules/stack.go`,
`state/continuous.go`, `rules/foretell_grant_test.go`,
`rules/paramcensus_test.go`, `rules/stack_target_option_kind_test.go`,
`cmd/botbench/actioncoverage.go`) auto-merged and were retained unmodified.

## Commands run and output

```
git merge main
  -> AGENTS.md, rules/legal.go auto-merged; .ds4/report-mrg1.md CONFLICT (only)
go test ./internal/testutil -run 'TestKnownApproximation' 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/internal/testutil   (knownApproximationRows = 73)
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|CastTarget|CastOffer'
  -> ok  github.com/adams-shaun/gorge/rules 0.827s
go test ./internal/archtest/ 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/internal/archtest 3.042s
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/cmd/botbench 1.074s
gofmt -l rules/legal.go rules/stack.go rules/cast.go internal/testutil/agentsdoc_test.go
  -> (no output, clean)
```

`.cards` was present in the worktree (real corpus), so the rules run was not
a vacuous skip.

## Ratchet cross-check after the merge

This branch registers no new trigger `Mode$` matcher and closes no
`knownUnsupported` / `knownUnsupportedParams` / `knownUnmodelledCountHeads`
entry that main does not already carry, so no ratchet table needed fixing as
part of the merge — confirmed by the green rules ratchet run above and the
green `internal/testutil` agentsdoc ratchet.

## Issues

None found beyond the resolved conflict itself. No engine behaviour changed
in this integration beyond what both reviewed sides already carried
(botbench byte-identical pin still holds, so the merge introduced no
behaviour drift a repo deck exercises).
