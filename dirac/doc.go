// Package dirac is a second Magic rules engine, also called the kernel: a 2-player game as a
// State (clone with CopyFrom; go vet rejects an assignment) driven as a
// Step/Apply state machine over a kop.Table of cards. It imports nothing from gorge; gorge is its oracle
// (package dirac/oracle).
//
// Import boundary: dirac, dirac/kop, dirac/feat and dirac/search import nothing
// from gorge's packages. dirac/oracle and dirac/kgen may import any of them.
//
// # Files
//
// The core keeps plain names (cast.go, combat.go, mana.go, state.go). Everything
// else is one file per mechanic, <topic>_<mechanic>.go, and a new mechanic adds a
// new file. The topics:
//
//   - kw: keywords (CR 702)
//   - act: keyword actions and effect verbs
//   - mana: producing and spending mana
//   - cost: costs and their payment
//   - cast: casting and what may be cast
//   - copy: copies of spells, abilities and permanents
//   - face: multi-face cards, face-down permanents, rooms and battles
//   - modal: modes
//   - ctr: counters
//   - trig: triggers
//   - repl: replacement effects
//   - static: static abilities and the layers
//   - tgt: targets
//   - filter: filter words
//   - value: counts and values
//   - zone: zones and moves between them
//   - combat: combat
//
// # API
//
//	s := dirac.NewConfig(tbl, deckA, deckB, seed, dirac.Config{Mulligans: 2})
//	for {
//		d := s.Step()            // or s.StepInto(&d): no Decision copy
//		if d.Kind == dirac.DkNone { break }
//		s.Apply(d.Options[i])    // one of d.Options[:d.N]
//	}
//	over, winner := s.Terminal()
//
// An Action packs kind<<24 | source<<16 | target<<8 | mode (types.go lists
// each ActionKind's fields). Sources and targets are Obj indices; players
// are PlayerTarget0/PlayerTarget1.
//
// # Objects
//
// Obj i is a card instance. Objects 0..len(deckA)-1 are seat 0's deck in
// deck order, the next len(deckB) seat 1's, then tokens in creation order:
// the same relative order as gorge's ObjIDs (AddObject per seat, deck
// order; tokens minted later), so the oracle maps ObjID order to Obj order.
// An object keeps its index across zones; State bumps a private version on
// every zone change so a stack target that left and came back is illegal
// (CR 400.7).
//
// # Projection
//
// Project writes a Projection: per Obj (index order) its card, owner,
// controller, zone, position in that zone (library from the top; hand,
// graveyard and exile in arrival order), token bit, tapped / attacking /
// blocking / summoning-sick flags, damage, counters by kop.CounterKind,
// derived power/toughness (battlefield creatures), attached-to,
// blocked-attacker and attacked-defender objects; then life, the stack
// bottom to top -- a spell or ability is on the stack from the moment gorge
// pushes it (after the cost picks, before modes and targets; a trigger
// before its targets) until its resolution finishes (kind
// 0 spell / 1 activated / 2 triggered, source, card, ability index or 255
// for a permanent spell, controller, targets, modes, X), turn, step
// (kop.Step; the kernel splits gorge's CombatDamage step into
// StepFirstStrikeDamage and StepDamage), active player, priority holder,
// RNG draw count (gorge Engine.RNGDraws), lands played, mana pools, zone
// sizes, game over and winner. Every field is a fixed array; equal
// positions project equal (==).
//
// # Rules followed from gorge (engine/rules)
//
//   - RNG: math/rand/v2 PCG seeded (seed, seed^0x9e3779b97f4a7c15), IntN as
//     rand.Rand, a draw counter (rng.go). Genesis: the toss IntN(2) first,
//     then per seat in index order a Fisher-Yates shuffle of the whole
//     library and 7 draws, library top = gorge Zone(ZLibrary)[0].
//   - London mulligan (Config.Mulligans = gorge Config.Mulligans; 0 skips
//     the round): passes in turn order from the starting player; a
//     keep-only decision once the limit is reached; redraw = hand to the
//     library end in hand order, shuffle, draw 7; then bottoming.
//   - Turn: the starter skips the turn-1 draw step; untap and cleanup pose
//     no priority; every other step runs a full priority round. gorge
//     tracks (holder, passes): an action gives the actor priority with 0
//     passes, a resolution gives it to the active player with 0, placing
//     triggers keeps both, and a "setStep" jump keeps both. With no
//     attackers DECLARED the game jumps from declare attackers to end of
//     combat (non-active player holding, one pass counted). The first-strike
//     step is decided when combat damage begins (an attacker or a live
//     blocker with first/double strike). State-based actions run right
//     after each damage pass. gorge's between-passes round is a single
//     decision for the active player: its first answer (a pass, or a
//     completed cast/activation) runs the regular damage pass at once, even
//     with the stack non-empty, and end of combat keeps the holder and pass
//     count that answer left. After regular damage the game jumps
//     to end of combat with no damage-step priority round (keeping the
//     holder when there was a first-strike step, else a fresh round).
//     Drawing from an empty library loses at once.
//   - The stack: a spell goes on the stack after its X and cost picks and
//     before its modes and targets (pushCast); an activated ability only
//     after its targets and costs (AbilityPush at the end of payCast); a
//     triggered ability before its targets. Ordered trigger groups are
//     pushed only after the whole order is chosen. Delayed "next end step"
//     triggers are pushed at the end step (source = the affected object,
//     ability 255 in the Projection).
//   - Kicker and flashback: gorge offers a kicked cast as a separate cast
//     option (Mode "kicked": AkCast mode bit 0) and a flashback cast from
//     the graveyard (bit 1); neither asks afterwards. A kicked cast has no
//     gorge payment plan, so the oracle lowers it by tapping the sources at
//     priority and casting from the floating pool: the kernel taps the
//     planner's sources into the pool at the kicked cast action to match.
//   - Autopay (mana.go): the floating pool pays first; sources are gorge's
//     planner units (pay/plancost.go, payment_plan.go:888,910: fixed
//     sources in battlefield order, any-colour sources after) ranked by
//     (creature sources, units, surplus, flex, -HandReserve, -Remainder,
//     least ObjID sequence in unit order) (pay/plan.go:195-334,
//     pay/planner.go:234-252); generic is spent from the pool in order
//     C W U B R G (pay/solve.go:350-445).
//
// # Decision grammar (how gorge's decisions map onto kernel picks)
//
// Every kernel decision is a single pick. Decision.Sub says what a
// DkChoose picks (ChoiceKind). gorge's multi-selects map as follows:
//
//   - Attackers (gorge KAttackers, one multi-select of (attacker, defender)
//     pairs): AkAttack{source: creature, target: PlayerTarget* or a
//     planeswalker Obj} picks in any order, closed by AkDone. A picked
//     attacker is not offered again. No decision when nothing can attack.
//   - Blockers (gorge KBlockers): AkBlock{source: blocker, target: attacker}
//     picks in any order, closed by AkDone; a blocker that blocked is not
//     offered again. A menace attacker that fewer than two of the
//     defender's creatures could block is never offered (gorge
//     minImpossible). Only picks after which the declaration can still be
//     completed are offered, and AkDone only when it is legal (a menace
//     attacker never left with a lone blocker), as gorge's Decision.Validate
//     and the oracle's multi-pick emulation require.
//   - Damage division (gorge KChoose "division"): ONE DkChoose/ChDivide
//     decision, AkChoose{source: attacker, mode: split index}; the splits
//     are every nonnegative split of the attacker's damage among its
//     blockers in block order, in lexicographic ascending order of (first
//     blocker's share, second's, ...) exactly as gorge lists them; SplitAt
//     decodes an index. Asked per attacker in battlefield order, per damage pass,
//     only without trample, with 2+ blockers and at most 128 splits.
//     Config.AutoDivide answers it automatically instead (default off).
//   - Scry / surveil (gorge KArrange, one ordered-subset decision):
//     DkChoose/ChArrange picks AkChoose{source: a looked-at card}, each
//     putting that card next from the top (the first pick is the new top),
//     closed by AkDone (or by picking every card). The unpicked cards go to
//     the bottom (scry) or the graveyard (surveil) in library order: gorge's
//     answer with Choices = the picks in order and no Rest.
//     Config.ArrangeRestOrder (default off) adds the pile-B order (gorge's
//     Intent.Rest; CR 404.3, 701.22a): AkDone with two or more cards
//     unpicked opens a second phase of the same DkChoose/ChArrange picks
//     over those cards (no AkDone), the first pick the first card put into
//     the graveyard / on the bottom, all but the last card (implied); the
//     answer is Choices = the top picks, Rest = the pile-B order. Picking
//     every card, or leaving at most one, closes the decision as before.
//   - Cost picks (gorge KChoose "tapcost" / "sacrifice" / "discard",
//     Min=Max=N): N picks of AkChoose{source: object}, Sub ChCostTap /
//     ChCostSacrifice / ChCostDiscard, in gorge's order: tap-creatures costs
//     before X, then sacrifice, then discard, all before the spell or
//     ability goes on the stack. A tap cost with exactly N candidates is not
//     asked (gorge skips it); sacrifice and discard are asked even then. The
//     chosen objects move when the cost is paid (after targets), as in gorge.
//   - Effect discards (gorge KModes "discard", options in hand order): Sub
//     ChDiscard, one pick per card; the picks are recorded and the cards
//     discarded together in pick order, so the hand (and each card's
//     Projection ZoneIdx = gorge's option index) is stable across the picks;
//     not asked when the hand holds N or fewer.
//   - Either-or additional cost (gorge KChoose "altaddcost", Eaten Alive):
//     DkChoose/ChAltAdd before every other cost pick, AkChoose{target: part
//     (0 the spell's own additional cost, 1 CardDef.AltAddCost), mode: option
//     position}, payable parts first; choosing an unpayable part undoes the
//     cast (back to priority), as in gorge.
//   - Cleanup discard (gorge KChoose "discard", Min=Max=excess): Sub
//     ChCleanupDiscard, one pick per card. Mulligan bottoming (gorge
//     KMulligan bottom): Sub ChBottom, one pick per card, in bottom order.
//   - Trigger order (gorge KTriggerOrder, Min=Max=n): DkTriggerOrder picks
//     AkOrder{source: the trigger's source Obj, target: pending index,
//     mode: occurrence index among pending triggers from that source}; every
//     one of the n picks is posed (the last one too) and the group is pushed
//     in pick order after the last pick, each then asking its targets.
//     PendingTrigger(i) describes one.
//   - X (gorge KChoose "x"): DkChoose/ChX after the cast action, mode = X.
//   - Legend rule (gorge KChoose "keep"): DkChoose/ChLegend, the permanent
//     to keep; groups are found seat 0 first.
//   - Optional trigger at resolution (gorge KTriggerOptional): ChOptional,
//     mode 1 = yes. Ward payment: ChWardPay.
//
// # Deviations from gorge (the oracle must map or avoid these)
//
//   - Casts autopay: a cast is offered whenever untapped sources plus pool
//     can pay (Hall's condition), mana abilities are never priority options,
//     and there is no concede option. gorge offers a plain cast only when
//     the floating pool covers it; the oracle uses gorge's PaymentActions.
//     X spells (which gorge's planner declines) are offered X = 0..max
//     affordable and paid by the same planner.
//   - Eaten Alive, Luminous Rebuke, {X} spells: gorge's cast planner has
//     no plan for them; the oracle reaches them by lowering a potential
//     play (Config.GorgeLowering) or offers them only when the floating
//     pool pays (Config.GorgeAutopaySurface).
//   - Sub-counter cost asks (gorge KChoose "subcounter", one counter per
//     pick): the only FDN one, Quilled Greatwurm's graveyard cast "by
//     removing six counters from among creatures you control", is
//     DkChoose/ChCostSubCtr, AkChoose{source: a creature, mode: its
//     counter kind}, after the sacrifice picks and before the discard
//     ones; a lone (creature, kind) unit is taken unasked, and the counters
//     leave at payment. A counter cost on the source itself is never asked
//     in gorge or here.
//   - Ward and granted prowess are kernel-native triggers (ability index
//     abWard / abProwess); printed prowess is the card's own SpellCast
//     trigger, as gorge expands it.
//   - With Config.NoElide off (the default), every decision with a single
//     option is answered in place and counted in State.Skipped; set
//     NoElide to pose them all (the oracle does).
//   - Config.RecordChance records every draw's bound; ChanceTranscript
//     returns (bound, value) pairs (values recomputed from the seed) for the
//     oracle's forced-outcome replay.
//
// # Interpreter
//
// interp.go implements the kop ops DealDamage, Draw, Destroy, Exile,
// Bounce, ToLibrary, PumpUntilEOT, GrantKeyword, PutCounter, RemoveCounter,
// CreateToken, GainLife, LoseLife, Discard, Scry, Surveil, Mill, Fight,
// Bite, Tap, Untap, Sacrifice, Counter, Attach, Charm, IfKicked, DamageAll,
// DestroyAll, PumpAll, AddMana, Return, Repeat, SkipIf, LoseGame, Shuffle,
// AddTurn, DelayedTrigger, GainControlEOT, SetPT, LoseAbilities, Animate,
// Regenerate. Every other op panics naming itself, except on a generated
// table (below).
//
// # Generated tables
//
// A table built by dirac/kgen (prep.gen != nil) runs the ops, refs, VExpr /
// Cond kinds, statics, replacements, filter predicates, keyword bits and
// overrides listed in OPS.md (kgen -ops). That code lives in the *_gen.go
// files, exprs.go and interp_ask.go and is reached from one-line hooks in
// the files above; on a vanilla table every hook is skipped. Its extra
// state is State.g (genState), the LKI snapshot in pendingTrig / stackEntry
// (lki) and the extra objects of a multi-object target slot (stackEntry.xt).
//
// Its decisions keep the grammar above: one pick per item, AkDone where the
// answer may stop early, Decision.Sub naming what a DkChoose picks.
//
//   - Multi-object target slots (TargetSpec.Min/Max, the kicked range,
//     distinct / same / different controller): one DkTarget pick per
//     object, AkTarget{target}, a picked object not offered again, closed by
//     AkDone once the minimum is met (automatically at the maximum; with
//     only AkDone offered when no candidate is left, as gorge asks, except
//     for a trigger with no pick yet). Damage divided (TargetSpec.Divide)
//     is announced while casting/activating, after the targets and before
//     any cost is paid (CR 601.2d): one DkChoose/ChCastDivide per point
//     beyond each target's 1, source = the target given it (a player target
//     named by Player). At resolution each still-legal target is dealt its
//     announced share and an illegal target's share is lost. Only an
//     unannounced division (a copy) divides at resolution as gorge asks it:
//     one DkChoose/ChGenDivide per point, source = the target dealt it
//     (repeatable; a target never picked is dealt nothing, as gorge allows).
//   - Resolution asks, each a DkChoose to the player the op names:
//     ChGenBranch (mode = branch), ChGenPay (mode 1 = pay an unless / if-you-do
//     cost, offered only when it can be paid: an unpayable cost is still
//     asked, decline only, as gorge asks it; ChWardPay for ward-pay-life),
//     ChGenPayPick / ChSacrifice /
//     ChCostDiscard (source = one object paying, one pick per object),
//     ChGenTopBottom (mode 1 = top), ChGenSearch (source = a found card, or
//     AkDone unless the search is mandatory; ChGenHiddenPick from a
//     graveyard or exile and ChGenHandMove from a hand, gorge's
//     hidden_pick / hand_move), ChGenDig (source = a
//     looked-at card taken, AkDone-closed), ChGenPileSplit (source = a card
//     for the face-down pile, AkDone-closed) then ChGenPileChoose to the
//     opponent (mode 0 face-down / 1 face-up pile), ChGenChooseEach (source
//     = the permanent kept for the card type in mode, one ask per type in
//     artifact, creature, enchantment, land, planeswalker order),
//     ChGenCastFree (source = a card to cast without paying its mana cost,
//     AkDone-closed; only cards with a legal target are offered; the chosen
//     cards are cast in pick order once the resolution ends, each through
//     the ordinary build), ChGenChooseCard (source = the card).
//   - As-enters choices (OpChooseColor / OpChooseType from an enters
//     replacement): asked of the permanent's controller before the next
//     priority, before triggers are put on the stack: ChGenColor (mode = W
//     U B R G as 0..4) and ChGenType, whose options are gorge's whole
//     creature-type vocabulary in vocabulary order (effects
//     .CreatureTypeWordList, kop.Table.TypeVocab), the 16-bit vocabulary
//     index as (target<<8 | mode), so the option lists match gorge's 1:1.
//   - Leylines in the opening hand (CR 103.6a): one ChGenLeyline per
//     Leyline in hand (hand order, starting player first), source = the
//     card, mode 1 = put it onto the battlefield, 0 = don't (gorge's
//     opening_yes / opening_no).
//   - Play permissions and play-from-zone statics add AkCast options with
//     mode bit 3 (and bit 4: without paying the mana cost); a granted
//     activated ability (an Equipment's) is AkActivate{source: the creature,
//     target: the granting permanent, mode: 0x80 | its ability index}; a
//     graveyard ability is AkActivate{source: the card in the graveyard}.
//
// Triggered-object refs (RefTriggered, RefSacrificed, the source of a
// dies trigger) read last known information: the object's controller, P/T
// and counters are snapshot into the pending trigger when it is queued.
// Drakuseth-style damage maps (OpDealDamageEx then OpResolveDamageMap)
// deal all their damage as one simultaneous event.
//
// Generated-table deviations from gorge:
//
//   - A game that outgrows the kernel's fixed capacity (battlefield or
//     object table full, more than MaxPending simultaneous triggers, the
//     stack at MaxStack, the override / multi-target pools full, a hand,
//     graveyard, exile or library array full of tokens waiting to cease, a
//     combat declaration with more pairs than MaxOptions) ends as a
//     draw, State.StallReason saying why (gorge's "bigboard" stall); so does
//     a turn that resolves more than 2000 triggers (an endless mandatory
//     loop, CR 104.4b).
//   - A permanent sacrificed to pay a cost that could still tap for mana
//     is sacrificed after the mana is paid (CR 601.2g-h).
//   - An as-enters colour / type choice whose replacement program starts
//     with the choice is asked while the permanent spell is still on the
//     stack, as gorge asks it (preEnterAsk), and a played land's while it
//     is still in the hand, the land play (and the land count) completing
//     once it is answered (preLandAsk: Secluded Courtyard); a later one in
//     the program is asked once the permanent has entered.
//   - Hybrid pips (Fiend Artisan's {B/G}): gorge's planners decline a
//     hybrid cost, so its cast is offered when the pool pays it or a
//     permission's potential play is lowered (its witness the first
//     resolution, pip 0 fastest, that the sources afford). A hybrid {X}
//     activation (Fiend Artisan) is offered only when the pool pays it
//     (Config.GorgeUnpriced); one without {X} is planned like any other
//     (gorge planHybridCost; Puresight Merrow, kverify-c4). Each
//     pip's colour is asked after modes, before targets (ChGenPip, gorge
//     manaAsk), over the faces the pool and untapped sources leave payable;
//     a hybrid pip in hand demands both its colours (pay.CostPips).
//   - Bloom Tender ("one mana of each colour among permanents you
//     control", AfManaEachColor) is no autopay source: gorge's planner
//     census misses it, so a play the planner proves insufficient gets a
//     scripted prefix that activates it first (rules potentialPrefixScript;
//     only the first sixteen insufficient plays of the walk are searched; the
//     autopay surface drops a scripted plain cast). script_gen.go mirrors
//     it: the play is offered when the sources plus its mana pay, and at the
//     play it is tapped into the pool first. The sixteen searched plays are
//     taken in gorge's walk order (hand, other zones, then activations in
//     battlefield entry order), and the gaps are tapped in battlefield
//     order (two Tenders). A restricted source is no gap since gorge
//     (it is a planner unit, below); only a reflected one (Plaza of
//     Heroes, gfRestrScript) still is: gorge's search taps it when it comes
//     first and its mana prices the play, and the oracle then completes the
//     script only when the unrestricted pool pays the play alone.
//     A deferred source (Pyromancer's Goggles' TriggersWhenSpent$ {R},
//     kop.Ability.ManaDeferred, gfDeferred; mana_deferred.go) is a gap too:
//     gorge's planner defers it, so autopay never taps it, and its census
//     is relaxable (a free {R}), so it is searched, in scriptRestr's order,
//     by the same sixteen-play prefix search; its {R} floats in the plain
//     pool (typed, an artifact's), the cast that spends it records the
//     source (genState.defSrc, build.obsSl) and the source's own SpellCast
//     rider (Tf2ManaFromSelf) copies the spell. The window (a ward payment)
//     taps it as a gap as well.
//     Restricted mana floats beside the pool, restricted
//     (State.rb, gorge's RestrictedMana: up to four batches, each of one
//     restriction, in the projected pool). gorge's planners read it as the
//     payment does (pay.PlanPool: a batch counts exactly when the payment
//     admits it, CR 106.6), so a cast is offered when the pool and the
//     untapped sources pay it together (CR 601.2g-h). A
//     spell the restriction allows (an Angel for Giada's {W}) sees the
//     restricted mana in its pool and spends it first (gorge AvailableFor,
//     emitRestrictedManaSpend; restrSpendable, payRestr). With only
//     restricted gaps gorge's census is relaxable: a play the gaps' mana
//     cannot pay takes none of the searches. A reflected restricted
//     any-colour ability is a gap too (genPrep.anyR): the search branches
//     on its answers ({C} first, then W U B R G) depth first along the
//     battlefield-order chain (scriptDFS); so is a restricted two-mana
//     Combo Any (Interdimensional Web Watch, restrWideAnyMana), whose
//     answers are the 15 colour pairs (W W first). The potential walks list
//     a may-play cast gorge's planner leaves unpriced (Tinybones' any-type
//     mana), so a script can make the pool pay it (pair 7489's Hawk took
//     the Courtyard's {C}).
//     gorge's ward / unless mana window lists every mana ability, so it
//     activates a Tender or Giada in battlefield order with the autopay
//     sources (mana.go windowTap); Giada's {W} is kept off the cost and
//     floats afterwards, restricted (State.rb). A loyalty ability's resolving
//     restricted mana (Chandra, Chill of Compliance's noncreature-only {U};
//     CR 605.1a, 106.6) floats in State.rb too: AddMana's F names the
//     restriction (mana_resolverestriction.go).
//   - A tap mana ability with a spending restriction (CR 106.6; gorge
//     ) is a planner unit for exactly the payments its RestrictValid$
//     admits (mana_restrictedsource.go, pay.RestrictValidMatches): each plan
//     carries a payment descriptor (a spell's card, an activated ability's
//     source, or other: echo, cumulative upkeep, a triggered cost);
//     Spell.<spec> admits a matching spell, Activated.<spec> an ability of a
//     matching source (Secluded Courtyard's Activated.Creature+ChosenType,
//     kop.Ability.ManaRestrictAb), CantCastNonArtifactSpells every non-spell
//     payment and artifact spells (the Powerstone, AfManaNonSpell); a
//     ChosenType reads the mana source's own choice. A source with no
//     census ability of its own (Giada, a Powerstone, The Ringhart Crest) is
//     a unit with the choice-shaped ones in battlefield order; one beside a
//     census ability (a BLB Village, Secluded Courtyard) widens that unit at
//     its census position and, when withheld, still ranks (keys 5 and 7) by
//     every colour it makes; Helga's Amount$ X joins the evaluated-amount
//     layer, its surplus floating restricted (State.rb; StallRestrSrc past
//     the batches it holds). The offer gates count the admitted units too. The
//     ward / unless window keeps activating restricted sources in
//     battlefield order. A mana ability with a dynamic amount (Elvish
//     Archdruid) and no restriction is not an autopay source; a chosen-colour
//     one (Heraldic Banner) is a source of the chosen colour. A granted
//     one (Imprisoned in the Moon's "{T}: Add {C}") is, and an override
//     that removes abilities removes the mana ability too. Quilled
//     Greatwurm's six counters are removed automatically (slot, then
//     counter-kind order). The either-or additional cost (Eaten Alive) is
//     ChAltAdd.
//   - Damage, token, counter and graveyard replacements (repl_gen.go:
//     Twinflame Tyrant, Crystal Barricade, Doubling Season, Progenitus)
//     are computed in a fixed order: amount changes first (each Twinflame
//     doubles), then prevention. gorge asks the affected player to order
//     them (KReplacement, CR 616.1) when two match one damage event. Every
//     order gives the same result; on a combat damage event the kernel
//     poses the same ask, ChGenReplOrder (source = the replacement's
//     source, mode = its index in gorge's scan: the source controller's
//     Twinflames in battlefield order, then the recipient's Fleeting
//     Flight prevention), re-asked while two or more remain and a
//     prevention was not picked. The pass's events are then dealt one at
//     a time in gorge's assignment order (each attacker's, then its
//     blockers'), the earlier ones before the ask, and nobody dies until
//     the pass ends (CR 704.3). The noncombat ask (two
//     Twinflames on a burn spell, Twinflame with Crystal Barricade on the
//     Barricade controller's other creature) pauses the resolution after
//     the damage op (raGenReplOrder, the spell still on the stack; parkNC)
//     and the answer deals the parked damage. Two Doubling Seasons
//     commute in gorge too (x4, no ask). Following gorge: a fight's or bite's damage source is
//     the resolving spell or ability (its controller's Twinflame applies,
//     not the fighting creature's); a counter cost is doubled when the
//     stack is non-empty (gorge's EffectOnly test); Progenitus is put on
//     the library top before the shuffle, and a discard of it still
//     triggers "whenever an opponent discards" (which finds no card).
package dirac
