package canon

import "github.com/adams-shaun/gorge/dirac"

// ChGenUnleash's option (dirac/kw_unleash.go): gorge's "unleash" option kind
// is outside ChooseKinds, so both sides canonicalise as choose:other over the
// entering permanent (the two options canon-equal, paired by occurrence
// order).
func unleashCanon(src dirac.TCode) Canon {
	return Pack(ActChoose+CanonKind(ChooseOther), Ref(src), RefNone, 0)
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
