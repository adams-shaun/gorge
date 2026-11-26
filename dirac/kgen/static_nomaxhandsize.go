package kgen

import (
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The NoMaxHandSize static build arm (was an arm of (*cardBuilder).static in
// build_ab.go): the hand-maximum set, raised, or dropped (kop.StatNoMaxHandSize;
// the kernel's maxHand, static_playerstats.go).
func init() {
	registerStaticBuild("NoMaxHandSize", cbNoMaxHandSizeStatic)
}

func cbNoMaxHandSizeStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	st.Kind = kop.StatNoMaxHandSize
	if slices.Contains(s.Flags, "HandDelta") {
		// CR 402.2: the additive RaiseMaxHandSize$ statics apply on top of
		// the set maximum, never below zero (gorge maxHandSizeFor): A is
		// the signed delta, read after the set-maximum walk (kernel
		// maxHand, static_playerstats.go)
		if !s.Amount.isConst() || s.Amount.N < -255 || s.Amount.N > 255 {
			return nil, fmt.Errorf("NoMaxHandSize delta %v", s.Amount)
		}
		st.StFlags |= kop.StHandDelta
		st.A = int16(s.Amount.N)
	} else {
		switch {
		case s.Amount.Kind == vNone: // Unlimited: A 0
		case s.Amount.isConst() && s.Amount.N >= 0 && s.Amount.N <= 255:
			st.A = int16(s.Amount.N + 1) // a numeric maximum: A = N+1
		case len(s.Flags) == 1 && s.Flags[0] == "HandOpp":
			// the opponents' maximum from an SVar (Winter): VA, read per
			// cleanup under Cond2 (kernel maxHand)
			st.StFlags |= kop.StDynamic
			var err error
			if st.VA, err = cb.value(s.Amount); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("NoMaxHandSize amount %v", s.Amount)
		}
	}
	for _, fl := range s.Flags {
		if fl != "HandOpp" && fl != "HandDelta" {
			return nil, fmt.Errorf("NoMaxHandSize flag %q", fl)
		}
		if fl == "HandOpp" {
			st.StFlags |= kop.StHandOpp
		}
	}
	return nil, nil
}
