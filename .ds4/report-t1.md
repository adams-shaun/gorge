# fb-20260923T005805Z-1301f55a — offer Dargo when its sacrifice pays for the reduction

## What changed and why (per file)

### `rules/mana.go` — the offer gate's announced-X affordability sweep
`offerCastableUsing` priced every offer with the pre-announcement modifier
snapshot, evaluated at X=0 (its own doc says so). A cost carrying
`Sac<X/Spec>` whose reduction reads the paid X is therefore priced at its
unreduced cost, so a player who can only afford the cast after announcing
some sacrifices got no cast option — the reported symptom.

Added `offerSacXMods`. When the ordinary X=0 feasibility check (and the
existing target-potential retry) both fail, and the cost announces a
sacrifice count (`costAnnouncesSacX`), it walks every legal announcement
`x = 1 .. maxX` and reprices each exactly the way `manaToPay` /
`manaToPayX` do after the announcement: the X-folded cost under
`costModifiersWithTargetsXUsing(..., x)` plus the commander tax. It returns
the first feasible announcement's modifier snapshot (so the caller's
composed cost and the payment agree) or `ok=false`. On `ok=false` the cast
stays withheld. **No path bypasses mana feasibility** — the sweep only
accepts when some announced cost is genuinely payable, and X=0 is not swept
because the caller already priced it.

`maxX` comes from the same candidate walk `xAsk` uses (see below), so the
offer's bound and the ask's bound cannot drift. The hypothetical-pool
(`hyp`) potential-action walk `legalActionsPriced` routes through the same
`offerCastableUsing`, so bot-generated legal actions see the identical
offer — the offer/payment rule has one home.

### `rules/cast.go` — xAsk monotonicity and the shared sacrifice-candidate walk
`xAsk` stopped at the first unpayable X (`break`), justified by "generic
only grows with x". That is false when an announced X feeds a `Count$xPaid`
reduction: the total falls as the reduction grows, so the first unpayable X
can precede the only payable one. With the offer now appearing, `break`
would have withheld X=3 and wedged the fetched cast. The break is now taken
only for costs that do **not** announce a paid X (`costAnnouncesPaidX`);
every other announced-X cost keeps the early-break optimisation.

Added `sacrificeCostCandidates(p, source, part, ability)` — one walk for the
permanents that can pay a `Sac` cost part (self-reference normalisation via
`sacrificeMatchSpec`, `CantSacrifice`/`ForCost` blocking via
`sacrificeBlockedForCost` and the caller's cast/activation cause). `xAsk`
(the announcement bound), `sacAsk` (the settle) and `offerSacXMods` (the
offer) now all derive from it, replacing three near-identical hand-rolled
loops — the structural fix that cannot miss the next sibling stage.

### `rules/dargo_cast_offer_test.go` — new tests (new file, per repo rule)
The brief named a sibling test in `rakdos_params_sacx_test.go`, but the repo
rule "New tests go in a new file (2026-09-22)" is in force, so the two tests
live in a new file.

- `TestDargoCastOfferedWhenSacrificeReducesManaCost`: real corpus Dargo,
  three eligible permanents (artifact, two creatures) and a `{1}{R}` pool —
  below the unreduced `{6}{R}` = 7, payable as `{R}` after X=3 ({2} less per
  sacrifice). Asserts the preconditions (all three permanents on the
  battlefield; pool total below 7), that the cast option appears, that X=3 is
  an offered announcement, that the sacrifice ask is Min=Max=3, and that all
  three land in the graveyard, Dargo resolves, and the pool ends at `{1}`.
- `TestDargoWithheldWhenNoSacrificeCountIsPayable`: one eligible permanent
  and `{1}{R}` — the best announcement (X=1 → `{4}{R}` = 5) is still
  unpayable, so no cast option may appear. Asserts the permanent is on the
  battlefield, the pool is below 5, and `Pending()` is non-nil before
  scanning for a cast option (so an empty decision cannot pass vacuously).

No census, allowlist, ratchet or `Known approximations` row was touched: no
row describes this defect.

## Gates run (real output)

### Targeted rules tests (the brief's exact command)
```
$ go test -run 'TestDargoCastOfferedWhenSacrificeReducesManaCost|TestDargoAnnouncesSacrificeCountAndReduces|TestDargoSacrificingNothingPaysFullCost' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	0.815s
```

The new fail-closed test also passes (run together in the same package):
```
$ go test -run 'TestDargoCastOfferedWhenSacrificeReducesManaCost|TestDargoAnnouncesSacrificeCountAndReduces|TestDargoSacrificingNothingPaysFullCost|TestDargoWithheldWhenNoSacrificeCountIsPayable' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.645s
```

### Behaviour goldens outside `rules/`
```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	1.612s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)
```

**No botbench re-pin was needed** — `TestConstructedDefaultIsByteIdentical`
passes against the committed pin, so the Dargo behaviour change did not move
the 20-game split.

### Adjacent announced-X / sacrifice family (targeted, run once)
```
$ go test -run 'TestDargo|TestSuspendXBenalish|TestConvokeXAnnouncement|TestConvokeMarchOfMultitudes|TestConvokeOverSelection|TestNyxbornHydraBestowX|TestCascadeXSpell|TestReanimatedEtbXPaid|TestChangeXUnbound|TestToxicDelugePaysAnnouncedLife|TestAnnouncedSubCounterCost|TestSVarFixedPayLifeX|TestFreePayLifeXSharedAnnouncement|TestCR601|TestUnpayableSacrificeAbortHoldsTheOptionOut|TestAdditionalSacrificeCostGatesTheOffer|TestAlternativeCostWithSacPartIsGatedOnCastable|TestCardnameSacrificeAbilityOfferedAndPaid|TestDoubleSacCostRequiresDistinctCandidates|TestRealCorpusSacCostWithAlternationAndDescriptionPays|TestDeclinedDelveSpinIsBounded|TestAuthorizedDelveAnswersStayLegal|TestUnderDelveAbortsTheCast|TestTwobridAnnouncementSeesRaiseCost|TestColorReductionSeesTheAnnouncedHybridFace|TestThaliaPhyrexianAnnouncementFiltersUnpayableFace' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.526s
```

### Format / type gates
```
$ gofmt -l rules/cast.go rules/mana.go rules/dargo_cast_offer_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output)
$ go vet ./rules/
(no output)
```

The brief says `rules/heads_test.go` was not to be edited and `TestHeads` is
a daemon gate; it was not run here, so no goldens were touched. No head
movement is expected (the change only makes an already-legal Dargo cast
offerable; the acceptance decks' bot decisions are unaffected — confirmed by
the unchanged botbench split).

## Fails without the fix

I reverted only the two production behaviour hunks (the `offerSacXMods`
branch in `rules/mana.go` back to `return false`, and the `nonMonotonic`
guard in `rules/cast.go` back to an unconditional `break`), copied the fixed
files to `.ds4/scratch/` first, then restored them and verified byte-for-byte
with `cmp` (`RESTORED BYTE-IDENTICAL`).

Failing output with the fix reverted:
```
$ go test -run 'TestDargoCastOfferedWhenSacrificeReducesManaCost|TestDargoWithheldWhenNoSacrificeCountIsPayable' ./rules/
--- FAIL: TestDargoCastOfferedWhenSacrificeReducesManaCost (0.62s)
    dargo_cast_offer_test.go:46: cast option missing: [{Index:0 Kind:pass Label:Pass priority Obj:0 ...} {Index:1 Kind:concede Label:Concede Obj:0 ...}]
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.650s
FAIL
```

The regression fails exactly at the missing cast option, which is the
reported symptom; the fail-closed test still passes under the mutant (as it
must — the mutant withholds more, not less).

## Deviations from the brief

- The regression lives in a **new file** `rules/dargo_cast_offer_test.go`,
  not appended to `rules/rakdos_params_sacx_test.go`. The brief's "exact
  regression insertion point" conflicts with the repo-wide rule that new
  tests go in a new file; the repo rule wins and the test helpers
  (`dargoEngine`, `castOption`) are shared unchanged.
- A **negative test** was added (`TestDargoWithheldWhenNoSacrificeCountIsPayable`)
  in addition to the one named test, to satisfy "the rule that an actually
  unpayable option must not be offered".

## Issues

- **Broader `Sac<X>` family not covered by the sweep's regression**: 33
  corpus files carry `Sac<X/...>`. The three reduce-cost carriers
  (`dargo_the_shipwrecker`, `torgaar_famine_incarnate`,
  `rottenmouth_viper`) and the activated-ability carrier
  (`extus_oriq_overlord_awaken_the_blood_avatar`) all share the shape the
  sweep handles structurally, but only Dargo has a corpus regression test.
  Torgaar (`{6}{B}{B}`, `Sac<X/Creature>`) and Rottenmouth Viper (`{5}{B}`,
  `Sac<X/Permanent.nonLand>`) additionally exercise the offer gate for an
  ability activation (Extus) and a non-creature filter not covered by the
  two new tests. Not fixed here (the brief scoped the regression to Dargo);
  filing a new-ticket file for a Torgaar/Extus offer regression.
- **`xAsk`'s `maxOld` fallback is still monotonic-flavoured**: when the
  payable set is empty, the fallback offers `min..maxOld`, and `maxOld` is
  only advanced on a payable X. For a non-monotonic (X-driven reduction)
  cost whose every announcement is unpayable, this offers X=0 and the cast
  then aborts at payment — the pre-existing abort path, not a livelock (the
  offer gate withholds such a cast, so the fallback is only reached by a
  hand-built intent). No action taken; noting it because the loop's
  monotonicity assumption was the pre-existing weakness this ticket
  partially corrects.
- **No CR-lane test was written**: this defect is offer feasibility, not a
  CR-rule number, so there is no clean CR citation for the ledger. If a
  lane test is wanted it would cite CR 601.2b/601.2f (announcement precedes
  total-cost composition). Reporting per the report contract.

## Brief premise check

Re-measured the brief's corpus numbers with `/usr/bin/grep`:
`Sac<X/...>` = **33** files and `SacToReduceCost` = **4** files — both
matched the brief exactly. The brief's claim that existing affordability
tests fund the unreduced cost also held
(`TestDargoAnnouncesSacrificeCountAndReduces` uses `"RRRRRRR"` = 7).
