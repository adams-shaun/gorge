# Merge conflict resolution: mrg1 (round 3 — branch wt/cli-20260922T225138Z-636f892f, main at c2596487)

## Starting state

`git status` found **no operation in flight** (clean tree, HEAD `e3372a23`): the daemon's
rebase (`e7d5bcec` onto main) and its merge fallback had both been aborted, leaving the
conflicts unresolved. Merge-base of HEAD and main was `d43b4915`; main had advanced to
`c2596487` (22 commits: Count$Compare thresholds, TargetType qualifiers, TargetUnique
accumulator, effCharm Suspended guard, Exile/Discard/Return cost verbs, ChooseEach$,
stack-kind un-stamp — each closing its Known-approximations row).

Re-ran the integration as `git merge main` from the branch (rebase is forbidden here;
main's own history shows prior rounds of this same merge were landed as merges too).

## Conflicted files and resolution

Both conflicts were the frozen Known-approximations register, where the two sides deleted
**disjoint** rows from the same region:

1. **`AGENTS.md`** (one conflict block, lines 217–224):
   - HEAD (branch) side: kept the `effCharm` Suspended-guard, `TargetUnique$`,
     sacrifice-CARDNAME/cost-verbs and `TargetType$` rows — main deleted all four
     (commits 1eb845f5, 506e7167, b3fd6a77, b89e7869).
   - main side: kept the `UnlessCost$` row — the branch deleted it (e7d5bcec/e3372a23,
     the dynamic unless-cost grammar fix).
   - Resolution: **all five rows removed** — the merged table carries both sides'
     deletions. `Count$Compare`'s deletion (41ae0e4a/c64c792f) auto-merged cleanly at its
     own location. Verified with `git diff main -- AGENTS.md`: exactly one line differs,
     the UnlessCost row's deletion. Measured merged table: **78 data rows**.
2. **`internal/testutil/agentsdoc_test.go`**: HEAD `knownApproximationRows = 83`, main
   `= 80` (main's own table measured 79 — main's constant was off by one, on the safe
   side). The merged constant must equal the merged table: **78** (84 at the merge-base
   minus the branch's 1 row minus main's 5 rows). Set to 78; the register is never raised.

Nothing else conflicted; `effects/registry.go`, `rules/resolution.go`, `rules/stack.go`
auto-merged and were verified by the test runs below.

## Commands run and output

```
go build ./... && go vet ./rules ./effects ./internal/testutil   -> clean
go test -run 'TestKnownApproximations|TestKnownOversize' ./internal/testutil/
  -> ok  0.001s
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
  -> ok  0.732s   (-v confirms all 5 ran and PASSED; 0 SKIPs; .cards symlink present)
go test ./rules -run 'Unless|TestCharm|TargetType|TargetUnique|CostVerbs|ChooseEach'
  -> ok  0.746s   (both sides' new suites over the auto-merged files)
go test ./effects -run 'Charm|Compare|ChooseEach'
  -> ok  0.602s
go test ./events ./state   -> ok 5.428s / ok 0.014s
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/  -> ok (byte-identical, unmoved)
go test ./internal/archtest/   -> ok
git commit --no-edit   -> c5589975 "Merge branch 'main' into wt/cli-20260922T225138Z-636f892f"; tree clean
```

## Notes / uncertainties

- The constant 78 is one lower than main's 80 because main's own constant over-counted
  its table by one (79 rows measured at `main`). Shrinkage-only, so safe.
- Main tracks `.ds4/report-mrg1.md` (its round-2 report); this file replaces it per the
  report-path contract, committed as a separate `docs:` commit like prior rounds.
- One transient slip during resolution (a lost backtick on the `ValidTgts$ Spell` row in
  AGENTS.md) was caught and fixed before staging; final diff vs main is exactly the one
  row deletion.

## Issues

None found. No new defects surfaced during integration; both sides' tests and the
ratchets pass against the merged tree.
