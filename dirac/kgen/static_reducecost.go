package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The ReduceCost static build arm (was an arm of (*cardBuilder).static in
// build_ab.go): a cost reduction (kop.StatReduceCost) or an activated
// ability's reduction (kop.StatReduceAbility, lower_reduceab.go).
func init() {
	registerStaticBuild("ReduceCost", cbReduceCostStatic)
}

func cbReduceCostStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	if s.AbilityRed != 0 {
		// activated abilities of battlefield permanents (lower_reduceab.go)
		st.Kind = kop.StatReduceAbility
		fz := kop.ZoneBattlefield
		if s.Zone == zGraveyard {
			st.Zone, fz = kop.ZoneGraveyard, kop.ZoneGraveyard // lower_reduceab.go: a graveyard card's own activation
		}
		if st.Filter, err = cb.staticFilter(s.F, fz); err != nil {
			return nil, err
		}
		if err := cb.staticAmounts(st, s.Amount, lValue{}); err != nil {
			return nil, err
		}
		rb, rf := reduceAbBits(s)
		st.B, st.StFlags = st.B|rb, st.StFlags|rf
		if s.F2 != nil {
			if err := cb.staticTargets2(st, s.F2); err != nil { // a target-gated reduction (the equip's Card.Self; Bladegraft's ability target)
				return nil, err
			}
		}
		return nil, nil
	}
	st.Kind = kop.StatReduceCost
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneStack); err != nil {
		return nil, err
	}
	if err := cb.staticAmounts(st, s.Amount, lValue{}); err != nil {
		return nil, err
	}
	if err := cb.staticTargets2(st, s.F2); err != nil {
		return nil, err
	}
	if s.Zone == zAny {
		st.Zone = kop.ZoneCommand
	}
	for _, fl := range s.Flags {
		switch fl {
		case "Bargained", "OptPaid":
			st.StFlags |= kop.StBargained // lower_reducespell.go, lower_optcost.go
		case "ReduceColor":
			st.StFlags |= kop.StReduceColor
			st.B = int16(s.RedCol)
		case "AnyCaster":
			st.StFlags |= kop.StReduceAny
		case "EmbNum":
			// the amount is the emblem's frozen
			// SetChosenNumber$, read by adjustCostGen from embN
			st.StFlags |= kop.StEmbNum
		default:
			return nil, fmt.Errorf("reduce cost flag %q", fl)
		}
	}
	return nil, nil
}
