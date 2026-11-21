package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	instrOp("MonarchJail", (*instrCtx).opMonarchJail)
}

func (ic *instrCtx) opMonarchJail() error {
	ki := &ic.ki
	// OpExt ExtMonarchJail, Obj the target (dirac/act_monarchjail.go)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtMonarchJail))
	return nil
}
