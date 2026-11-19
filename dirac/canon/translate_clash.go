package canon

import "github.com/adams-shaun/gorge/dirac"

// clashCanon is ChGenClash's option (dirac/act_clash.go): gorge's
// clash_placement KChoose of "bottom" / "top" over the revealed card, and
// ChGenMyriad's (dirac/kw_myriad.go): gorge effMyriad's KChoose of "yes" /
// "no", both over the Myriad source. ok is false for any other ask.
func clashCanon(sub dirac.ChoiceKind, src dirac.TCode, mode uint8) (Canon, bool) {
	var k string
	switch {
	case sub == dirac.ChGenClash && mode == 1:
		k = "top"
	case sub == dirac.ChGenClash:
		k = "bottom"
	case sub == dirac.ChGenMyriad && mode == 1:
		k = "yes"
	case sub == dirac.ChGenMyriad:
		k = "no"
	default:
		return 0, false
	}
	c, _ := ChooseCode(k)
	return Pack(ActChoose+CanonKind(c), Ref(src), RefNone, 0), true
}
