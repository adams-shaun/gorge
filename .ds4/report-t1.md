# Report — agent-20260923T042552Z-0936e140

Implement DigUntil withheld rider semantics.

## Summary

`effects/cardflow.go` `effDigUntil` used to parse a withhold list, emit one
loud Note per parameter, and run only the core reveal-until move. All riders in
the brief except `DigZone$ PlanarDeck` are now implemented; the withhold list
and Note emission shrank accordingly.

NOTE ON ROUND HISTORY: an earlier t1 run was lost mid-task (per
`.ds4/findings-t1.md`). It had already implemented the riders and committed them
as `c749b700` (`feat(effects): implement DigUntil withheld rider semantics`).
I audited that work, found and fixed one real defect in its tests (below), added
the three real-carrier tests the brief names but the earlier run omitted, and
re-ran every gate myself. The report below covers the whole deliverable, not
just my delta.

## What changed, per file

### `effects/cardflow.go` (committed in `c749b700`, part of this ticket)
`effDigUntil` (~line 1744 onward):

- **`Amount$ <token>` non-literal** — resolves the token as an SVar name via
  the new `digUntilAmountSVar` (reads `Ctx.SVars[token]` and runs
  `EvalCountOK`), the same read `effDig`'s `DigNum$ X` arm uses. `X`, `MassX`,
  `Y`, `VoteNum` bodies (`Count$xPaid`, `Count$Valid …`, `Remembered$Amount`,
  `Number$<n>`) all resolve. An absent/unresolvable SVar keeps its loud Note
  and amount 1 (fail-safe).
- **`Shuffle$ True`** — after the found move and revealed-rest moves, shuffles
  the dug player's library (`h.ShuffleLibrary(p, …)` + one Secret
  `events.Shuffle`), the exact `effShuffle` contract.
- **`ShuffleCondition$ NoneFound`** — restricts that shuffle to a scan that
  found nothing; any other value is withheld loudly.
- **`NoMoveFound$ True`** — skips the found-card move; the card stays in the
  library (and a `FoundLibraryPosition$ -1` still bottoms it).
- **`FoundLibraryPosition$`** — `-1` = one library-to-library `MoveZone`
  (bottom); `0`/absent = top = the card never left, no event. Any other value
  is withheld loudly.
- **`ImprintFound$` / `ImprintRevealed$`** — accumulate the found / all-revealed
  cards across the player walk and emit one `events.Imprint` on the resolving
  source after the walk, on the Seek `<Text:"seek-found">` list rather than the
  ordinary `Imprinted` list. This is the brief's sanctioned fallback: the
  ordinary list's CR 607.2a exile-only reader (`effects/context.go`
  `imprintPileTargets`) would hide a card in Exile-from-library / on the
  battlefield, which is exactly what Venture Forth's `Defined$ Imprinted |
  Origin$ Exile` continuation needs. Both real carriers (Venture Forth, Part in
  Friendship) verify against the association.
- **`NoneFoundDestination$` / `NoneFoundLibraryPosition$`** — when the scan
  found nothing, the revealed pile takes these instead of
  `RevealedDestination$`/`RevealedLibraryPosition$`.
- **Still withheld (fail-safe + loud Note):** `DigZone$` (every corpus value is
  `PlanarDeck`; no planar tier), an unresolvable `Amount$`, and any unmodelled
  value for a modelled key (`ShuffleCondition$` other than `NoneFound`,
  `Imprint*` other than `True`, non-`0`/`-1` positions).
- Helpers added: `digUntilAmountSVar`, `digUntilTrueFlag`.

### `effects/diguntil_aura_test.go` (committed in `c749b700`)
`TestDigUntilWithholdsUnsupportedParamsAndStillMoves` shrank its want list to
`{"Amount$ X", "DigZone$ PlanarDeck"}` (the SA carries no `SVar:X`, so
`Amount$ X` stays withheld; `DigZone$ PlanarDeck` stays withheld), keeping the
"core move still runs" assertion.

### `rules/paramcensus_test.go` (committed in `c749b700`)
Dropped the `digUntilWithheldRange` key-gathering-loop exemption and the
`scanRangeWhitelist` call site. The loops are gone: `ImprintFound`,
`ImprintRevealed`, `NoneFoundDestination`, `NoneFoundLibraryPosition` are now
read through `digUntilParamValue`/`digUntilTrueFlag` call sites and attributed
by the ordinary dynamic-key rule. This is the reclassification the brief asks
for — no other census edit. `grep -rn digUntilWithheldRange` returns nothing.

### `effects/diguntil_riders_test.go` (new file; `c749b700` + `85810ba1` + `ed321875`)
14 `TestDigUntil*` tests in the ticket's own file (a new file, per the
"new tests go in a new file" rule):

| rider | test |
|---|---|
| `Amount$` SVar | `…AmountSVarCountsMatchesToTheTally` (real Mass Polymorph), `…AmountSVarZeroRevealsNothing` (real Selvala's Stampede), `…AmountSVarUnresolvableStillWithholds` |
| `Shuffle$` / `ShuffleCondition$` | `…ShuffleShufflesTheDugLibrary`, `…ShuffleConditionNoneFoundOnlyShufflesOnAnEmptyScan` |
| `NoMoveFound$` / `FoundLibraryPosition$` | `…NoMoveFoundKeepsTheFoundCardInTheLibrary`, `…FoundLibraryPositionPlacesOrKeepsTheFoundCard` |
| `ImprintFound$` / `ImprintRevealed$` | `…ImprintFoundFeedsTheExileReader` (real Venture Forth), `…ImprintRevealedRecordsEveryRevealedCard` (real Part in Friendship) |
| `NoneFound*$` | `…NoneFoundBranchSwapsTheRevealedDestination` |
| re-entry | `…RidersEmitOnceAcrossTheOptionalAsk` |
| real carriers (added this round) | `…KindredSummonsAmountSVarCountsChosenTypeCreatures`, `…EmptyTheLaboratoryAmountSVarCountsRemembered`, `…TunnelVisionNoneFoundShufflesAndKeepsLibrary` |

**Defect found in the earlier run and fixed (`85810ba1`):** the earlier
`TestDigUntilNoMoveFoundKeepsTheFoundCardInTheLibrary` used
`FoundDestination$ Library`, so the found card stayed in the library whether or
not the rider ran — the test could not fail. Proven: with the rider reverted,
the original test still passed. Rewritten to `FoundDestination$ Hand` so the
rider is observable; with the rider reverted it now fails (evidence below).
`ed321875` removed one redundant post-`t.Fatalf` assertion in the same test.

Brief claim checked: the brief says empty_the_laboratory carries
`SVar:X:Count$xPaid`. Measured at the pin, `Empty the Laboratory`'s DigUntil
reads `Amount$ Y` with `SVar:Y:Remembered$Amount` — the `X:Count$xPaid` SVar
belongs to its Sacrifice sub. I tested the actual carrier (`Y:Remembered$Amount`).

## Fails without the fix

Every new/changed test is proven to fail with its rider reverted. Method:
copy `effects/cardflow.go` to `.ds4/scratch/`, neuter one rider, run, restore
with `cp` and verify with `cmp` (never `git checkout`/`stash`).

Batch 1 — `Shuffle$` + `Imprint*$` neutered
(`go test -count=1 -run 'TestDigUntilShuffle|TestDigUntilImprint|TestDigUntilAmountSVar' ./effects/`, exit 1):

```
--- FAIL: TestDigUntilAmountSVarZeroRevealsNothing (0.00s)
    diguntil_riders_test.go:169: Shuffle events = 0, want 1 (the DigUntil ran; Amount$ 0 only empties the scan)
--- FAIL: TestDigUntilShuffleShufflesTheDugLibrary (0.00s)
    diguntil_riders_test.go:208: Shuffle events = [], want exactly one Secret Shuffle
--- FAIL: TestDigUntilShuffleConditionNoneFoundOnlyShufflesOnAnEmptyScan (0.00s)
    diguntil_riders_test.go:238: Shuffle events = 0, want 1 when the scan found nothing (ShuffleCondition$ NoneFound)
--- FAIL: TestDigUntilImprintFoundFeedsTheExileReader (0.00s)
    diguntil_riders_test.go:322: found land zone = library, want battlefield via DBToPlay's Defined$ Imprinted reader
--- FAIL: TestDigUntilImprintRevealedRecordsEveryRevealedCard (0.00s)
    diguntil_riders_test.go:360: seek-found Imprint events = [], want one association of the revealed [3 4]
FAIL
```

Batch 2 — `Amount$` SVar, `NoMoveFound$`, `FoundLibraryPosition$`, `NoneFound*$`
neutered (exit 1):

```
--- FAIL: TestDigUntilAmountSVarCountsMatchesToTheTally (0.44s)
    diguntil_riders_test.go:133: found creature 6 zone = library, want battlefield (Amount$ MassX = 2, not 1)
--- FAIL: TestDigUntilFoundLibraryPositionPlacesOrKeepsTheFoundCard (0.00s)
    diguntil_riders_test.go:277: library = [3 4 5 6], want the found Aura 4 at the bottom (FoundLibraryPosition$ -1)
--- FAIL: TestDigUntilNoneFoundBranchSwapsTheRevealedDestination (0.00s)
    diguntil_riders_test.go:399: revealed card 3 zone = graveyard, want library (NoneFoundDestination$ Library)
FAIL
```

Batch 3 — the three carrier tests added this round, with `Amount$` SVar,
`Shuffle$` and `NoneFound*$` neutered (exit 1):

```
--- FAIL: TestDigUntilKindredSummonsAmountSVarCountsChosenTypeCreatures (0.40s)
    diguntil_riders_test.go:480: found Bear 4 zone = library, want battlefield (Amount$ X = 2, not 1)
--- FAIL: TestDigUntilEmptyTheLaboratoryAmountSVarCountsRemembered (0.00s)
    diguntil_riders_test.go:509: found Zombie 4 zone = library, want battlefield (Amount$ Y = 2, not 1)
--- FAIL: TestDigUntilTunnelVisionNoneFoundShufflesAndKeepsLibrary (0.00s)
    diguntil_riders_test.go:540: Shuffle events = 0, want 1 (ShuffleCondition$ NoneFound with nothing found)
FAIL
```

NoMoveFound fix proof, rider reverted (exit 1):

```
--- FAIL: TestDigUntilNoMoveFoundKeepsTheFoundCardInTheLibrary (0.00s)
    diguntil_riders_test.go:257: found Aura zone = hand, want library (NoMoveFound$ True)
FAIL
```

`cmp effects/cardflow.go .ds4/scratch/cardflow.go.fixed` printed
"restored byte-identically" after each revert.

## Gates (exact commands + real output)

Worktree fixture check: `.cards` was already present as a symlink to
`/home/sadams/projects/gorge/.cards` (found, not created), so the corpus tests
really ran (the `effects` run took ~0.4 s with corpus lookups, not a skip).

```
$ go build ./...
build ok

$ go test -count=1 -run 'TestDigUntil' ./effects/
ok  github.com/adams-shaun/gorge/effects  0.414s
# 24 --- PASS TestDigUntil* (verbose list below, abridged):
#   WithholdsUnsupportedParams, AuraEntryAsksForBearer, AuraCanEnchantOpponentsCreature,
#   RememberFoundDoesNotRetainTriggerCapture, RememberFoundAndRevealedPreserveRevealedPrefix,
#   AmountSVarCountsMatchesToTheTally, AmountSVarZeroRevealsNothing,
#   AmountSVarUnresolvableStillWithholds, ShuffleShufflesTheDugLibrary,
#   ShuffleConditionNoneFoundOnlyShufflesOnAnEmptyScan, NoMoveFoundKeepsTheFoundCardInTheLibrary,
#   FoundLibraryPositionPlacesOrKeepsTheFoundCard, ImprintFoundFeedsTheExileReader,
#   ImprintRevealedRecordsEveryRevealedCard, NoneFoundBranchSwapsTheRevealedDestination,
#   RidersEmitOnceAcrossTheOptionalAsk, KindredSummons…, EmptyTheLaboratory…, TunnelVision…,
#   RevealsUntilTheMatchMovesFoundAndRest, DefaultDestinationsAreHandAndStayInPlace,
#   OptionalFoundMoveAsksAndHonoursBothBranches, NoHostDeclinesToTheRevealedPile,
#   KetriaRememberFoundFeedsTheChainedMove

$ go test -count=1 -run 'TestParamCensusScanIsComplete|TestEveryRepoDeckParamsAreRead' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.745s

$ gofmt -l effects/cardflow.go effects/diguntil_riders_test.go effects/diguntil_aura_test.go rules/paramcensus_test.go
(no output)

$ go run ./cmd/gentypes -check
(exit 0, no output)

$ go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  1.388s

$ go test -count=1 -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  0.684s
```

botbench did not move (as the brief predicted: no repo-deck card carries
DigUntil). No re-pin, no attribution needed. No Known-approximations row to
delete — the diguntil1 row was already gone and `DigZone$`'s remainder is
covered by the placeholder planechase row, which this ticket does not touch.
`go test -race` was not run (not required; daemon gate).

## Issues

- **Imprint riders use the Seek `seek-found` list, not the plain `Imprinted`
  list.** Sanctioned by the brief ("if a measured corpus reader fails because
  of the CR 607.2a exile-only filter … mirror the Seek-found pattern and name
  the deviation in the commit message"). Named in `c749b700`'s message. A
  generic (non-Seek) `Defined$ Imprinted` reader that wants a DigUntil-imprinted
  association where the card sits outside exile reads it correctly because
  `imprintPileTargets` merges `SeekFound`; readers that inspect
  `state.Object.Imprinted` directly would not. No measured corpus reader of the
  latter kind was found.
- **`DigZone$ PlanarDeck` stays withheld** (4 corpus carriers, all the
  planechase dig bodies) — gorge has no planar tier. Additionally, those
  carriers' `FoundDestination$ PlanarDeck` still falls through `ParseZone` to
  the graveyard; that pre-existing fallback is not this ticket's problem and
  was deliberately left unchanged (the brief says so).
- **Unresolvable `Amount$` stays withheld with amount 1** (fail-safe). If a
  corpus carrier's SVar body uses a count head the evaluator does not model, it
  silently digs 1 instead of the intended N. Measured: 12 non-literal `Amount$`
  DigUntil lines; the four carrier families named in the brief all resolve.
- **`RevealRandomOrder$`** remains the pre-existing deterministic existing-order
  stand-in (the brief explicitly scopes it out; ambient randomness is forbidden).
- **Testing note (not a defect):** a DigUntil SA with `ValidTgts$` (Tunnel
  Vision's `FindThePrecious`) cannot be driven through `effects.Resolve` in a
  unit test without also setting `Ctx.TargetsOffered` (or `Ctx.OfferedSA`),
  because the generic ValidTgts pre-ask (`effects/targets_ask.go`
  `chosenTargetsFor`) otherwise poses a target ask and suspends the walk before
  the body runs. The carrier test documents this. Anyone adding a real-carrier
  DigUntil test for a `ValidTgts$` body should set it.
- **No new CR-lane test proposed:** the riders are engine-internal placement
  semantics, not a CR rule with an obvious conformance citation.

STATUS handoff is in the final message.

## Deviations from the brief

- The brief listed empty_the_laboratory as the `SVar:X:Count$xPaid` carrier; the
  real DigUntil SVar at the pin is `SVar:Y:Remembered$Amount` (`X:Count$xPaid`
  is the Sacrifice sub's). Tested the real body. See the claim-check above.
- The brief's rider list included `one Shuffle$ carrier` as a real-carrier test;
  `Shuffle$ True` is exercised on real carriers inside
  `…KindredSummons…` and `…TunnelVision…` in addition to the inline tests.
