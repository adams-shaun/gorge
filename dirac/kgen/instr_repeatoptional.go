package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() {
	instrOp("RepeatOptional", (*instrCtx).opRepeatOptional) // kop.ExtRepeatOptional (dirac/act_repeatoptional.go)
}

func (ic *instrCtx) opRepeatOptional() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.A, ki.B = kop.OpExt, kop.Const(int(kop.ExtRepeatOptional)), kop.Const(in.B.N)
	if in.B.N == 0 || in.B.N == 2 {
		id, err := ic.cb.cond(in.Cond)
		if err != nil {
			return err
		}
		ki.C = int16(id)
	}
	return nil
}
