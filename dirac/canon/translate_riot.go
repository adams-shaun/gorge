package canon

import "github.com/adams-shaun/gorge/dirac"

// ChGenRiot's option (dirac/kw_riot.go): gorge's "riot" option kind, the
// as-enters counter-or-haste election over the entering permanent. The two
// options name the same permanent, so gorge's disambiguate gives the k-th
// repeat mode k in option order; the kernel lists the counter (mode 0,
// gorge's option 0, "Enter with a +1/+1 counter") first and the haste (mode
// 1, option 1, "Gain haste") second and carries the same ordinal, so the
// two engines share option 0 (CR 702.136).
func riotCanon(src dirac.TCode, mode uint8) Canon {
	code, _ := ChooseCode("riot")
	return Pack(ActChoose+CanonKind(code), Ref(src), RefNone, mode)
}

// asEntersCanon is the as-enters election of generatedChoose: riot's
// counter-or-haste pick when riot is set, unleash's yes/no pick otherwise.
func asEntersCanon(riot bool, src dirac.TCode, mode uint8) Canon {
	if riot {
		return riotCanon(src, mode)
	}
	return unleashCanon(src, mode)
}
