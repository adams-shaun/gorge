package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The TapPower static build arm (was an arm of (*cardBuilder).static in
// build_ab.go): a Mode$ TapPowerValue static (kop.StatTapPower; the kernel's
// value_tappower.go).
func init() {
	registerStaticBuild("TapPower", cbTapPowerStatic)
}

func cbTapPowerStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	// Filter is ValidCard$, AddSub the host subtype its ValidSA$ qualifier
	// names, A the literal bonus or TapPowToughness, and VA the tapped
	// creature's own value under StDynamic
	st.Kind = kop.StatTapPower
	var err error
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	if s.F2 != nil {
		if st.Filter2, err = cb.staticFilter(s.F2, kop.ZoneBattlefield); err != nil {
			return nil, err
		}
	}
	for _, fl := range s.Flags {
		switch fl {
		case "Crew":
			st.StFlags |= kop.StTapPowerCrew
		case "Saddle":
			st.StFlags |= kop.StTapPowerSaddle
		case "Station":
			st.StFlags |= kop.StTapPowerStation
		case "Toughness":
			st.StFlags |= kop.StTapPowerTough
		default:
			return nil, fmt.Errorf("tap power flag %q", fl)
		}
	}
	if s.Amount.Kind != vNone {
		if s.Amount.isConst() {
			st.A = int16(s.Amount.N)
		} else {
			st.StFlags |= kop.StDynamic
			if st.VA, err = cb.value(s.Amount); err != nil {
				return nil, err
			}
		}
	}
	if s.Amount.Kind != vNone && st.StFlags&kop.StTapPowerTough != 0 {
		return nil, fmt.Errorf("tap power static with both a value and Toughness")
	}
	return nil, nil
}
