package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Escape (CR 702.138a), cold: reached when the table has a card with printed
// escape (prep.escape) or a StatEscapeGrant static (prep.costObj through
// genPrep.anyEscapeGrant).
//
// Printed escape: a card in its owner's graveyard may be cast for the
// keyword's cost (CardDef.AltCost) while exiling the printed cost's other
// cards from the graveyard (CardDef.EscCost, kop.CostExileGrave; gorge rules
// altcast.go escapeCost, legal_walk_alt.go's graveyard walk, Option Kind
// "cast" Mode "escape"). Lunar Hatchling's printed cost adds one
// Exile<1/Land> part from the battlefield, a counted part of its own beside
// the grave exile; the kernel's cast carries one counted part, so kgen
// refuses that cost shape (Lunar Hatchling stays refused by name).
//
// Granted escape: a card in its owner's graveyard granted escape by a
// battlefield static (kop.StatEscapeGrant, Underworld Breach's AffectedZone$
// Graveyard AddKeyword$ Escape) may be cast for its escape cost: the card's
// mana cost plus N OTHER cards from the same graveyard. The fodder rides
// buildPickCost's pick machinery (act_forage.go), and bsCosts' escape arm
// poses the entry's own Sac/Discard additional asks before the exile picks,
// gorge's costObjectAsks order (exAsk is last).
//
// Either way the cast is an ordinary one from the graveyard: timing rules
// apply, and unlike a flashback there is no exile tail -- the exiles are paid
// at the cast, the spell goes where it would otherwise go. The spell stamps
// gorge's FlagEscaped provenance: the kernel marks the entry trigPlEscape
// (beginCast prices a printed cast at CardDef.AltCost and a granted one at
// CardDef.Cost, and picks the printed CardDef.EscCost or the grant's fodder
// for it) and the entering permanent the arena's mechEscaped slot mark
// (turn_gen.go), which every "escaped" filter read (kop.PredEscaped, gorge
// filter.go's 'escaped' predicate over CastFlags&FlagEscaped) matches until
// the permanent leaves the battlefield -- gorge resets CastFlags there
// (events/apply_zone.go), so a re-entered permanent is a new object nobody
// escaped (arena.go's arenaClrSlot on re-entry). A copy keeps no mark
// (mintSpellCopy, CR 707.10), and pushBuild records the graveyard zone in
// the entry's prov byte, so ExCastFromGrave (gorge
// state.ObjectWasCastFromGraveyard names FlagEscaped) reads true through
// castFromGrave without a flag of its own.
//
// gorge's planner prices no "escape" cast (potential_plan.go's
// potentialModeBaseCost has no arm, as for mayhem), so under a gorge surface
// the option exists only when the floating pool pays it (FeatPoolPays), and
// it is never lowered.
//
// The mode-2 action's target byte separates the graveyard casts a card can
// have at once: a flashback grant (Sphinx of Forgotten Lore) beside printed
// escape gives the card both the granted flashback and the escape cast, so
// the printed escape offer carries castPrintedEscapeTgt and the granted one
// castEscapeTgt in the target byte; a mode-2 cast with neither is the
// flashback (beginCast's target read). A printed Escape card is offered at
// its printed cost only: the grant's mana-cost escape on such a card is not
// a second offer.

// DebugEscapeSkipExiles is the mutation switch (dirac/oracle's
// TestEscapeMutation): the escape cast pays no printed exile part. A
// non-default value is a test-only lie; nothing ships with it set.
var DebugEscapeSkipExiles bool

// escapeEntry reports whether e is a spell cast through its escape cost.
func escapeEntry(e *stackEntry) bool {
	return e.kind == seSpell && e.trigPl == trigPlEscape
}

// castEscapeTgt is the target byte of an AkCast of mode bit 2 that casts a
// graveyard card for the escape cost a StatEscapeGrant static gives it
// (legal.go applies it like castImpTgt; kw_beammeup.go's beam bytes are 1
// and 2), the target byte that does not name the printed keyword.
const castEscapeTgt uint8 = 3

// FeatEscapeTgt is castEscapeTgt as the oracle sees it: the kernel's mode-2
// target-3 grant cast canonicalises to gorge's "escape" option (canon
// translate.go, sbt.go).
const FeatEscapeTgt = castEscapeTgt

// castPrintedEscapeTgt is an AkCast's target byte for a printed escape cast,
// the mode-2 cast of an Escape card (a target of 0 is a plain flashback cast;
// the beam me up casts use 1 and 2; a face cast's target is its face).
const castPrintedEscapeTgt uint8 = 5

// FeatCastEscapeTgt is castPrintedEscapeTgt for the oracle's canonical
// translation.
const FeatCastEscapeTgt = castPrintedEscapeTgt

// escapeCast reports whether an AkCast of card c in mode, target tgt is a
// printed escape cast: the mode-2 cast of a printed Escape card whose target
// byte marks escape. The card check keeps a future target-5 mode-2 cast of
// another card from being priced as escape.
func escapeCast(k *prep, c CardID, mode, tgt uint8) bool {
	return k.escape && mode&2 != 0 && tgt == castPrintedEscapeTgt && k.t.Cards[c].Escape
}

// escapeMarks reports whether the resolving spell entry was escape-cast: the
// mechEscaped stamp (turn_gen.go enterGen), read before the enter programs
// so a Card.Self+escaped gate on the card's own ETB counter or animate
// replacement matches it.
func escapeMarks(e *stackEntry) bool {
	return escapeEntry(e)
}

// escapeFilterRead reports whether some table filter reads PredEscaped, the
// only reader of the mechEscaped mark: the arena assigns the mark a slot
// only then (kw_sneak.go's shape).
func escapeFilterRead(t *kop.Table) bool {
	for i := range t.Filters {
		if t.Filters[i].Pred == kop.PredEscaped {
			return true
		}
	}
	return false
}

// FeatEscapeCard reports whether o's card prints escape (the cast itself is
// classified by the action's target byte, castPrintedEscapeTgt).
func (s *State) FeatEscapeCard(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Escape
}

// escapeStatic is the first StatEscapeGrant static ability in force that p
// controls whose filter matches card c owned by owner (read with p as
// "you"), or nil.
func (s *State) escapeStatic(k *prep, p, owner uint8, c CardID) *kop.Ability {
	if k.gen == nil || !k.gen.anyEscapeGrant {
		return nil
	}
	var got *kop.Ability
	s.eachStaticC(k, sclsEscapeG, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatEscapeGrant || st.ctrl != p || st.sl == NoSlot {
			return true
		}
		// the card is read as a card, wherever it lies (no zone, no context:
		// kgen lowers a YouOwn type filter only)
		if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, c, owner, p, NoObj, nil) {
			return true
		}
		got = a
		return false
	})
	return got
}

// escapeFodder is the exile part of grant a's escape cost: the static
// ability's own table Cost (kgen build_ab.go: CostExileGrave of N cards
// matching the grant's ExileFromGrave spec), the cast card itself excluded
// by the callers (gorge exAsk skips pc.card; graveCands' FOther). A table
// pointer, so buildPickCost returns it with no copy to the heap.
func escapeFodder(a *kop.Ability) *kop.Cost { return &a.Cost }

// offerEscapeCasts offers the graveyard casts of p's Escape cards and of the
// cards a battlefield StatEscapeGrant gives escape (gorge
// legal_walk_alt.go's escape walk: spellTimingOK, castSuppressed and
// castTargetsAvailable; priced with the escape cost's parts and the spell's
// own additional costs, which gorge folds into the offer
// (pay.WithSpellAbilityExtras) as it folds them into the cast). A live
// flashback grant does not hide the printed escape cast: both are offered,
// the grant's walk as a target-0 mode-2 flashback and this one as target
// castPrintedEscapeTgt. gorge lists the escape walk before mayhem's
// (legal_walk_alt.go), so the caller poses it before offerMayhemCasts.
func (s *State) offerEscapeCasts(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	// the avail is the caller's walk picture: each cast's own delve credit
	// prices its twobrid faces and is restored after, so the next option does
	// not price with it (offerPlay's shape, kw_mayhem.go)
	saved := av.delve
	defer func() { av.delve = saved }()
	pl := &s.pl[p]
	have := false
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		cd := &k.t.Cards[c]
		// a printed Escape card takes its own cost and fodder; otherwise a
		// battlefield static can give the card escape at its mana cost
		printed := cd.Escape
		var ga *kop.Ability
		if !printed {
			ga = s.escapeStatic(k, p, s.objs[o].owner(), c)
			if ga == nil {
				continue
			}
		}
		if !k.cards[c].instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !have {
			s.availFor(k, p, av)
			have = true
		}
		var fc kop.ManaCost
		tgt := castEscapeTgt
		if printed {
			tgt = castPrintedEscapeTgt
			if !s.costObjPayable(k, p, &cd.EscCost, o, o) {
				continue // the printed exile parts: no option that cannot be paid is ever offered
			}
			fc = s.adjustCost(k, p, c, o, cd.AltCost)
		} else {
			if !s.costObjPayable(k, p, escapeFodder(ga), o, o) {
				continue // the fodder is not there (gorge offerCostFor prices ec's parts)
			}
			fc = s.adjustCost(k, p, c, o, cd.Cost) // the card's mana cost (CR 702.138a)
		}
		av.delve = s.tbDelveCredit(k, p, c, o, false) // the faces' credit (CR 702.66a)
		if !av.tbLeafCredited(&fc) {
			fc = s.delveOfferCredit(k, p, c, o, false, fc) // gorge's choke point credits the generic at this walk too
		}
		if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, av) || !s.spellTargetsFeasibleFrom(k, c, p, o) || !s.spellCostsFeasible(k, c, p, o) {
			continue
		}
		if s.castHeldOut(o) {
			continue
		}
		if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.FeatPoolPays(p, o, 2) {
			// gorge's planner prices no escape mode (potentialModeBaseCost has
			// no arm, as for mayhem): the cast is offered only when the
			// floating pool pays the cost
			continue
		}
		d.add(MakeAction(AkCast, uint8(o), tgt, 2))
	}
}
