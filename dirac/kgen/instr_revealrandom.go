package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

func init() { instrOp("RevealRandom", (*instrCtx).opRevealRandom) }

// opRevealRandom is the kop form of Reveal Random$ over a target player's hand
// (lower_reveal.go): OpExt ExtRevealRandom over the player in Obj.
func (ic *instrCtx) opRevealRandom() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtRevealRandom))
	return nil
}
