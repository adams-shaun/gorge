# Merge-conflict resolution — task cli-20260922T225141Z-48972afc

## Situation found

`git status` was CLEAN on entry — no rebase or merge in flight (the daemon's
earlier attempt had left nothing behind). The branch tip was `7c4182ff`
("fix(rules): resolve mulligan redraws at the end of the declaration pass"),
one commit past the merge base `0104252d`; main had advanced to `2040e5d9`,
39 commits ahead. The owed integration was a merge of main into the branch
(rebase is forbidden in a seat).

`.cards` was **present** — a real symlink to
`/home/sadams/projects/gorge/.cards`, target exists — so every corpus-backed
ratchet below ran for real rather than skipping.

## Conflicted files and resolutions

`git merge main` produced **one** content conflict: `internal/testutil/agentsdoc_test.go`.
`AGENTS.md` auto-merged (it carried no textual conflict — each side had deleted
a different row).

### `internal/testutil/agentsdoc_test.go` (conflict: the `knownApproximationRows` constant)

Both sides changed the register constant:

- **HEAD (branch):** `knownApproximationRows = 49`. The branch base's table was
  50 rows; the mulligan ticket deleted one row and lowered 50 → 49.
- **main:** `knownApproximationRows = 44`, with a comment recording main's own
  closures (non<X> `d56e404f`, NameCard ChooseFromList$/AtRandom$ `78d3b764`,
  the battle protector row `f76f59fd`, plus pc1/each1/CR 616.1) and the
  layer-4 closure from **another** merge of this same branch (`63c07260`),
  which main had already absorbed.

Neither number is the merged count. The merged table was **measured** (same
method the test uses: lines starting `"| "` inside `## Known approximations …
next "## "`, minus the header): **43 data rows**. HEAD carried 48, main 44,
and the union of both sides' deletions (mulligan row on the branch; main's
four) gives 43. Verified the four closed rows are absent from the merged
`AGENTS.md`: the old mulligan row (`mulligan declaration's REDRAW`), the
`non<X> negation` row, the `layer-4 type grants reach only` row and the
`NameCard asks over` row — all grep to 0 occurrences.

**Resolution:** kept main's commented shape (which explains the other
closures) and wrote the merged count accurately:

    // The merged table measures 43 rows: main's closures merged here
    // (non<X> via d56e404f, NameCard ChooseFromList$/AtRandom$ via 78d3b764,
    // the battle protector row via f76f59fd, and the earlier pc1/each1/CR
    // 616.1 closures), plus this branch's own mulligan-redraw deferral
    // (fix 7c4182ff), which deletes the "mulligan declaration's REDRAW
    // resolves immediately" row.
    knownApproximationRows = 43

This is the union of the two sides' intents, not a redesign. The row-count
gate passes (`TestKnownApproximationsOnlyShrinks`, `TestKnownApproximationRowsAreShort`
— the package test is green).

## Integration failures found after the merge — and why they are part of resolving it

The merge itself is a **semantic** integration conflict, not just a textual
one. main added a whole feature after this branch forked — the hypothetical
chance planner (`rules/chance.go`, `rules/chance_test.go`) — and three of its
tests observe a mulligan's REDRAW inside the mulligan submit, because on main
`handleMulligan` still resolves the redraw immediately. This branch's reviewed
fix deliberately *defers* the redraw to the declaration-pass boundary (CR
103.4/103.5, `resolveMulliganRedraws`), which is the entire purpose of the
ticket. Both sides kept, so main's three tests now observe the wrong moment.

The three failures, measured on the merged tree before the fix below:

    --- FAIL: TestHypotheticalPlannerControlsMulliganShuffle (0.00s)
        chance_test.go:223: mulligan shuffle=[9 2 11 6 5 12 10 3 4 7 1 8] want []
    --- FAIL: TestHypotheticalReplayAndCloneOwnChanceState (0.00s)
        chance_test.go:448: mulligan did not consume chance
    --- FAIL: TestHypotheticalSubmitFailurePoisonsOnlyThatBranch (0.00s)
        chance_test.go:504: error = <nil>

I confirmed the underlying behaviour is intact by instrumenting a scratch test:
after the mulliganing seat declares and the **rest of the pass keeps**, the
planner's Ordinal-1 callback fires with exactly the context main's test
expects (`hand=0 lib=12`) and produces the planned permutation. Only the
*when* moved (from the submit to the pass boundary), exactly as CR 103.4/103.5
requires.

`rules/chance_test.go` was NOT a conflicted file. This is a documented
deviation from "do not touch files the conflict does not involve": leaving it
red would wedge the daemon's full gate, and reverting the branch fix would
destroy the ticket. The integration change is test-only and preserves each
test's intent:

- **TestHypotheticalPlannerControlsMulliganShuffle** — records the mulliganing
  seat, then drives the rest of the pass with a keep (new `submitPregameKeep`
  helper) until the planner callback has run, then asserts `lastShuffle` equals
  the planned order. The planner still controls the mulligan shuffle.
- **TestHypotheticalReplayAndCloneOwnChanceState** — after the clone's mulligan
  submit, asserts the source engine is untouched, then completes the clone's
  pass and only then asserts chance was consumed; completes `e`'s and the
  replay's pass identically before the transcript comparison. The clone/chance
  ownership contract is unchanged.
- **TestHypotheticalSubmitFailurePoisonsOnlyThatBranch** — the mulligan submit
  now succeeds (the poisoned draw is consumed by the deferred redraw); the
  pass-completing keep is the call that surfaces the `bound` chance failure;
  the poison/head-immutability and base-vs-branch assertions follow. The
  chance-failure boundary contract is unchanged.

A new helper `submitPregameKeep` documents the deferral in one place.

## Commits produced

- `61779fd9` — `Merge branch 'main' into wt/cli-20260922T225141Z-48972afc`
  (default merge message; parents `7c4182ff` + `2040e5d9`). Contains the
  `agentsdoc_test.go` resolution.
- `f0bc40a3` — `test(rules): drive the deferred mulligan redraw in the chance
  planner tests` (the `rules/chance_test.go` integration; no engine change).

## Commands run and output (real)

`.cards`: **present** (real symlink, target exists) — corpus tests ran.

    $ git merge main
    Auto-merging AGENTS.md
    Auto-merging internal/testutil/agentsdoc_test.go
    CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go

Merged-row measurement (test's own method):

    raw rows: 44 => data rows: 43
    HEAD (branch) table: 48 rows; main table: 44 rows; merged: 43 rows

Conflict-resolution gate — the conflicted file's package:

    $ go test -run 'TestKnownApproximation' ./internal/testutil/
    ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
    $ go test ./internal/testutil/
    ok  	github.com/adams-shaun/gorge/internal/testutil	1.475s

The three main-planner tests, before and after the integration fix:

    before: 3 FAIL (above)
    $ go test -run 'TestHypothetical' ./rules/ -v
    --- PASS: TestHypotheticalPlannerControlsMulliganShuffle (0.00s)
    --- PASS: TestHypotheticalReplayAndCloneOwnChanceState (0.00s)
    --- PASS: TestHypotheticalSubmitFailurePoisonsOnlyThatBranch (0.00s)
    (... every other TestHypothetical* PASS)
    ok  	github.com/adams-shaun/gorge/rules	0.007s

Post-merge ratchets (the command the brief names):

    $ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
    ok  	github.com/adams-shaun/gorge/rules	0.753s

Broader targeted mulligan/pregame suite in `rules/`:

    $ go test ./rules -run 'Mulligan|Pregame|Bottoming|Hypothetical|Redraw|StartingPlayer|Kept|Keep'
    ok  	github.com/adams-shaun/gorge/rules	0.770s

Mandatory goldens outside `rules/` (per system context):

    $ go test ./internal/archtest/
    ok  	github.com/adams-shaun/gorge/internal/archtest	3.013s
    $ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
    ok  	github.com/adams-shaun/gorge/cmd/botbench	1.101s

Format / build:

    $ gofmt -l internal/testutil/agentsdoc_test.go rules/chance_test.go
    (no output)
    $ go run ./cmd/gentypes -check
    (no output)
    $ go build ./rules/ ./internal/testutil/
    (clean)

Final state:

    $ git status --short          → (empty, clean)
    $ grep -rn '^<<<<<<<|^>>>>>>>' → (no conflict markers)

## TestHeads — expected branch movement, deliberately NOT edited

    $ go test -run 'TestHeads$' ./rules/
    --- FAIL: TestHeads (1.71s)
        heads_test.go:1323: 4 seats: chain head acce7d850cfb176a, golden 20028059e8c88ec3
        heads_test.go:1323: 6 seats: chain head 3bd695df72d9d4c9, golden 400d8d9ae2777ded
        heads_test.go:1323: 8 seats: chain head 5c90b1b3a0b38f25, golden 5e988854bf022347

These are **exactly** the values the branch's own ticket measured and
attributed (`acce7d850cfb176a` / `3bd695df72d9d4c9` / `5c90b1b3a0b38f25`; 2
seats unmoved at `19a4893657e5d549`). My resolution introduced no engine
behaviour change, so nothing from main moved them further. Per the system
context, `rules/heads_test.go` is not edited by agents — the orchestrator
re-pins it at the gate from measured values (FL-107), and the ticket's report
(`.ds4/report-t1.md`) already carries the attribution. NOT a blocker for this
resolution.

## Uncertainties / notes

- The `rules/chance_test.go` edit is the one deliberate scope extension, forced
  by main's new feature observing the behaviour this ticket changes. It is
  test-only; the alternative (leave 3 tests red, or revert the reviewed fix)
  both fail the brief.
- The `knownApproximationRows` number is the measured merged count (43), not
  either side's stale constant; I verified the four closed rows are gone.

## Issues

- No new defects found beyond the integration above.
- Coverage note (already recorded by the ticket): the repo-deck acceptance
  suite runs exactly one mulligan per game and at 2 seats that mulliganer is
  the last declarer, so the 2-seat acceptance golden does not exercise the
  deferred-redraw interleaving at all; the branch's
  `rules/mulligan_redraw_order_test.go` covers the multi-declarer shapes
  directly.
- Out of scope and untouched: the adjacent AGENTS.md row "The starting player
  is uniformly random but the toss winner never CHOOSES" remains in the table.
