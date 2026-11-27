package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Continuous static build arm (was an arm of (*cardBuilder).static in
// build_ab.go). The file is not static_continuous.go, the parse-mode body of
// Mode$ Continuous (static_mode.go): registry name collisions aside, this is
// the build side (static_reg.go).
func init() {
	registerStaticBuild("Continuous", cbContinuousStatic)
}

func cbContinuousStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var add []uint8
	var err error
	zone := continuousKind(st, s.Flags, s.Zone) // lower_offzone.go
	if s.F != nil && s.F.Players != 0 {
		// "You and <objects> you control have hexproof" (Captain America,
		// Shalai, Sigarda): the controller half rides StYouHexproof (the
		// kernel's playerHexproof reads it under Cond2). Any other
		// player half was dropped by filter() before shield: refused.
		if s.F.Players != plYou || len(s.Kw) != 1 || s.Kw[0] != "Hexproof" || s.Power.Kind != vNone || s.Tough.Kind != vNone ||
			len(s.Grant) != 0 || len(s.AddSub) != 0 || s.AddChosen {
			return nil, fmt.Errorf("continuous static affecting players %d (only \"you have hexproof\" is read)", s.F.Players)
		}
		st.StFlags |= kop.StYouHexproof
	}
	st.StFlags |= gyContStFlag(s.Flags)
	if st.Filter, err = cb.staticFilter(s.F, zone); err != nil {
		return nil, err
	}
	if err := cb.staticAmounts(st, s.Power, s.Tough); err != nil {
		return nil, err
	}
	if err := continuousStFlags(s, st); err != nil {
		return nil, err
	}
	if st.Kw, err = kwMask(s.Kw); err != nil {
		return nil, err
	}
	st.Ward = uint8(s.Ward)
	st.WardLife = uint8(s.WardLife)
	if add, err = cb.subIndexes(s.AddSub); err != nil {
		return nil, err
	}
	st.AddChosen = s.AddChosen
	st.OutlivesLoss = s.OutlivesLoss
	if err := cb.staticGrant(st, s); err != nil {
		return nil, err
	}
	if st.ProtFrom, err = cb.staticFilter(s.ProtFrom, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	st.ProtChosen = s.ProtChosen
	if st.ProtExc, err = cb.staticFilter(s.ProtExc, kop.ZoneBattlefield); err != nil {
		return nil, err
	}
	if st.Landwalk, st.GainsOf, err = continuousFilterPair(cb.staticFilter, &s, st); err != nil {
		return nil, err
	}
	st.Evasion, st.Infect, st.Toxic = s.Evasion, s.Infect, s.Toxic // lower_kwmark.go
	st.Riot = s.Riot                                               // lower_riotgrant.go
	st.Unleash = s.Unleash                                         // lower_unleash.go
	if err := cb.staticOverride(st, s); err != nil {
		return nil, err
	}
	return add, nil
}
