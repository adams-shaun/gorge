package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for the remembered, chosen and imprinted sets (instr.go).
func init() {
	instrOp("ClearRemembered", (*instrCtx).opClearRemembered)
	instrOp("ClearChosen", (*instrCtx).opClearChosen)
	instrOp("RememberPlayer", (*instrCtx).opRememberPlayer)
	instrOp("ClearImprinted", (*instrCtx).opClearImprinted)
	instrOp("RememberSearcher", (*instrCtx).opRememberSearcher)
	instrOp("RememberAll", (*instrCtx).opRememberAll)
	instrOp("RememberImprinted", (*instrCtx).opRememberImprinted)
	instrOp("ForgetImprinted", (*instrCtx).opForgetImprinted)
	instrOp("ImprintTriggered", (*instrCtx).opImprintTriggered)
	instrOp("ImprintRef", (*instrCtx).opImprintRef)
}

// opImprintRef is ExtRememberAll's imprint arm over the one object Obj names,
// skipping a token (kop.RememberAllImprintCard): the pre-move half of a
// Return's Imprint$ (lower_imprintzone.go returnImprint).
func (ic *instrCtx) opImprintRef() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRememberAll))
	ki.C = kop.RememberAllImprint | kop.RememberAllImprintCard
	return nil
}

// opImprintTriggered emits the ExtRememberAll imprint arm: the trigger's
// object (Obj RefTriggerObj) joins the resolution's imprinted set, and a
// source that keeps its list records it (gorge Pump ImprintCards$ TriggeredCard,
// effects/context.go imprint).
func (ic *instrCtx) opImprintTriggered() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRememberAll))
	ki.C = kop.RememberAllImprint
	return nil
}

// opRememberPlayer records the player Obj names (a Pump's remembered target
// controller) as the resolution's player, for the Draw that follows a
// GainControl (lower_remembered.go pumpRememberOppController).
func (ic *instrCtx) opRememberPlayer() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRememberPlayer))
	return nil
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

func (ic *instrCtx) opForgetImprinted() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtForgetImprinted))
	return nil // Obj is RefTriggerObj (build.go ref), gorge Pump ForgetImprinted$ TriggeredCard
}
