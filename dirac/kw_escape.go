package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Escape (CR 702.42a), cold: reached only when the table has a card with a
// StatEscapeGrant static (prep.costObj through genPrep.anyEscapeGrant).
//
// "Escape — [the card's mana cost], Exile<N/other cards from your
// graveyard>": a card in its owner's graveyard granted escape by a
// battlefield static (kop.StatEscapeGrant, Underworld Breach's AffectedZone$
// Graveyard AddKeyword$ Escape) may be cast for its escape cost: the card's
// mana cost plus N OTHER cards from the same graveyard. The cast is an
// ordinary one from the graveyard: CR 702.42a gives no post-resolution
// destination change (unlike flashback the spell returns to its owner's
// graveyard), so beginCast stamps no sfFlashback, and the entry's cast zone
// (stackEntry.prov, cast_fromgrave.go) answers gorge's FlagEscaped through
// ObjectWasCastFromGraveyard exactly as gorge's escape walk's modeFlags do.
//
// The kernel action is AkCast mode bit 2 with castEscapeTgt in the target
// byte (a flashback cast's mode with a distinct target byte, as beam me up:
// kw_beammeup.go), so a card can offer a printed flashback beside the
// grant's escape, and a Mayhem card beside the grant's escape costs both
// ways. legal.go reads the byte into genState.escapeCast, beginCast marks
// the entry trigPlEscape, and the cost builder prices the mana at the card's
// mana cost (buildCostPreRaw: no sfFlashback, no mayhem mark). The fodder
// rides buildPickCost's pick machinery (act_forage.go), and bsCosts' escape
// arm poses the entry's own Sac/Discard additional asks before the exile
// picks, gorge's costObjectAsks order (exAsk is last). A copy keeps no mark
// (copy_gen.go, CR 707.10).

// castEscapeTgt is an escape cast's target byte in an AkCast of mode bit 2
// (legal.go applies it like castImpTgt; kw_beammeup.go's beam bytes are 1
// and 2).
const castEscapeTgt uint8 = 3

// FeatEscapeTgt is castEscapeTgt as the oracle sees it: the kernel's mode-2
// target-3 cast canonicalises to gorge's "escape" option (canon translate.go,
// sbt.go).
const FeatEscapeTgt = castEscapeTgt

// escapeStatic is the first StatEscapeGrant static in force that p controls
// whose filter matches card c owned by owner (read with p as "you"), or nil.
func (s *State) escapeStatic(k *prep, p, owner uint8, c CardID) *kop.Static {
	if k.gen == nil || !k.gen.anyEscapeGrant {
		return nil
	}
	var got *kop.Static
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
		got = &a.Static
		return false
	})
	return got
}

// escapeFodder is the exile part of the grant st's escape cost: N cards
// matching the grant's ExileFromGrave spec, the cast card itself excluded by
// the callers (gorge exAsk skips pc.card; graveCands' FOther).
func escapeFodder(st *kop.Static) kop.Cost {
	return kop.Cost{Flags: kop.CostExileGrave, N: uint8(st.A), Filter: st.Filter2}
}

// offerEscapeCasts offers the graveyard casts of p's cards a battlefield
// StatEscapeGrant gives escape (gorge legal_walk_alt.go's escape walk:
// spellTimingOK, castSuppressed and castTargetsAvailable; priced with the
// escape cost's parts -- the card's mana cost plus the fodder -- and the
// spell's own additional costs, which gorge folds into the offer
// (pay.WithSpellAbilityExtras) as it folds them into the cast). gorge lists the escape walk before
// mayhem's (legal_walk_alt.go), so the caller poses it before
// offerMayhemCasts.
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
		st := s.escapeStatic(k, p, s.objs[o].owner(), c)
		if st == nil {
			continue
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
		ec := escapeFodder(st)
		if !s.costObjPayable(k, p, &ec, o, o) {
			continue // the fodder is not there (gorge offerCostFor prices ec's parts)
		}
		if !s.spellCostsFeasible(k, c, p, o) {
			continue // the spell's own Sac/Discard part is owed on top (CR 118.9d); gorge folds it into the offer
		}
		fc := s.adjustCost(k, p, c, o, cd.Cost) // the card's mana cost (CR 702.42a)
		av.delve = s.tbDelveCredit(k, p, c, o, false)
		if !av.tbLeafCredited(&fc) {
			fc = s.delveOfferCredit(k, p, c, o, false, fc) // gorge's choke point credits the generic at this walk too
		}
		if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, av) || !s.spellTargetsFeasibleFrom(k, c, p, o) {
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
		d.add(MakeAction(AkCast, uint8(o), castEscapeTgt, 2))
	}
}
