package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Funding sources of the filter search (filtSearch.visit's second pass).
// gorge's prefix search (rules scriptSearch.visit) activates every covered
// "activate" option while funding lasts, a Treasure's "{T}, Sacrifice this
// token" among them, and answers each colour ask in the ask's order with the
// play's needed colours first (scriptOrderAnswers, a stable partition). Every
// one of those branches is a node of its 160-node budget, so the kernel's
// search must branch the same way or it finds a script gorge's search runs out
// of budget before reaching (CR 605.3a: a Treasure's mana ability is
// activatable while the play is being priced).

// fundOrder writes the colour answers of funding source sl's ask in gorge's
// order: the ability's printed colour order (manaColourOrder: Birnin Zana
// Plaza's Combo G W asks G before W), the colours it does not name after in
// W U B R G C order, then the play's needed colours moved first (needFirst).
func (fs *filtSearch) fundOrder(k *prep, t *State, sl Slot, pc *pcard, mask uint8, out *[kop.NumManaKinds]uint8) int {
	n := 0
	var seen uint8
	if cd := &k.t.Cards[t.cardAt(sl)]; pc.manaAb >= 0 && int(pc.manaAb) < len(cd.Abilities) {
		var ord [5]uint8
		m := manaColourOrder(&cd.Abilities[pc.manaAb], &ord)
		for _, c := range ord[:m] {
			if mask&(1<<c) != 0 && n < len(out) {
				out[n] = c
				seen |= 1 << c
				n++
			}
		}
	}
	for m := mask &^ seen; m != 0 && n < len(out); m &= m - 1 {
		out[n] = uint8(bits.TrailingZeros8(m))
		n++
	}
	needFirst(out[:n], fs.need)
	return n
}

// fundColours tries funding source sl's colour answers (mask holds two or
// more) in fundOrder's order.
//
//go:noinline
func (fs *filtSearch) fundColours(k *prep, t *State, sl Slot, pc *pcard, mask uint8, funded int, gapUsed bool) bool {
	var order [kop.NumManaKinds]uint8
	for _, c := range order[:fs.fundOrder(k, t, sl, pc, mask, &order)] {
		if fs.try(k, t, filtStep{sl: sl, ans: c}, funded+1, gapUsed) {
			return true
		}
	}
	return false
}

// fundSrc is visit's second pass for one funding source sl (pc, sl's mana
// profile, sacOnly its sacrifice-only membership): a tap source's census
// answer, then its restricted ability's answers, and -- when the source has
// no {T} -- its sacrifice (an Eldrazi Spawn, mana_sacsource.go,
// CR 605.3a/602.2b). Reports whether a step found the script.
func (fs *filtSearch) fundSrc(k *prep, t *State, sl Slot, pc *pcard, sacOnly bool, funded int, gapUsed bool) bool {
	rm := uint8(0)
	if k.restrSrc {
		rm = t.filtRestrMask(k, sl)
	}
	free := t.canTapForMana(k, sl)
	if !free && rm == 0 && !sacOnly {
		return false
	}
	if free && rm != 0 && !fs.tick() {
		return false // the ask between its census and its restricted ability
	}
	if free || (sacOnly && rm == 0) {
		// a sacrifice-only source funds the fee too, sacrificed (an Eldrazi
		// Spawn's "Sacrifice this creature: Add {C}"): it has no {T}, so the
		// tap arm does not cover it
		if fs.fundAnswers(k, t, sl, pc, t.srcMaskAt(pc, sl), funded, gapUsed) {
			return true
		}
	}
	if bits.OnesCount8(rm) > 1 && !fs.tick() {
		return false // the restricted ability's colour ask
	}
	for m := rm; m != 0; m &= m - 1 { // the restricted ability's answers, after the census one's
		if fs.try(k, t, filtStep{sl: sl, ans: uint8(bits.TrailingZeros8(m)), restr: true}, funded+1, gapUsed) {
			return true
		}
	}
	return false
}

// fundAnswers plays funding source sl's colour answers (mask) after its mana
// is made: the ask in gorge's order, a Treasure's and a sacrifice-only
// source's sacrifice included (fundSac). Reports whether one found the script.
func (fs *filtSearch) fundAnswers(k *prep, t *State, sl Slot, pc *pcard, mask uint8, funded int, gapUsed bool) bool {
	if bits.OnesCount8(mask) > 1 && !fs.tick() {
		return false // the colour ask
	}
	if bits.OnesCount8(mask) > 1 {
		// a Treasure funds too, sacrificed; the ask in gorge's order
		return fs.fundColours(k, t, sl, pc, mask, funded, gapUsed)
	}
	return fs.try(k, t, filtStep{sl: sl, ans: uint8(bits.TrailingZeros8(mask))}, funded+1, gapUsed)
}

// fundSac sacrifices funding source sl after its mana is added, when its mana
// ability sacrifices it (a Treasure's "{T}, Sacrifice this token", or a
// sacrifice-only source's "Sacrifice this creature: Add {C}", which fundSac
// reaches through sacOnlyFund's no-tap step): gorge pays that cost at the
// search's priority node, and the token ceases at the next priority's
// state-based check, before the next activation (the grant search's sacrifice,
// mana_grantedoffers.go). The visited-state key needs nothing more: the
// sacrificed source's mana floats, so the pool tells that state from one where
// it is still there (gorge's scriptStateKey lists the battlefield by id).
//
//go:noinline
func (s *State) fundSac(k *prep, sl Slot) {
	pc := s.manaPCGen(k, sl)
	if !pc.manaSac && !(k.sacOnlyMana && pc.manaSacOnly) {
		return
	}
	s.sacFilterSource(k, sl)
}

// sacFilterSource sacrifices the mana source at sl: its Sacrificed triggers
// fire, it leaves for its owner's graveyard, and a token ceases. A Treasure's
// funding step (fundSac) and a filter whose activation cost sacrifices the
// source (Omni-Cheese Pizza, filtDo) share it, as does a sacrifice-another
// source's cost, given the sacrificed permanent's slot (sacXDo).
func (s *State) sacFilterSource(k *prep, sl Slot) {
	o := s.perms[sl].obj
	s.fireSac(k, sl)
	s.leave(k, sl, kop.ZoneGraveyard, false)
	if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard {
		s.ceaseManaToken(o)
	}
}
