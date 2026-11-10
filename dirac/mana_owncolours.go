package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A granted "{T}: Add one mana of any of this creature's colors"
// (Katilda, Dawnhart Prime: AB$ ManaReflected, Valid$ Defined.Self,
// ReflectProperty$ Is, granted to Human creatures you control). gorge
// (10772317e, pay/planreflected.go reflectsStableObjects) prices the
// grantee's own colours in the planner census instead of the {C} stand-in,
// so the grant stays a census gap exactly like Cryptolith Rite's (the
// tokmana prefix search, mana_grantedoffers.go), but each granted unit is
// the colours of the slot it is granted to: a colourless creature reflects
// nothing (gorge lists no such ability).

// selfColMask is the production mask of a self-reflecting grant on the
// permanent at sl: its own colours, WUBRG.
func (s *State) selfColMask(sl Slot) uint8 {
	if debugOwnColoursMutate == 1 {
		return uint8(kop.ColW)
	}
	return uint8(s.colorsAt(s.k(), sl) & kop.ColAny)
}

// debugOwnColoursMutate is the test-only mutation switch (0 none; 1 a self
// grant produces W alone whatever the grantee's colours). TestCardKatildaLockstepMutation
// requires each to diverge.
var debugOwnColoursMutate uint8

// SetDebugOwnColoursMutate sets the mutation switch; tests only.
func SetDebugOwnColoursMutate(m uint8) { debugOwnColoursMutate = m }

// selfGrantLive reports whether p controls a permanent that can tap for a
// self-reflecting grant now. gorge's census then misses an ability it has no
// relaxed alternative for (potential_plan.go paymentPlanRelaxedAlternatives
// admits API Mana only), so paymentPlanCensusOf.relaxable is false and the
// unpayability proof is skipped: every play its PotentialMana bound lists
// (a source counts each colour it can make) that the witness does not pay
// takes one of the prefix searches.
func (s *State) selfGrantLive(k *prep, p uint8) bool {
	if !k.gen.anySelfGrant {
		return false
	}
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		var gi [maxGrants]int16
		n := s.grantIdxs(k, sl, &gi)
		for i := 0; i < n; i++ {
			if k.gen.grantSelf[gi[i]] && s.canTapForMana(k, sl) {
				return true
			}
		}
	}
	return false
}

// addGrantUnits adds a source's unit to a: whole (a unit makes one of its
// colours), or under availHyp one unit per colour (PotentialMana's bound).
func (s *State) addGrantUnits(a *avail, mask, n uint8) {
	if s.g.availX&availHyp == 0 || mask&(mask-1) == 0 {
		a.addUnits(mask, n)
		return
	}
	for c := uint8(0); c < kop.NumManaKinds; c++ {
		if mask&(1<<c) != 0 {
			a.addUnits(1<<c, n)
		}
	}
}

// prepSelfGrant builds a self grant's 32 planner views (gp with the colour
// mask m as its production) and records that the table has one.
func (g *genPrep) prepSelfGrant(gp pcard) {
	g.anySelfGrant = true
	for m := range g.selfPC {
		v := gp
		v.manaCol = kop.ColorMask(m)
		g.selfPC[m] = v
	}
}

// grantView is the planner view of granted ability gi on the permanent at
// sl: the table's pcard, or for a self grant the view of the permanent's own
// colours.
func (s *State) grantView(g *genPrep, gi int16, sl Slot) *pcard {
	if !g.grantSelf[gi] {
		return &g.grantPC[gi]
	}
	return &g.selfPC[s.selfColMask(sl)]
}
