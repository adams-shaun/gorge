package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for destroy, sacrifice and counter (instr.go).
func init() {
	instrOp("Destroy", (*instrCtx).opDestroy)
	instrOp("Regenerate", (*instrCtx).opRegenerate)
	instrOp("DestroyAll", (*instrCtx).opDestroyAll)
	instrOp("Counter", (*instrCtx).opCounter)
	instrOp("CounterAll", (*instrCtx).opCounterAll)
	instrOp("TwoPilesSac", (*instrCtx).opTwoPilesSac)
	instrOp("SacrificeDelayed", (*instrCtx).opSacrificeDelayed)
	instrOp("Sacrifice", (*instrCtx).opSacrifice)
	instrOp("SacrificeAll", (*instrCtx).opSacrificeAll)
	instrOp("NoRegenMark", (*instrCtx).opNoRegenMark)
}

func (ic *instrCtx) opNoRegenMark() error {
	ic.ki.Op = kop.OpMarkNoRegen
	return nil
}

func (ic *instrCtx) opDestroy() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpDestroy, int16(in.C)
	return nil
}

func (ic *instrCtx) opRegenerate() error {
	ki := &ic.ki
	ki.Op = kop.OpRegenerate
	return nil
}

func (ic *instrCtx) opDestroyAll() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpDestroyAll
	ki.C = int16(in.C & 3)
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opCounter() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpCounter, int16(in.C)
	if in.B.Kind != vNone {
		ki.B = kop.Const(in.B.N) // CtrRemMV / CtrRemSpell
	}
	return nil
}

func (ic *instrCtx) opCounterAll() error {
	ki := &ic.ki
	// OpExt ExtCounterAll (no operands)
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCounterAll))
	return nil
}

func (ic *instrCtx) opTwoPilesSac() error {
	ki := &ic.ki
	// (Liliana of the Veil) OpExt ExtTwoPilesSac
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtTwoPilesSac))
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opSacrificeDelayed() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtSacDelayed over the group's survivors
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtSacDelayed))
	if in.CS == "Many" {
		ki.C = 1 // the DpMany program's up to four objects (Mobilize 3-4)
	}
	return nil
}

func (ic *instrCtx) opSacrifice() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpSacrifice, int16(in.C) // in.C: kop.OptAsk
	if strings.HasPrefix(in.Note, noteRememberSacrificed) {
		ki.B = kop.Const(1 + remLKBits(in.Note)) // RememberSacrificed$; the LKI sum bits (value_remlki.go)
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opSacrificeAll() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpSacrificeAll
	if err := remember3(in.CS, ki); err != nil {
		return err
	}
	if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
		return err
	}
	return nil
}
