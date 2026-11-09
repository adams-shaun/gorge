# Continuous `SharedKeywordsZone$` / `SharedRestrictions$` (Eater of Virtue)

Date: 2026-11-09.
Outcome: **design only; nothing implemented.** Recommends one small build
(section 5) and one deferred build (section 6).

## 1. Problem, measured today

`Eater of Virtue` (NEO, the only Standard card with the shape) says:

    S:Mode$ Continuous | AffectedDefined$ Equipped | Affected$ Creature |
      AddKeyword$ Flying & First Strike & ... & Protection & ... & Vigilance |
      SharedKeywordsZone$ Exile | SharedRestrictions$ Card.ExiledWithSource

Oracle: "As long as a card exiled with Eater of Virtue has flying, equipped
creature has flying. The same is true for first strike, ..."

`SharedKeywordsZone$` and `SharedRestrictions$` are read only by the Pump /
PumpAll path (`effects/pump_params.go:sharedPumpGrant`). `layers.go` (the
`PKAddKeyword` branch of `staticEffects`) never reads them, so the static is
**fail-open**. I measured it with a throwaway fixture (parsed from the real
`.cards` script, deleted afterwards): Eater attached to a 2/2 Bear, nothing
exiled:

    kw=[Flying First Strike Double Strike Deathtouch Haste Hexproof
        Indestructible Lifelink Menace Protection Reach Trample Vigilance]
    pt=4/2

and the same list after a Flying+Protection-from-red card was put in exile
with `ExiledWith = Eater`. So every Eater is a 13-keyword grant (including a
bare, meaningless `Protection`) from the moment it is equipped. The param
census records this as `param:stat:Continuous.SharedKeywordsZone` and
`...SharedRestrictions` in `engine/rules/testdata/paramcensus-sets/NEO.tsv`.
That is wrong in the wrong direction: the repo convention for an
unevaluable gate is fail-closed (`layers.go`, the `continuousGateKeys`
comment).

The same defect hits every Continuous carrier in the corpus (all grant
everything today): Altar of the Wretched // Wretched Bonemass, Urborg
Scavengers, Death-Mask Duplicant, Cairn Wanderer, Rayami First of the Fallen,
Soulflayer, Escaped Shapeshifter. Eater is the only Standard one; the others
are Pioneer-or-wider, so they are a bonus, not the target.

## 2. What the rule needs

For each keyword K in `AddKeyword$`: the static grants K to its affected
objects iff some card in a `SharedKeywordsZone$` zone, matching
`SharedRestrictions$` (default `Card`) relative to the static's source, has K.
`Landwalk`, `Protection`, `Hexproof` and `Trample` stand for any spelling of
that family the matching card has; the granted keyword is the card's own
spelling ("Protection from red", "Landwalk:Forest"). That is exactly what
`sharedPumpGrant` + `sharedKeywordFamily` already do for Pump, with a
Forge-pinned test (`shared_keywords_pump_test.go`).

Eater specifics that make it easy:

- The affected set is the equipped creature (`AffectedDefined$ Equipped`),
  already resolved correctly (the probe shows the grant lands on the Bear).
- The keyword source is cards in **exile** with printed characteristics
  (CR 400.7; nothing in exile has layer effects in practice), so the source's
  keywords need no layer walk: `Face().Keywords` plus keyword counters, which
  is `effects.objectHasKeyword`'s reading (`PrintedHasKeyword`).
- Provenance is already recorded: `SharedRestrictions` is in
  `exiledWithSourceKeyCodes` (`effects/context.go`), so the S: line makes
  `exileProvenanceNeeded`/`faceNamesExiledWithSource` true and the trigger's
  `ChangeZone` writes `ExiledWith` + `ExiledWithInc` through
  `moveZoneEvent` / `exiledWithAssociation`. **Not measured**: my probe could
  not drive a dies-trigger end to end (no live decision loop in that fixture).
  Test 1 below must assert this precondition rather than assume it.

## 3. Precedent to copy

`GainsAbilitiesOf$` / `GainsTriggerAbsOf$` (The TARDIS, Agatha's Soul
Cauldron) is the same problem: a static whose grant depends on foreign cards
in a named zone, filtered with `Card.ExiledWithSource` evaluated with
`src` = the static's source (`layers_static.go:gainedFacesForSpec`). Its
mechanics are the ones to reuse:

- resolved at **scan time** inside `staticEffects` (so the memo re-runs per
  event and a card exiled later is picked up, one that leaves loses the grant);
- sets `e.staticMemoStateRead = true`, so `staticSafeSince` stops admitting
  "static-cold" `MoveZone`/`Imprint` events as quiet. This flag is the one
  correctness-critical line: without it, a Flying card moving into exile
  (cold object, admitted as quiet) would leave a stale memo and Eater would
  not gain flying until some unrelated event;
- fail-closed: no match emits no effect.

## 4. Design

### 4.1 Parse once, key table
Add `PKSharedKeywordsZone` and `PKSharedRestrictions` to the `cards` param-key
table next to `PKGainsAbilitiesOfZones` (`cards/params.go`), and read them via
`st.HasParam`/`st.ParamStr` in `layers.go`'s keyword branch, not `Params[...]`
at the use site. A static with neither key takes today's path byte for byte
(one mask test), which is what keeps the layer hot path unchanged.

### 4.2 One home for the matching rule
Move the matching core out of `sharedPumpGrant` into an exported pure function
in `engine/effects` (rules may import effects; effects must not import rules):

    // SharedKeywords narrows want to the keywords some card in zones
    // matching spec has; family words grant the card's own spelling.
    func SharedKeywords(g *state.Game, zones []state.Zone, spec string,
        sc SpecContext, want []string, kwOf func(*state.Object) []string) []string

`sharedPumpGrant` becomes a caller with `kwOf = h.Chars(id).Keywords` (its
current behaviour, unchanged); the static caller passes the printed reader
(section 4.3). Two readings of one rule must not drift (the repo's
"a legal-answer rule has one home" principle applies in spirit); the existing
pump tests keep pinning the shared core.

### 4.3 Where it runs, and why printed keywords
In `staticEffects`, in the `PKAddKeyword` branch, before the
`len(kw.AddKeywords) > 0` check: if the static has `PKSharedKeywordsZone`,
set `staticMemoStateRead = true`, and replace `kw.AddKeywords` with
`SharedKeywords(...)` evaluated with `src = id`, `controller = o.Controller`
(as `gainedFacesForSpec` does). Empty result means no effect is emitted for the
keyword branch (fail-closed). `RemoveKeyword`/`CantHaveKeyword` are not shared
(no corpus card combines them); leave them untouched.

`kwOf` reads **printed** keywords + keyword counters, not `Derived`: calling
`Derived`/`h.Chars` from inside `staticEffects` re-enters the layer walk
(`activeDepth`/`charsWalk`) and is a CR 613.8 dependency cycle. For
Exile / Graveyard / Hand / Library zones the printed read is also the correct
one (no layer-6 effect applies to those cards in practice; if one ever does,
the "Known approximations" table gets a line, delete-only). This covers
Eater, Altar, Urborg, Death-Mask, Cairn, Rayami, Soulflayer. Zones are parsed
with `effects.ParseZones`; an unparseable list yields no grant (as
`sharedPumpGrant` does, plus its `Note` event).

### 4.4 What ends up on the effect
Nothing new: `ContinuousEffect.AddKeywords` carries the already-narrowed list,
so every downstream consumer (`keywordmay`, `trigger_granted`, `quiet_proof`,
`observed_abilities`, the derived memo, `equal_verify`) is unchanged and no
`ContinuousEffect` field, `events.Kind` or `events.Event` field is added, so
the replay / hash chain is untouched. Keywords that the card also prints stay
deduplicated by the existing layer-6 add.

### 4.5 Ratchets that move
- `paramcensus-sets/NEO.tsv`: delete the Eater row's two `param:stat:` entries
  (only those); the same read now appears in `layers.go`'s scan. Re-pin with
  the repo's census writer, not by hand.
- The census's static read attribution is derived by `go/parser` scan; a
  `HasParam`/`ParamStr` read inside `staticEffects` is already a rooted
  Continuous read, so no new hand root is expected. If
  `TestParamCensusScanIsComplete` fails, classify the read there, do not edit
  its tables.
- `TestHeads`: should not move (no head deck uses Eater; measure, and report
  the seat count and card if it does).
- `dirac`: `Eater of Virtue` is in `dirac/cardset/setlists/NEO.txt`.
  I did not check whether dirac lowers this static at all; the implementer
  must run the dirac gate and, if lockstep diverges, file the dirac ticket
  rather than extend this one.

## 5. Recommended build (one ticket, medium)

1. `cards` keys + `effects.SharedKeywords` extraction (pump tests stay green).
2. `layers.go` keyword-branch hook, `staticMemoStateRead = true`.
3. Tests, new file `engine/rules/static_shared_keywords_test.go`, each with a
   precondition assert and shown failing without the fix:
   - Eater, nothing exiled: Bear has **no** granted keyword (fails today:
     13 keywords).
   - A Flying + Protection-from-red card with `ExiledWith = Eater` and
     `ExiledWithInc` set: Bear gains exactly `Flying` and
     `Protection from red`, not `Reach`; P/T 4/2.
   - Memo: exile a Flying card **after** a first `Derived` read, with no other
     event class in between (the stale-memo case for `staticMemoStateRead`);
     flying appears. Move it out of exile: flying goes.
   - Provenance end to end: kill the equipped Bear through the real
     dies-trigger, assert it is in exile, `ExiledWith == Eater`, and the
     next creature Eater is equipped to has the Bear's keywords.
   - A card exiled by someone else (`ExiledWith` zero or another source) does
     not count; Eater leaving and re-entering does not keep the link
     (`ExiledWithInc`).
   - Family word: a card with `Landwalk:Forest` grants nothing for Eater
     (Eater's list has no Landwalk); a card with `Trample:...` /
     `Hexproof from red` spellings grants the card's spelling.
4. Delete the Eater rows from the census TSV via the writer; run archtest,
   codeshape, the NEO census test, `TestHeads`, dirac gate.

Do **not** do the interim "just withhold the grant" patch as its own change: it
costs the same edit site and test as the real fix.

## 6. Deferred: `SharedKeywordsZone$ Battlefield` (Escaped Shapeshifter)

Escaped Shapeshifter reads other **battlefield** creatures' current keywords
("an opponent controls a creature with flying"), which are layer-6 results.
That needs apply-time evaluation inside the layer-6 loop in dependency order,
with a CR 613.8 rule for two Shapeshifters (each depends on the other's
granted keywords; timestamp order breaks the loop). It is a different
mechanism from section 4 (no printed read, a cycle guard, probably a
`layers_derived` hook), has zero Standard cards, and shares only
`SharedKeywords`'s matching core. Treat it as its own ticket; until then the
battlefield-zone case should be fail-closed: in 4.3, if the zones include
`Battlefield` (or any zone with layered keywords), emit no grant and a
`Note` event rather than the printed read. The other Pump carriers that name
the Battlefield are unaffected (their path is `sharedPumpGrant`).

## 7. Risks

- **Stale memo** if `staticMemoStateRead` is missed: the memo test in 5.3 is
  the guard. `staticGates`/`staticColorGate` machinery is not involved
  (Eater has no `IsPresent$`/`Condition$`).
- **Printed vs. current** for exiled cards: a face-down card, a split /
  adventure card or a transformed DFC in exile have characteristics that
  `Face()` may not reflect (CR 708.2, 709.3 / 712.8). Eater exiles from a
  graveyard face-up, so the main line is fine; record any deviation in
  AGENTS.md's Known-approximations table only if tested.
- **Forge fidelity**: I did not have Forge's `StaticAbilityContinuous` source
  here. The Continuous reading is assumed to match the Pump reading that
  `shared_keywords_pump_test.go` pins; the implementer should confirm against
  Forge (or against a compliance scenario) before pinning, and say so in the
  test comment if unconfirmed.
- **Perf**: one extra param-mask test per Continuous keyword static; the
  zone walk only runs for the eight carriers while one is on the battlefield.
  No new allocation on the common board.
