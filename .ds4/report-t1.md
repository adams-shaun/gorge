# compound-statics1 — Bast's CantAttack half: face whitelist learned the present-gate family

## What changed and why

The compound-split half of the report was already merged at main (`f81f996e`),
so this ticket fixed only the surviving half: the face `S:Mode$ CantAttack`
parameter whitelist never admitted the present-gate family that
`continuousGateHolds` already evaluates. Bast, Panther Goddess's split
`CantAttack` half carries `IsPresent$ Creature.YouCtrl | PresentCompare$ LE2`
(shared Params from the compound line), so it was skipped whole while the
`CantBlock` half — whose loop runs `continuousGateHolds` with no whitelist —
bound at runtime. Bast attacked freely at any creature count.

Per file:

- **`effects/misc.go`** — `CantAttackParamsReadableForRules` key set extended
  with `IsPresent`, `IsPresent2`, `PresentCompare`, plus two fail-closed
  guards (see Hazards below). The doc comment was rewritten to name the
  compound-split pairing (`f81f996e`) that made the family reachable and the
  guards' rationale, replacing the stale "NONE pairs it with IsPresent$"
  measurement.
- **`rules/layers.go`** — `attackBlocked`'s doc comment updated: it asserted
  only `CheckSVar$/SVarCompare$/Condition$` were read; now it names the
  present family and the compound-split origin. Comment-only change.
- **`rules/compound_statics1_test.go`** (new) — `TestBastCompoundModeStaticGates`:
  real corpus card, Bast + one bear on seat 1 (2 creatures), a third bear
  moved in via an emitted `MoveZone`; both `attackBlocked(bast, 2)` and
  `blockRestricted(bast, otherID)` are TRUE at 2, both FALSE at 3; the plain
  bear is unrestricted in both states (`Card.Self` scoping precondition). The
  split statics and the gate params are asserted on the corpus face first.
  Ends `replayCheck`.
- **`effects/cantattack_readable_test.go`** (new) — unit pin for the two
  guards: orphan `PresentCompare` rejected, unread spec
  (`Creature.PairedWith+withSoulbond`) rejected at both EQ0 and GE1, readable
  specs admitted, and an unrelated param still fails.

Structural approach: the guard is a per-spec readability check via
`UnknownPredicates` — the SAME classifier the matcher's `recognisedPredicate`
uses — so it cannot drift from what `countPresent` actually resolves. Any
future present spec with an unimplemented predicate is skipped whole rather
than blanket-restricting; the next sibling is covered without a list.

## Hazards

1. **Orphan `PresentCompare$`** (measured 0 corpus rows): `presentGate` reads
   `PresentCompare` only when a present spec is set, so a blind key admission
   would let an orphan compare fall through the gate unread → blanket
   restriction. Guard added: a `PresentCompare` with neither `IsPresent` nor
   `IsPresent2` returns false.
2. **Unread spec + EQ0** (measured 1 corpus row): Flowering Lumberknot's
   `IsPresent$ Creature.PairedWith+withSoulbond | PresentCompare$ EQ0` names
   the unimplemented `withSoulbond` predicate. `countPresent` counts through
   `matchesSpec`, so the count is always 0 and EQ0 would hold unconditionally
   → blanket over-restriction. I verified against `UnknownPredicates` that ALL
   21 distinct specs on the 24 other in-scope lines resolve (powerGE4,
   powerEQ1+toughnessEQ1, PairedWith, YouOwn, etc.), and that
   `Creature.PairedWith+withSoulbond` is the ONLY unresolvable one; the guard
   keeps that line skipped — the permissive direction. `matchesSpec` was NOT
   weakened.
3. **Registration-path whitelist untouched**: `CantRestrictionParamsReadable`
   was not modified (confirmed by diff), so `effEffect` still refuses to
   register a gate-bearing `CantAttack`/`CantSacrifice` body blanket.
4. **`CantAttackUnless`** untouched (own reader/whitelist).

## Gates run (real output)

Targeted test command (Done means item 3), forced uncached:

```
$ go test -run 'TestBastCompoundModeStaticGates|TestPacifismCommaModeStaticSuppressesAttackAndBlock' -count=1 ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.733s
```

`effects/` (edited package):

```
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	16.751s
```

Format + types:

```
$ gofmt -l effects/misc.go rules/layers.go rules/compound_statics1_test.go effects/cantattack_readable_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output)
```

Behaviour goldens outside `rules/`:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.834s
```

Botbench split UNMOVED, as predicted: 0 of the 25 affected cards appear in any
`internal/testutil/decks/*.json` (grep over the 25 card names → 0 files), so
no repo deck exercises the change. No re-pin needed.

Premise re-measurement (all held):
- compound-mode `S:` lines: 87 files (`/usr/bin/grep -rlE '^S:Mode\$ [A-Za-z]+,[A-Za-z]+'`).
- CantAttack+IsPresent: 25 files.
- CantAttack+PresentCompare with no IsPresent (orphans): 0 files.
- All 24 in-scope specs resolve except Flowering Lumberknot's withSoulbond.

## Fails without the fix

Whitelist keys reverted (only the `IsPresent`/`IsPresent2`/`PresentCompare`
addition and both guards removed), then:

```
$ go test -run 'TestBastCompoundModeStaticGates|TestPacifismCommaModeStaticSuppressesAttackAndBlock' ./rules/
--- FAIL: TestBastCompoundModeStaticGates (0.00s)
    compound_statics1_test.go:89: Bast NOT attack-restricted with 2 creatures: the CantAttack half's IsPresent gate never bound
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.442s
```

Guards reverted (key set kept), then:

```
$ go test -run 'TestCantAttackParamsReadablePresentGate' ./effects/
--- FAIL: TestCantAttackParamsReadablePresentGate (0.00s)
    cantattack_readable_test.go:37: orphan compare: got true, want false (map[Mode:CantAttack PresentCompare:EQ0 ValidCard:Card.Self])
    cantattack_readable_test.go:37: unread spec + EQ0: got true, want false (map[IsPresent:Creature.PairedWith+withSoulbond Mode:CantAttack PresentCompare:EQ0 ValidCard:Card.Self])
    cantattack_readable_test.go:37: unread spec + GE1: got true, want false (map[IsPresent:Creature.PairedWith+withSoulbond Mode:CantAttack ValidCard:Card.Self])
FAIL
```

`effects/misc.go` restored byte-identically after each revert (`cmp` clean).

## AGENTS.md

No row edit: this closes no Known-approximations row, and the
`combatrestriction1` row's remaining text stays accurate.

## Deviations from the brief

- The brief's PREMAP named only the key extension + orphan guard. Hazard 2
  measured live (one corpus row), so I added the per-spec readability guard
  the brief explicitly authorizes ("a per-value readability check in the
  whitelist is acceptable, a blanket restriction is not"). No `matchesSpec`
  change.
- I added `effects/cantattack_readable_test.go` because the two guards protect
  against shapes measured at 0 (orphan) and 1 (unread spec) corpus rows — a
  corpus-only pin cannot exercise them, so a unit pin is required for the
  "every new test must be able to fail" contract.

## Issues

1. **Flowering Lumberknot's `withSoulbond` predicate is unimplemented.**
   `.cards/cardsfolder/f/flowering_lumberknot.txt`:
   `S:Mode$ CantAttack,CantBlock | ValidCard$ Creature.Self | IsPresent$
   Creature.PairedWith+withSoulbond | PresentCompare$ EQ0`. `PairedWith` is
   implemented (`effects/filter.go:443`); `withSoulbond` (paired with a
   creature that HAS soulbond) is not in the predicate table, so this split
   line's CantAttack half stays skipped whole by the new readability guard and
   its CantBlock half (no whitelist) over-restricts: with the count always 0
   under EQ0, Lumberknot can never block even when paired with a soulbond
   creature. Fix: add the `withSoulbond` filter predicate (a predicate on the
   paired-with object's own keywords). Corpus prevalence: 1 row. File:
   `effects/filter.go` predicate table + the `PairedWith` neighbour at :443.
   Separate ticket.
2. **`CantAttackUnless` + `IsPresent$`** (out of scope per brief, 1 row):
   `rules/attack_cost.go:62` `cantAttackUnlessParamsReadable` already admits
   `IsPresent`/`IsPresent2`; no action here. Named for completeness.
3. **Unregistered compound members** (`stat:CantCrew`/`CantTransform`/
   `CantPlayLand`, 5 corpus rows per `f81f996e`'s message) remain separate
   tickets, per the brief's scope boundary.
4. `PresentZone$` on a CantAttack line: 0 corpus rows measured; the whitelist
   still rejects it (stays in the skip-whole direction), no ticket needed.
