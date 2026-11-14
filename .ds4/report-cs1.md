# Report cs1 — Combo <colours> Chosen payment bound

## What changed
- `engine/rules/potential.go`: `addPotentialChosenColour` now also prices a `Combo <colours> Chosen` production
  (new helper `potentialComboChosen`, hand-scanned so it adds no `strings.*` calls) as its listed colours plus the
  chosen colour, each once, for a payment's bound only. No chosen colour / other Combo words fall through to the old
  open fold. The plain PotentialMana over-bound (payment == nil) is unchanged.
- `engine/rules/potential_combo_chosen_test.go` (new): `TestPotentialComboChosenPaymentBound` — Frenzied Goblin's
  "you may pay {R}" with a `Combo W Chosen` land chosen G offers decline only; control with chosen R offers pay.
  (A Goblin is used because it is the existing in-tree pay-election fixture; the land is an inline Thriving-shaped land.)

## Done means walk
- [x] new test, fails without the fix — `make test-one PKG=./engine/rules RUN='^TestPotentialComboChosenPaymentBound$'` → PASS
- [x] pair-7 repro — substitution: pool `dr-mbc-ab` no longer exists on this base (split into dr-mbc-abaa..abbb) and
  Emiel the Blessed is "not a card dr-mbc-abba plays" in any of them, so the exact repro cannot run. Equivalent: the
  engine-level test above reproduces the divergent option list; `kfuzz -synth dr-mbc-abba -pairs 20 -seed 1`
  gives 1 divergent (implicates Mikokoro, Center of the Sea; no Combo Chosen card) — not caused by this diff.
- [x] `make test-one PKG=./engine/rules RUN='^TestHeads$'` → PASS, no head movement.
- [x] archtest PASS, codeshape PASS (after removing 2 strings.* calls), `go vet ./engine/rules` clean, gofmt clean.

## Fails without the fix
```
potential_combo_chosen_test.go:56: W-or-chosen-G land under a pay {R} trigger: options [trigger_cost_pay trigger_cost_decline], want [trigger_cost_decline]
--- FAIL: TestPotentialComboChosenPaymentBound
```

## Compliance check
`.ds4/scratch/compliance.mod` absent: skipped.

## Issues
- The ticket's `dr-mbc-ab` pool name is stale on this base (see above).
