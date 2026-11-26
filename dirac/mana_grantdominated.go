package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugGrantDominatedGaps (TestRhonasDruidGrantDominatedMutation) restores the
// pre-fix census: every holder of a granted mana ability is a prefix gap, even
// one whose printed ability already covers the grant. Only the mutation test
// sets it.
var DebugGrantDominatedGaps bool

// plainTapView reports whether pc is a lone {T} ability making exactly one
// mana of one fixed colour (or, with anyOK, of any one colour): gorge's
// manaTapSingle with no fee, extra, restriction, choice or sacrifice. It
// returns the colours the ability can make.
func plainTapView(pc *pcard, anyOK bool) (mask uint8, ok bool) {
	if pc.manaAb < 0 || !pc.manaTap || pc.manaN != 1 || pc.manaDyn || pc.manaChosen || pc.manaOne ||
		pc.manaSac || pc.manaSacOnly || pc.manaMulti || pc.manaIdent || pc.manaRefl || pc.manaIntrinsic ||
		pc.restrU != 0 || pc.restrFee != 0 || pc.wideN != 0 || pc.painCol != 0 || pc.manaXv != ([kop.NumManaKinds]uint8{}) {
		return 0, false
	}
	mask = srcMask(pc)
	if mask == 0 || mask&(mask-1) != 0 && !(anyOK && mask == uint8(kop.ColAny)) {
		return 0, false
	}
	return mask, true
}

// grantsDominated reports whether every mana ability an effect grants sl (the
// grantPC indices gi[:ng]) is a lone {T} making one fixed-colour mana that
// sl's own printed ability already makes: a source's free {T} abilities are
// alternatives of one tap (CR 605.3b, 118.3c), so the grant adds nothing the
// payment planner cannot produce and gorge's census does not count it a gap
// (engine/rules/potential_plan.go manaTapGapDominated, Paradise Druid holding
// Rishkar's "{T}: Add {G}" beside its own any-colour {T}). Such a holder is no
// prefix candidate: the search taps the others and leaves the holder to the
// planner (CR 601.2g). A source a tap extra reaches, one a granted land type
// changes, or one whose own view is not its plain printed ability keeps every
// grant a gap.
func (s *State) grantsDominated(k *prep, sl Slot, gi *[maxGrants]int16, ng int) bool {
	g := k.gen
	if DebugGrantDominatedGaps || ng == 0 {
		return false
	}
	c := s.cardAt(sl)
	if g.flags[c]&(gfManaVar|gfManaGate|gfManaRefl) != 0 || !s.ownManaUsable(k, sl) || s.ownRestrAbListed(k, sl) != nil {
		return false
	}
	pc := &k.cards[c]
	if g.blgOn && s.manaPCGen(k, sl) != pc {
		return false // a granted basic land type's intrinsic abilities join the source's (static_landtype.go)
	}
	own, ok := plainTapView(pc, true)
	if !ok {
		return false
	}
	if g.anyTapExtra {
		if _, hit := s.srcExtraHit(k, sl, pc); hit {
			return false // the tap's extra mana makes it no lone one-mana ability (manaTapSingle)
		}
	}
	for i := 0; i < ng; i++ {
		gm, ok := plainTapView(s.grantView(g, gi[i], sl), false)
		if !ok || gm&^own != 0 || grantHasCost(s.grantView(g, gi[i], sl)) {
			return false
		}
	}
	return true
}
