package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A reflected Produce mana ability with an Amount$ (Incubation Druid's "{T}:
// Add one mana of any type that a land you control could produce. If it has a
// +1/+1 counter on it, add three mana of that type instead"). gorge's payment
// planner lists it in its evaluated-amount layer (rules
// paymentPlanManaUnitsOnlyCompute: pay.PaymentPlanReflectedShape admits the
// Amount$, paymentPlanReflectedAlt prices it with the engine's own evaluator and
// refuses one a tap could move): one alternative per reflected symbol, each at
// the evaluated amount, a tapped land still "could produce" its mana (CR
// 106.7). The potential bound folds the same alternative
// (addPotentialReflected). The kernel mirrors it as a planner unit of its own:
// prepReflAmt builds one view per reflected symbol set and amount, laid out
// mask-major, and manaReflView picks the live one (reflAmtView). A view is a
// choice among the set's colours making the amount of one of them (manaOne,
// the exact one-colour entry avail.oneExact, which prices the pick over the
// set's own colours, {C} included); like gorge's evaluated-amount layer
// (manaDyn) it is counted after the choice-shaped sources. A set of no colour,
// or an amount of nothing, makes no mana.

// reflAmtManaAb reports whether a is a reflected Produce mana ability with a
// non-constant Amount$ (Incubation Druid): plain {T}, the reflected read over
// ManaAmong, one mana of any colour scaled by the Amount$ expression
// (AfManaDynamic; kop.ManaV).
func reflAmtManaAb(a *kop.Ability) bool {
	return a.Kind == kop.AbMana &&
		a.AFlags&(kop.AfManaReflect|kop.AfManaChoice|kop.AfManaDynamic|kop.AfLatent) == kop.AfManaReflect|kop.AfManaChoice|kop.AfManaDynamic &&
		a.ManaColors == kop.ColAny && a.ManaAmong != 0 && a.ManaProduce && a.ManaRestrict == 0 && a.ManaRestrictAb == 0 &&
		!a.ManaDeferred && !a.ManaEach && !a.ManaIdentity && !a.ManaIdentityPotential && a.ManaRider == 0 &&
		a.SplitGate == 0 && a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// reflAmtMaxN is the largest evaluated amount the reflected Amount$ views
// price (Incubation Druid's is one or three); a larger one stalls by name
// (dynManaN's capFull) rather than truncating.
const reflAmtMaxN = 3

// reflAmtAbOf is the card's reflected Amount$ mana ability (reflAmtManaAb), nil
// when it has none.
func reflAmtAbOf(cd *kop.CardDef) *kop.Ability {
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; reflAmtManaAb(a) {
			return a
		}
	}
	return nil
}

// prepReflAmt builds the reflected Amount$ source's views (prepManaRefl): one
// per reflected symbol set (64: a ColorOrType$ Type read keeps {C} as bit 5)
// and evaluated amount, laid out mask-major (reflAmtView). gorge's planner
// lists the source in its evaluated-amount layer, last, as one alternative
// per reflected symbol at the amount (rules paymentPlanReflectedAlt, CR
// 106.7): a view is a choice among the set's colours making the amount of
// one of them (manaOne). A set of no colour, or an amount of nothing, makes
// no mana.
func prepReflAmt(g *genPrep, id CardID, pc *pcard, ra int, a *kop.Ability) {
	g.setFlag(id, gfManaVar|gfManaRefl)
	g.smallFor(id).manaVar = int16(len(g.varPC))
	for m := 0; m < 64; m++ {
		for n := 0; n <= reflAmtMaxN; n++ {
			v := *pc
			v.manaAb, v.manaCol, v.manaN = int8(ra), kop.ColorMask(m), uint8(n)
			v.manaTap, v.manaDyn, v.manaMulti, v.manaRefl = true, true, false, true
			v.manaOne = n > 1 && m&(m-1) != 0
			if m == 0 || n == 0 {
				v.manaAb = -1
			}
			g.varPC = append(g.varPC, v)
		}
	}
	g.anyVergeOrder, g.anyRefl = true, true
}

// reflAmtView is the view offset of the reflected Amount$ source sl for the
// reflected symbol set m: its evaluated amount now (dynManaN).
func (s *State) reflAmtView(k *prep, sl Slot, a *kop.Ability, m kop.ColorMask) int {
	n := max(s.dynManaN(k, a, sl), 0)
	if n > reflAmtMaxN {
		s.capFull(StallDynMana)
		n = reflAmtMaxN
	}
	return int(m&63)*(reflAmtMaxN+1) + n
}

// addSrcOneColour adds the one-colour entry of source sl's view pc (manaOne):
// a reflected Amount$ view's pick is limited to the reflected colours
// (addOneColourExact), any other source's is any colour.
func (s *State) addSrcOneColour(a *avail, pc *pcard, sl Slot) {
	if pc.manaDyn && pc.manaRefl {
		s.addOneColourExact(a, s.srcMaskAt(pc, sl), pc.manaN)
		return
	}
	s.addOneColour(a, pc.manaN)
}

// addOwnOneColour adds the one-colour entry of a source's own ability on a
// table with Continuous mana grants (availGrant): a reflected Amount$ view
// keeps its exact reflected colours as addSrcOneColour does (the grant
// branches priced it over any of the five, so Incubation Druid's {G}{G}{G}
// beside a tapped Forest, CR 106.7, paid no {1}{G} cast with a grant card
// elsewhere on the table), any other source's own is any colour. mask and n
// are availGrant's own face (the view's mask, its yield).
func (s *State) addOwnOneColour(a *avail, pc *pcard, mask, n uint8) {
	if pc.manaDyn && pc.manaRefl {
		s.addOneColourExact(a, mask, n)
		return
	}
	s.addOneColour(a, n)
}
