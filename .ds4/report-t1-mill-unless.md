# Report — agent-20260923T002719Z-c0b56143

**Ticket:** `UnlessCost$ Mill<N>` is a hard decline (Deep Spawn always
sacrifices). **Status:** DONE.

## What changed and why

Deep Spawn's real compiled upkeep trigger is
`SVar:TrigUpkeep:DB$ Sacrifice | UnlessPayer$ You | UnlessCost$ Mill<2>`
(`.cards/cardsfolder/d/deep_spawn.txt:7`). Before this change
`ParseUnlessCost` rejected `Mill<N>`, the `unless_pay` arm in
`rules/resolution.go` read that strict-parser failure as a hard decline, and
the sacrifice always resolved even when the payer chose Pay with a stocked
library.

Per file:

- **`rules/mana.go` — `ParseUnlessCost`.** Added a fixed `Mill<N>` case,
  matching through the SAME `millCost` regex (`^Mill<(\d+)>$`) the ordinary
  cast/activation `ParseCost` path uses, with the same int32 bounds check.
  Malformed/dynamic spellings (`Mill<X>`, `Mill<>`, `Mill<2`, `Mill 2`,
  `Mill<-1>`, `Mill<2/you>`) do not match and stay strict failures. Updated
  the function's doc comment to name the newly accepted token.
- **`rules/stack.go` — `payUnlessCost`.** Added
  `e.payMillCost(p, cost.Mill)` after every payability check (mana/life,
  energy, counters, draws) and alongside the other charges, so the unless
  cost and the ordinary cast/activation cost settle mill through the ONE
  shared site. `payMillCost` (`rules/cast.go`) snapshots the payer's
  available library prefix and emits deterministic top-first `MoveZone`
  events, clamping to the available cards — CR 701.13a's partial/empty
  library rule. Updated the doc comment.
- **`rules/unless_unpriceable_test.go` — `TestUnlessCostStrictParsePopulation`.**
  Removed `Deep Spawn` from the sorted expected population list (its real
  `UnlessCost$` is now priceable) and updated the surrounding doc comment.
  The test's balanced `len(got) != len(want)` plus per-index sorted
  comparison (bidirectional) is preserved unchanged.
- **`rules/deep_spawn_mill_unless_test.go` (new).** The regression suite:
  `TestParseUnlessCostMill` (parser boundary), `TestDeepSpawnUnlessMillCost`
  (real-corpus end-to-end: Pay offered, top two library cards mill through
  events, Deep Spawn survives), and the short/empty-library halves.

No offerability change was needed: a Mill-only unless cost has no mana part,
so `unlessCostPayable` reaches the `!cost.hasManaPayment()` branch and
returns payable (mill is choice-free at any library size). Mill is not
choice-bearing, so it correctly stays on the synchronous `payUnlessCost`
path and never routes through `beginUnlessPayment`. No new decision kind, no
event/ordinal change, no Known-approximations row added or grown (this
defect was not a row in that register).

## Structural / "fix the class" note

The parser reuses the existing `millCost` regex rather than introducing a
second Mill grammar, and payment goes through the existing `payMillCost`, so
the ordinary `Cost$ Mill<N>` and the unless `UnlessCost$ Mill<N>` cannot
diverge. A future `Mill<N>` in any other unless-cost context (composed with
mana, in a different payer role) is covered by the same two sites with no
new branch: the parse case is token-generic and `payUnlessCost` settles
whatever `cost.Mill` carries. This is the single corpus carrier today
(`/usr/bin/grep -rlE 'UnlessCost\$[^|]*Mill<' .cards/cardsfolder | wc -l`
→ 1); the brief's measured prevalence held.

## Gates run (real output)

### Targeted rules tests (brief's "Done means")

```
$ go test -run 'TestDeepSpawnUnlessMillCost|TestUnlessCostStrictParsePopulation' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	0.651s
```

Verbose confirmation that the corpus loaded and nothing skipped:

```
$ go test -v -run 'TestDeepSpawnUnlessMillCost|TestParseUnlessCostMill|TestUnlessCostStrictParsePopulation' ./rules/ 2>&1 | tail -30
=== RUN   TestParseUnlessCostMill
--- PASS: TestParseUnlessCostMill (0.00s)
=== RUN   TestDeepSpawnUnlessMillCost
--- PASS: TestDeepSpawnUnlessMillCost (0.56s)
=== RUN   TestDeepSpawnUnlessMillCostShortLibrary
--- PASS: TestDeepSpawnUnlessMillCostShortLibrary (0.00s)
=== RUN   TestDeepSpawnUnlessMillCostEmptyLibrary
--- PASS: TestDeepSpawnUnlessMillCostEmptyLibrary (0.00s)
=== RUN   TestUnlessCostStrictParsePopulation
--- PASS: TestUnlessCostStrictParsePopulation (0.04s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.634s
```

Wider targeted unless/mill regression (same package, one extra filtered
invocation):

```
$ go test -run 'Unless|Mill' ./rules/ 2>&1 | tail -15
ok  	github.com/adams-shaun/gorge/rules	0.759s
```

### Formatting and generated types

```
$ gofmt -l rules/mana.go rules/stack.go rules/unless_unpriceable_test.go rules/deep_spawn_mill_unless_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output)
```

### Behaviour goldens outside rules/

```
$ go test ./internal/archtest/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/internal/archtest	3.469s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -8
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.219s
```

No allowlist edits. The botbench split did NOT move: Deep Spawn is not in any
repo deck (`grep -rn 'Deep Spawn' internal/testutil/decks/*.json` → no
matches), so no attribution/re-pin was required.

### `.cards` presence

`.cards` was already present as a symlink to
`/home/sadams/projects/gorge/.cards` when this worktree was created; the
corpus-dependent tests ran (0.56s Deep Spawn test, not a skip) rather than
skipping.

## Fails without the fix

Reverted only the two non-test hunks (the `millCost` case in
`rules/mana.go` and the `payMillCost` call in `rules/stack.go`), copied the
fixed files to `.ds4/scratch/{mana,stack}.go.fixed` first, ran the targeted
tests, then restored the files and confirmed `cmp` byte-identity against the
scratch copies.

```
$ go test -run 'TestDeepSpawnUnlessMillCost|TestParseUnlessCostMill' ./rules/ 2>&1 | tail -40
--- FAIL: TestParseUnlessCostMill (0.00s)
    deep_spawn_mill_unless_test.go:25: ParseUnlessCost(Mill<2>) = !ok, want the fixed mill cost accepted
--- FAIL: TestDeepSpawnUnlessMillCost (0.62s)
    deep_spawn_mill_unless_test.go:130: expected a 2-option Pay/Don't pay ask, got [{Index:0 Kind:mode Label:Don't pay Obj:81 ...}]
--- FAIL: TestDeepSpawnUnlessMillCostShortLibrary (0.00s)
    deep_spawn_mill_unless_test.go:165: short library exposed 1 options, want the full Pay/Don't pay pair
--- FAIL: TestDeepSpawnUnlessMillCostEmptyLibrary (0.00s)
    deep_spawn_mill_unless_test.go:187: empty library exposed 1 options, want the full Pay/Don't pay pair
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.643s
FAIL
```

Then:

```
$ cp .ds4/scratch/mana.go.fixed rules/mana.go && cp .ds4/scratch/stack.go.fixed rules/stack.go
$ cmp .ds4/scratch/mana.go.fixed rules/mana.go && cmp .ds4/scratch/stack.go.fixed rules/stack.go
RESTORED byte-identical
$ go test -run 'TestDeepSpawnUnlessMillCost|TestParseUnlessCostMill|TestUnlessCostStrictParsePopulation' ./rules/
ok  	github.com/adams-shaun/gorge/rules	(cached)
```

(The final `(cached)` result is the test cache recognising the byte-identical
restored tree from the earlier green run — the correct signal that the fix was
restored exactly.)

## Test-precondition discipline

Each new test asserts the precondition its real assertion depends on:
`TestDeepSpawnUnlessMillCost` asserts the library has 5 cards, Deep Spawn is on
the battlefield, the ask is a 2-option Pay/Don't-pay pair whose two labels
differ, and the payer is the controller (UnlessPayer$ You), before asserting
two specific top cards moved and the creature survived. The short/empty tests
assert the library size they set up (1, then 0) and that the full 2-option ask
is still exposed. `TestParseUnlessCostMill` asserts the parsed part count and
amount and that a composed cost keeps generic+Mill. The "nothing happens"
direction (not sacrificed) is paired with the positive mill event assertions,
so a feature left unregistered fails the mill assertion rather than passing
vacuously.

## Known-approximations register

This defect was NOT a row in the frozen "Known approximations" table and no
row was added, grown or rewritten. `knownApproximationRows` in
`internal/testutil/agentsdoc_test.go` is unchanged.

## Deviations from the brief

None. All "Done means" items are satisfied.

## Issues

No additional unresolved defect was found. The measured corpus finding is
limited to the one Deep Spawn `UnlessCost$ Mill<2>` carrier, as the brief
stated. No CR-lane test is proposed.
