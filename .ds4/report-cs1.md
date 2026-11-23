# Report cs1: Lady Caleria `options options` (tail-15-c000 pair 12)

## What changed
- dirac/mana_filter.go, dirac/mana_filterhybrid.go: when a hybrid-fee filter land (Wooded Bastion, Flooded Grove) makes a colour the searched play's cost names, the affordability search walks that filter's flattened ask in gorge's order (needed colours, then {C}, then the rest: `filtHybAsk`). Before, the kernel walked {C} first, spent the 160-node budget and withheld the cast. `filtColours` is now `filtColoursOf` with a colour part and an ask-charge flag; behaviour is unchanged for other callers.
- dirac/oracle/mbc_lady_caleria_lockstep_test.go + testdata/repro/hybrid_filter_needed_colour_ask_first.repro.json: `TestMBCLadyCaleriaReprosLockstep`.
- The cs0 unit test dirac/mana_hybridfilter_cast_test.go is removed: archtest TestOnlyTheOracleReadsRawTables forbids cardsettest.Synth in ./dirac. The lockstep repro covers it.

## Done means walk
- [x] TestMBCLadyCaleriaReprosLockstep passes, failed before — red.log tail: `divergence at 403 of 403: options options []`; now PASS (./dirac/oracle).
- [x] Repro rerun prints no `divergence at decision` line (count 0, `absolute: OK`).
- [x] `make test-one PKG=./dirac RUN='^TestStateSize$|^TestColdArraysHoldNoPointers$|^TestTransitionAllocatesNothing$|^TestStateLayoutPinned$|^TestTestFileNames$'` → PASS.
- Also: ./dirac/oracle TestReproCompactRoundTrip/GlobMatchesTable PASS; kgen acceptance (Corpus/Set/CorpusBuild/SupportDoc) PASS; archtest PASS; codeshape PASS; gofmt/vet clean.

## Fails without the fix
.ds4/scratch/red.log: `mbc_lady_caleria_lockstep_test.go:11: divergence at 403 of 403: options options []`.

## Issues
none
