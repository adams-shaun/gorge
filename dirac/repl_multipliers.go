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
// A filter with an alternative naming no controller (ValidSource$ Card,Emblem)
// admits a source of any controller, gorge's bare Card/Emblem words: the
// doubler then scans both seats' battlefields (anyDblAnySrc, dblFiltAnyCtrl).
// Repl.A over 2 multiplies by A (City on Fire and Fiery Emancipation are the
// doubler's own Thrice beside Twinflame Tyrant's Twice), and the new
// ReplDmgYou / ReplDmgSelf words read ValidTarget$ You / Card.Self (Goldnight
// Castigator doubles the damage to its controller and to itself, from any
// source).
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

// prepReplMult sets genPrep.anyFiltRepl and anyDblAnySrc.
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
				if r.Filter != 0 && dblFiltAnyCtrl(t, r.Filter) {
					g.anyDblAnySrc = true // a source of any controller: the scans widen (dblDmgMatch)
				}
				if t.Filters[r.Filter].Flags2&kop.F2EnteredThisTurn != 0 {
					g.enteredNeed = true // a source that left reads its entry (srcEnteredThisTurn)
				}
			case kop.ReplDoubleTokens:
				if r.Filter != 0 || r.A > 2 {
					g.anyFiltRepl = true
				}
			case kop.ReplRewriteTokens, kop.ReplAddToken, kop.ReplCopyFirstToken:
				g.anyTokRew = true // repl_tokrewrite.go; the adder: repl_tokorder.go
			}
		}
	}
}

// dblFiltAnyCtrl reports whether the filter chain at f names an alternative
// with no controller word (ValidSource$ Card,Emblem: any card or emblem, any
// controller's). A clause's Ctrl is read relative to the doubler's controller
// (the reader), so a CtrlAny clause matches a source of either seat.
func dblFiltAnyCtrl(t *kop.Table, f kop.FilterID) bool {
	for ; f != 0; f = t.Filters[f].Or {
		if t.Filters[f].Ctrl == kop.CtrlAny {
			return true
		}
	}
	return false
}

// dblDmgMatch reports whether a ReplDoubleDamageToOpps of the permanent at
// b applies to damage a source (srcSl, or the card srcCard when it is not a
// permanent: srcObj its object when known, NoObj else) controlled by srcCtrl
// would deal to the recipient (the permanent at sl controlled by rcp, or
// player rcp when sl is NoSlot). A source gone from the battlefield is read as
// the permanent it last was (CR 608.2h; srcEnteredThisTurn). A doubler whose
// filter names a controllerless alternative (ValidSource$ Card,Emblem) applies
// to a foreign controller's source too (the filter decides; gorge
// damageReplacementMatches's bare Card/Emblem words match any source).
func (s *State) dblDmgMatch(k *prep, b Slot, srcCtrl uint8, srcSl Slot, srcCard CardID, srcObj Obj, rcp uint8, sl Slot, combat bool, mult *int) bool {
	lc := s.perms[b].ctrl
	cd := &k.t.Cards[s.cardAt(b)]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDoubleDamageToOpps {
			continue
		}
		r := &a.Repl
		if lc != srcCtrl && (r.Filter == 0 || !dblFiltAnyCtrl(k.t, r.Filter)) {
			// the scan is the source controller's battlefield: an unfiltered
			// doubler is a source-its-controller-controls line (Twinflame
			// Tyrant), and a filter whose every alternative names the reader's
			// controller can never match a foreign source
			continue
		}
		if r.Cond != 0 && !s.replCondHolds(k, b, lc, r.Cond) { // repl_conditions.go
			continue
		}
		if DebugReplMultNoFilter {
			dblMulBy(mult, dblLineMult(r))
			return true
		}
		if r.Filter == 0 && r.C == 0 {
			if rcp != lc {
				dblMulBy(mult, dblLineMult(r))
				return true // Twinflame Tyrant
			}
			continue
		}
		if r.C&kop.ReplDmgNonCombat != 0 && combat || r.C&kop.ReplDmgCombat != 0 && !combat || r.C&kop.ReplDmgSrcInPlay != 0 && srcSl == NoSlot {
			continue
		}
		var to int16
		switch {
		case sl == NoSlot && rcp != lc:
			to = kop.ReplDmgPlayer | kop.ReplDmgOppPlayer
		case sl == NoSlot:
			to = kop.ReplDmgPlayer | kop.ReplDmgYou // rcp == lc
		case rcp != lc:
			to = kop.ReplDmgPerm | kop.ReplDmgOppPerm
		default:
			to = kop.ReplDmgPerm
			if sl == b {
				to |= kop.ReplDmgSelf
			}
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
		dblMulBy(mult, dblLineMult(r))
		return true
	}
	return false
}

// dblLineMult is the multiplier a damage doubler's line applies: 2, or its own
// Repl.A over 2 (City on Fire and Fiery Emancipation: Thrice).
func dblLineMult(r *kop.Repl) int {
	if r.A > 2 {
		return int(r.A)
	}
	return 2
}

// dblPickAn applies a picked doubler candidate's multiplier m to the running
// amount an of the pick-as-made walks (replOrderPick's roPlusPend arm and
// ncPlusPickNC): the candidate's own line's word, bounded as replDamage bounds
// its product.
func dblPickAn(an int16, m int) int16 {
	if an >= 1<<12 {
		return an
	}
	n := int(an) * m
	if n > 1<<13-2 {
		n = 1<<13 - 2
	}
	return int16(n)
}

// dblMulBy folds one match's multiplier m into the running product, bounded
// so a long chain of doublers cannot overflow it (replDamage clamps the
// damage). A nil mult is not read.
func dblMulBy(mult *int, m int) {
	if mult != nil && *mult < 1<<13 {
		*mult *= m
	}
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
		if e := &s.cold.entered[i]; e.o == o && e.to() == uint8(kop.ZoneBattlefield) {
			return true
		}
	}
	return false
}

// dblDmgN is the number of doublers matching the damage event (dblDmgMatch
// over the source controller's battlefield, both seats' when a doubler's
// filter admits a foreign source: anyDblAnySrc). mult, when not nil, takes
// the product of the matched doublers' multipliers (Repl.A over 2; City on
// Fire's Thrice beside Twinflame Tyrant's Twice).
func (s *State) dblDmgN(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, mult *int) int {
	if srcCtrl >= MaxSeats {
		return 0 // no seat controls the source
	}
	g := k.gen
	n := 0
	m := s.statM
	if !g.anyDblAnySrc {
		m = m.and(s.ctrlM[srcCtrl])
	}
	for ; m.any(); m = m.rest() {
		b := m.first()
		if g.replBits[s.cardAt(b)]&rbDblDmg != 0 && !s.genLost(k, b) && s.dblDmgMatch(k, b, srcCtrl, srcSl, srcCard, NoObj, rcp, sl, combat, mult) {
			n++
		}
	}
	return n
}

// dblDmgN2 is dblDmgN with the unfiltered reading for a table without a
// filtered doubler: the source controller's doublers when the recipient is
// the opponent or theirs. mult is dblDmgN's (nil: not read).
func (s *State) dblDmgN2(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, mult *int) int {
	m := 0
	if k.gen.anyDmgMark {
		m = s.dmgMarkEv(rcp, srcCtrl) // repl_dmgmark.go
	}
	if mult != nil {
		for i := 0; i < m; i++ {
			dblMulBy(mult, 2) // each mark doubles once
		}
	}
	if k.gen.anyFiltRepl {
		return m + s.dblDmgN(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, mult)
	}
	if rcp == srcCtrl || srcCtrl >= MaxSeats {
		return m
	}
	c := s.replN(k, srcCtrl, rbDblDmg)
	if mult != nil {
		for i := 0; i < c; i++ {
			dblMulBy(mult, 2) // an unfiltered doubler (Twinflame Tyrant): Twice
		}
	}
	return m + c
}

// dblDmgLive reports whether the doubler at b still matches the asked
// combat event (replCandLive); mult, when not nil, takes the multiplier the
// live candidate applies as it is picked (dealAsked).
func (s *State) dblDmgLive(k *prep, b Slot, mult *int) bool {
	ro := &s.g.ro
	rcp := ro.apl
	if ro.askD != NoSlot {
		rcp = s.perms[ro.askD].ctrl
	}
	return s.dblDmgMatch(k, b, ro.actl, ro.askS, s.cardAt(ro.askS), NoObj, rcp, ro.askD, true, mult)
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
