# Merge-conflict resolution report — cli-20260923T060000Z-layers-pt7kw (mrg1)

## State found

`git status` on arrival: branch `wt/cli-20260923T060000Z-layers-pt7kw`, working
tree clean, HEAD = a3a3bf1e ("fix(rules): bind the finished layer-6 keyword
list through the layer-7 P/T walk") on top of e28e4131. No rebase or merge was
in flight — the daemon had aborted both its attempts (rebase, then merge
fallback), both of which had conflicted on `internal/testutil/agentsdoc_test.go`.

`.cards` was present as a symlink to the shared corpus (`/home/sadams/projects/gorge/.cards`).

## Redone integration

`git merge main` — reproduced the same single conflict (AGENTS.md auto-merged
clean; only `internal/testutil/agentsdoc_test.go` was UU).

## Conflicted file: `internal/testutil/agentsdoc_test.go`

The conflict was comment-plus-constant only, inside the `knownApproximationRows`
block:

- **HEAD (branch) wanted:** comment crediting the branch's own row deletion —
  the (kw:Flanking) row, deleted by cli-20260923T060000Z-layers-pt7kw (the
  layer-7 P/T walk binds the finished layer-6 keyword list) — and
  `knownApproximationRows = 28`.
- **main wanted:** comment crediting main's row deletion — the (battle1) row,
  deleted by cli-20260923T060000Z-battle-defeated (CR 310.7 defender, combat
  damage to a planeswalker, CR 310.11 defeated exile-and-cast, real protector
  policy) — and `knownApproximationRows = 28`.

Both sides independently deleted ONE different row from a common base of 29
data rows and each lowered the constant to 28. The auto-merged AGENTS.md keeps
both deletions, so the merged table measures 27 data rows.

**Resolution (both intents kept, constant corrected for the combined deletion):**

- Comment now names all three deletions: (mtsp1) by 82d3ba68 (in the branch's
  history), (kw:Flanking) by this branch's ticket, (battle1) by main's
  cli-20260923T060000Z-battle-defeated.
- `knownApproximationRows` lowered to **27**. I measured this against the
  merged AGENTS.md directly: base (merge-base e28e4131) AGENTS.md has 30 `| `
  lines in the section = 29 data rows (header dropped by `rows[1:]`); the
  merged AGENTS.md has 28 `| ` lines = 27 data rows; and neither
  `kw:Flanking`, `(battle1)` nor `(mtsp1)` remains anywhere in the merged
  AGENTS.md. Keeping 28 would have passed `TestKnownApproximationsOnlyShrinks`
  (it only fails on growth) but would have left the register's constant one
  row too high — the deletion rule says the constant drops in the same commit
  that deletes the rows, and the merge is that commit for the combined state.

The merged AGENTS.md itself needed no edits: git combined both row deletions
cleanly and the (ap1) row main swapped in is present (line 231).

## Commands run

- `git merge main` → `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`
- Edit as above; `gofmt -l internal/testutil/agentsdoc_test.go` → no output (clean)
- `git add internal/testutil/agentsdoc_test.go && git commit --no-edit` →
  merge commit `e93e7337` ("Merge branch 'main' into
  wt/cli-20260923T060000Z-layers-pt7kw"), default merge message preserved;
  `git status --short` afterwards → empty (clean).
- `go test ./internal/testutil/` → `ok  github.com/adams-shaun/gorge/internal/testutil 1.183s`
- Ratchets after the merge, per the 2026-09-22 note:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok  github.com/adams-shaun/gorge/rules 0.829s`
- Behaviour goldens (system notes, done before DONE):
  - `go test ./internal/archtest/` → `ok ... 3.922s`
  - `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` → `ok ... 1.209s`

No golden, head or ratchet movement was needed by the resolution: the conflict
was metadata-only and no engine behaviour was chosen or changed by it.

## Notes / unsure

- The branch registers no new trigger `Mode$` and closes no ratchet entry, so
  the merge needed no `addedAfterTheSplit` or table edits.
- This report is tracked in git (as prior pipeline reports are), so it is
  committed separately after the merge commit to keep the tree clean.
- Per the brief I did not run TestHeads / full acceptance / `make sim` — those
  are the daemon's post-DONE gates.

## Issues

None found beyond the conflict itself.
