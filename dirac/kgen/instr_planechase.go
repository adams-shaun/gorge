package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// instr_planechase.go is the kop form of the planechase LIR ops: the
// command-zone chaos marker (dirac/zone_planechase.go's ExtChaosEnsues
// handler) and the plane-source curse (dirac/zone_planecurse.go's ExtPlaneCurse).
func init() {
	instrOp("ChaosEnsues", (*instrCtx).opChaosEnsues)
	instrOp("PlaneCurse", (*instrCtx).opPlaneCurse)
	instrOp("Planeswalk", (*instrCtx).opPlaneswalk)
}

func (ic *instrCtx) opChaosEnsues() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtChaosEnsues))
	return nil
}

func (ic *instrCtx) opPlaneCurse() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtPlaneCurse))
	return nil
}

func (ic *instrCtx) opPlaneswalk() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtPlaneswalk))
	return nil
}
