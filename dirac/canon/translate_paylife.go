package canon

import "github.com/adams-shaun/gorge/dirac"

// numberChoose is the choose action of the as-enters number asks, the default
// of Translator.Action's KChoose arm: gorge's ETB "number" option (RefNone, the
// value on the mode), or ChGenPayLife's "paylife" option over the entering
// permanent (gorge etbPayLifeOptions: Kind "paylife", Obj the card, the amount
// paid on Amount), or ChGenReadAhead's "readahead" option over the entering Saga
// (gorge readAheadOptions: Obj the Saga, the chapter on Amount), each its own
// kind and never the number's.
func (r *Translator) numberChoose(src Ref, mode uint8) Canon {
	kind, s := "number", RefNone
	switch r.D.Sub {
	case dirac.ChGenPayLife:
		kind, s = "paylife", src
	case dirac.ChGenReadAhead:
		kind, s = "readahead", src
	}
	code, _ := ChooseCode(kind)
	return Pack(ActChoose+CanonKind(code), s, RefNone, mode)
}
