package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Filtered damage doublers and token multipliers.
//
// A ReplDoubleDamageToOpps with Repl.Filter or Repl.C set is a filtered
// doubler (Gratuitous Violence: Creature.YouCtrl+inZoneBattlefield to a
// permanent or player; Neriv: a creature you control that entered this turn;
// Trance Kuja: a Wizard you control): it doubles the damage of a source its
// controller controls matching Filter to a recipient in C (kop.ReplDmg*
// bits; gorge replacement_damage.go damageReplacementMatches). Filter 0 and
// C 0 is Twinflame Tyrant's reading (an opponent or their permanent).
//
// A ReplDoubleTokens with Repl.A > 2 multiplies by A (Ojer Taq: three
// times); with Repl.Filter it applies only to the tokens matching it (gorge
// replacement_match.go: ValidToken$ per minted token). Type$ Amount
// multipliers commute (gorge replacement_token.go tokenReplacementsCommute),
// so they are applied in any order, no ask.
//
// Tables without either reading (genPrep.anyFiltRepl false) keep the
// unfiltered counts (replN).

// DebugReplMultNoFilter is a mutation hook for TestReplmultMutation: the
// doublers' filters are ignored (every doubler applies to every source its
// controller controls, any recipient).
var DebugReplMultNoFilter bool

// prepReplMult sets genPrep.anyFiltRepl.
func prepReplMult(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind != kop.AbReplacement {
				continue
			}
			switch r := &a.Repl; r.Kind {
			case kop.ReplDoubleDamageToOpps:
				// a condition-only doubler must take the
				// dblDmgMatch path too: replN never reads Repl.Cond
				if r.Filter != 0 || r.C != 0 || r.Cond != 0 {
					g.anyFiltRepl = true
				}
				if t.Filters[r.Filter].Flags2&kop.F2EnteredThisTurn != 0 {
					g.enteredNeed = true // a source that left reads its entry (srcEnteredThisTurn)
				}
			case kop.ReplDoubleTokens:
				if r.Filter != 0 || r.A > 2 {
					g.anyFiltRepl = true
				}
			case kop.ReplRewriteTokens, kop.ReplAddToken:
				g.anyTokRew = true // repl_tokrewrite.go; the adder: repl_tokorder.go
			}
		}
	}
}

// dblDmgMatch reports whether a ReplDoubleDamageToOpps of the permanent at
// b applies to damage a source (srcSl, or the card srcCard when it is not a
// permanent: srcObj its object when known, NoObj else) controlled by srcCtrl
// would deal to the recipient (the permanent at sl controlled by rcp, or
// player rcp when sl is NoSlot). A source gone from the battlefield is read as
// the permanent it last was (CR 608.2h; srcEnteredThisTurn).
func (s *State) dblDmgMatch(k *prep, b Slot, srcCtrl uint8, srcSl Slot, srcCard CardID, srcObj Obj, rcp uint8, sl Slot, combat bool) bool {
	lc := s.perms[b].ctrl
	if lc != srcCtrl {
		return false
	}
	cd := &k.t.Cards[s.cardAt(b)]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDoubleDamageToOpps {
			continue
		}
		r := &a.Repl
		if r.Cond != 0 && !s.replCondHolds(k, b, lc, r.Cond) { // repl_conditions.go
			continue
		}
		if DebugReplMultNoFilter {
			return true
		}
		if r.Filter == 0 && r.C == 0 {
			if rcp != lc {
				return true // Twinflame Tyrant
			}
			continue
		}
		if r.C&kop.ReplDmgNonCombat != 0 && combat || r.C&kop.ReplDmgSrcInPlay != 0 && srcSl == NoSlot {
			continue
		}
		var to int16
		switch {
		case sl == NoSlot && rcp != lc:
			to = kop.ReplDmgPlayer | kop.ReplDmgOppPlayer
		case sl == NoSlot:
			to = kop.ReplDmgPlayer
		case rcp != lc:
			to = kop.ReplDmgPerm | kop.ReplDmgOppPerm
		default:
			to = kop.ReplDmgPerm
		}
		if r.C&to == 0 {
			continue
		}
		if srcSl != NoSlot {
			if !s.matchPermGen(k, r.Filter, srcSl, lc, b, layerFull) {
				continue
			}
		} else if fl := k.t.Filters[r.Filter]; fl.Flags2&kop.F2EnteredThisTurn != 0 {
			fl.Flags2 &^= kop.F2EnteredThisTurn
			if !s.srcEnteredThisTurn(k, srcCard) || !s.matchCardOne(k, &fl, srcCard, srcCtrl, lc, NoObj) {
				continue
			}
		} else if !s.matchCardGen(k, r.Filter, srcCard, srcCtrl, lc, srcObj) {
			continue
		}
		return true
	}
	return false
}

// srcEnteredThisTurn: the resolving entry's source, a card that has left the
// battlefield, had entered it this turn. The ledger keeps an entry past the object's
// next move; zoneTurn would read its arrival where it is now.
func (s *State) srcEnteredThisTurn(k *prep, srcCard CardID) bool {
	if s.res.e.card != srcCard {
		return false
	}
	o := s.res.e.obj
	for i := uint8(0); i < s.g.enteredN; i++ {
		if e := &s.cold.entered[i]; e.o == o && e.to == uint8(kop.ZoneBattlefield) {
			return true
		}
	}
	return false
}

// dblDmgN is the number of doublers matching the damage event (dblDmgMatch
// over the source controller's battlefield).
func (s *State) dblDmgN(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool) int {
	if srcCtrl > 1 {
		return 0
	}
	g := k.gen
	n := 0
	for m := s.statM.and(s.ctrlM[srcCtrl]); m.any(); m = m.rest() {
		b := m.first()
		if g.replBits[s.cardAt(b)]&rbDblDmg != 0 && !s.genLost(k, b) && s.dblDmgMatch(k, b, srcCtrl, srcSl, srcCard, NoObj, rcp, sl, combat) {
			n++
		}
	}
	return n
}

// dblDmgN2 is dblDmgN with the unfiltered reading for a table without a
// filtered doubler: the source controller's doublers when the recipient is
// the opponent or theirs.
func (s *State) dblDmgN2(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool) int {
	if k.gen.anyFiltRepl {
		return s.dblDmgN(k, srcCtrl, srcSl, srcCard, rcp, sl, combat)
	}
	if rcp == srcCtrl || srcCtrl >= MaxSeats {
		return 0
	}
	return s.replN(k, srcCtrl, rbDblDmg)
}

// dblDmgLive reports whether the doubler at b still matches the asked
// combat event (replCandLive).
func (s *State) dblDmgLive(k *prep, b Slot) bool {
	ro := &s.g.ro
	rcp := ro.apl
	if ro.askD != NoSlot {
		rcp = s.perms[ro.askD].ctrl
	}
	return s.dblDmgMatch(k, b, ro.actl, ro.askS, s.cardAt(ro.askS), NoObj, rcp, ro.askD, true)
}

// replTokensMult is replTokens for a table with a multiplier or a filtered
// one: each ReplDoubleTokens p controls whose Filter matches the token card
// tok multiplies by its A (0: twice).
func (s *State) replTokensMult(k *prep, p uint8, tok CardID, n int) int {
	g := k.gen
	for m := s.tokSrcM(p); m.any() && n < 1<<8; m = m.rest() {
		b := m.first()
		if g.replBits[s.cardAt(b)]&rbDblTok == 0 || s.genLost(k, b) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(b)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDoubleTokens {
				continue
			}
			if !DebugReplMultNoFilter && a.Repl.Filter != 0 && !s.matchCardGen(k, a.Repl.Filter, tok, p, p, NoObj) {
				continue
			}
			x := 2
			if a.Repl.A > 2 {
				x = int(a.Repl.A)
			}
			n *= x
		}
	}
	return n
}
