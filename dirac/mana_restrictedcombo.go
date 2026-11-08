package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A wide any-colour restricted ability as a source's second mana ability
// (Hermitic Herbalist: {T}: Add one mana of any colour; {T}: Add two mana in
// any combination of colours, spent only on a Lesson spell). gorge's planner
// prices the first ability (the census unit) and misses the second (a Combo
// Any allocation stays manual, pay.PaymentPlanUnitAlternatives), so the
// source is a census gap whose relaxed alternatives, the fifteen colour
// multisets, sit beside its census unit (availAnyR adds them, removing
// nothing). The prefix search (rules scriptSearch.visit) activates the gap
// object and branches: the "choose a mana ability" ask (one node, no
// ManaSymbol, printed order), the first ability's colour ask (one node, the
// play's colours first: scriptOrderAnswers), then the combination's
// allocation ask (one node, the per-unit pairs unit-major, scriptAnswers,
// reordered the same way). The answers here, in engine order: 0..4 the first
// ability's colour, then wideAnswers multisets of the second.

// wide2Answers is a second-ability wide gap's engine-order answers.
const wide2Answers = kop.NumManaKinds - 1 + wideAnswers

// wideBesideAny: a, a wide any-colour restricted ability of cd, follows a
// first mana ability that is a plain unrestricted {T}: one mana of any colour
// (the only shape kgen lowers beside one: restrictedComboModelled).
func wideBesideAny(cd *kop.CardDef, a *kop.Ability) bool {
	f := firstManaAb(cd)
	if f < 0 || &cd.Abilities[f] == a || !restrWideAnyMana(a) {
		return false
	}
	fa := &cd.Abilities[f]
	return fa.ManaRestrict == 0 && fa.ManaColors&kop.ColAny == kop.ColAny && fa.ManaN <= 1 &&
		fa.Cost.Flags == kop.CostTap && fa.Cost.Mana.MV() == 0 && fa.Cost.Mana.X == 0
}

// wideSecond: gap sl's wide ability is its second mana ability.
func (s *State) wideSecond(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	j := k.gen.anyR(c)
	return j >= 0 && int(j) != firstManaAb(&k.t.Cards[c])
}

// wide2EngineAnswer maps the search's answer index c of a second-ability wide
// gap to the engine-order answer tapGapChoice decodes: the first ability's
// colours with the play's first, then the multisets (wideEngineAnswer).
func wide2EngineAnswer(need [kop.NumManaKinds]bool, c int) int {
	if c >= wide2Answers {
		return c
	}
	if c >= kop.NumManaKinds-1 {
		return kop.NumManaKinds - 1 + wideEngineAnswer(need, c-(kop.NumManaKinds-1))
	}
	order := [kop.NumManaKinds - 1]uint8{0, 1, 2, 3, 4}
	needFirst(order[:], need)
	return int(order[c])
}

// wide2FirstColours is the first ability's colour ask in filtSearch
// (otherGap): its node, then each colour, the play's first. try submits an
// engine-order answer.
func (fs *filtSearch) wide2FirstColours(try func(uint8) bool) bool {
	if !fs.tick() { // the first ability's colour ask
		return false
	}
	order := [kop.NumManaKinds - 1]uint8{0, 1, 2, 3, 4}
	needFirst(order[:], fs.need)
	for _, c := range order {
		if try(c) {
			return true
		}
	}
	return false
}

// wide2RelaxExtra adds to a the TapsForMana extra a second-ability wide gap's
// relaxed alternatives carry (gorge paymentPlanRelaxedAlternatives adds the
// tap's largest extra to each multiset): Badgermole Cub's {G} when the source
// is a creature.
func (s *State) wide2RelaxExtra(k *prep, sl Slot, a *avail) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xv := s.srcExtra(k, sl, s.manaPCGen(k, sl))
	for c, n := range xv {
		if n > 0 {
			a.addUnits(1<<c, n)
		}
	}
}
