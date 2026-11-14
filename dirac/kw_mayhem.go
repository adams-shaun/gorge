package dirac

// Mayhem (CR 702.187), cold: reached only when the table has a
// card with printed mayhem (prep.mayhem).
//
// A Mayhem card in its owner's graveyard may be cast for the keyword's
// alternative mana cost (CardDef.AltCost) if the owner discarded it this
// turn (gorge rules altcast.go mayhemCastCost / mayhemDiscardedThisTurn;
// legal_walk_alt.go's graveyard walk, Option Kind "cast" Mode "mayhem"). The
// "discarded this turn" bit is one object-width arena mask, mechMayhem
// (0 B): set by every discard that puts a card into a graveyard (discardObj
// and cycleDiscard, the two writers of genState.disc; a discard that madness
// exiles marks nothing), cleared for every card at the turn's start
// (newTurnGen: gorge's last-TurnChange bound). The discarder is always the
// card's owner in a two-player hand discard, which is gorge's "naming the
// caster as the discarder" for an own-graveyard cast. A card milled or put
// into the graveyard some other way has no discard event and no offer.
//
// A zone move clears the bit (placeFromK): a card that left the graveyard and came
// back is a new object nobody discarded (CR 400.7).
//
// The cast is an ordinary one from the graveyard: timing rules apply, the
// spell stamps only FlagMayhem (no exile tail, and not a graveyard-origin
// cast for gorge's wasCastFromGraveyard, which names Flashback, Harmonize,
// Jumpstart and Escaped). The kernel action is AkCast mode 2; beginCast does
// not stamp sfFlashback for it but marks the entry trigPlMayhem, which
// buildCost prices at CardDef.AltCost. A copy keeps no mark (mintSpellCopy,
// CR 707.10).

// trigPlMayhem (trigpl.go) marks a spell entry cast for its mayhem cost.

// DebugMayhemAnyGrave is the mutation switch (dirac/oracle
// TestMayhemMutation): the graveyard offer ignores the discard bit, so the
// lockstep must diverge on a milled Mayhem card.
var DebugMayhemAnyGrave bool

// mayhemMark records that o, just discarded into a graveyard, was discarded
// this turn.
func (s *State) mayhemMark(k *prep, o Obj) {
	if k.t.Cards[s.objs[o].card].Mayhem {
		s.mObjSet(k, mechMayhem, o)
	}
}

// mayhemCast reports whether an AkCast of card c in mode is a mayhem cast:
// the mode-2 cast of a Mayhem card.
func mayhemCast(k *prep, c CardID, mode uint8) bool {
	return k.mayhem && mode&2 != 0 && k.t.Cards[c].Mayhem
}

// offerMayhemCasts offers the graveyard casts of p's Mayhem cards (gorge
// legal_walk_alt.go's mayhem walk: spellTimingOK, castRestrictedCost,
// castSuppressed and castTargetsAvailable, priced with the potential
// planner's ordinary cast price).
func (s *State) offerMayhemCasts(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	// the avail is the caller's walk picture: each cast's own delve credit
	// prices its twobrid faces and is restored after, so the next option does
	// not price with it (offerPlay's shape)
	saved := av.delve
	defer func() { av.delve = saved }()
	pl := &s.pl[p]
	have := false
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		cd := &k.t.Cards[c]
		if !cd.Mayhem || !DebugMayhemAnyGrave && !s.mObj(k, mechMayhem, o) {
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
		fc := s.adjustCost(k, p, c, o, cd.AltCost)
		av.delve = s.tbDelveCredit(k, p, c, o, false) // the faces' credit (CR 702.66a)
		if !av.tbLeafCredited(&fc) {
			fc = s.delveOfferCredit(k, p, c, o, false, fc) // gorge's choke point credits the generic at this walk too
		}
		if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, av) || !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) {
			continue
		}
		if s.castHeldOut(o) {
			continue
		}
		if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.FeatPoolPays(p, o, 2) {
			// gorge's planner prices no mayhem mode (potentialModeBaseCost has
			// no arm, as for harmonize): the cast is offered only when the
			// floating pool pays the cost
			continue
		}
		d.add(MakeAction(AkCast, TCode(o), 0, 2))
	}
}

// FeatMayhemCard reports whether o's card is a Mayhem card (the oracle
// canonicalises its mode-2 cast as gorge's "mayhem" option).
func (s *State) FeatMayhemCard(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Mayhem
}
