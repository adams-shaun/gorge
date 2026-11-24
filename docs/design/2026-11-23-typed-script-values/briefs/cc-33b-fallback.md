# feat(cardc): CC-33b fallback rows for mtgish bugs the fork does not carry a fix for

Kind: feature

Target branch: `cardc`. Complexity: low. Implementer: Sonnet or the wave lead.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 5 ("Fallback"). Runs
after CC-32 (the ledger) and CC-33a (the fork register).

## Goal

A card whose ledger holds a `mtgish-bug` row, and whose bug no `carried` fork patch fixes, is served from Forge
through a fallback file, and its ledger rows go. Hold the fallback register against the ledger and the fork register,
and report the pass-bar figure.

## Design

Decided.

1. For each card with a `mtgish-bug` ledger row (`engine/cardc/testdata/ledger/`) not named in a `carried` row's
   `cards` (`engine/cardc/testdata/mtgish-fork.tsv`): write `engine/cardc/testdata/fallback/<gorgebe.Stem(name)>.txt`
   (`name`, `class mtgish-bug`, `issue <the row's issue>`; `gorgebe.ReadFallbacks` format), and delete that card's
   ledger file. Re-run `go run ./cmd/cardc ledger -write`, which also rewrites `ledger-counts.tsv`.
2. `cardc compile`'s totals line gains `fallback=<n>` (entries whose reason starts `fallback:`), after `forge=`.
3. `TestFallbackRegister` (`engine/cardc/lockstep`, offline): every fallback card is in the FDN or Legacy slice; no
   fallback card has a ledger file; no fallback card is named by a `carried` fork row; a `mtgish-bug` fallback with
   issue 0 is logged (the operator files it), not failed.
4. `TestFallbackCount` (offline): the fallback file count equals a constant in the test, so the register changes only
   in review; PLAN.md's "only shrinks" applies after the proof.

## Done means

- [ ] `make test-one PKG=./engine/cardc/lockstep RUN='^(TestFallbackRegister|TestFallbackCount|TestLedgerFormat|TestLedgerStatic|TestLedgerCounts|TestLedgerWitnessesLive)$'` passes.
- [ ] `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestReadFallbacks|TestBuildOrigins|TestServedCardsMatchForge)$'` passes; lower `servedFloor` by exactly the
  fallback count when a fallback card was served.
- [ ] `make test-one PKG=./cmd/cardc RUN='^(TestLedgerUsage|TestDiffUsage|TestLockstepUsage)$'` passes; `go run ./cmd/cardc ledger` exits 0.
- [ ] The hand-back gives: the fallback cards with class and issue; the pass-bar figure, cards compiled with no
  refusal and no fallback out of 412 (FDN + Legacy distinct names), and the issue-0 list for the operator.

## Out of scope

- Fork commits, upstream issues (the operator's), any card fix, any engine edit.
- Anything on GitHub outside gorge. Never open a pull request to i5jb/mtgish.
