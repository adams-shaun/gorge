# Merge conflict resolution: mrg1 (branch wt/cli-20260922T225138Z-c4106938, main at e53c80a2)

## Starting state

The worktree was clean, HEAD = `2ae14f36`, no rebase/merge in flight (the
daemon's failed rebase had been rolled back). The branch's approved fix
(`0a23564e` label stack target options by stack kind, `578affca` test,
`8782f1e8` botbench coverage seed, plus the docs repairs `568cae83`/`08825579`
/`2ae14f36`) closes the AGENTS.md row "A spell on the stack is offered with
`Option.Kind` `"permanent"`…" and lowers the ratchet constant to 75.

`main` had advanced by three commits (`852ffe4f` the K:Affinity:Affinity fix,
then the two merges `57cff580`/`e53c80a2`): `852ffe4f` deleted the Affinity
row from AGENTS.md and lowered main's constant to 76. Rebase is forbidden in
this worktree, so the integration was done as `git merge main`.

## Conflicted files and resolution

1. **`AGENTS.md`** — auto-merged with NO textual conflict: the two sides
   deleted different rows (branch: the stack-option-kind row; main: the
   K:Affinity:Affinity row). Verified the merged table measures **74 data
   rows**, both closed rows absent, and the file differs from main by exactly
   the branch's one row deletion.
2. **`internal/testutil/agentsdoc_test.go`** — one conflict on
   `knownApproximationRows` (HEAD 75 vs main 76). Resolved to **74**, the
   merged table's measured data row count (each side held 75 rows; the merge
   deletes main's Affinity row on top of the branch's stack-kind closure).
   Never raised.
3. **`.ds4/report-mrg1.md`** — both sides were prior rounds' reports. Replaced
   with THIS report per the report-path contract.

All other main-side changes (`effects/count.go`, `effects/filter.go`,
`rules/affinity_affinity_test.go`) auto-merged and were retained unmodified.

## Ratchet cross-check after the merge

The brief's ratchet command list
(`TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead`)
plus the agentsdoc ratchet itself — see the commands below. No ratchet table
needed fixing: the branch registers no new trigger `Mode$` matcher, and the
merged tree's `knownUnsupported` / `knownUnsupportedParams` /
`knownUnmodelledCountHeads` tables are untouched by either side (main's
Affinity fix was a filter/count capability, already paired with its row
deletion; the branch's change was an option-kind labelling fix, already paired
with its row deletion).

## Commands run and output

```
git merge main
  -> Auto-merging .ds4/report-mrg1.md CONFLICT; AGENTS.md auto-merged;
     internal/testutil/agentsdoc_test.go CONFLICT
python3 row-count of merged AGENTS.md (the test's own parsing algorithm)
  -> data rows: 74; affinity row absent; stack-kind row absent
git diff main -- AGENTS.md -> only the branch's stack-option-kind row deletion

go test ./internal/testutil -run 'TestKnownApproximation' 2>&1 | tail -5
  -> ok  github.com/adams-shaun/gorge/internal/testutil
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' 2>&1 | tail -5
  -> ok  github.com/adams-shaun/gorge/rules
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
  -> ok  github.com/adams-shaun/gorge/cmd/botbench
go test ./internal/archtest/ 2>&1 | tail -5
  -> ok  github.com/adams-shaun/gorge/internal/archtest
```

## Result

Merge commit with the default merge message; working tree clean.

## Issues

None found beyond the resolved conflict itself.
