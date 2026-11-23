package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() { instrOp("Unattach", (*instrCtx).opUnattach) }

// opUnattach is the kop form of api:Unattach (api_unattach.go): OpExt
// ExtUnattach over the one defined object.
func (ic *instrCtx) opUnattach() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtUnattach))
	return nil
}
