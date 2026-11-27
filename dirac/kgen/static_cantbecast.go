package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CantBeCast static build arm (was an arm of (*cardBuilder).static in
// build_ab.go): a cast gate (kop.StatCantBeCast; cast_cantself.go and the
// kernel's cast gate).
func init() {
	registerStaticBuild("CantBeCast", cbCantBeCastStatic)
}

// selfZoneKop maps a self static's zone flag to its kernel zone.
var selfZoneKop = map[string]kop.Zone{
	"SelfZoneLibrary": kop.ZoneLibrary, "SelfZoneHand": kop.ZoneHand, "SelfZoneGraveyard": kop.ZoneGraveyard,
	"SelfZoneExile": kop.ZoneExile, "SelfZoneStack": kop.ZoneStack, "SelfZoneCommand": kop.ZoneCommand,
}

func cbCantBeCastStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	st.Kind = kop.StatCantBeCast
	var err error
	if s.F != nil { // nil: the card's own static (Card.Self | EffectZone$ All)
		if st.Filter, err = cb.staticFilter(s.F, kop.ZoneStack); err != nil {
			return nil, err
		}
	}
	if s.Zone == zAny {
		st.Zone = kop.ZoneCommand
	} else if s.Zone == zHand {
		st.Zone = kop.ZoneHand // Origin$ Hand: the origin the cast gate reads (cast_cantself.go)
	}
	for _, fl := range s.Flags {
		switch fl {
		case "ActivatorYou":
			st.StFlags |= kop.StRaiseYou
		case "ActivatorOpp":
			st.StFlags |= kop.StRaiseOpp
		case "CasterActive":
			st.StFlags |= kop.StCasterActive
		case "SelfZoneLibrary", "SelfZoneHand", "SelfZoneGraveyard", "SelfZoneExile", "SelfZoneStack", "SelfZoneCommand":
			// the zones a self static binds in (lower_cantbecast.go selfCastZones)
			st.B |= kop.CantCastSelfZones | 1<<kop.CantCastSelfShift<<selfZoneKop[fl]
		case "NamedCard":
			st.B |= kop.CantCastNamed
		case "SorcerySpeed":
			st.B |= kop.CantCastSorcery
		default:
			return nil, fmt.Errorf("cant be cast flag %q", fl)
		}
	}
	if s.Amount.Kind != vNone {
		if !s.Amount.isConst() {
			return nil, fmt.Errorf("cant be cast limit %v", s.Amount)
		}
		st.A = int16(s.Amount.N)
	}
	return nil, nil
}
