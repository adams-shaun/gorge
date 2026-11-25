package canon

import "github.com/adams-shaun/gorge/dirac"

// learnCanon is ChLearn's option (dirac/act_learn.go): gorge effLearn's
// KChoose of "nothing", "lesson" over the sideboard card, or "rummage".
func learnCanon(src dirac.TCode, mode uint8) Canon {
	k, ref := "nothing", RefNone
	switch mode {
	case 1:
		k, ref = "lesson", Ref(src)
	case 2:
		k = "rummage"
	}
	c, _ := ChooseCode(k)
	return Pack(ActChoose+CanonKind(c), ref, RefNone, 0)
}
