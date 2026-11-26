package kgen

import (
	"fmt"
	"slices"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MayPlayFrom static build arm (was an arm of (*cardBuilder).static in
// build_ab.go): a play-from-wherever grant (kop.StatMayPlayFrom; the kernel's
// mayPlayFrom* walk).
func init() {
	registerStaticBuild("MayPlayFrom", cbMayPlayFromStatic)
}

func cbMayPlayFromStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	st.Kind = kop.StatMayPlayFrom
	st.Zone = zoneMap[s.Zone]
	fz := st.Zone
	if slices.Contains(s.Flags, "AlsoSideboard") || slices.Contains(s.Flags, "FreeOnly") {
		fz = kop.ZoneNone // the filter also reads a sideboard card; the static's zones are tested apart (mayPlayFromZone)
	}
	if slices.Contains(s.Flags, "FunctionsInGraveyard") && slices.Contains(s.Flags, "FunctionsInExile") {
		fz = kop.ZoneNone // a two-zone self grant (Squee, the Immortal): the self walk keys on the flags, and one zone in the filter would withhold the other
	}
	var err error
	if st.Filter, err = cb.staticFilter(s.F, fz); err != nil {
		return nil, err
	}
	// the count of "RemoveAnyCounter<N/Any/Creature>" (RaiseCostRemove6...)
	if err := cb.staticAmounts(st, s.Amount, lValue{}); err != nil {
		return nil, err
	}
	for _, fl := range s.Flags {
		switch fl {
		case "OncePerTurn":
			st.StFlags |= kop.StOncePerTurn
		case "AnyManaType":
			st.StFlags |= kop.StAnyManaType
		case "WithoutManaCost":
			st.StFlags |= kop.StWithoutManaCost
		case "SeatActive":
			st.StFlags |= kop.StSeatActive
		case "FunctionsInGraveyard":
			st.StFlags |= kop.StFunctionsInGraveyard
		case "FunctionsInExile":
			st.StFlags |= kop.StFunctionsInExile
		case "AlsoSideboard":
			st.StFlags |= kop.StAlsoSideboard
		case "FreeOnly":
			st.StFlags |= kop.StFreeOnly
		case "MayPayLife":
			st.StFlags |= kop.StMayPayLife
		case "MayZeroAlt":
			st.StFlags |= kop.StMayZeroAlt // a non-granting alternative cost (cast_mayzeroalt.go)
		case "MayPayEnergy":
			st.StFlags |= kop.StMayPayEnergy // the alternative's energy rides Static.A
		case "MayPayEvidence":
			st.StFlags |= kop.StMayPayEvidence // the alternative's evidence rides Static.A
		case "RaiseCostRemove6CountersFromCreatures":
			st.StFlags |= kop.StRemove6Counters
		case "MayRaise":
			st.StFlags |= kop.StMayRaise
			j, ok := cb.abMap[s.RaiseAb-1]
			if !ok {
				return nil, fmt.Errorf("MayRaise ability %d not emitted", s.RaiseAb-1)
			}
			st.B = int16(j + 1)
		default:
			return nil, fmt.Errorf("MayPlay flag %s", fl)
		}
	}
	return nil, nil
}
