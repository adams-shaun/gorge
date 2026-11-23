package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A GainsAbilitiesOf$ recipient's gained reflected mana ability (CR 613.1f
// copy: Hazel's Brewmaster exiles Naga Vitalist, Foods its controller controls
// gain "{T}: Add one mana of any type a land you control could produce"). The
// copy's "you" is the recipient's controller, so its colours are read per
// recipient off that controller's board -- the same board read the printed
// reflected ability of the source itself takes (mana_reflected.go's
// reflColours), at the recipient's slot. The planner view is the source card's
// own varPC block (prepManaRefl's imprint arm: the admission
// gainedManaReflPlanned requires the ability to be the card's only mana
// ability), picked by the colours (view 0 is no mana ability: gorge lists the
// ability through paymentPlanReflectedAlt, which drops the {C} stand-in when
// the reflection names nothing).
//
// Everything else stays what mana_gainedgrants.go's header lists it: a census
// gap (unmodelled, never divergent) -- an imprint read, the controller's
// commander identity, a graveyard read, a spend restriction, a life-cost
// shape, or an ability beside another printed mana ability of its card (the
// source's own view block is the {C}-union shape a gained copy does not
// list).

// gainedManaReflPlanned is gainedManaPlanned's arm for a printed reflected
// mana ability: modelled only when the recipient's controller's board resolves
// its colours and the source card's own view block is the imprint shape.
func gainedManaReflPlanned(cd *kop.CardDef, a *kop.Ability) bool {
	if a.ManaAmong == 0 || a.ManaIdentity || a.ManaGrave || a.ManaRestrict != 0 ||
		a.ManaColors == 0 || a.ManaDeferred || comboAnyMana(a) || gainedManaLimited(a) {
		return false
	}
	if _, pain := manaLifePain(a); pain {
		return false // the pain-only alternative's shape is the printed one's (mana_imprint.go)
	}
	return loneManaAb(cd) // the source's varPC block is the imprint shape
}

// gainedReflAt reports whether grantPC entry gi is a gained reflected mana
// ability (gainedManaReflPlanned): grantView reads its colours per recipient.
func (s *State) gainedReflAt(g *genPrep, gi int16) bool {
	if !g.uses.has(useGainsOf) {
		return false
	}
	r := g.grantOf[gi]
	cd := &s.k().t.Cards[r.card]
	a := &cd.Abilities[r.ab]
	return a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.AFlags&kop.AfManaReflect != 0 &&
		gainedManaReflPlanned(cd, a)
}

// gainedReflColours is reflColours for the gained copy on the recipient at
// sl: the ability read from grantOf, the controller the recipient's (the
// copy's "you", CR 613.1f). Only the arms the admission models: the filter's
// produce read, and its Is read over the controller's permanents.
func (s *State) gainedReflColours(k *prep, sl Slot, a *kop.Ability) kop.ColorMask {
	if a.ManaProduce {
		m := s.produceColours(k, sl, a.ManaAmong)
		if a.ManaProduceType && s.producesColourless(k, sl, a.ManaAmong) {
			// ColorOrType$ Type: {C} coexists with the coloured masks (the
			// block's bit-5 views)
			m |= kop.ColC
		}
		return m
	}
	p := s.perms[sl].ctrl
	var m kop.ColorMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		o := b.first()
		if s.matchPerm(k, a.ManaAmong, o, p, sl) {
			m |= s.colorsAt(k, o)
		}
	}
	return m & kop.ColAny
}

// gainedReflView is grantView's arm for a gained reflected entry: the source
// card's varPC view of the colours the copy on the recipient at sl reads now.
func (s *State) gainedReflView(g *genPrep, gi int16, sl Slot) *pcard {
	r := g.grantOf[gi]
	vi := g.manaVar(r.card)
	if vi < 0 || !g.has(r.card, gfManaRefl) {
		return &noManaPC // unreachable for an admitted entry: loud, never a stray view
	}
	m := s.gainedReflColours(s.k(), sl, &s.k().t.Cards[r.card].Abilities[r.ab])
	if debugReflMutate == 1 {
		m = 0 // the mutation switch: the colours are never read (mana_reflected.go)
	}
	return &g.varPC[int(vi)+int(m)]
}
