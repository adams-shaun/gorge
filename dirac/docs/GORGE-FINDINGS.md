# Gorge findings

Where gorge departs from the Comprehensive Rules, or from what a card says, dirac plays what gorge
plays, because gorge leads and lockstep compares the two. Each such row says what gorge does, what the
rules say where they differ, and how dirac follows it. When gorge fixes one, dirac follows in the
same change and the row goes.

The second list is the differences still open: a game that reaches one stops as a named stall, or the
shape is not reachable in any pool. Each is fixed, or its cards are refused. They are not tolerated.

## dirac copies gorge

- Cascade (CR 702.85a) puts the exiled cards on the bottom "in a random
  order"; dirac copies gorge's fixed stand-in: the non-found cards in
  library order right after the scan, then the found card last if it was
  not cast (dirac/kw_cascade.go). Printed K:Cascade only (CardDef.Cascade);
  granted cascade and Averna's replacement are not lowered.
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
  poses gorge's one mixed announcement (convokeOptions, convokeXCost). gorge offers an improvise cast only when the floating pool
  plus the greedy artifact credit pays the whole cost (its planner refuses
  the shape), so a cast with a coloured pip is offered only with that colour
  floating, which the lockstep reproduces.
- Free cast with an either-or additional cost (Alternate Additional Cost,
  Eaten Alive via Etali, Louisoix's Sacrifice via Hurl into History's
  Discover): gorge's altAddAsk classifies the parts with `castable` ->
  `pay.CostPayable`, which reads only the floating pool, and a free cast is
  no lowered potential play, so untapped lands never reach the pool. A free
  cast whose pip-free or coloured mana part the pool does not pay (and whose
  other part is unpayable) is reversed with no ask (CR 733.1) even with
  lands untapped, although CR 118.9d / 601.2h would let them pay. dirac
  emulates this at the ask (cast.go bsAltAdd, `altAddPayable(ask=true)`),
  gated on `cfgGorgeLowering|cfgGorgeAutopay`; with neither part payable at
  all it reverses with no ask and no taps on every configuration. The answer
  path keeps the lands-inclusive reading (`ask=false`), since gorge pays a
  chosen part at the mana window, where the lands are tapped (pair 9954).
  Tests: TestFreeCastEitherOrNoPayablePart, TestFreeCastEitherOrFloatingPoolPays;
  lockstep: ctrshapes (Hurl into History deck).
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
  keep their census offer. Pricing these casts from untapped
  lands lets the kpermcopy lockstep's greedy caster push more than
  `castRingN` spells in a turn: a cast-ring capacity stall, measured
  0/720 games (TestPermcopyLockstep). Off the oracle surface (self-play's
  default) every route is the plain rules census, CR 601.2h.
- Replacement amounts are computed in a fixed order, amounts then
  prevention (Twinflame Tyrant before Crystal Barricade; two Twinflames
  x4). gorge asks the affected player to order two matching damage
  replacements (KReplacement, CR 616.1); every order gives the same result,
  and dirac poses the same no-op ask (ChGenReplOrder) on a combat
  damage event, dealing the pass's events one at a time in gorge's order
  (state-based actions after each answer, the asked event dealt as
  asked), and on a noncombat one mid-resolution (the spell still on the
  stack). One noncombat event is parked per op; a second the same op
  would ask is dealt at once (not seen). The combat pass's events and
  their order are frozen when the first ask is posed, and the walk deals
  a dead source's event as it stood at the freeze (its controller's live
  Twinflames and the recipient's prevention still match it, as gorge's
  re-collection does; pair 88223, a blocker the pause's state-based
  actions killed), while an event whose recipient is gone deals nothing.
- The cast ring (cast_castledger.go) is gorge's
  this-turn cast ledger, bounded: the first `castRingN` = 12 pushes per
  player per turn (the 13th stalls with StallCastRing), as Obj+1 in
  `hist`, their from-hand bits in `histCastFl`, the
  Treasure-spend bits in `histCastTr`, and one origin byte per entry in
  `genState.castProv [2][castRingN]uint8` (bits 0-1: 0 other, 1 hand, 2
  graveyard, 3 exile; bit 2 reserved for ManaFromTreasure, which
  `histCastTr` holds today). packing the bits into the Obj+1 bytes was rejected (MaxObj = 192 needs
  all 8 bits); castProv is a plain fixed array next to `hist`, written only
  when `castRingOn` (FDN/BLB keep it zero) and, like `hist`, in no hash. Gorge's reads:
  - `wasCastFromExile` is answered per push from its origin (gorge
    `castOriginAdmits` on the ledger's cast view, any caster).
  - The bare `wasCastFromGraveyard` reads the push's origin like the others
, so Laboratory Drudge's "if you've cast a spell from a
    graveyard this turn" stays true at end step after a resolved flashback
    spell (CR 400.7, 601.2). dirac reads the ring entry's origin.
- D-entry-order: a tap and an untap body competing over one
  entering permanent (a printed enters-tapped land beside "lands you control
  enter untapped", The Wandering Minstrel / Spelunking, `ExtEnterUntapped`)
  are gorge's CR 616.1 order ask on the MoveZone event
  (`composeUpdatedReplacements`: the Note replaces the move, the move is
  emitted at the first answer, then the chosen body runs and the rest is
  re-checked; a tap and an untap still left are asked again, the permanent
  already entered, otherwise the rest applies in scan order). dirac
  poses it as `DkChoose` + `ChGenReplOrder` over `g.ro` (the borrowed
  combat-only bytes, State size unchanged) to the entering object's
  controller before the move, with the card still in its zone, on two
  routes only: a land play (`playLandNow`; gorge counts the play at once,
  dirac counts it at the park) and a resolving program's one-card move to
  the battlefield (`OpReturnEx`, or a one-card hand `OpChooseMove` as
  Spelunking's). The candidates are derived in gorge's scan order: each seat
  in turn, its library and hand (the entering card's own enters-tapped line)
  before its battlefield, so the entering card's own line precedes its
  owner's carriers. Not modelled, and loud (`StallEntryOrder`, `StallReplCands`
  past eight candidates): a tap/untap pair meeting any other route (a token,
  a clone, a permanent spell resolving, a dig or search placement, a
  graveyard or exile entry, an object entering under another player's
  control), and a pair beside a self enters program, a self counter, or an
  other-enter body that is neither a tap nor an untap (gorge poses those as
  further options). An untap carrier's `Repl.Cond` is not evaluated by
  `enterGen` for an other-enter program, so the conditional pair of Archelos,
  Lagoon Mystic (`Card.Self+tapped` / `+untapped`) stays refused at lowering.
- D-aura-reask: an Aura returning from an
  until-leaves exile outside a resolution (its source died to an SBA) with
  several hosts is asked of its owner at once, mid departure batch, card
  still in exile (`queueAskRet`; the hosts are the permanents entered by
  then, so one returned later in the batch is no answer). gorge parks only
  one entry (`applyETBChoiceReplacement` needs `e.pending == nil`): a
  second non-cast Aura returning in the same batch enters at once attached
  to its first candidate in battlefield order (`firstHost`), and its entry
  overwrites gorge's single `auraEntry` record, so the first Aura's answer
  is dropped (`answerAuraEntry` no longer matches) and it too attaches to
  its first candidate when it enters (CR 303.4f says its owner's choice
  stands). dirac marks the pending ask (`askRec.pc`) and ignores the
  answer. Surfaced by TestWardLockstep seed 197 (two Pacifisms under two
  Sheltered by Ghosts).
- The optional top-of-library carriers (exact). Traveling Botanist, Bucolic Ranch and Gathering Stone are
  `Supported` in the `peekreveal` pool. (1) A taken reveal: the sub `Defined$
  Remembered | Origin$ Library | Destination$ Hand` is a fetch, not a search
  (CR 701.23a), and the printed text has no shuffle. The pinned gorge
  (`c4f1ece62`, pin `70920b041`) shuffles after a `Defined$` library fetch
  only through its `implicitShuffle` (a library destination, `NoLooking$`, or
  a search that deferred its shuffle), so a plain reveal-and-take shuffles
  nothing; dirac copies gorge's `Origin$ Library` gate
  (`BncFromLibrary` / `RetFromLibrary`) and follows the separate owed-shuffle
  gate (`BncShuffleOwed` / `RetShuffleOwed`; `interp_gen.go` execReturnObj,
  `zone_rememberparams.go` bounceRemember), which stays clear
  for the carriers' Bounce (`C: 4096` = `BncFromLibrary` alone).
  `ktoplibPolicy` takes the may-reveal three times in four.
  `TestPeekrevealLockstep` takes the may-reveal and covers the taken fetch
  gate on Traveling Botanist and Gathering Stone, holding the unchanged
  library order and RNGDraws in lockstep (the absence of a shuffle).
  Its policy elects Bear at Gathering Stone's whole-vocabulary creature-type ask so the
  chosen-type match, and thus its reveal, is reached deterministically.
  (2) Closed: Gathering Stone's chosen-type reads are exact
  (`RevealValid$ Card.ChosenType` through `matchCardSrc` in `peekMayReveal`,
  the `ReduceCost | ValidCard$ Card.ChosenType` static through
  `matchSpellCard`'s source slot: `TestPeekRevealChosenType`,
  `TestCostChosenType`, `TestGatheringStoneLockstep`) and it is back in
  `ktoplibDecks`. The Top-of-library move itself (the reveal-not-taken else
  branch) is exact (`TestToplibLockstep`, `TestTopLibraryGuard`); Bucolic
  Ranch's reveal is never taken (`RevealValid$ Card.Mount` has no Mount in
  the pool), so those tests exercise only its declined else branch.
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
- Two or more "can't be countered" replacements matching one Counter event
  (a spell's own plus a battlefield Chimil / Sphinx of the Final Word, or two
  such sources): gorge CounterAllowed parks the event
  on a CR 616.1 order ask (replacement_choice.go replChoiceCounter, answered
  by the spell's controller) though every candidate only stops the event.
  dirac does not pose that ask: the game stalls with StallUncounterOrder
  (repl_uncounterable.go), loud, never a silent divergence. One match
  (the usual case) is exact.
- Ward—Sacrifice a <filter>: CantSacrifice restrictions (gorge
  sacrificeBlockedForCost) are not modelled: no pooled card has one.
- Hollowmurk Siege's Sultai draw (granted
  `Mode$ CounterAdded | ValidCard$ Creature.YouCtrl | ActivationLimit$ 1`):
  gorge does not enforce `ActivationLimit$` on an `AddTrigger$`-granted
  trigger (pin b4e6762a27f9, `engine/rules/trigger_granted.go`
  `checkGrantedStaticTriggersUsing` never calls `triggerActivationLimitAllows`
  / `reserveTriggerLimits`; Forge's `Trigger.checkActivationLimit` does), so
  a second counter in one turn draws a second card in gorge only. dirac
  keys the limit per (object, ability), which is the card text, and does not
  emulate gorge. With four or more Hollowmurks about half of 1000 seeds of
  pool `siege` diverge (seed 1, a `trigger_order` decision: ref two
  triggers, dirac none), as do two Hollowmurks beside Barrensteppe (several
  counters in one turn); `khollowDecks` caps the pool at two copies and no
  Barrensteppe, where only one counter a turn is reachable (`TestSiegeHollowmurkLockstep`).
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
- The legendary-source combat-hit bit (Blitzball's
  `HasPropertywasDealtCombatDamageThisTurnBy Creature.Legendary GE1`): gorge
  snapshots the hit source's printed face and matches the spec at the READ,
  where a layer-4 type entry for the source's ID (a crewed Vehicle, an
  animated artifact) is read live and a dead source falls back to its printed
  face. dirac reads the source's types at the HIT (`typesAt`, layers
  applied). The two agree unless a type-changing effect on a legendary source
  begins or ends between the hit and the read (the same turn); no pool card
  has the shape (an animated legendary non-creature, or a legendary creature
  that loses Creature mid-turn). `NumStall` is 131 (`StallCombatHit`, a fifth
  source that reads the BySource word dealt combat damage to a player in one
  turn).
- DealDamage `DamageSource$ <trigger object>` to each opponent
  (OpDealDamageFrom with B.N = 1,
  `dirac/act_damagefrom.go` `dmgSrcPlayers`): two gaps against gorge
  `newDamageRider`. (1) A source that left the battlefield reads infect from
  its printed card, where gorge reads `lki.Infect` (CR 113.7a); the LKI
  record keeps no infect bit, so a source that gained or lost infect before
  leaving would differ. Its lifelink and controller are read from the LKI
  record (`TestDmgsrcplMutationLKIKw`, `TestDmgsrcplMutationLKICtrl`).
  (2) damageTo's replacement-order ask (`parkNC`, CR 616.1) is not posed;
  two noncombat damage replacements that both apply to the player would
  order differently. Neither shape is in a pool (`dmgsrcpl` has at most
  one noncombat damage replacement and no infect grant).
- A "single target" filter on a stack object (`singleTarget`,
  `engine/effects/filter.go`) reads `len(o.Targets)` and leaves out the
  sub-abilities' targets, so Redirect Lightning is offered on The Last Agni
  Kai's fight or How to Start a Riot, which have two targets. The card says a
  spell or ability with a single target; Forge's property counts
  `getAllTargetChoices`, read, not run. dirac counts the whole chain
  (`singleTargetOK`), so on the full surface these pairs diverge until gorge
  is fixed (std-tla 125, 381, 400).
- A paid Offspring whose source leaves with its enter trigger on the stack: gorge's
  `Count$OffspringPaid` reads what the source left (CR 608.2h) and `CopyPermanent` mints the
  1/1 from the departed object; dirac's lowering (`Count$Valid Card.Self+kicked`) reads
  0. Read, not played: gorge offers the offspring cast only against a floating pool that pays
  all of it, and 800 lockstep games with Burning-Tree Emissary never reached one. Multikicker,
  `Count$TimesKicked`, Mutate and OptionalCost on a permanent spell are refused by kgen, so
  they have no such read.
- The look-back observer `checkTriggersNow` builds doesn't carry `Engine.startingLife`, so
  `h.StartingLife()` reads 0 on it, and Leyline of Hope's `Count$YourStartingLife/Plus.7` gate is 7
  there. A creature dying at 17 life then reads +2/+2 in its leaves-the-battlefield trigger: Infernal
  Phantom deals 4 with power 2 at every decision (std-dsk 53). Adding the field to the observer's literal
  in a patched gorge export makes the pair run clean to its end. CR 603.10a reads the board as it was,
  whose condition was false.
- A Changeling card off the battlefield shares no creature type for `TargetsWithSameCreatureType$`:
  `setPropTokens` reads `e.Derived(obj).Types`, which carries only the printed Shapeshifter there, so
  gorge doesn't offer Unbury's two-card mode for a Changeling and an Elf in a graveyard (std-ecl 38, 141,
  194, 271, 365, 443, run). Changeling is a characteristic-defining ability that works in every zone (CR
  702.73a, 604.3). Forge's static is `EffectZone$ All` and XMage's `shareCreatureTypes` reads
  `isAllCreatureTypes` (both read, not run). All six pairs run clean on a patched gorge export whose
  `setPropTokens` gives an `effects.IntrinsicAllCreatureTypes` card off the battlefield every creature
  type. dirac counts it (`crTypesOf`), so these pairs diverge until gorge is fixed.
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
