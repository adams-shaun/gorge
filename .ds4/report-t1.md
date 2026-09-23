# Report — honor `S:Mode$ IgnoreLegendRule` in the CR 704.5j SBA

Ticket `agent-20260918T232250Z-29aed5d6`.
Commit `5b45f3b3` (`feat(rules): honor S:Mode$ IgnoreLegendRule in the CR 704.5j legend SBA`).

## What changed and why

### `rules/sba.go` (+37)

`(*Engine).legendGroups()` gathers CR 704.5j duplicate sets. It now:

1. collects the live statics once, in the canonical deterministic order, with
   `exempt := e.activeStatics("IgnoreLegendRule")`;
2. skips any candidate for which `e.legendRuleExempt(exempt, id)` is true,
   BEFORE it enters grouping.

Because the skip happens before the `seen[name]` grouping, an exempt permanent
cannot form a duplicate set: two exempt same-named legends produce no set, and
a set that loses members to an exemption can fall below the two-member
threshold and drop entirely. This is the single collection choke point the
existing CR 704.5j path uses (`destroyLethalDamage` at `rules/sba.go:856` is
the only caller of `legendGroups`), so it preserves the deterministic
scan-order grouping and the existing controller-choice behavior unchanged.

New helper `(*Engine).legendRuleExempt(statics []staticView, id state.ObjID)`:

- evaluates each static's `ValidCard$` with `e.matchesSpec(spec, id,
  e.staticSpecCtx(sv))` — the static's OWN source/controller context, so
  `Creature.YouCtrl` is scoped to the static's controller rather than the
  duplicate set's (this is the second scope boundary);
- treats an absent/empty `ValidCard$` as Forge's "all cards" spelling (Mirror
  Gallery's unconditional "The legend rule doesn't apply."), exempting every
  candidate;
- takes the statics slice already collected so the walk is one pass, not one
  `activeStatics` call per candidate.

This is a structural fix, not a Council-of-Reeds special case: every
`IgnoreLegendRule` carrier with a `ValidCard$` is covered by the same
`activeStatics` + `matchesSpec` route, so the next sibling (Mirror Box,
Cadric, Sliver Gravemother, The Master Multiplied, Sakashima, Spider Verse, …)
is covered with no further change.

### `rules/ignorelegendrule_test.go` (new, 5 tests)

Built on `layerEngine`/`onBoard`/`onBoardCard`/`corpusCard` and the existing
`legendPending`/`submitKeep` helpers (all reused, none redefined). Every test
asserts its own preconditions through `assertCouncilExemptionLive` /
`assertLegendTwinPair` (source and compared permanents on the battlefield,
legendary status, name, controller, distinct ids, `ValidCard$` string, and
that the exemption helper actually admits/rejects the objects the assertion
depends on). The "nothing happens" positive test also asserts the exemption
walk is what suppressed the choice (the static is collected and admits both
objects) and that no `IgnoreLegendRule` Note stands in for it.

1. `TestIgnoreLegendRuleExemptsMatchingCreatures` — positive (Council + two
   same-named legendary creatures it controls): no choice, no batch, both stay.
2. `TestIgnoreLegendRuleDoesNotExemptNoncreatures` — boundary (a legendary
   artifact pair is still asked while an exempt creature pair on the same
   board is not).
3. `TestIgnoreLegendRuleDoesNotExemptOtherPlayersCreatures` — boundary
   (opponent's same-named creature pair still asks seat 1).
4. `TestIgnoreLegendRuleExemptionEndsWhenSourceLeaves` — REAL `MoveZone`
   departure of Council; asserts `activeStatics("IgnoreLegendRule")` is empty,
   then the ordinary CR 704.5j choice returns and settles.
5. `TestIgnoreLegendRuleEventStreamIsDeterministic` — the same mixed board run
   twice in fresh engines from the same seed; compares event kind stream and
   chain head.

Boundary tests 2 and 3 deliberately include an exempt pair placed FIRST in
battlefield order, so with the fix reverted the engine parks its ask over the
exempt pair and the tests fail too — every one of the five fails without the
fix (see below).

## Gate commands and real output

Targeted (Done means), `.ds4/scratch/t2.log`:

```
$ go test -run 'TestIgnoreLegendRule|TestParamCensusScanIsComplete' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	0.755s
```

Formatting / types (the Go half of `make lint`):

```
$ gofmt -l rules/sba.go rules/ignorelegendrule_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output)
$ go vet ./rules/
(no output)
```

Behaviour goldens outside `rules/`:

```
$ go test ./internal/archtest/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/internal/archtest	3.755s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.322s
```

No allowlist edits; no re-pin needed.

## Fails without the fix

Backed up `rules/sba.go` to `.ds4/scratch/sba.go.bak`, changed the filter to
`if false && e.legendRuleExempt(exempt, id)` (the vardecl stays, so the build
compiles), ran the one test command, then restored with `cp` and verified
`cmp` reported the file byte-identical. No `git stash`/`git checkout` used.

```
$ go test -run 'TestIgnoreLegendRule' ./rules/ 2>&1 | tail -20
--- FAIL: TestIgnoreLegendRuleExemptsMatchingCreatures (0.60s)
    ignorelegendrule_test.go:104: a decision choose is pending under a live IgnoreLegendRule exemption
--- FAIL: TestIgnoreLegendRuleDoesNotExemptNoncreatures (0.00s)
    ignorelegendrule_test.go:148: legend option 0 names obj 82, want 84 (battlefield order)
--- FAIL: TestIgnoreLegendRuleDoesNotExemptOtherPlayersCreatures (0.00s)
    ignorelegendrule_test.go:186: legend choice asked seat 0, want the duplicates' controller seat 1
--- FAIL: TestIgnoreLegendRuleExemptionEndsWhenSourceLeaves (0.00s)
    ignorelegendrule_test.go:215: a decision choose is pending while the exemption is live
--- FAIL: TestIgnoreLegendRuleEventStreamIsDeterministic (0.00s)
    ignorelegendrule_test.go:256: legend option 0 names obj 82, want 84 (battlefield order)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.616s
FAIL
```

Restore proof:

```
$ cp .ds4/scratch/sba.go.bak rules/sba.go && cmp rules/sba.go .ds4/scratch/sba.go.bak
RESTORED-IDENTICAL
$ go test -run 'TestIgnoreLegendRule|TestParamCensusScanIsComplete' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.755s
```

## Brief premises re-measured

- Corpus prevalence held at 11: `/usr/bin/grep -rl 'Mode$ IgnoreLegendRule'
  .cards/cardsfolder | wc -l` → `11`.
- The only `legendGroups` caller is `destroyLethalDamage` (`rules/sba.go:856`).
- `.cards` was PRESENT as a symlink to
  `/home/sadams/projects/gorge/.cards` (I did not need to create it), so the
  corpus-backed tests genuinely ran. `corpusCard(t, "Council of Reeds")`
  Fatals on a missing registry entry; it did not fire.
- No repo deck (`internal/testutil/decks/`) contains any of the 11 carriers —
  so botbench is unaffected and was not re-pinned.

## Known-approximations table

Not touched. My ticket does not close the row at `AGENTS.md:218` ("The CR
704.5j legend rule keeps the first duplicate … with no controller choice"):
that row's subject is the controller choice, which commit `74870371` already
landed and which the brief explicitly puts out of scope. I did not delete a
row I did not close, and did not add or grow any row. (The row is nonetheless
STALE at main — it names a function `legendCasualties` that no longer exists
and claims no controller choice while the SBA asks for one; flagged under
Issues for its owner.)

## Deviations from the brief

- None in scope. The implementation is the single collection-path filter the
  brief describes, plus empty-`ValidCard$` global handling (needed for Mirror
  Gallery's real carrier text and harmless for all others).

## Issues

1. **Conditional `IgnoreLegendRule` carriers ignore their condition.**
   `Brothers Yamazaki` and `Syr Joshua and Syr Saxon` pair `ValidCard$` with
   `IsPresent$`/`PresentCompare$ EQ2` ("If there are exactly two permanents
   named … the legend rule doesn't apply TO THEM"). My helper reads only
   `ValidCard$`, so those two exempt their matching permanents even when the
   condition does not hold (e.g. three copies on the battlefield, where the
   card text says the rule DOES apply). Prevalence: 2 of the 11
   `Mode$ IgnoreLegendRule` files. Symptom, file and function:
   `rules/sba.go` `(*Engine).legendRuleExempt`. The fix is to evaluate the
   static's `IsPresent$`/`PresentCompare$` gate through the shared static
   presence evaluator before admitting the exemption. Neither carrier is in a
   repo deck. File separately if wanted.
2. **`AGENTS.md:218` (CR 704.5j row) is stale independent of this ticket.** It
   references `rules/sba.go`'s `legendCasualties`, deleted when the controller
   choice landed (`74870371`), and asserts "no controller choice" while
   `parkLegendChoice`/`askLegendChoice` pose exactly that. It should be
   deleted with `knownApproximationRows` lowered from 18 to 17, by the ticket
   that owns that row — not silently here. A CR-lane test already exercises
   the choice, so this is not an invisible defect.
