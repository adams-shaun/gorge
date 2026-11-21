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

// unleashRiotCanon is the canonical option of the two as-enters elections.
func unleashRiotCanon(sub dirac.ChoiceKind, src dirac.TCode, mode uint8) Canon {
	if sub == dirac.ChGenRiot {
		return riotCanon(src, mode)
	}
	return unleashCanon(src, mode)
}

// riotCanon is ChGenRiot's option (dirac/kw_riot.go): gorge's "riot" option
// kind over the entering permanent. The two options name the same permanent, so
// gorge's disambiguate gives the k-th repeat mode k in option order; the
// kernel lists the counter (mode 0) first and haste (mode 1) second and
// carries the same ordinal.
func riotCanon(src dirac.TCode, mode uint8) Canon {
	code, _ := ChooseCode("riot")
	return Pack(ActChoose+CanonKind(code), Ref(src), RefNone, mode&1)
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
