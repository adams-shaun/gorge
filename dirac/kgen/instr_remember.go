package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for the remembered, chosen and imprinted sets (instr.go).
func init() {
	instrOp("ClearRemembered", (*instrCtx).opClearRemembered)
	instrOp("ClearChosen", (*instrCtx).opClearChosen)
	instrOp("ClearImprinted", (*instrCtx).opClearImprinted)
	instrOp("RememberSearcher", (*instrCtx).opRememberSearcher)
	instrOp("RememberAll", (*instrCtx).opRememberAll)
	instrOp("RememberImprinted", (*instrCtx).opRememberImprinted)
}

func (ic *instrCtx) opClearRemembered() error {
	ki := &ic.ki
	ki.Op = kop.OpClearRemembered
	return nil
}

func (ic *instrCtx) opClearChosen() error {
	ki := &ic.ki
	ki.Op = kop.OpClearChosen
	return nil
}

func (ic *instrCtx) opClearImprinted() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtClearImprinted))
	return nil
}

func (ic *instrCtx) opRememberSearcher() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRememberSearcher)) // zone_remembersearcher.go
	return nil
}

func (ic *instrCtx) opRememberAll() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRememberAll)) // lower_rememberwritersmass.go
	if in.CS == "ObjOrder" {
		ki.C = kop.RememberAllObjOrder // lower_attachgroup.go
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opRememberImprinted() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRememberImprinted)) // Tainted Pact
	return nil
}
