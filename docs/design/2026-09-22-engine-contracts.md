# Engine contracts (not approximations)

These are deliberate, decided behaviours of the gorge engine. None of them is
a defect and none of them is owed to a milestone. They lived in AGENTS.md's
"Known approximations" table, which is a CLOSING REGISTER of real debt; a
contract sitting in that table made the table look longer than the debt is
and invited a ticket that would have been wrong to write.

If one of these ever stops being true, change it here, in the same commit as
the code.

## R-9: the no-ask host degradation contract

A host that cannot answer a decision — the fuzzer, a test harness, a seat
that has left — must never wedge the engine. Every asking primitive
therefore carries a deterministic fallback that completes the resolution.
The fallback is the CONTRACT, not a stand-in for a missing ask: the ask
itself is real for a host that can answer. The named fallbacks are:

- `Charm` takes its first mode and records a Note
  (`engine/effects/misc.go`, `effCharm`'s fallback).
- An exact-`Origin$ Library` `ChangeZone` takes the deterministic
  first-`Min` pick (a quantity-only search) or fails to find (a
  stated-quality search) — `engine/effects/zone.go`
  (`effSearchLibrary`, `applyLibrarySearch`).
- `Scry`/`Surveil` keep every card on top in its existing order (Scry's
  pile B empty, Surveil's graveyard pile empty), recorded as one
  `events.LibraryOrder` — `engine/effects/cardflow.go` (`effLookAndArrange`).
- `RearrangeTopOfLibrary` keeps the existing order (pile A = the offered
  options in offered order) — `engine/effects/cardflow.go`
  (`effRearrangeTopOfLibrary`), `engine/rules/arrange.go` (`handleArrange`).
- `api:TimeTravel` declines every add/remove election —
  `engine/effects/time_travel.go` (`effTimeTravel`).
- Fixed `Choices$` and `VoteCard$` ballots take the first option —
  `engine/effects/vote.go`.
- Any decision kind with no bot-policy arm of its own takes option 0
  through `botpolicy`'s clamp fallback (`bots/botpolicy/policy.go`, `Decide`).
  Where that produces a BAD but legal play rather than merely a neutral one,
  it is real debt and carries a ticket; the clamp itself is the contract.

## api:TimeTravel is implemented, not approximated

Every repetition re-enumerates the affected objects (owned suspended exile
cards, then the controller's battlefield permanents with a TIME counter) in
zone order and poses one `KChoose` add/remove/skip election per object,
`Amount$` repetitions deep. Each ask rides an immutable snapshot
(`Decision.ResumeObjects`) with its cursor and repetition in `ResumeTarget`
and `ResumeRound`, so an answer that drops a counter cannot shift the next
object's cursor. `engine/effects/time_travel.go` (each answer is applied in
place). The only stand-in is R-9 above.

## ActivationLimit$ is enforced for every shape the corpus carries

`resolveActivationLimit` (`engine/rules/legal.go`) reports `ok=false` for a value
that is neither a literal, an SVar reference nor an inline `Count$`, and
`activationLimitReached` then does not enforce. The compiled corpus carries
no such shape, so this is unreachable. It becomes debt only if a future
corpus pin introduces one — the census would surface it.

## A Class level's granted body depends on its own primitive

`engine/cards/kw_class.go` (`addLevelGate`) and `engine/rules/class_level.go`
(`classBandGateHolds`) implement the level machinery. A level whose
`AddStaticAbility$`/`AddTrigger$`/`AddReplacementEffect$` names an
unimplemented mode or effect is exactly as dead as that primitive, and the
debt belongs to that primitive's own ticket, not to Class.

One decision to honour: a `ClassBand$` band must NOT be folded into
`IsPresent$`/`IsPresent2$`. The trigger gate reads those as a UNION and a
granted replacement reads no `IsPresent2$` at all, so folding silently
widens the gate.

## Trigger remembered readers intentionally differ

For a trigger body, `Defined$ Remembered` is a card-selection read: when the
source has no persistent `Remembered` list, it falls back to the firing
trigger's event referent carried in `Ctx.Remembered`. Numeric and condition
readers instead count only explicit memory, excluding the seeded event capture
through `rememberedExcludingCapture`: this includes
`Count$RememberedNumber`, `ConditionDefined$ Remembered`,
`Remembered$Amount`, and `Defined$ RememberedLKI`. This is a deliberate contract,
not an inconsistency to unify mechanically; the trigger context does not
structurally distinguish a referent that is useful to a selection effect from
one that should not count as authored memory.

The selection side is pinned by `engine/rules/trigger_test.go:639`
(`TestTriggerEffectAppliesToTheRememberedObject`, the Watcher pump) and
`engine/rules/gains_abilities_test.go:823`
(`TestGainedTriggerCarriesItsRememberedReferent`). The exclusion side is pinned
by `engine/effects/remembered_capture_test.go:28`
(`TestRememberedCaptureExcludedFromPlainReads`),
`engine/rules/remembered_capture_event_trigger_test.go:39`
(`TestRememberedEventCaptureExcludedInTriggerBody`), and
`engine/effects/rememberedlki_capture_test.go:30`
(`TestDefinedRememberedLKICaptureExcluded`).

Gathering Stone is the player-visible carrier of the split: its empty-reveal
path previously treated the trigger referent as a library card to move and
shuffle. The `moveDefinedLibraryObjects` shuffle guard (main `9d0b4216b`) closed
that symptom: it shuffles only when `Shuffle$ True`, or when a non-position
fetch moved library cards. That fix does not change the reader contract.

Reopen the separate (B) program only if a real corpus card is found whose
`Defined$ Remembered` must exclude the capture, or whose numeric/condition
reader must include it. Such a finding requires separating explicit remembered
objects from trigger referents throughout trigger context construction and
readers; it is not a one-line reader change.

## Regeneration

A permanent with an unused this-turn Shield replaces lethal damage and
`Destroy`/`DestroyAll` through the destruction-replacement path: consume the
shield, clear damage, tap, remove from combat. Shields expire at cleanup.
`NoRegen$` and the Effect-registered `Mode$ CantRegenerate` restriction are
honoured. `engine/effects/regeneration.go`, `engine/effects/zone.go`, `engine/rules/sba.go`,
`engine/rules/combat.go`.

## TargetingPlayer$ Opponent selection

When `TargetingPlayer$` names `Opponent` or `Player.Opponent` and at least two
opponents are alive, the ability's controller first chooses which living
opponent answers the target ask. The selected opponent then receives the
original ask; target legality and `TargetEffect` remain relative to the
ability's controller. A sole living opponent receives the ask directly, with
no redundant selection, and no living opponent fails closed to the controller
rather than leaving an unanswered decision. This rule covers cast and
activation announcement, trigger placement and resolution-sub asks, and
mid-resolution `ValidTgts$` asks (`chosenTargetsFor` and
`changeZoneChosenTargets`). The selection is transient engine flow state, not
a logged event. Trigger-relative `TargetingPlayer$` referents continue to
fail closed to the controller when their binding is absent.

## Chain targets are announced on cast (CR 601.2c)

Every target of a spell or activated ability is chosen as it is cast or
activated: the root declaration's and each `SubAbility$` link's that declares
`ValidTgts$`. The cast flow asks the root first (`targetAsk`), then each link
in chain order (`subTargetAsk`), all before payment. A link's ask is an
ordinary `KTarget` decision with `ResumeKind` `cast_sub`; it carries the
link's `TargetEffect`, routes `TargetingPlayer$` to the named chooser, and
honours `TargetUnique$` against the targets already announced.

- The answer is a targeting: one `TargetsChosen` per target with
  `Text == events.SubTargetNotice`, folded into `Object.SubTargets` (the
  root's stay in `Object.Targets`). Ward, "becomes the target" and the crime
  check match it by kind. `view.StackView.Targets` lists root targets, then
  chain targets.
- Which link chose which target is engine flow state
  (`Engine.castSubTargets`, keyed by the link's line), rebuilt by replaying
  the answers. Resolution reads it through `Ctx.SubPreAsk` and never
  re-asks; CR 608.2b prunes illegal chain targets (`recheckCastSubTargets`).
- The offer census (`targetsAvailable`) withholds a cast or activation whose
  chain has a mandatory target with no legal candidate.
- A link behind `Condition$ Kicked` / `OptionalCost` announces nothing when
  that cost is not being paid.

Not announced by the cast flow, and still asked as the link resolves (debt,
not contract; each is a later stage): ChangeZone-family links, a modal
(Charm) mode's deeper links, Fuse, copies, a trigger's chain (CR 603.3d), and
a link whose legality reads an earlier target of the same spell
(`castSubPreAskable`). An overloaded or face-down cast announces no targets
at all. `engine/rules/cr601_subtargets_test.go`.

## The board clock's round number is exact

`view.RoundOf` folds the ordered event stream, anchored on the starting
player and tracking eliminations, so an elimination alone never moves the
round. Every log-bearing caller (host fan-out, `viewAt`, the seat view,
mtgsim, botbench, keywordbench) uses it; `roundOf` is only the snapshot
fallback. It never touches engine state. Revisit if a same-seat repeat turn
(`AddTurn`) ever lands.

## Staged hypothetical engines (`rules.NewStaged`)

`rules.NewStaged` builds a two-player engine at an arbitrary position from a
rules-level `Stage` (research harnesses only: `lab/training/searchbench`). Every
placement after the genesis-style object arena is an event folded by
`events.Apply`, emitted raw, so staging queues no trigger, runs no
replacement or ETB effect and writes no rules-side "this turn" ledger. A
permanent is summoning sick exactly when the Stage says so and nothing
"entered this turn". The sickness of the active player's sick permanents is
set by parking them under the other seat across the last `TurnChange`, so
they sit at the end of their controller's battlefield list. The engine is
hypothetical: its later shuffles are seeded by `Config.Seed` and it is not
replayable from `Config`. The full contract is the doc comment in
`engine/rules/staging.go`.

## A mixed-Hand ChangeZone that moves nothing emits one diagnostic Note

An explicit multi-zone `ChangeZone Origin$` that includes `Hand` with a fetch
selector resolves through the search path: one private option list across
every named origin (CR 400.1, "from your hand and/or graveyard"), as Forge's
`changeHiddenOriginResolve` and XMage's Worldsoul's Rage do. A `Defined$` that
names its objects keeps the object path. When such an object-path resolution
moves nothing, `effChangeZone` emits exactly one replay-visible Note naming the
origin; a resolution that moved its objects emits none. The Note is a
diagnostic, not a stand-in. `TestMixedOriginNoteStillFiresWhenNothingMoves`
(`engine/effects/mixed_origin_test.go`) pins it;
`TestWorldsoulsRagePicksFromHandAndGraveyardInOneAnswer`
(`engine/rules/worldsouls_rage_mixed_origin_test.go`) pins the multi-pick
search.

## Extra phases: CR 500.8 order, passed splice points, nested ranges

`DB$ AddPhase` grants one `ExtraPhase` event whose fold is `Game.ExtraPhases`
(a queue spliced into the CURRENT turn). Four edges are deliberate, match Forge
(`PhaseHandler`: a per-phase LIFO stack of extra phases, `extraPhases.clear()`
when the turn ends) and XMage (`Turn.java`, LIFO extra phases), and are pinned:

- **Order (CR 500.8):** extra phases created after the same phase occur most
  recent first. `TestCR500_8MostRecentExtraPhaseOccursFirst`
  (`engine/rules/cr500_extra_phase_lifo_test.go`).
- **A passed splice point is dropped:** a grant whose splice step the walk has
  already left this turn is cleared at `TurnChange`, never carried into the next
  turn. `TestExtraPhaseQueueDoesNotSurviveTheTurn`
  (`engine/rules/addphase_test.go`).
- **A splice mid-range of another active extra phase** continues the outer range
  afterwards. `TestExtraPhaseSpliceMidRangeContinuesTheOuterRange`
  (`engine/rules/addphase_multistep_test.go`).
- **Several extra phases completing at the same step** resume at the LAST
  completed grant's resume point (the innermost one's).
  `TestExtraPhaseMultiCompletionTakesTheLastResume`
  (`engine/rules/addphase_multistep_test.go`).

`effAddPhase`'s Notes (an unresolvable `ExtraPhase$`/`AfterPhase$`/`FollowedBy$`/
`NumPhases$`/delayed-trigger value, a non-contiguous multi-step set) are
fail-closed guards no corpus card reaches;
`TestAddPhaseCorpusValuesResolveWithoutNote` (`engine/effects/addphase_corpus_census_test.go`)
holds that. `BeforeFirstPostCombatMainEnd$ True` (World at War) refuses the
grant once a main phase after the first postcombat main has begun, read from the
folded `Game.MainPhasesThisTurn` (Forge's `nMainsThisTurn <= (is(MAIN2) ? 2 : 1)`):
`TestWorldAtWarAfterTheFirstPostcombatMainGrantsNothing`.

## A stack copy keeps alternative-cost provenance (CR 707.10)

`events.StackCopy` inherits the original's `CastFlags` except the bits in
`state.CastProvenanceFlags`, and that split is the rule, not an approximation:

- **Kept:** `FlagEvoked`, `FlagDashed` and `FlagWarped` record that an
  alternative cost was paid, and CR 707.10 copies all alternative costs. The
  token the copy becomes therefore gets the same battlefield-entry hook:
  evoke's sacrifice (CR 702.74a), dash's haste and end-step return
  (CR 702.109a), and warp's end-step exile (CR 702.185a). Pinned by
  `TestCopyOfEvokedSpellIsSacrificed`,
  `TestCopyOfDashedSpellReturnsAtEndStep` and
  `TestCopyOfWarpedSpellIsExiledAtEndStep`
  (`engine/rules/altcost_copy_provenance_test.go`).
- **Lost:** `state.CastProvenanceFlags` lists the bits whose readers are
  conditioned on the spell having been CAST, so a copy -- put on the stack,
  never cast -- must not inherit them. `FlagMayFlashSac` is one: its rider is
  "if you cast it any time a sorcery couldn't have been cast".
  `TestMayFlashSacCopiedSpellIsNotSacrificed` (`engine/rules/mayflashsac_test.go`)
  pins that half.
