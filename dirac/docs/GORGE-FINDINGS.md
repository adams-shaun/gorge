# Gorge findings

Where gorge departs from the Comprehensive Rules, or from what a card says, dirac plays what gorge
plays, because gorge leads and lockstep compares the two. Each such row says what gorge does, what the
rules say where they differ, and how dirac follows it. When gorge fixes one, dirac follows in the
same change and the row goes.

The second list is the differences still open: a game that reaches one stops as a named stall, or the
shape is not reachable in any pool. Each is fixed, or its cards are refused. They are not tolerated.

## dirac copies gorge

- Improvise (CR 702.126a) rides the convoke payment path (kw_convoke.go,
  CardDef.Improvise): the pick is a ChGenConvoke choose with mode 6 and pays
  generic only; the candidates are untapped artifacts (Engine.IsArtifact,
  creatures included, no attached-permanent exclusion: gorge excludes only
  bestowed, which dirac has none of; gorge also excludes a
  ReconfiguredAttached permanent, so a future Reconfigure lift must add that
  exclusion to the improvise branch of tapPayCand). Improvise taps are not
  mana: the per-turn expend tally and CastTotalManaSpent both exclude them
  (convoke picks count toward the expend tally but not CastTotalManaSpent).
  A printed card with both keywords is
  refused in kgen; a grant beside the other keyword (or on an {X} cost)
  poses gorge's one mixed announcement (convokeOptions, convokeXCost).
- Restricted mana that floats beside the pool (`State.rb`, `mana_restrbatch.go`)
  from a loyalty ability's resolving restriction (Chandra, Chill of
  Compliance's noncreature-only {U}: no mana ability, CR 605.1a, but the
  restriction rides it, CR 106.6; gorge's ManaAdd restriction): AddMana's F
  names the restriction, prepareGen registers it, and a fifth batch (`maxRB`)
  stalls StallRestrSrc (`TestChandraRestrictedLoyaltyMana`,
  `TestRememberWritesLockstep`). A dynamic-amount restricted source (Elvish
  Archdruid) is listed after every fixed and choice-shaped source as gorge's
  evaluated-amount layer. Quilled Greatwurm's six counters are asked one at a
  time (ChCostSubCtr), as gorge asks them.
- Face casts: gorge's potential planner prices every face route, and its
  cast payment planner plans the face the card shows (CR 709.3: a Room back
  in the hand on door 1 is a plain cast of that door, priced from untapped
  lands like any other), so `plannedCastCost` prices a plain cast of a card
  showing a nonzero face. dirac offers the Adventure (including its exile
  recast), split, modal and Room routes from the ordinary mana census
  (floating pool plus untapped sources) on the oracle surface
  (`Config.GorgeUnpriced`). The planner's limits apply only to those newly
  priced routes: a scripted prefix whose
  restricted mana floats dead-ends, and a witness whose plan consumes a
  source the face's own targets need is dropped ("unsupported:
  witness_unreachable"; Battle Display). Omen and Prepare
  keep their census offer. Off the oracle surface (self-play's
  default) every route is the plain rules census, CR 601.2h.
- DB$ Clone on the battlefield (`dirac/copy_clone.go`, Ext CloneCopy): dirac models gorge's copy markers, not a stack of
  layer-1 timestamps. A copy layer's card is locked when the layer lands (an
  `fxCopyPerm` record, written by the Ext and by an ETB copy in a table that
  has the Ext, and one `fxCopyEOT` record while an until-end-of-turn copy is
  live) and a re-base restores the locked card, never reading the source again
  (CR 707.2b; dirac `copyTop`, `TestCloneCleanupRebaseKeepsLockedCard`; gorge
  `settleExpiredClones`). (1) A resolving trigger whose
  source's current card is not the trigger's card is not asked its `may`
  (gorge reads OptionalDecider$ off the source's current face; a copy of
  something else has no such trigger). (2) The projection shows every copy
  effect's permanent as its printed card in such a table (as an Aegis
  bearer). (3) A permanent copying itself is an ordinary copy layer (its
  inherited exceptions are held beside the layer as `fxCopySelf` records), and
  an Aegis bearer's copy layers are ordered by their layer timestamps
  (`copy_aegisunder.go`). Only the
  cleanup-step priority after a legend-rule ask is NOT followed (finishCleanupStep
  returns early on the pending ask and the step then ends); no pool reaches it.

## Still open

- A prevent-the-next-N shield (Healing Salve, Combat Medic; `effPreventDamage`) is
  registered for the recipient's object, so a creature that leaves the
  battlefield and returns the same turn keeps it (CR 400.7: it is a new object).
  dirac keys its record by the same object code (dirac/repl_prevnext.go) and
  copies that.
  Follow-up: check `Incarnation` in `damageReplacementMatches` for shield ObjIDs
  and keep dirac in step.
- A graveyard may-play permission's non-mana `RaiseCost$` (Noctis, Alien Symbiosis, Wickerfolk Indomitable, and BLB's Festival of
  Embers) BESIDE the spell's own additional pick cost: the kernel prices one
  pick Cost, so the cast is a loud `StallMayRaise` ("mayraise"). One raise
  alone prices in both engines: a cast is made under one permission (CR
  601.2b/f), so gorge charges the FIRST granting static's raise rather than
  summing every grant (`engine/rules/mayplay.go` `mayPlayRaiseCost`) and dirac
  prices that same first static (`dirac/cost_mayplayraise.go` `mayRaise`).
  BLB's pool reaches this stall class with Festival only beside a spell that
  also owes its own additional cost. A land a raising permission grants (an
  artifact land under Noctis) is withheld, payable or not, as gorge's
  `mayPlayLandIds` does (`dirac/cost_mayplayraise.go` `mayRaiseLandHeld`).
- A prevent-the-next-N shield (dirac/repl_prevnext.go, Ext PreventNext) and any
  other damage replacement that would be a candidate for the same event (a
  colour or scope prevention, a SHIELD counter, a printed prevention, a
  doubler, a plus-N, a prevent-and-mill or self-damage replacement, a damage
  mark): gorge asks the CR 616.1 order and the order changes what is
  prevented and what is left of the shield. dirac stalls with StallPrevNext
  (loud, never a silent divergence). Two or more shields on one recipient are
  modelled (the ask is posed and each pick applied as made) in `damageTo` and in
  a combat damage pass; the other damage sites (a damage sweep, a batch of
  recipients, fights) stall on them. Three prevention records a
  turn of any kind (the shield, colour and scope records share
  `genState.prevCol`), a fourth is the capacity stall StallPrevCol.
