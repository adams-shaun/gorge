# fix(cardc): CC-24 a bare-string enum argument is a word, not an error

Kind: defect

Target branch: `cardc`. Complexity: low. Implementer: the wave lead (by hand).
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 1.2 and 4. Runs first
in wave 4, before CC-30, so the static diff and the ledger see the real served set.

## Defect

`filtWord` (`engine/cardc/mtgish/fe_filter.go:107`) wants a tagged node, but the data writes a plain enum's value
as a bare string: `{"_Permanents":"IsCardtype","args":"Creature"}`, `IsCreatureType` `"Bird"`, `IsSupertype`
`"Basic"`. Every such filter fails with `cardc: expected a unit variant, got string`, a plain error, so `Compile`
records `error:<rule key>` and hides the variant the rule would really stop at.

Measured at `de85384ff1`: 265 rule lowerings on the proof slice fail this way, and 227 of the 422 slice entries
(FDN 286 + Legacy 136) carry an `error:` refusal. The unit tests built the argument as `tn("CardType", ...)`, a
shape the data never uses.

## Fix

1. `filtWord` accepts a non-empty bare string as the word, and keeps the tagged form.
2. `TestFilterLowering` gains the data's shape (`IsCardtype "Creature"`, `IsSupertype "Basic"`).
3. `TestCompileProofSliceNoPanic` fails on any `error:` refusal on a proof-slice face: the class census that keeps a
   plain error from hiding a refusal again.
4. `servedFloor` (`engine/cardc/gorgebe/gorgebe_test.go:376`) rises to the measured count.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish RUN='^(TestFilterLowering|TestCompileProofSliceNoPanic)$'` passes.
- [ ] `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestServedCardsMatchForge|TestServedAPIsSupported|TestBuildOrigins)$'` passes
  with the floor raised, no shape comparison loosened.
- [ ] Served before and after, `go run ./cmd/cardc compile -slice fdn` and `-slice legacy` totals.
