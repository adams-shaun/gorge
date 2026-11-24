package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	instrOp("OptInSeat", (*instrCtx).opOptInSeat) // kop.ExtOptInSeat (dirac/act_optinseat.go)
}

// opOptInSeat is the head of a RepeatOptionalForEachPlayer$ body: B the
// instructions a no skips.
func (ic *instrCtx) opOptInSeat() error {
	ki := &ic.ki
	ki.Op, ki.A, ki.B = kop.OpExt, kop.Const(int(kop.ExtOptInSeat)), ic.skip()
	return nil
}
