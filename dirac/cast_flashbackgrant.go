package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// flashbackReachable: the flashback cast of card c (object o) at cost fc is one
// gorge's priority list can carry. Its additional cost's permanents are held
// back from the mana (flashbackPartsDisjoint), and on the autopay surface a
// cost payable only with a mana ability an effect grants (Abundant Growth's
// "{T}: Add one mana of any color" on the only land making {R}) is left out:
// gorge prices a flashback cast through its potential planner, whose census
// holds printed and intrinsic abilities only (rules/pay PaymentAbility), so it
// answers census_incomplete and drops the play, while the floating pool alone
// pays a cast it lists as offered (FeatPoolPays).
//
// The withhold holds only where gorge's search cannot finish. The search that
// prices the cast (engine/rules potential_script.go) walks distinct tap states
// under a budget of potentialScriptNodes (160): when the untapped sources offer
// more ways to pay the cost's taps than that budget (flashbackSearchNodes), it
// runs out before it funds the cast through the grant (Wild Hunger's {3}{R}
// beside thirteen lands, tail-15-c000 decision 1248), while a board whose
// choices fit the budget gets its script and the cast is offered (Think Twice's
// {2}{U} beside an Island, a Plains and an Island granted any colour; Solstice
// Revelations' seven mana beside eight sources). Nor does it run out when the
// printed sources make every coloured pip (Gysahl Greens' {G} beside a
// Treasure): the granted mana only fills the generic part, and the first line
// the search walks finds it. The count only decides where the search is not
// run: a board whose choices fit the budget is offered from it (the kernel's
// search does not find every route gorge's walk does there), while a board
// whose choices exceed it is left to grantCap's grantSearch, the mirror of
// gorge's prefix chain (potentialPrefixScript), which prices the cast at its
// offer in gorge's walk order.
func (s *State) flashbackReachable(k *prep, p uint8, c CardID, o Obj, fc kop.ManaCost) bool {
	if !s.flashbackPartsDisjoint(k, p, c, fc) {
		return false
	}
	if s.cfg&cfgGorgeAutopay == 0 || k.gen == nil || len(k.gen.grantPC) == 0 || s.FeatPoolPays(p, o, 2) || !s.flashbackSearchExhausts(k, p, &fc) {
		return true
	}
	d := payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}
	if s.affordsUngrantedCast(k, p, &fc, d) {
		return true // gorge's planner prices it from the printed census: no search runs
	}
	if s.g.availX&availWalk != 0 {
		// gorge's potential walk lists the cast and its search may finish
		// within the budget: grantCap's grantSearch, the mirror of that
		// search, decides on the offer (a flashback cast takes a slot of the
		// walk in gorge's order like any other cast).
		return true
	}
	// No search runs: the planner's census_incomplete verdict alone withholds
	// the cast, unless the census gap is not a live grant gap.
	return s.grantGapLive(k, p)
}

// flashbackSearchNodes is gorge's potentialScriptNodes: the distinct tap states
// one prefix search may visit.
const flashbackSearchNodes = 160

// flashbackSearchExhausts reports whether the ways to pick the taps that pay fc
// from p's untapped mana sources outnumber gorge's search budget, on a board
// where the search funds a filter's fee at all (an untapped filter with a fee
// and two or more units: gorge's census.fees() above zero).
func (s *State) flashbackSearchExhausts(k *prep, p uint8, fc *kop.ManaCost) bool {
	if !s.filtFeeGaps(k, s.gapMask(k, p, gfFilt)).any() {
		// No untapped filter's fee is left to fund: gorge's search then funds
		// nothing (census.fees() is 0, so gapsOnly stops it after the gap
		// activations, mana_filterfeeorder.go) and never walks the covered
		// tap choices this count prices; it finishes in a few nodes (Wild
		// Hunger's {3}{R} once Equip's {3} tapped Flooded Grove).
		return false
	}
	taps := int(fc.Generic) + hybN(fc)
	for _, n := range fc.Pips {
		taps += int(n)
	}
	n := 0
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		if s.canTapForManaPC(k, sl, s.manaPCGen(k, sl)) {
			n++
		}
	}
	if taps > n-taps {
		taps = n - taps // C(n, taps) = C(n, n-taps)
	}
	if taps < 0 {
		return false
	}
	c := 1
	for i := 1; i <= taps; i++ {
		c = c * (n - taps + i) / i
		if c > flashbackSearchNodes {
			return true
		}
	}
	return false
}
