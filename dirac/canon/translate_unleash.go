package canon

import "github.com/adams-shaun/gorge/dirac"

// ChGenUnleash's option (dirac/kw_unleash.go): gorge's "unleash" option kind,
// the "unleash" choose kind over the entering permanent. The two options name
// the same permanent, so gorge's disambiguate gives the k-th repeat mode k in
// option order; the kernel lists "yes" (mode 1) first and "no" (mode 0) second
// and carries the same ordinal.
func unleashCanon(src dirac.TCode, mode uint8) Canon {
	code, _ := ChooseCode("unleash")
	return Pack(ActChoose+CanonKind(code), Ref(src), RefNone, 1-mode&1)
}

// playerPick is the player-or-object pick of generatedChoose: a player named
// by PlayerTarget0+p takes the kind k player option, anything else the kind
// k2 option over the object at src (ChGenProlif, ChCastDivide, ChGenDivide,
// ChGenEnterDefender).
func playerPick(k, k2 string, src dirac.TCode) (Canon, bool) {
	if src >= dirac.PlayerTarget0 {
		c, _ := ChooseCode(k)
		return Pack(ActChoose+CanonKind(c), RefNone, RefPlayer+Ref(src-dirac.PlayerTarget0), 0), true
	}
	c, _ := ChooseCode(k2)
	return Pack(ActChoose+CanonKind(c), Ref(src), RefNone, 0), true
}

// OptInSeatCanon is the repeated player's own yes / no offer
// (RepeatOptionalForEachPlayer$; dirac ChGenOptInSeat, gorge's
// repeat_each_optional): the explicit named-seat kinds yes_seat / no_seat,
// never the generic yes / no a controller's election uses. Both sides name it
// here: the kernel's translation and the oracle's reading of gorge's decision.
func OptInSeatCanon(yes bool) Canon {
	name := "no_seat"
	if yes {
		name = "yes_seat"
	}
	code, _ := ChooseCode(name)
	return Pack(ActChoose+CanonKind(code), RefNone, RefNone, 0)
}

// OptInSeatName is the gorge option kind (yes / no) of a canonical named-seat
// kind (yes_seat / no_seat); ok is false for any other kind name.
func OptInSeatName(kind string) (string, bool) {
	switch kind {
	case "yes_seat":
		return "yes", true
	case "no_seat":
		return "no", true
	}
	return "", false
}
