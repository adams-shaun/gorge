# Report — agent-20260923T072310Z-8affc438

## Task

Verify and pin the already-landed fix for "multiple MustBlock creatures may
block one attacker" (CR 509.1c). The brief explicitly states this defect is
already fixed on current main by `4fe4eadcc546a45242df5ccf987ef135ed4de296`
("fix(rules): satisfy MustBlock with legal whole blocking teams"), that the
regression test already exists, and that this is a **verification-only**
task: no production change, no new test, no ratchet edit, no
approximation-table edit.

## What changed and why

**No production code changed. No test added. No ratchet or AGENTS.md row
touched.** The brief asked only to verify the landed regression test on the
real corpus, and it passes.

The workspace already carries the fix and its test:

- `decision/block_required.go` — `Decision.blockRequiredCore`, the
  whole-team solver.
- `rules/combat.go` — `askBlockers` / `validateBlockers` consume
  `d.BlockRequiredTeam()` and `RequiredQuota()`.
- `rules/mustblock_min_team_test.go` — `TestMustBlockTwoWatchdogsShareAttacker`
  at line 59.

## Workspace facts re-checked (brief says trust these; I spot-checked)

- `.cards` exists in the worktree as a symlink to the shared corpus
  (`lrwxrwxrwx ... .cards -> /home/sadams/projects/gorge/.cards`), so the
  corpus-backed test ran rather than skipped. Confirmed non-vacuous: the
  `-v` run takes 0.60s (a skipped corpus run on this repo measures ~ms).
- Fix commit present and its file list matches the brief:
  `decision/block_required.go`, `decision/decision.go`,
  `decision/required.go`, `rules/combat.go`, `rules/mustblock_min_team_test.go`
  (430 insertions, 139 deletions).
- The reported choke-point names `budgetedBlockMatching` and
  `mustBlockAssignmentsIn` are **not present** in the current tree:
  `/usr/bin/grep -rn 'budgetedBlockMatching\|mustBlockAssignmentsIn' rules/ decision/`
  returns no matches. The complete maximum-team search is centralized in
  `decision.Decision.blockRequiredCore`, as the brief states.
- Corpus prevalence holds at 27 files:
  `/usr/bin/grep -rlE 'Mode\$ MustBlock' .cards/cardsfolder | wc -l` → `27`.

## Gates run (exact commands + real output)

### 1. Exact targeted command from the brief

```
$ go test -run 'TestMustBlockTwoWatchdogsShareAttacker$' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	0.600s
```

Verbose confirmation that the test actually ran (not skipped):

```
$ go test -v -run 'TestMustBlockTwoWatchdogsShareAttacker$' ./rules/ 2>&1 | tail -15
=== RUN   TestMustBlockTwoWatchdogsShareAttacker
--- PASS: TestMustBlockTwoWatchdogsShareAttacker (0.60s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.617s
```

### 2. `go test ./internal/archtest/` (no allowlist edits)

```
ok  	github.com/adams-shaun/gorge/internal/archtest	3.667s
```

### 3. `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`

```
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.826s
```

No re-pin was needed, so no attribution is owed.

## Fails without the fix

Not applicable: no new test was added (the brief explicitly requested none),
so the "revert the fix and show the new test fails" clause does not apply.
The existing test `TestMustBlockTwoWatchdogsShareAttacker` is the one the fix
commit `4fe4eadc` itself added, and it passes on current main.

## Done-means checklist

- [x] Ran the exact targeted command and pasted its output; no production
      change was needed because the test does not expose a regression.
- [x] No new test added (as requested).
- [x] `go test ./internal/archtest/` passes, no allowlist edits.
- [x] `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`
      passes.
- [x] This report exists with an `## Issues` section.

## Issues

None found, none fixed. To state it explicitly as the brief requires: **this
defect is already fixed by `4fe4eadc`, and no other defect was investigated or
fixed in this round.** The test's `knownUnsupported` / acceptance census /
Known-approximations table are all unchanged by this verification.

## Deviations from the brief

None. All brief instructions followed as written.
