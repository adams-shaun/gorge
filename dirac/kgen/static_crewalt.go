package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A Vehicle's own crew alternative cost (CR 118.9, 702.122): Heart of Kiran's
// "You may remove a loyalty counter from a planeswalker you control rather
// than pay this Vehicle's crew cost", the static
//
//	S:Mode$ AlternativeCost | ValidSA$ Activated.Crew+Vehicle | ValidCard$ Card.Self
//	  | ValidPlayer$ You | Cost$ SubCounter<1/LOYALTY/Planeswalker.YouCtrl/...>
//
// gorge offers it as a second "ability" option on the crew ability
// (AltCostIndex 1, engine/rules activate_static_altcost.go
// staticAltCostScopesCrew), paying the static's cost in place of the tap-power
// cost. The kernel models it as a second activated ability: a copy of the crew
// ability whose Cost is the alternative (kop.Ability.AltOf names the crew
// ability), so its offer, payment picks and resolution are the ordinary
// activation's and the spell alternative's build.altAdd is not involved.

// crewAltKind is the lStatic kind of the lowered static: not "AlternativeCost",
// which the kernel's cast walks read as a spell's alternative cost.
const crewAltKind = "CrewAltCost"

// crewAltCost lowers the static's shape (modeAlternativeCost's Crew arm). It
// is the Vehicle's own grant only: the card's own ValidCard$ Card.Self,
// ValidPlayer$ You, on the battlefield, with no condition beside it.
func (sc *staticCtx) crewAltCost() ([]lStatic, bool, error) {
	sp := sc.sp
	lc := sc.lc
	if sp.str("ValidCard") != "Card.Self" || sp.str("ValidPlayer") != "You" {
		return nil, true, refuse("AlternativeCost shape")
	}
	if _, ok := sp.get("EffectZone"); ok || sc.st.Cond.Kind != cNone {
		return nil, true, refuse("AlternativeCost shape")
	}
	lc.crewAltCost = true
	cost, err := lc.cost(sp.str("Cost"))
	lc.crewAltCost = false
	if err != nil {
		return nil, true, err
	}
	if cost.SubAmongN != 1 || cost.SubAmongF == nil || cost.SubAmongKnd != "LOYALTY" || !crewAltPlain(cost) {
		return nil, true, refuse("crew alternative cost %q", sp.str("Cost"))
	}
	sc.st.Kind, sc.st.Cost = crewAltKind, cost
	return nil, false, nil
}

// crewAltPlain: the lowered cost is the loyalty removal alone.
func crewAltPlain(c lCost) bool {
	n := c
	n.SubAmongN, n.SubAmongKnd, n.SubAmongF = 0, "", nil
	m := n.Mana
	return n.pureMana() && m.Generic == 0 && m.Pips == [5]int{} && len(m.Hybrid) == 0 && !m.X
}

// crewAltAbility builds the crew alternative's ability: the card's crew
// ability with the static's cost in its place, AltOf pointing at it.
func (cb *cardBuilder) crewAltAbility(s lStatic) (kop.Ability, []uint8, error) {
	d := &cb.out.def
	host := -1
	for i := range d.Abilities {
		if a := &d.Abilities[i]; a.Kind == kop.AbActivated && a.Cost.Flags&kop.CostCrew != 0 {
			if host >= 0 {
				return kop.Ability{}, nil, fmt.Errorf("crew alternative cost beside two crew abilities")
			}
			host = i
		}
	}
	if host < 0 {
		return kop.Ability{}, nil, fmt.Errorf("crew alternative cost without a crew ability")
	}
	ka := d.Abilities[host]
	if ka.Cost.Flags != kop.CostTapPower|kop.CostCrew || ka.CostProg != 0 || ka.AltOf != 0 || ka.AFlags != 0 {
		return kop.Ability{}, nil, fmt.Errorf("crew alternative cost beside a crew ability with riders")
	}
	cost, err := cb.cost(s.Cost)
	if err != nil {
		return ka, nil, err
	}
	ka.Cost = cost
	ka.AltOf = uint8(host + 1)
	return ka, nil, nil
}

// crewAltOrigin is the gorge ability a static's kernel ability stands for: the
// crew alternative's stack object is the crew ability's (gorge pushes the one
// SA at AltCostIndex 1), every other static has none.
func crewAltOrigin(origins []uint8, ka kop.Ability) uint8 {
	if ka.AltOf != 0 && int(ka.AltOf) <= len(origins) {
		return origins[ka.AltOf-1]
	}
	return 0xFF
}
