# cardc wave 5: the burn-down, rules shared by CC-34 to CC-39

Target branch: `cardc`. Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 4
("Pass bar") and 6 (CC-34..39). Each ticket owns one class family; its own spec lists the cards.

## Where the slice stands (measured at `a0be533401`)

- Proof slice (FDN 286 + Legacy 136 = 412 distinct names): 220 served (53.4%), 2 fallback, 190 served by Forge
  because cardc stops on them: 51 front-end refusals (`refused:<Enum.Variant>`) and 139 backend refusals
  (`unprintable:<what>`). The pass bar is 392 (95%).
- `cardc compile` records the first reason only. Unblocking a card may expose a second; when that one belongs to
  another family, leave it and name it in the hand-back.
- Static diff over the served cards: 136 rows on 85 cards. Ledger (`cardc ledger`): `cardc-gap` 129 rows on 66 cards,
  `equivalent` 43 rows on 35 cards, 6 unattributed lockstep witnesses.

## Rules every wave-5 ticket follows

1. **Forge's shape.** For each construct, print the form Forge writes for that construct on the listed cards (read the
   card's script in `.cards/cardsfolder`). `TestServedCardsMatchForge` fails a served card whose ability shape differs
   from Forge's; `TestSpellingsAttested`, `TestModesAttested`, `TestBodiesAttested` and `TestCostsAttested` fail an
   unattested spelling. Replacement-form statics and `K:etbCounter` stay Forge's form. Trigger lines print
   `Origin$ Any` wherever Forge does: `engine/rules/trigger_zone_gate.go` treats absent and `Any` differently.
2. **From IR facts only.** A printer reads the IR; it never copies or looks up a Forge script line. CC-39's
   closed default list (its own spec) is the one exception.
3. **IR edits.** A family may append enum values in its own `engine/cardc/ir/<family>.go` (PLAN.md section 6). A new
   struct field, a new node type or a codec change is not decided: stop on that construct, leave the card refused,
   and put it in the hand-back under "Decisions" with the options and a recommendation.
4. **Front-end refusals.** A lowering registers from its family's `engine/cardc/mtgish/fe_*.go` file (`register`,
   `engine/cardc/mtgish/lower.go:24`). Every construct the front end cannot lower still refuses by name.
5. **mtgish bugs** (a tree that is wrong, not one cardc loses): no fork commit this wave. Write a fallback file
   (`engine/cardc/testdata/fallback/`, `gorgebe.ReadFallbacks` format, class `mtgish-bug`, the issue number or 0;
   amounts cite i5jb/mtgish#1), raise `fallbackCount` in `engine/cardc/lockstep/fallback_test.go` by one per file,
   and say so in the hand-back. Upstream gets nothing from a seat.
6. **The ledger moves with the code.** After the printers change:
   - `go run ./cmd/cardc ledger -write`, then `go run ./cmd/cardc ledger` exits 0;
   - `ledger-counts.tsv` is what `-write` writes; new rows arrive `cardc-gap` / `untriaged`;
   - a newly served card may carry ledger rows (they are `cardc-gap`, with a note naming the field); a card that was
     served with no ledger file at the base must still have none;
   - raise `servedFloor` (`engine/cardc/gorgebe/gorgebe_test.go`) to the new served count.
7. **Tests** name no Forge line of 30 characters or more (`TestNoVerbatimForgeLines`): assert on parameter lists.
   Committed text carries no Oracle text, no script text and no diff values.

## Done means (every ticket, plus its own list)

- [ ] `make test-one PKG=./engine/cardc/gorgebe RUN='^(TestServedCardsMatchForge|TestServedAPIsSupported|TestBuildOrigins|TestBuildProofSlice|TestSpellingsAttested|TestModesAttested|TestBodiesAttested|TestCostsAttested|TestValueHeadsModelled|TestDefinedTable|TestValidTerms)$'` passes (not ENV).
- [ ] `make test-one PKG=./engine/cardc/lockstep RUN='^(TestLedgerFormat|TestLedgerStatic|TestLedgerCounts|TestLedgerWitnessesLive|TestFallbackRegister|TestFallbackCount)$'` passes.
- [ ] `make test-one PKG=./engine/cardc/lockstep RUN='^(TestLedgerLockstepFDN[0-9]+|TestLedgerLockstepLegacy.*)$'` passes.
- [ ] `make test-one PKG=./engine/cardc/mtgish` with the family's own tests named, when the front end changed.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestDependencyOrderHolds|TestNothingImportsCardc|TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] `make test-targeted BASE=origin/cardc` passes. No `make test-smart` per ticket: one runs at the wave head.
- [ ] Hand-back: served count before and after; each listed card served, or its next blocker; ledger counts before
  and after; any new fallback with its issue; Decisions.

## Out of scope (every ticket)

- Any edit outside `engine/cardc`, `cmd/cardc` and these specs. No engine package changes.
- Fork commits and anything on GitHub outside gorge. Never open a pull request or issue on i5jb/mtgish.
- Diff canonicalisations (`engine/cardc/diff/canon.go`) for defaults: gorgebe respells instead (CC-39).
- Another family's cards. CC-40 (the reading-agreement export).

## Order (ranked by cards a family unblocks; families cover all 190 Forge-served cards)

| ticket | family | cards | runs beside |
|---|---|---:|---|
| CC-34 | conditions, values, events, predicates, references, targets | 54 | CC-36 |
| CC-35 | statics, cost changes, costs, replacements, keyword arguments | 48 | CC-37 |
| CC-36 | modes, copies, choices, delayed triggers, mana, repeats | 41 | CC-34 |
| CC-37 | library searches, looks, reveals, zone moves | 25 | CC-35 |
| CC-38 | one-shot continuous effects, control, attachment | 22 | CC-39 |
| CC-39 | Forge's spellings, the ledger's `equivalent` and static rows, lockstep rows | ledger | CC-38, lands last |
