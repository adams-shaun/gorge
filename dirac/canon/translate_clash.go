package canon

import "github.com/adams-shaun/gorge/dirac"

// clashCanon is ChGenClash's option (dirac/act_clash.go): gorge's
// clash_placement KChoose of "bottom" / "top" over the revealed card. ok is
// false for any other ask.
func clashCanon(sub dirac.ChoiceKind, src dirac.TCode, mode uint8) (Canon, bool) {
	if sub != dirac.ChGenClash {
		return 0, false
	}
	k := "bottom"
	if mode == 1 {
		k = "top"
	}
	c, _ := ChooseCode(k)
	return Pack(ActChoose+CanonKind(c), Ref(src), RefNone, 0), true
}
