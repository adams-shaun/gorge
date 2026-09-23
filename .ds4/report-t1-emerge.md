# Task agent-20260918T225913Z-5db23024 — implement Emerge as an alternative cast

## What changed and why

CR 702.118a: "You may cast this spell by sacrificing a creature and paying
the emerge cost reduced by that creature's mana value." Emerge is a casting
option read directly off the `K:` line, but it is **not** a plain
alternative-cost substitution (the existing `keywordAltCost` family), because
the cast must both sacrifice a creature and reduce the cost by that
creature's mana value. It therefore gets its own reader, offer, charge and
reduction.

Per file:

- **`rules/emerge.go`** (new).
  - `init()` registers `kw:Emerge` via `effects.RegisterNonAPI`, the
    `bestow.go`/`mayflash.go` pattern, so the coverage census recognizes the
    primitive.
  - `emergeCost(f)` parses `K:Emerge:<cost>`, cutting Forge's colon-suffixed
    metadata line (`K:Emerge:5 B B:Artifact` — one of the 15 corpus lines),
    and withholds (`ok=false`) any token `ParseCost` reports `Unknown` or any
    `{X}`, the `bestowCost`/`mayflashExtraCost` fail-closed convention. A
    withheld emerge leaves the plain cast offered, never an incorrect payment.
  - `emergeSacrificePart()` returns the mandatory `Sac<1/Creature>` additional
    cost as an ordinary `CostPart`, so the existing Sac machinery
    (`sacAsk`, `nonManaCastable`, `sacrificeCostCandidates`) enforces it with
    no parallel path.
  - `emergeOfferCost(p, id, f)` prices the offer: the emerge cost composed
    with the sacrifice and reduced by the **largest** mana value among the
    caster's sacrificeable creatures. That is the best case the player can
    reach, so the offer is present whenever *some* legal sacrifice makes the
    cast payable. It returns `ok=false` when no creature can pay.
  - `applyEmergeReduction(pc)` folds the actual chosen sacrifice's mana value
    out of `pc.cost` exactly once (idempotent via `pc.emergeDone`), called
    after every Sac part is settled. `reduceGenericThenColored` subtracts
    generic first then coloured pips in WUBRG order, clamping every slot at
    zero — the same generic-then-coloured leftover direction
    `costMods.apply` uses.
- **`rules/legal.go`** — the hand walk's alternative-cost loop gains the
  `emerged` cast option, gated on `emergeOfferCost` succeeding,
  `targetsAvailable`, and `offerCastable(..., spellScope("emerged"), false)`.
- **`rules/cast.go`**:
  - `pendingCast` gains `emerge`/`emergeDone` (plain scalars, carried by the
    existing copy of the struct).
  - `beginCast`'s mode switch gains `case "emerged"`: it charges the printed
    emerge cost plus the `Sac<1/Creature>` part in place of the mana cost, and
    marks the cast (`e.cast.emerge = true`) so the reduction cannot be applied
    to a different cast. The reduction is deliberately not applied here — the
    creature is not chosen until `sacAsk`.
  - `sacAsk`, after every Sac part is settled and before the mana window reads
    `pc.cost`, calls `applyEmergeReduction`. Every downstream consumer
    (`manaToPay`, `paymentMana`, the CR 601.2f modifier composition) then sees
    the reduced cost.
- **`rules/emerge_test.go`** (new) — two end-to-end tests over the real corpus
  card **Elder Deep-Fiend** (printed `{8}`, `K:Emerge:5 U U`) and controlled
  battlefield-creature fixtures.

All game-state mutation is via `events` (`findCardObj`/`addMana` emit
`MoveZone`/`ManaAdd`); no `state.Game` field is written from `rules`.
`replayCheck` passes in both tests.

## Tests

`TestEmergeCastPaysReducedCost`:
- Preconditions asserted before the real assertion: Elder Deep-Fiend is in
  seat 0's hand; the fuel is a **controlled battlefield** creature with mana
  value exactly 3; the parsed emerge cost is `{5}{U}{U}` (generic 5, U×2) and
  the printed cost is `{8}` (generic 8). A vacuous setup fails loudly.
- Funds the pool with the full printed emerge cost `{5}{U}{U}` = 7 mana,
  submits the `(emerged)` option, answers the mandatory sacrifice ask with the
  fuel, drains the 601.2g mana window and Elder Deep-Fiend's own "when you
  cast" target ask.
- Asserts the fuel is in the graveyard with a `Text == "sacrificed"`
  `MoveZone`, and the pool holds exactly **3** — i.e. 7 paid − 3 reduction.

`TestEmergeCastReductionFloorsAtZero`:
- Fuel mana value 10 > emerge total 7; pool funded with only **5** mana (less
  than the unreduced cost, more than the floored-to-zero cost).
- Asserts the fuel is sacrificed and all 5 mana remain: the reduction floored
  at zero and the cast was free, not negative.

## Exact gates run (real output)

Done-means targeted command:

```
go test -run 'TestEmergeCastPaysReducedCost|TestEveryRepoDeckIsFullySupported' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	0.609s
```

`-v` confirmation that both new tests and the acceptance test actually ran
(not skipped; `.cards` symlink present at `/home/sadams/projects/gorge/.cards`):

```
=== RUN   TestEveryRepoDeckIsFullySupported
--- PASS: TestEveryRepoDeckIsFullySupported (0.63s)
=== RUN   TestEmergeCastPaysReducedCost
--- PASS: TestEmergeCastPaysReducedCost (0.00s)
=== RUN   TestEmergeCastReductionFloorsAtZero
--- PASS: TestEmergeCastReductionFloorsAtZero (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.669s
```

Behaviour goldens outside `rules/`:

```
go test ./internal/archtest/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/internal/archtest	3.241s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.171s
```

The botbench split did **not** move — as the brief predicted, no repo deck
contains Elder Deep-Fiend (`grep -Ril 'Elder Deep-Fiend' internal/testutil/decks`
= 0 files), so no re-pin and no attribution is needed.

Format/type gates on changed Go files:

```
gofmt -l rules/emerge.go rules/emerge_test.go rules/cast.go rules/legal.go
(no output)

go run ./cmd/gentypes -check
(no output)
```

Known-approximations guard (no row added or grown; this ticket closes no row,
so `knownApproximationRows` is unchanged at 18):

```
go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/
ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
```

## Fails without the fix

Non-test files changed: `rules/emerge.go`, `rules/cast.go`, `rules/legal.go`
backed up to `.ds4/scratch/*.bak`; each hunk reverted in place, the tests run,
then the file restored and `cmp`-verified byte-identical against the backup.

### Revert A — the reduction (`e.applyEmergeReduction(pc)` no-op in `sacAsk`)

```
--- FAIL: TestEmergeCastPaysReducedCost (0.62s)
    emerge_test.go:137: mana remaining 0, want 3 ({5}{U}{U} reduced by mana value 3)
--- FAIL: TestEmergeCastReductionFloorsAtZero (0.00s)
    emerge_test.go:173: unexpected KChoose during emerge cast: &{... Prompt:turn 1 — discard 1 card(s) down to the hand-size limit ...}
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.649s
```

(The floor test fails because the un-reduced `{5}{U}{U}` cannot be paid from a
5-mana pool, so the cast aborts and play advances to the cleanup discard ask.)

### Revert B — the registration (`effects.RegisterNonAPI("kw:Emerge")` made inert)

```
--- FAIL: TestEmergeCastPaysReducedCost (0.58s)
    emerge_test.go:96: effects.Supported() is missing "kw:Emerge"
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.592s
```

Restore verification:

```
cast.go restored byte-identically
emerge.go restored byte-identically
legal.go unchanged
```

## Deviations from the brief

- None in scope. The brief's premise on corpus prevalence
  (`grep -rl 'K:Emerge' .cards/cardsfolder | wc -l` = 15) and on the deck
  carrier (`grep -Ril 'Elder Deep-Fiend' internal/testutil/decks` = 0) both
  held when re-measured.
- The brief suggested gating the offer on a safely priced cost. I priced the
  offer at the best-case (largest mana value) reduced cost, which offers a
  legal emerge cast whenever one exists. See Issues for the residual.

## Open concerns / residual

- The offer is priced at the best-case sacrifice (largest mana value among
  candidates). If the player then chooses a lesser creature whose reduced cost
  the pool cannot pay, `payManaCastSpent` fails and `abortCast` rewinds the
  cast (CR 601.2h) — legal, and the option is re-offered. This is a
  convenience/liveness nuance, not an incorrect payment. It is not a
  Known-approximations row (the table is frozen and this ticket adds none).
- Emerge is read from the printed face only (`f.KeywordParam("Emerge")`), the
  same read the other alt-cost keywords use. A layer-6 `AddKeyword$ Emerge`
  grant is not honoured.
- `K:Emerge` parameters that carry an `{X}` or an unparsable token withhold
  the emerge option (plain cast stays offered). No corpus line does today.

## Issues

- **Emerge from a granted keyword is unsupported.** `emergeCost` reads the
  printed face, so a continuous `AddKeyword$ Emerge` grant (the
  `offspringCost`/`escapeCost` derived-keyword read handles the analogous
  case) is not offered. No corpus card grants Emerge today; if one appears,
  the offer and `beginCast` must both switch to a `derivedKeywordParam` read.
  Prevalence to check if it matters:
  `/usr/bin/grep -rlE 'AddKeyword\\$[^|]*Emerge' .cards/cardsfolder | wc -l`.
- **Best-case offer vs. actual sacrifice** described under Open concerns. A
  CR-lane test would cite CR 601.2h (reversal of an illegally/affordably
  unpayable cast) and assert the re-offer behaviour for the "chose a
  too-small creature" answer; I did not write it (out of the brief's scope).
- No existing ledger entry is closed by this ticket (Emerge was not in the
  Known-approximations table).

## Commit

`6221af8c feat(rules): implement Emerge as an alternative cast (CR 702.118a)`
