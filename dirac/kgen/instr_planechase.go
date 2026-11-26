package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// instr_planechase.go is the kop form of the planechase LIR ops: the
// command-zone chaos marker (dirac/zone_planechase.go's ExtChaosEnsues handler).
func init() {
	instrOp("ChaosEnsues", (*instrCtx).opChaosEnsues)
}

func (ic *instrCtx) opChaosEnsues() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtChaosEnsues))
	return nil
}
