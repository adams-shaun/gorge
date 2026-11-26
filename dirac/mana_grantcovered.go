package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// grantCovered reports whether the mana ability grant gi gives sl is a census
// gap gorge does not have: a lone "{T}: Add one mana of [one colour]" that one
// of sl's own usable printed lone-{T} abilities already makes (an equal
// ability, or an any-colour one: paymentPlanCensusOf's manaAbilityCovered,
// manaTapGapDominated). gorge's census lists a unit for the printed ability,
// so the grant makes the census no less complete and its holder is not a
// candidate of the prefix search. A Paradise Druid under Rishkar's "{T}: Add
// {G}" is the planner's own unit: the search taps the granted sources the
// census lacks and leaves the Druid to the planner, which pays at the window
// after the targets (CR 601.2c, 601.2f-g).
//
// Only the plain case is read: a gate, a reflected or variable ability, a
// restriction, a rider or a self-colour grant keeps the grant a gap.
func (s *State) grantCovered(k *prep, sl Slot, gi int16) bool {
	g := k.gen
	c := s.cardAt(sl)
	if g.grantSelf[gi] || g.flags[c]&(gfManaGate|gfManaRefl|gfManaVar) != 0 || !s.ownManaUsable(k, sl) {
		return false
	}
	r := g.grantOf[gi]
	if r.card == c {
		return false
	}
	ga := &k.t.Cards[r.card].Abilities[r.ab]
	if !loneTapMana(ga) || bits.OnesCount8(uint8(ga.ManaColors)) != 1 {
		return false
	}
	own := k.t.Cards[c].Abilities
	for i := range own {
		if loneTapMana(&own[i]) && (own[i].ManaColors == ga.ManaColors || own[i].ManaColors == kop.ColAny) {
			return true
		}
	}
	return false
}

// loneTapMana: a is a plain "{T}: Add one mana" ability (pay.manaTapSingle's
// shape): no other cost, amount, restriction, rider, gate or program.
func loneTapMana(a *kop.Ability) bool {
	return a.Kind == kop.AbMana && a.Cost == kop.Cost{Flags: kop.CostTap} && a.ManaN == 1 && a.ManaColors != 0 &&
		a.AFlags&^kop.AfLatent == 0 && a.ManaColOrd == 0 && a.ManaV == (kop.ValueRef{}) && a.ManaRestrict == 0 &&
		a.ManaRestrictAb == 0 && a.ManaAmong == 0 && !a.ManaProduce && !a.ManaIdentity && !a.ManaIdentityPotential &&
		a.Gate == (kop.Gate{}) && a.ActZone == kop.ZoneNone && len(a.Prog) == 0
}

// uncoveredGrants drops from gi[:n] the grants grantCovered reads as covered
// by sl's own ability, keeping the rest in order; it returns the new count.
func (s *State) uncoveredGrants(k *prep, sl Slot, gi *[maxGrants]int16, n int) int {
	if n == 0 {
		return 0
	}
	m := 0
	for i := 0; i < n; i++ {
		if !s.grantCovered(k, sl, gi[i]) {
			gi[m] = gi[i]
			m++
		}
	}
	return m
}
