# Report cs1: Polymorph reveal/shuffle lockstep divergence

## What changed
- `dirac/mana_window.go` `lkiCtrlSlot`: reads `movedTargetLKIRec` (the snapshot `snapResTarget` takes when the resolution moves a target off the battlefield) first.
  Root cause (measured with temporary prints, reverted): Polymorph's Destroy moves Battle Brawler (owned by p1, controlled by p0 via Binding Grasp). `DigUntil` read "TargetedController" from the departed slot (p0) and dug p0's library. The creature it found then entered into the freed slot, so the following `Shuffle` op re-read the controller, found no departed record, and fell back to the owner (p1). It shuffled p1's 25-card library (24 draws) where gorge shuffled p0's 21-card one (20 draws): RNGDraws ref=99 / kernel=103. Gorge is right (LKI controller, CR 608.2h); dirac was wrong.
- New `dirac/oracle/testdata/repro/polymorph_reveal_shuffle.repro.json` (kfuzz repro of pair 53 seed 1234; shrink found nothing shorter, 318 actions) and `dirac/oracle/mbc_polymorph_reveal_shuffle_lockstep_test.go` (`TestMBCPolymorphReprosLockstep`, `stdReproNamed`).

## Fails without the fix (first run, before the dirac edit)
`--- FAIL: TestMBCPolymorphReprosLockstep ... divergence at 318 of 318: projection Cards[3].Pos [...] RNGDraws: ref=99 kernel=103` (full tail in `.ds4/scratch/red.log`).

## Done means walk
- [x] TestMBCPolymorphReprosLockstep fails before, passes after: red.log above; after: `make test-one PKG=./dirac/oracle RUN='^TestMBCPolymorphReprosLockstep$|^TestReproCompactRoundTrip$|^TestReproGlobMatchesTable$'` → PASS
- [x] Repro prints no divergence: `-first-pair 53 -pairs 1 -seed 1234` → 0 `divergence at decision` lines, `absolute: OK`
- [x] `-first-pair 40 -pairs 30` seed 1234: 22 clean, 0 divergent, 0 unmodelled/hang; seed 688497: 19 clean, 0 divergent (8 / 11 refused-pair, unchanged shape)
- [x] `make test-one PKG=./dirac RUN='^TestStateSize$|^TestColdArraysHoldNoPointers$|^TestTransitionAllocatesNothing$|^TestStateLayoutPinned$'` → PASS
- [x] archtest → PASS; codeshape → PASS; also `LKI|Lki|TargetController|TgtCtrl` in ./dirac → PASS (32 tests); gofmt/go vet clean.

## Issues
- The filed example (pair 4, seed 688497) no longer replays at HEAD (deal drifted); retargeted per the rescue guidance.
