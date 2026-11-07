# Report cs0 — Sting, Bilbo's Sword / Dwarven Mauler equip divergence

## Root cause (measured)
pair 549 decision 88: gorge offered only pass, kernel offered Sting's equip. p1 had two untapped lands and a
Dwarven Mauler ("Equip abilities that target this creature cost {2} less"), so equip {3} costs {1} on Mauler.
Gorge was wrong: `planAbilityPaymentLifePips` (engine/rules/potential_plan.go) planned the autopay witness at the
untargeted {3}, so on lands (not a floating pool) the offer was withheld. Floating pool worked (offer gate prices the cheapest target).

## Changes
- engine/rules/potential_plan.go: on "insufficient", retry via new `planAbilityTargetPriced` (potentialCostModsUsing over
  the legal targets, as `potentialTargetPricedPlan` does for casts) (CR 601.2f).
- engine/rules/equip_target_reduce_test.go: `TestEquipTargetGatedReduceOffered` (failed before the fix: equip not offered on 2 lands),
  `TestEquipTargetGatedReduceNeedsMauler` (negative control).
- dirac/support/refused/cargo-ship.json removed (`kgen -unrefuse "Cargo Ship"`): TestRefusedEntriesStillDiverge reported it stale
  (std-fin pair 1760 no longer diverges; the fix moved it). dirac/docs/support.md regenerated (DIRAC_UPDATE_SUPPORT=1).

## Done means walk
- [x] pair 549 autopay clean — `kfuzz -synth std-hob -first-pair 549 ... -surface autopay -absolute` → `absolute: OK`; pairs 560, 701, 1100, 1221 (autopay) and 549, 1221 (full) also `absolute: OK`.
- [ ] `make precheck BASE=land` ends ok — FAIL a49c432f2. Removing Cargo Ship from the refused list changes the kernel table key
  (e8fa10793ebb19aa-a6c1e703096bc5f0), and `make dirac-tables` cannot write it: `.cards/kernel-tables` is a read-only filesystem here
  (even unsandboxed). Every remaining precheck failure is "no card tables for this binary / run make dirac-tables" (13 lines).
  The earlier precheck, before the un-refuse, failed only on Cargo Ship staleness. Needs a run where `make dirac-tables` can write.

## Other checks
- archtest + codeshape: ok. `-run 'Equip|Steelshaper|CostModifier|Parity' ./engine/rules`: ok.
- Not done: fails-without-fix revert copy (the new test was observed failing before the edit: "equip should be offered at {1}: map[activate:2 ...]").
- Compliance: .ds4/scratch/compliance.mod not present, skipped.

## Issues
None new.
