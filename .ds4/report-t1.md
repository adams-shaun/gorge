# fb-20260923T005857Z-c1a24352 — Count$ResolvedThisTurn (Sephiroth transform)

## Summary

Two reported symptoms, one root cause. `Count$ResolvedThisTurn` — the SVar body
behind Sephiroth, Fabled SOLDIER's "If this is the fourth time this ability has
resolved this turn, transform Sephiroth" — was unmodelled, so the SVar gate
failed OPEN and `DB$ SetState | Mode$ Transform` flipped the creature on the
FIRST resolution. The back face prints `ManaCost:no cost`, so `Rakdos, the
Muscle`'s `TriggeredCard$CardManaCost` read 0 and exiled nothing (symptom 2,
downstream of symptom 1).

The head is now modelled; the tally is a per-ability, per-turn count folded
from the existing `Resolve` event and bound onto `effects.Ctx` by `rules`.

## What changed, per file

- `state/game.go` — new `Game.ResolvedThisTurn map[string]int32`. Keyed by
  `events.ResolvedAbilityKey` (source ObjID + root `Ability$` body content).
  The `CombatsThisTurn` shape: folded from events, cleared at `TurnChange`.
  `Game.Clone` deep-copies it (the `ExtraTurns` contract) because `Apply`
  increments it in place — a shared map would let a clone corrupt the original.
- `events/abilitykey.go` (new) — `ResolvedAbilityKey(source, sa)`: a
  deterministic pre-order walk of the root `*cards.SA`'s Kind/API/params (sorted
  keys) plus its `Sub` chain. Content, NOT pointer: `cards.Link`/`ResolveSVar`
  parse the `Execute$` SVar text fresh, so one card's two T: lines that share an
  `Execute$` SVar carry pointer-distinct but structurally-equal SAs (Victor,
  Valgavoth's Seneschal's `ChangesZone` and `FullyUnlock` both `Execute$
  TrigSurveil`). Pointer identity would split their tally; content merges it.
- `events/apply.go` — split `Resolve` out of the inert marker group into its own
  `case`, which increments the tally for the resolving ability stack object
  (`o.Source` + `o.Ability`; a spell carries no `Ability`). Reset added at the
  `TurnChange` boundary next to `CombatsThisTurn`. No new `Kind` or `Event`
  field: the tally is folded from the existing `Resolve` event.
- `effects/registry.go` — `Ctx.ResolvedThisTurn int32` (bound data; effects
  cannot import rules).
- `effects/count.go` — `case "ResolvedThisTurn": return c.ResolvedThisTurn, true`
  in `evalCountBody`, so `EvalCountOK` reports it modelled. An unbound Ctx reads
  a legitimate zero (fail CLOSED), not the unresolvable verdict.
- `rules/stack.go` — `resolvedAbilityTally(o)` / `resolvedAbilityTallyFor(source, sa)`
  (one home) read the tally; bound in `resolveTop`'s ability branch and in
  `resolveAbility` (the direct-resolution path the brief names).
- `rules/resolution.go` — `resumeResolution`'s Ctx rebuild re-binds the tally,
  so a chain suspended at a mid-resolution ask reads the same ordinal on re-entry.
- `rules/count_head_ratchet_test.go` — deleted the `"Count$ResolvedThisTurn"`
  map entry and its 3-line comment; comment now says "five bodies remain".
- `rules/paramcensus_gates_test.go` — retargeted `TestGateChainWiring`'s
  fail-open assertion from the now-modelled `Count$ResolvedThisTurn` to the
  still-unmodelled `Count$CardNumAttacksThisTurn`, so the contract stays pinned.
- `rules/replacement_turn_mana_test.go` — `TestSephirothTransformRunsTheDestinationFaceReplacement`
  drove `DBTransform` directly and relied on the old fail-open. It now presents
  the fourth-resolution context (seeds `ResolvedThisTurn` for the body's key)
  before calling `resolveAbility`, which is what the leaf is reached under. Its
  focus (the destination face's `repl:Transform` body runs) is unchanged, and
  its `replayCheck` still passes (`diffGames` does not compare the tally, which
  is derived from the log anyway).
- `effects/resolved_this_turn_test.go`, `events/abilitykey_test.go`,
  `rules/sephiroth_resolved_test.go`, `state/clone_resolved_test.go` (new tests).

## Structural fix, not a card allowlist

The fix is generic: the head is modelled for every carrier, and the identity is
derived from `(source, body content)`, so the next `Count$ResolvedThisTurn` card
is covered without touching code. The two known hazards are covered
structurally: **repeated resolutions of one ability accumulate** (Sephiroth test:
1,2,3,4) and **two T: lines sharing one `Execute$` SVar merge into one tally**
(`TestResolvedAbilityKeyMergesContentEqualBodies`, which proves the two SAs are
pointer-distinct first). Prowl's back face (`DB$ SetState | Mode$ Transform |
ConditionCheckSVar$ TrigAmount | ConditionSVarCompare$ EQ2`) carries the
identical shape and is fixed by the same head.

## Gates run (real output)

Corpus present as a symlink to `/home/sadams/projects/gorge/.cards` (verified);
`TestEveryRepoDeckCountHeadResolves` ran 0.84s, so corpus tests executed rather
than skipped.

Targeted command (brief's "Done means"; `TestParamcensus` does not exist as a
test name — the fail-open assertion lives in `TestGateChainWiring`), plus the
events key test:

```
$ go test -run 'TestSephiroth|TestEveryRepoDeckCountHeadResolves|TestParamcensus|TestRefPropertyCounts|TestResolvedThisTurn|TestResolvedAbilityKey|TestGateChainWiring|TestRakdosMuscle' ./rules/ ./effects/ ./events/
ok  	github.com/adams-shaun/gorge/rules	0.640s
ok  	github.com/adams-shaun/gorge/effects	0.014s
ok  	github.com/adams-shaun/gorge/events	0.003s
```

Verbose confirmation the corpus-backed tests ran (not skipped):

```
--- PASS: TestEveryRepoDeckCountHeadResolves (0.84s)
--- PASS: TestGateChainWiring (0.00s)
--- PASS: TestRakdosMuscleSacTriggerExilesAndMayPlaysWithAnyTypeMana (0.00s)
--- PASS: TestSephirothTransformsOnlyOnFourthResolution (0.00s)
--- PASS: TestRakdosExilesUntransformedSephirothManaValue (0.00s)
--- PASS: TestRefPropertyCounts (0.00s)
--- PASS: TestResolvedThisTurnCountHead (0.00s)
```

Behaviour goldens:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	2.440s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.207s
```

The botbench split did NOT move — the repo-deck behaviour change (Sephiroth,
plus Nissa/Tannuk in `pro-shaper.json`) did not flip any of the 20 games, so no
re-pin and no attribution was needed.

Packages edited with no `-run` target in the brief, run once each:

```
$ go test ./events/ ./state/
ok  	github.com/adams-shaun/gorge/events	4.553s
ok  	github.com/adams-shaun/gorge/state	(cached)
```

Formatting / generated types (the Go half of `make lint`):

```
$ gofmt -l cards/ state/ events/ effects/ rules/     # (no output)
$ go run ./cmd/gentypes -check                        # (no output)
```

## Fails without the fix

Reverted ONLY the `case "ResolvedThisTurn"` hunk in `effects/count.go` (scratch
copy taken first, file restored byte-identically — `cmp` confirmed), then:

```
$ go test -run 'TestSephirothTransformsOnlyOnFourthResolution|TestRakdosExilesUntransformed|TestResolvedThisTurnCountHead|TestEveryRepoDeckCountHeadResolves' ./rules/ ./effects/

--- FAIL: TestEveryRepoDeckCountHeadResolves (0.58s)
    count_head_ratchet_test.go:103: "Count$ResolvedThisTurn" evaluates unresolvable from a repo deck (carried by [Nissa, Resurgent Animist Sephiroth, Fabled SOLDIER Tannuk, Memorial Ensign]), which is not in knownUnmodelledCountHeads -- new gap, add it to the table
--- FAIL: TestSephirothTransformsOnlyOnFourthResolution (0.00s)
    sephiroth_resolved_test.go:72: after 1 resolution(s) face = "Sephiroth, One-Winged Angel", want the front face (transformed too early)
--- FAIL: TestRakdosExilesUntransformedSephirothManaValue (0.00s)
    sephiroth_resolved_test.go:112: after one resolution face = &{Sephiroth, One-Winged Angel no cost [...]}, want the front face with mana value 3
--- FAIL: TestResolvedThisTurnCountHead (0.00s)
    resolved_this_turn_test.go:17: EvalCountOK(bound) = (0, false), want (4, true)
FAIL	github.com/adams-shaun/gorge/rules	0.615s
FAIL	github.com/adams-shaun/gorge/effects	0.010s
```

Every new test fails with the fix reverted (the ratchet test because the head
reverts to unmodelled; the two rules tests because the first resolution
transforms; the effects test because the head is unresolvable; the clone test
because the shared map leaks). Preconditions
asserted in each test: Sephiroth is on the battlefield showing the front face
with mana value 3; the library holds ≥4 cards with the top three distinguishable
from the fourth; the trigger actually queued (`observedTriggerCount > 0`); the
two content-equal SAs are pointer-distinct.

The clone fix has its own revert proof — removing ONLY the `ResolvedThisTurn`
deep-copy from `state/game.go` (scratch copy restored byte-identically):

```
$ go test -run TestCloneOwnsResolvedThisTurn ./state/
--- FAIL: TestCloneOwnsResolvedThisTurn (0.00s)
    clone_resolved_test.go:22: writing the clone changed the original tally to 4, want 3 -- Clone shares the map
FAIL
```

## Precondition / vacuity notes

- `TestSephirothTransformsOnlyOnFourthResolution` asserts the front-face mana
  value is 3 before the deaths, so the face check is not vacuous.
- `TestRakdosExilesUntransformedSephirothManaValue` asserts the sacrificed
  object is the front face with mana value 3 *after one resolution* and that the
  fourth library card stays in the library (proving exactly 3 exiled, not more).
  It fires one real death trigger first, matching the player's sequence.
- `TestSephirothResolvedTallyReplaysExactly` is event-driven and calls
  `replayCheck` (log-only replay must rebuild the tally and transform on the
  fourth). Sephiroth's own ETB trigger is cleared before the deaths so it is not
  a confound; the trigger count is asserted per death.

## Scope / deviations

- `AGENTS.md` is NOT touched; `knownApproximationRows` is NOT changed. The
  `(devthr1)` row at `AGENTS.md:231` still names `Count$ResolvedThisTurn` among
  its six bodies — correctly, since the row closes only when ALL six resolve and
  this ticket closes one. It is already stale on `YourStartingLife` (closed by
  `827ca863`), which is not this ticket's business.
- The other five ratchet bodies (`MaxOppDamageThisTurn`, `CardNumAttacksThisTurn`,
  `NonCombatDamageThisTurn`, `ChosenNumber`, and the stale `YourStartingLife`)
  are out of scope and untouched.
- `rules/replacement_turn_mana_test.go` was modified out of necessity: its
  direct `DBTransform` invocation relied on the head failing open. The change is
  minimal and preserves its intent.

## Issues (found, not fixed)

1. **Five `Count$ResolvedThisTurn`-sibling heads remain unmodelled** in
   `rules/count_head_ratchet_test.go`'s `knownUnmodelledCountHeads`
   (`Count$MaxOppDamageThisTurn`, `Count$CardNumAttacksThisTurn`,
   `Count$NonCombatDamageThisTurn`, and the unbound-context `Count$ChosenNumber`;
   `Count$YourStartingLife` is already modelled but stale in the AGENTS.md row).
   Each is a separate ticket.
2. **`resolveAbility`'s direct-resolution path reads the tally as 0** unless a
   map entry exists, because only a stack `Resolve` event increments it. This is
   correct (a synthetic direct resolution never "resolved this turn"), but it is
   the reason `TestSephirothTransformRunsTheDestinationFaceReplacement` needed
   the tally presented explicitly. Worth a note if a future engine flow resolves
   a real repeatable ability through `resolveAbility` rather than `resolveTop`.
3. **`Count$ResolvedThisTurn` on an ACTIVATED ability** (Ashling the Pilgrim,
   Bronze Cudgels, Inner-Flame Igniter, Soulbright Seeker/Flamekin, Temporal
   Aperture) is now supported by the same content key, but only the triggered
   path is covered by a test in this repo's decks (none of those activated
   carriers are in a repo deck). If one is added, the activated path should be
   pinned end to end.

## Ironies / ledger

The defect was invisible to `.ds4/ledger.json` (no CR-lane test named the
transform gate). A CR-lane test citing CR 608.2m / CR 603.4 for
"an ability that has resolved this turn" would surface the head's family; not
written here (not in brief).
